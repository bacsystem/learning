package questions

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/cbaciliod/learning/internal/fuentes"
	"github.com/jmoiron/sqlx"
)

const migracion065 = "../../migrations/065_add_ciclo_preguntas.sql"

// seccionesGoose splits a goose file into its Up and Down SQL.
func seccionesGoose(t *testing.T, ruta string) (up, down string) {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	_, resto, ok := strings.Cut(string(b), "-- +goose Up")
	if !ok {
		t.Fatal("no Up section")
	}
	up, down, ok = strings.Cut(resto, "-- +goose Down")
	if !ok {
		t.Fatal("no Down section")
	}
	return up, down
}

// Escenario «Migración reversible», with rows in the new states: the Down of
// 065 maps aprobada → activo and the rest of the new states → eliminada (so
// nothing unreviewed is served), drops the citations, and the Up again leaves
// every row as it was, with origen 'manual'. It all runs inside a transaction
// that is rolled back (DDL is transactional in PostgreSQL), so the shared test
// database is left at 065.
func TestMigracion065_DownYUp(t *testing.T) {
	db := dbtest.Open(t)
	ctx := context.Background()
	cat := dbtest.SeedCatalog(t, db)
	proceso := dbtest.SeedProceso(t, db, cat, 2031, "activo")
	asignatura := dbtest.SeedAsignatura(t, db)
	pasaje := pasajeDePrueba(t, db)

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	estados := map[string]string{ // before → expected after the Down
		"activo": "activo", "aprobada": "activo", "borrador": "eliminada",
		"en_revision": "eliminada", "rechazada": "eliminada", "eliminada": "eliminada",
	}
	ids := map[string]int64{}
	for estado := range estados {
		var motivo *string
		if estado == "rechazada" {
			m := "Clave incorrecta"
			motivo = &m
		}
		var id int64
		if err := tx.GetContext(ctx, &id, `INSERT INTO preguntas (institucion_id, proceso_id, asignatura_id, enunciado, estado, origen, motivo_rechazo)
			VALUES ($1, $2, $3, $4, $5, 'ia', $6) RETURNING id`, cat.InstitucionID, proceso, asignatura, "Migración "+estado, estado, motivo); err != nil {
			t.Fatalf("insert %s: %v", estado, err)
		}
		ids[estado] = id
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO pregunta_citas (pregunta_id, pasaje_id) VALUES ($1, $2)`, ids["aprobada"], pasaje); err != nil {
		t.Fatal(err)
	}

	up, down := seccionesGoose(t, migracion065)
	if _, err := tx.ExecContext(ctx, down); err != nil {
		t.Fatalf("Down: %v", err)
	}
	for estado, despues := range estados {
		var got string
		if err := tx.GetContext(ctx, &got, `SELECT estado FROM preguntas WHERE id = $1`, ids[estado]); err != nil || got != despues {
			t.Errorf("after Down, %s = %q (%v), want %s", estado, got, err, despues)
		}
	}
	esquema(t, tx, "after Down", "'activo'::character varying", false)

	if _, err := tx.ExecContext(ctx, up); err != nil {
		t.Fatalf("Up again: %v", err)
	}
	for estado, despues := range estados {
		var fila struct {
			Estado string `db:"estado"`
			Origen string `db:"origen"`
		}
		if err := tx.GetContext(ctx, &fila, `SELECT estado, origen FROM preguntas WHERE id = $1`, ids[estado]); err != nil ||
			fila.Estado != despues || fila.Origen != "manual" {
			t.Errorf("after Up, %s = %+v (%v), want %s / manual", estado, fila, err, despues)
		}
	}
	esquema(t, tx, "after Up", "'borrador'::character varying", true)
}

// esquema checks the default of preguntas.estado and whether 065's table and
// columns exist.
func esquema(t *testing.T, tx *sqlx.Tx, cuando, defecto string, con065 bool) {
	t.Helper()
	var s struct {
		Defecto string `db:"defecto"`
		Citas   bool   `db:"citas"`
		Origen  bool   `db:"origen"`
		Viejo   bool   `db:"viejo"`
	}
	if err := tx.Get(&s, `SELECT
		(SELECT column_default FROM information_schema.columns WHERE table_name = 'preguntas' AND column_name = 'estado') AS defecto,
		to_regclass('pregunta_citas') IS NOT NULL AS citas,
		EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'preguntas' AND column_name = 'origen') AS origen,
		EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'preguntas'::regclass AND conname = 'preguntas_estado_check') AS viejo`); err != nil {
		t.Fatal(err)
	}
	if s.Defecto != defecto || s.Citas != con065 || s.Origen != con065 || s.Viejo == con065 {
		t.Errorf("%s: schema = %+v", cuando, s)
	}
}

// pasajeDePrueba publishes a one-passage fuente through the port and returns
// the passage id; the fuente is removed when the test ends.
func pasajeDePrueba(t *testing.T, db *sqlx.DB) int64 {
	t.Helper()
	_, pasaje := fuenteDePrueba(t, db)
	return pasaje
}

// fuenteDePrueba is pasajeDePrueba that also returns the fuente id. Its
// cleanup removes the citations first (pregunta_citas.pasaje_id has no ON
// DELETE), so it can run before the fixture deletes the preguntas.
func fuenteDePrueba(t *testing.T, db *sqlx.DB) (fuenteID, pasajeID int64) {
	t.Helper()
	ctx := context.Background()
	src := fuentes.NewPgSource(db)
	id, v, err := src.GuardarFuente(ctx, fuentes.NuevaFuente{Titulo: "Migración " + dbtest.Uniq(), Tipo: "guia",
		NombreArchivo: "m.md", Contenido: []byte("migración " + dbtest.Uniq())}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, q := range []string{
			`DELETE FROM pregunta_citas WHERE pasaje_id IN (SELECT p.id FROM pasajes p JOIN fuente_versiones v ON v.id = p.fuente_version_id WHERE v.fuente_id = $1)`,
			`UPDATE fuentes SET version_vigente_id = NULL, estado = 'procesando' WHERE id = $1`,
			`DELETE FROM pasajes WHERE fuente_version_id IN (SELECT id FROM fuente_versiones WHERE fuente_id = $1)`,
			`DELETE FROM fuente_versiones WHERE fuente_id = $1`,
			`DELETE FROM fuentes WHERE id = $1`,
		} {
			if _, err := db.Exec(q, id); err != nil {
				t.Errorf("cleanup %q: %v", q, err)
			}
		}
	})
	if err := src.PublicarVersion(ctx, id, v, []fuentes.PasajeNuevo{{Orden: 1, LineaInicio: 1, LineaFin: 1, Texto: "Pasaje citado", Palabras: 2}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&pasajeID, `SELECT p.id FROM pasajes p JOIN fuente_versiones v ON v.id = p.fuente_version_id WHERE v.fuente_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	return id, pasajeID
}
