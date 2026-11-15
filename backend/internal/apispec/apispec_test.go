package apispec

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRutasCoincidenConElSpec compares the routes registered in the code (and
// the role their middleware chain enforces) with docs/api/openapi.
func TestRutasCoincidenConElSpec(t *testing.T) {
	raiz := raizDelRepo(t)
	codigo, err := escanearRepo(filepath.Join(raiz, "backend"))
	if err != nil {
		t.Fatalf("no se pudieron leer las rutas del código:\n%v", err)
	}
	spec, err := leerSpec(filepath.Join(raiz, "docs", "api", "openapi"))
	if err != nil {
		t.Fatalf("no se pudo leer el spec: %v", err)
	}
	for _, e := range comparar(codigo, spec) {
		t.Error(e)
	}
}

// raizDelRepo climbs from the package directory to the folder with go.mod and
// returns its parent: the repository root (backend/go.mod).
func raizDelRepo(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Dir(dir)
		}
		padre := filepath.Dir(dir)
		if padre == dir {
			t.Fatalf("no se encontró go.mod subiendo desde %s", dir)
		}
		dir = padre
	}
}
