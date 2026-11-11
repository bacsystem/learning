package salud

import (
	"context"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"

	"github.com/cbaciliod/learning/internal/db/dbtest"
)

// ComprobarBD over the real pool: SELECT 1 answers and its latency is reported.
func TestComprobarBD_Postgres(t *testing.T) {
	db := dbtest.Open(t)
	got := ComprobarBD(db).Comprobar(context.Background())
	if got.ID != IDBaseDatos || got.Estado == EstadoCaido || got.Categoria != nil {
		t.Fatalf("got %+v, want an answering database", got)
	}
	if ms, ok := got.Datos["latencia_ms"].(int64); !ok || ms < 0 {
		t.Errorf("latencia_ms %v", got.Datos["latencia_ms"])
	}
	// A closed pool (opened apart: dbtest.Open holds the test lock): the query
	// fails as sin_conexion.
	cerrada, err := sqlx.Open("postgres", os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	_ = cerrada.Close()
	if got := ComprobarBD(cerrada).Comprobar(context.Background()); got.Estado != EstadoCaido ||
		got.Categoria == nil || *got.Categoria != CategoriaSinConexion {
		t.Errorf("closed pool: %+v, want caido/sin_conexion", got)
	}
}
