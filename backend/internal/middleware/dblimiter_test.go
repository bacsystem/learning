package middleware_test

import (
	"context"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/cbaciliod/learning/internal/middleware"
)

const ventanaPrueba = 15 * time.Minute

// proximoBorde is the next real boundary of a window of w, so an instant just
// before it is inside the current real window (the sweep stays consistent).
func proximoBorde(w time.Duration) time.Time { return time.Now().Truncate(w).Add(w) }

// limitador opens the test DB once (dbtest.Open holds the test lock: a second
// Open in the same test would wait for it forever) and returns it too.
func limitador(t *testing.T, limit int, window time.Duration) (*middleware.DBLimiter, *sqlx.DB, string) {
	t.Helper()
	db := dbtest.Open(t)
	prefijo := "test_dbl_" + dbtest.Uniq() + ":"
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM auth_intentos_fallidos WHERE clave LIKE $1`, prefijo+"%")
	})
	return middleware.NewDBLimiter(db.DB, prefijo, limit, window), db, prefijo
}

// #139: with the clock fixed 1 s before a window boundary, N attempts fill
// the window and the N+1th is refused, however long the attempts take.
func TestDBLimiter_RelojFijoJuntoAlBorde(t *testing.T) {
	const limite = 5
	l, _, _ := limitador(t, limite, ventanaPrueba)
	ahora := proximoBorde(ventanaPrueba).Add(-time.Second)
	l.ConReloj(func() time.Time { return ahora })
	ctx := context.Background()
	for i := 1; i <= limite; i++ {
		res, ok, err := l.TryAcquire(ctx, "par")
		if err != nil || !ok || res.Count() != i {
			t.Fatalf("attempt %d: ok=%v count=%d err=%v", i, ok, res.Count(), err)
		}
	}
	if _, ok, err := l.TryAcquire(ctx, "par"); err != nil || ok {
		t.Fatalf("attempt %d: ok=%v err=%v, want refused", limite+1, ok, err)
	}
	if n, err := l.Count(ctx, "par"); err != nil || n != limite {
		t.Errorf("Count = %d (%v), want %d", n, err, limite)
	}
}

// The window comes from the clock: crossing the boundary on the clock starts
// a new window (fixed-window semantics). Without the clock, the real now()
// of both calls would fall in the same window and the second would be refused.
func TestDBLimiter_LaVentanaSaleDelReloj(t *testing.T) {
	l, _, _ := limitador(t, 1, ventanaPrueba)
	borde := proximoBorde(ventanaPrueba)
	ahora := borde.Add(-time.Second)
	l.ConReloj(func() time.Time { return ahora })
	ctx := context.Background()
	if _, ok, err := l.TryAcquire(ctx, "k"); err != nil || !ok {
		t.Fatalf("first: ok=%v err=%v", ok, err)
	}
	if _, ok, _ := l.TryAcquire(ctx, "k"); ok {
		t.Fatal("same window: want refused")
	}
	ahora = borde.Add(time.Second)
	if res, ok, err := l.TryAcquire(ctx, "k"); err != nil || !ok || res.Count() != 1 {
		t.Fatalf("after the boundary: ok=%v count=%d err=%v, want a new window", ok, res.Count(), err)
	}
}

// Production (no clock) keeps the Postgres clock: the stored window is now()
// of Postgres truncated. A one-year window keeps the check off any boundary.
func TestDBLimiter_SinRelojUsaNowDePostgres(t *testing.T) {
	const anio = 365 * 24 * time.Hour
	l, db, prefijo := limitador(t, 3, anio)
	if _, ok, err := l.TryAcquire(context.Background(), "k"); err != nil || !ok {
		t.Fatalf("acquire: ok=%v err=%v", ok, err)
	}
	var coincide bool
	if err := db.Get(&coincide, `SELECT ventana_inicio = to_timestamp(floor(extract(epoch FROM now()) / $2::float8) * $2::float8)
		FROM auth_intentos_fallidos WHERE clave = $1`, prefijo+"k", anio.Seconds()); err != nil {
		t.Fatal(err)
	}
	if !coincide {
		t.Error("without a clock the window must be now() of Postgres truncated")
	}
}
