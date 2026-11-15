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

// Names of the calls the scanner understands in the handler argument of a
// registration. Any other call fails the test so that somebody decides whether
// it is a role middleware or a handler factory and adds it to the right set.
var (
	middlewaresAdmin       = []string{"admin", "requireAdmin", "RequireAdmin"}
	middlewaresAutenticado = []string{"authn", "auth", "requireAuth", "RequireAuth"}
	// sinEfectoDeRol wraps a handler without changing who can call it.
	sinEfectoDeRol = []string{"HandlerFunc", "Limit"}
	// fabricasDeHandler build a plain handler from their arguments.
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
	var rutas []rutaCodigo
	var fallos []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !esRegistro(call) {
			return true
		}
		r, err := leerRegistro(fset, call, consts)
		if err != nil {
			fallos = append(fallos, err.Error())
		} else {
			rutas = append(rutas, r)
		}
		return false
	})
	if len(fallos) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(fallos, "\n"))
	}
	return rutas, nil
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

func leerRegistro(fset *token.FileSet, call *ast.CallExpr, consts map[string]string) (rutaCodigo, error) {
	pos := posicion(fset, call.Pos())
	if len(call.Args) != 2 {
		return rutaCodigo{}, fmt.Errorf("%s: registro con %d argumentos; se esperan patrón y handler", pos, len(call.Args))
	}
	patron, err := leerPatron(call.Args[0], consts)
	if err != nil {
		return rutaCodigo{}, fmt.Errorf("%s: %w", pos, err)
	}
	clave, err := normalizarPatron(patron)
	if err != nil {
		return rutaCodigo{}, fmt.Errorf("%s: %w", pos, err)
	}
	r, err := rolDe(fset, call.Args[1])
	if err != nil {
		return rutaCodigo{}, err
	}
	return rutaCodigo{Clave: clave, Rol: r, Pos: pos}, nil
}

func leerPatron(e ast.Expr, consts map[string]string) (string, error) {
	switch x := e.(type) {
	case *ast.BasicLit:
		if x.Kind == token.STRING {
			return strconv.Unquote(x.Value)
		}
	case *ast.Ident:
		if v, ok := consts[x.Name]; ok {
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
func rolDe(fset *token.FileSet, e ast.Expr) (rol, error) {
	switch x := e.(type) {
	case *ast.Ident, *ast.SelectorExpr: // a handler value such as h.Listar
		return rolPublico, nil
	case *ast.ParenExpr:
		return rolDe(fset, x.X)
	case *ast.CallExpr:
		return rolDeLlamada(fset, x)
	}
	return "", fmt.Errorf("%s: forma de handler desconocida (%T)", posicion(fset, e.Pos()), e)
}

func rolDeLlamada(fset *token.FileSet, call *ast.CallExpr) (rol, error) {
	nombre, ok := nombreLlamada(call.Fun)
	if !ok {
		return "", fmt.Errorf("%s: llamada con función desconocida (%T)", posicion(fset, call.Pos()), call.Fun)
	}
	propio := rolPublico
	switch {
	case slices.Contains(fabricasDeHandler, nombre):
		return rolPublico, nil
	case slices.Contains(middlewaresAdmin, nombre):
		propio = rolAdmin
	case slices.Contains(middlewaresAutenticado, nombre):
		propio = rolAutenticado
	case slices.Contains(sinEfectoDeRol, nombre):
	default:
		return "", fmt.Errorf("%s: llamada a %q desconocida: ¿middleware de rol o fábrica de handler? Añádala a la lista que corresponde en codigo_test.go",
			posicion(fset, call.Pos()), nombre)
	}
	for _, arg := range call.Args {
		r, err := rolDe(fset, arg)
		if err != nil {
			return "", err
		}
		if r.rango() > propio.rango() {
			propio = r
		}
	}
	return propio, nil
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
