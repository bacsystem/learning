package apispec

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

const estadoPlanificado = "planificado"

// metodosOpenAPI are the keys of a path item that hold an operation.
var metodosOpenAPI = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

// opSpec is one operation of the spec, with the fields the test compares.
type opSpec struct {
	Clave  string // "METHOD /path"
	Rol    string // x-rol as written; empty when missing
	Estado string // x-estado as written
	Tags   []string
}

// infoSpec is what the test needs from the spec.
type infoSpec struct {
	Ops map[string]opSpec
	// Huerfanas lists "archivo: /ruta" keys of a package file's `paths` that no
	// path of the root references.
	Huerfanas []string
}

// lectorSpec loads YAML files once and follows $ref between them.
type lectorSpec struct {
	archivos map[string]any
}

// leerSpec reads <raiz>/openapi.yaml and the files under <raiz>/paquetes.
func leerSpec(raiz string) (infoSpec, error) {
	l := &lectorSpec{archivos: map[string]any{}}
	rootPath := filepath.Join(raiz, "openapi.yaml")
	doc, err := l.cargar(rootPath)
	if err != nil {
		return infoSpec{}, err
	}
	paths, err := objeto(doc, "paths", rootPath)
	if err != nil {
		return infoSpec{}, err
	}
	info := infoSpec{Ops: map[string]opSpec{}}
	referenciadas := map[string]map[string]bool{} // archivo -> clave de paths
	for _, ruta := range clavesOrdenadas(paths) {
		item, err := l.seguirRef(paths[ruta], rootPath, func(archivo, clave string) {
			if referenciadas[archivo] == nil {
				referenciadas[archivo] = map[string]bool{}
			}
			referenciadas[archivo][clave] = true
		})
		if err != nil {
			return infoSpec{}, fmt.Errorf("%s: %w", ruta, err)
		}
		if err := l.leerOperaciones(&info, ruta, item, rootPath); err != nil {
			return infoSpec{}, err
		}
	}
	huerfanas, err := l.rutasSinReferencia(raiz, referenciadas)
	if err != nil {
		return infoSpec{}, err
	}
	info.Huerfanas = huerfanas
	return info, nil
}

func (l *lectorSpec) cargar(path string) (map[string]any, error) {
	path = filepath.Clean(path)
	if doc, ok := l.archivos[path]; ok {
		return doc.(map[string]any), nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	l.archivos[path] = doc
	return doc, nil
}

// seguirRef resolves value while it is a {"$ref": "file#/pointer"} object. For
// each hop of the form #/paths/<key>, enCamino reports the target file and key.
func (l *lectorSpec) seguirRef(valor any, desde string, enCamino func(archivo, clave string)) (any, error) {
	for saltos := 0; ; saltos++ {
		m, ok := valor.(map[string]any)
		ref, tieneRef := m["$ref"].(string)
		if !ok || !tieneRef {
			return valor, nil
		}
		if saltos > 10 {
			return nil, fmt.Errorf("$ref en cadena demasiado larga en %s", desde)
		}
		archivoRef, puntero, _ := strings.Cut(ref, "#")
		destino := desde
		if archivoRef != "" {
			destino = filepath.Join(filepath.Dir(desde), archivoRef)
		}
		doc, err := l.cargar(destino)
		if err != nil {
			return nil, fmt.Errorf("$ref %q: %w", ref, err)
		}
		fichas, err := fichasDePuntero(puntero)
		if err != nil {
			return nil, fmt.Errorf("$ref %q: %w", ref, err)
		}
		if enCamino != nil && len(fichas) == 2 && fichas[0] == "paths" {
			enCamino(filepath.Clean(destino), fichas[1])
		}
		valor, err = bajar(doc, fichas)
		if err != nil {
			return nil, fmt.Errorf("$ref %q: %w", ref, err)
		}
		desde = destino
	}
}

// fichasDePuntero splits a JSON pointer ("/paths/~1auth~1login") into tokens,
// decoding ~1 as "/" and ~0 as "~" (in that order, RFC 6901).
func fichasDePuntero(puntero string) ([]string, error) {
	if puntero == "" {
		return nil, nil
	}
	if !strings.HasPrefix(puntero, "/") {
		return nil, fmt.Errorf("puntero JSON %q sin «/» inicial", puntero)
	}
	var out []string
	for _, f := range strings.Split(puntero[1:], "/") {
		out = append(out, strings.NewReplacer("~1", "/", "~0", "~").Replace(f))
	}
	return out, nil
}

func bajar(doc any, fichas []string) (any, error) {
	actual := doc
	for _, f := range fichas {
		switch x := actual.(type) {
		case map[string]any:
			v, ok := x[f]
			if !ok {
				return nil, fmt.Errorf("no existe %q", f)
			}
			actual = v
		case []any:
			i, err := strconv.Atoi(f)
			if err != nil || i < 0 || i >= len(x) {
				return nil, fmt.Errorf("índice %q fuera de rango", f)
			}
			actual = x[i]
		default:
			return nil, fmt.Errorf("no se puede bajar a %q", f)
		}
	}
	return actual, nil
}

func (l *lectorSpec) leerOperaciones(info *infoSpec, ruta string, item any, desde string) error {
	m, ok := item.(map[string]any)
	if !ok {
		return fmt.Errorf("%s: el path item no es un objeto", ruta)
	}
	for _, metodo := range metodosOpenAPI {
		crudo, ok := m[metodo]
		if !ok {
			continue
		}
		valor, err := l.seguirRef(crudo, desde, nil)
		if err != nil {
			return fmt.Errorf("%s %s: %w", strings.ToUpper(metodo), ruta, err)
		}
		op, ok := valor.(map[string]any)
		if !ok {
			return fmt.Errorf("%s %s: la operación no es un objeto", strings.ToUpper(metodo), ruta)
		}
		clave := strings.ToUpper(metodo) + " " + ruta
		info.Ops[clave] = opSpec{
			Clave:  clave,
			Rol:    texto(op["x-rol"]),
			Estado: texto(op["x-estado"]),
			Tags:   listaDeTexto(op["tags"]),
		}
	}
	return nil
}

// rutasSinReferencia lists the `paths` keys of <raiz>/paquetes/*.yaml that the
// root never references with a $ref.
func (l *lectorSpec) rutasSinReferencia(raiz string, referenciadas map[string]map[string]bool) ([]string, error) {
	archivos, err := filepath.Glob(filepath.Join(raiz, "paquetes", "*.yaml"))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, a := range archivos {
		doc, err := l.cargar(a)
		if err != nil {
			return nil, err
		}
		paths, _ := doc["paths"].(map[string]any)
		for _, clave := range clavesOrdenadas(paths) {
			if !referenciadas[filepath.Clean(a)][clave] {
				out = append(out, fmt.Sprintf("%s: %s", filepath.Base(a), clave))
			}
		}
	}
	return out, nil
}

func objeto(doc map[string]any, clave, archivo string) (map[string]any, error) {
	m, ok := doc[clave].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: falta la clave %q o no es un objeto", archivo, clave)
	}
	return m, nil
}

func clavesOrdenadas(m map[string]any) []string {
	claves := make([]string, 0, len(m))
	for k := range m {
		claves = append(claves, k)
	}
	sort.Strings(claves)
	return claves
}

func texto(v any) string {
	s, _ := v.(string)
	return s
}

func listaDeTexto(v any) []string {
	lista, _ := v.([]any)
	var out []string
	for _, e := range lista {
		out = append(out, texto(e))
	}
	return out
}
