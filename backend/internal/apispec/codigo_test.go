package apispec

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// rol is the access level a middleware chain enforces (the x-rol of the spec).
type rol string

const (
	rolPublico     rol = "publico"
	rolAutenticado rol = "autenticado"
	rolAdmin       rol = "admin"
)

// rango orders the roles so that a chain takes the strictest one.
func (r rol) rango() int {
	switch r {
	case rolAdmin:
		return 2
	case rolAutenticado:
		return 1
	default:
		return 0
	}
}

// The role comes from what a middleware name resolves to, never from the name
// itself: `admin := auth.RequireAuth(...)` is an authenticated route. Only the
// two constructors of internal/auth define a role.
const (
	constructorAdmin = "RequireAdmin"
	constructorAuth  = "RequireAuth"
)

// Calls that wrap a handler without changing who can call it, or that build
// one from their arguments. Any other call fails the test so that somebody
// decides what it is and adds it to the right list.
var (
	sinEfectoDeRol    = []string{"HandlerFunc", "Limit"}
	fabricasDeHandler = []string{"healthHandler"}

	metodosHTTP = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
)

// rutaCodigo is one registration found in the Go sources.
type rutaCodigo struct {
	Clave string // "METHOD /path"
	Rol   rol
	Pos   string // file:line
}

// escanearFuente parses src (named nombre in positions) and returns its routes.
func escanearFuente(nombre, src string) ([]rutaCodigo, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, nombre, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	return escanearArchivo(fset, f)
}

// escanearArchivo returns every X.Handle / X.HandleFunc registration of f. A
// registration it cannot read is an error with file:line, never skipped.
func escanearArchivo(fset *token.FileSet, f *ast.File) ([]rutaCodigo, error) {
	consts := constantesDeTexto(f)
	globales := map[string][]ast.Expr{}
	for _, d := range f.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.VAR {
			sumarAsignaciones(globales, gd)
		}
	}
	var rutas []rutaCodigo
	var fallos []string
	for _, d := range f.Decls {
		a := &ambito{fset: fset, consts: consts, asignaciones: asignacionesDe(d, globales)}
		ast.Inspect(d, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !esRegistro(call) {
				return true
			}
			r, err := a.leerRegistro(call)
			if err != nil {
				fallos = append(fallos, err.Error())
			} else {
				rutas = append(rutas, r)
			}
			return false
		})
	}
	if len(fallos) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(fallos, "\n"))
	}
	return rutas, nil
}

// ambito is what the scanner knows where a registration sits: the text
// constants of the file and every expression assigned to each name in the
// enclosing declaration (or at package level).
type ambito struct {
	fset         *token.FileSet
	consts       map[string]string
	asignaciones map[string][]ast.Expr
}

// asignacionesDe collects `name := expr`, `name = expr` and `var name = expr`
// inside decl; names it does not assign fall back to the package-level ones.
func asignacionesDe(decl ast.Decl, globales map[string][]ast.Expr) map[string][]ast.Expr {
	locales := map[string][]ast.Expr{}
	if fd, ok := decl.(*ast.FuncDecl); ok && fd.Body != nil {
		sumarAsignaciones(locales, fd.Body)
	}
	for nombre, exprs := range globales {
		if _, ok := locales[nombre]; !ok {
			locales[nombre] = exprs
		}
	}
	return locales
}

func sumarAsignaciones(dst map[string][]ast.Expr, nodo ast.Node) {
	ast.Inspect(nodo, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if len(x.Lhs) == len(x.Rhs) {
				for i, l := range x.Lhs {
					if id, ok := l.(*ast.Ident); ok {
						dst[id.Name] = append(dst[id.Name], x.Rhs[i])
					}
				}
			}
		case *ast.ValueSpec:
			if len(x.Names) == len(x.Values) {
				for i, id := range x.Names {
					dst[id.Name] = append(dst[id.Name], x.Values[i])
				}
			}
		}
		return true
	})
}

func esRegistro(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && (sel.Sel.Name == "Handle" || sel.Sel.Name == "HandleFunc")
}

// constantesDeTexto maps the string constants of f declared with a literal
// (const X = "GET /x"), so a registration can name its pattern.
func constantesDeTexto(f *ast.File) map[string]string {
	out := map[string]string{}
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs := spec.(*ast.ValueSpec)
			for i, name := range vs.Names {
				if i >= len(vs.Values) {
					break
				}
				if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if v, err := strconv.Unquote(lit.Value); err == nil {
						out[name.Name] = v
					}
				}
			}
		}
	}
	return out
}

func (a *ambito) leerRegistro(call *ast.CallExpr) (rutaCodigo, error) {
	pos := posicion(a.fset, call.Pos())
	if len(call.Args) != 2 {
		return rutaCodigo{}, fmt.Errorf("%s: registro con %d argumentos; se esperan patrón y handler", pos, len(call.Args))
	}
	patron, err := a.leerPatron(call.Args[0])
	if err != nil {
		return rutaCodigo{}, fmt.Errorf("%s: %w", pos, err)
	}
	clave, err := normalizarPatron(patron)
	if err != nil {
		return rutaCodigo{}, fmt.Errorf("%s: %w", pos, err)
	}
	r, err := a.rolDe(call.Args[1])
	if err != nil {
		return rutaCodigo{}, err
	}
	return rutaCodigo{Clave: clave, Rol: r, Pos: pos}, nil
}

func (a *ambito) leerPatron(e ast.Expr) (string, error) {
	switch x := e.(type) {
	case *ast.BasicLit:
		if x.Kind == token.STRING {
			return strconv.Unquote(x.Value)
		}
	case *ast.Ident:
		if v, ok := a.consts[x.Name]; ok {
			return v, nil
		}
		return "", fmt.Errorf("el patrón usa %q, que no es una constante de texto del mismo archivo", x.Name)
	}
	return "", fmt.Errorf("forma de patrón desconocida (%T): use un literal o una constante de texto", e)
}

// normalizarPatron checks "METHOD /path" and returns it with one space.
func normalizarPatron(patron string) (string, error) {
	campos := strings.Fields(patron)
	if len(campos) != 2 || !slices.Contains(metodosHTTP, campos[0]) || !strings.HasPrefix(campos[1], "/") {
		return "", fmt.Errorf("patrón %q: se esperaba \"MÉTODO /ruta\"", patron)
	}
	return campos[0] + " " + campos[1], nil
}

// rolDe returns the role enforced by the handler expression e: the strictest
// role among the middlewares it is wrapped in.
func (a *ambito) rolDe(e ast.Expr) (rol, error) {
	switch x := e.(type) {
	case *ast.Ident, *ast.SelectorExpr: // a handler value such as h.Listar
		return rolPublico, nil
	case *ast.ParenExpr:
		return a.rolDe(x.X)
	case *ast.CallExpr:
		return a.rolDeLlamada(x)
	}
	return "", fmt.Errorf("%s: forma de handler desconocida (%T)", posicion(a.fset, e.Pos()), e)
}

func (a *ambito) rolDeLlamada(call *ast.CallExpr) (rol, error) {
	propio, handlerFinal, err := a.rolDeFuncion(call)
	if err != nil || handlerFinal {
		return propio, err
	}
	for _, arg := range call.Args {
		r, err := a.rolDe(arg)
		if err != nil {
			return "", err
		}
		if r.rango() > propio.rango() {
			propio = r
		}
	}
	return propio, nil
}

// rolDeFuncion classifies the function a call invokes. handlerFinal is true
// for a factory that builds the handler itself, whose arguments are not wrapped
// handlers.
func (a *ambito) rolDeFuncion(call *ast.CallExpr) (r rol, handlerFinal bool, err error) {
	pos := posicion(a.fset, call.Pos())
	nombre, local := "", false
	switch f := call.Fun.(type) {
	case *ast.Ident:
		nombre, local = f.Name, true
	case *ast.SelectorExpr:
		nombre = f.Sel.Name
	default:
		return "", false, fmt.Errorf("%s: llamada con función desconocida (%T)", pos, call.Fun)
	}
	if local {
		if exprs, ok := a.asignaciones[nombre]; ok {
			r, err := a.rolDeAsignacion(nombre, exprs, pos)
			return r, false, err
		}
	}
	switch {
	case slices.Contains(fabricasDeHandler, nombre):
		return rolPublico, true, nil
	case slices.Contains(sinEfectoDeRol, nombre):
		return rolPublico, false, nil
	}
	if r, ok := rolDeConstructor(nombre); ok {
		return r, false, nil
	}
	return "", false, fmt.Errorf("%s: llamada a %q desconocida: ¿middleware de rol o fábrica de handler? Añádala a la lista que corresponde en codigo_test.go",
		pos, nombre)
}

// rolDeAsignacion gives the role of a local name by what it is assigned:
// `admin := auth.RequireAdmin` or `authn := auth.RequireAuth(secret, check)`.
// Anything else, or more than one assignment, fails.
func (a *ambito) rolDeAsignacion(nombre string, exprs []ast.Expr, pos string) (rol, error) {
	if len(exprs) != 1 {
		return "", fmt.Errorf("%s: %q se asigna %d veces en la función; no se puede saber a qué middleware apunta", pos, nombre, len(exprs))
	}
	var destino string
	switch x := exprs[0].(type) {
	case *ast.Ident:
		destino = x.Name
	case *ast.SelectorExpr:
		destino = x.Sel.Name
	case *ast.CallExpr:
		destino, _ = nombreLlamada(x.Fun)
	}
	if r, ok := rolDeConstructor(destino); ok {
		return r, nil
	}
	return "", fmt.Errorf("%s: %q se asigna a una expresión que no es %s ni %s", pos, nombre, constructorAdmin, constructorAuth)
}

func rolDeConstructor(nombre string) (rol, bool) {
	switch nombre {
	case constructorAdmin:
		return rolAdmin, true
	case constructorAuth:
		return rolAutenticado, true
	}
	return "", false
}

// nombreLlamada returns the called name: f in f(...) and Sel in x.Sel(...).
func nombreLlamada(fun ast.Expr) (string, bool) {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name, true
	case *ast.SelectorExpr:
		return f.Sel.Name, true
	}
	return "", false
}

func posicion(fset *token.FileSet, p token.Pos) string {
	pos := fset.Position(p)
	return fmt.Sprintf("%s:%d", pos.Filename, pos.Line)
}

// escanearRepo reads the routes of every internal/*/routes.go and of
// cmd/server/main.go under backend, grouped by Go package ("server" for main).
// A registration in any other non-test file also fails: it would go unchecked.
func escanearRepo(backend string) (map[string][]rutaCodigo, error) {
	archivos, err := archivosDeRutas(backend)
	if err != nil {
		return nil, err
	}
	out := map[string][]rutaCodigo{}
	vistas := map[string]string{}
	var fallos []string
	for paquete, ruta := range archivos {
		rutas, err := leerRutas(backend, ruta)
		if err != nil {
			fallos = append(fallos, err.Error())
			continue
		}
		if len(rutas) == 0 {
			fallos = append(fallos, fmt.Sprintf("%s: no registra ninguna ruta; si cambió la forma de registro, adapte el test", ruta))
		}
		for _, r := range rutas {
			if previa, dup := vistas[r.Clave]; dup {
				fallos = append(fallos, fmt.Sprintf("%s: %q ya registrada en %s", r.Pos, r.Clave, previa))
			}
			vistas[r.Clave] = r.Pos
		}
		out[paquete] = rutas
	}
	fallos = append(fallos, registrosFueraDeRutas(backend, archivos)...)
	if len(fallos) > 0 {
		sort.Strings(fallos)
		return nil, fmt.Errorf("%s", strings.Join(fallos, "\n"))
	}
	return out, nil
}

// archivosDeRutas maps each package to its registration file (relative to backend).
func archivosDeRutas(backend string) (map[string]string, error) {
	coincidencias, err := filepath.Glob(filepath.Join(backend, "internal", "*", "routes.go"))
	if err != nil {
		return nil, err
	}
	out := map[string]string{"server": filepath.Join("cmd", "server", "main.go")}
	for _, c := range coincidencias {
		rel, err := filepath.Rel(backend, c)
		if err != nil {
			return nil, err
		}
		out[filepath.Base(filepath.Dir(c))] = rel
	}
	return out, nil
}

func leerRutas(backend, rel string) ([]rutaCodigo, error) {
	src, err := os.ReadFile(filepath.Join(backend, rel))
	if err != nil {
		return nil, err
	}
	return escanearFuente(rel, string(src))
}

// registrosFueraDeRutas lists the non-test files, other than the known
// registration files, that call Handle or HandleFunc.
func registrosFueraDeRutas(backend string, conocidos map[string]string) []string {
	esperado := map[string]bool{}
	for _, rel := range conocidos {
		esperado[rel] = true
	}
	var fallos []string
	patrones := []string{
		filepath.Join(backend, "internal", "*", "*.go"),
		filepath.Join(backend, "cmd", "*", "*.go"),
	}
	for _, p := range patrones {
		archivos, _ := filepath.Glob(p)
		for _, a := range archivos {
			rel, _ := filepath.Rel(backend, a)
			if strings.HasSuffix(a, "_test.go") || esperado[rel] {
				continue
			}
			src, err := os.ReadFile(a)
			if err != nil {
				fallos = append(fallos, err.Error())
				continue
			}
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, rel, src, parser.SkipObjectResolution)
			if err != nil {
				fallos = append(fallos, err.Error())
				continue
			}
			ast.Inspect(f, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok && esRegistro(call) {
					fallos = append(fallos, fmt.Sprintf("%s: registra una ruta fuera de routes.go y cmd/server/main.go; el test no la ve", posicion(fset, call.Pos())))
				}
				return true
			})
		}
	}
	return fallos
}
