package fuentes_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/cbaciliod/learning/internal/fuentes"
	"github.com/jmoiron/sqlx"
)

// sinOtrasEnProceso fails fast when the test DB keeps 'procesando' versions
// from an aborted run: the queue is global and would hand them out first.
func sinOtrasEnProceso(t *testing.T, db *sqlx.DB) {
	t.Helper()
	var n int
	if err := db.Get(&n, `SELECT COUNT(*) FROM fuente_versiones WHERE estado = 'procesando'`); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("the test DB has %d residual 'procesando' versions; recreate it", n)
	}
}

func guardarEnCola(t *testing.T, repo fuentes.SourceRepository, contenido string) (int64, int) {
	t.Helper()
	id, v, err := repo.GuardarFuente(context.Background(), fuentes.NuevaFuente{Titulo: "Cola " + dbtest.Uniq(), Tipo: "guia",
		NombreArchivo: "cola.md", Contenido: []byte(contenido)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return id, v
}

func TestPgCola_Reclamar(t *testing.T) {
	e, db := entornoPg(t)
	sinOtrasEnProceso(t, db)
	cola := fuentes.NewPgCola(db)
	ctx := context.Background()

	if _, ok, err := cola.Reclamar(ctx); err != nil || ok {
		t.Fatalf("empty queue: ok=%v err=%v", ok, err)
	}
	contenido := "contenido " + dbtest.Uniq()
	id, v := guardarEnCola(t, e.Repo, contenido)

	got, ok, err := cola.Reclamar(ctx)
	if err != nil || !ok || got.FuenteID != id || got.Version != v || got.Intentos != 1 || string(got.Contenido) != contenido {
		t.Fatalf("first claim: %+v ok=%v err=%v", got, ok, err)
	}
	if _, ok, err := cola.Reclamar(ctx); err != nil || ok {
		t.Fatalf("a live claim must not be handed out again: ok=%v err=%v", ok, err)
	}
	if _, err := db.Exec(`UPDATE fuente_versiones SET claimed_at = NOW() - INTERVAL '11 minutes' WHERE fuente_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if got, ok, err := cola.Reclamar(ctx); err != nil || !ok || got.FuenteID != id || got.Intentos != 2 {
		t.Fatalf("an expired claim is reclaimed and counted: %+v ok=%v err=%v", got, ok, err)
	}
}

// Two workers claiming at the same time never get the same version.
func TestPgCola_ReclamarConcurrente(t *testing.T) {
	e, db := entornoPg(t)
	sinOtrasEnProceso(t, db)
	const n = 6
	for range n {
		guardarEnCola(t, e.Repo, "concurrente "+dbtest.Uniq())
	}
	cola := fuentes.NewPgCola(db)
	var (
		mu         sync.Mutex
		reclamadas = map[int64]int{}
		wg         sync.WaitGroup
	)
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, ok, err := cola.Reclamar(context.Background())
			if err != nil || !ok {
				t.Errorf("claim: ok=%v err=%v", ok, err)
				return
			}
			mu.Lock()
			reclamadas[v.FuenteID]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	if len(reclamadas) != n {
		t.Fatalf("claimed %d distinct versions, want %d: %v", len(reclamadas), n, reclamadas)
	}
}

// The worker over Postgres: a version is published with its passages and
// audited as the system, and an empty one fails with sin_contenido.
func TestIngestaWorker_Postgres(t *testing.T) {
	e, db := entornoPg(t)
	sinOtrasEnProceso(t, db)
	var desdeAudit int64
	if err := db.Get(&desdeAudit, `SELECT COALESCE(MAX(id), 0) FROM audit_log`); err != nil {
		t.Fatal(err)
	}
	texto := "# Tema " + dbtest.Uniq() + "\n\n" + strings.Repeat("palabra ", 1000)
	ok, _ := guardarEnCola(t, e.Repo, texto)
	vacia, _ := guardarEnCola(t, e.Repo, "   \n\t\n")

	w := fuentes.NewIngestaWorker(fuentes.NewPgCola(db), e.Repo, fuentes.AuditorSistema(audit.New(db)))
	for w.ProcesarSiguiente(context.Background()) {
	}

	cases := []struct {
		fuenteID       int64
		estado, accion string
		pasajes        int
		motivo         *string
	}{
		{ok, fuentes.EstadoActiva, fuentes.AccionProcesar, 2, nil},
		{vacia, fuentes.EstadoFallida, fuentes.AccionFallar, 0, ptrMotivo(fuentes.MotivoSinContenido)},
	}
	for _, c := range cases {
		var got struct {
			Estado  string  `db:"estado"`
			Pasajes int     `db:"pasajes_count"`
			Motivo  *string `db:"motivo_fallo"`
		}
		if err := db.Get(&got, `SELECT f.estado, v.pasajes_count, v.motivo_fallo FROM fuentes f
			JOIN fuente_versiones v ON v.fuente_id = f.id WHERE f.id = $1`, c.fuenteID); err != nil {
			t.Fatal(err)
		}
		if got.Estado != c.estado || got.Pasajes != c.pasajes || (got.Motivo == nil) != (c.motivo == nil) ||
			(c.motivo != nil && *got.Motivo != *c.motivo) {
			t.Errorf("fuente %d: %+v, want estado %s, %d pasajes, motivo %v", c.fuenteID, got, c.estado, c.pasajes, c.motivo)
		}
		detalleAudit(t, db, desdeAudit, c.accion, c.fuenteID) // exactly one row, actor NULL
	}
}

func ptrMotivo(s string) *string { return &s }
