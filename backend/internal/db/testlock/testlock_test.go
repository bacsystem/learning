package testlock

import (
	"context"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// Hold is taken once per test: calling it again in the same test, or from its
// subtests, does not block on itself, and the lock is free when the test ends.
func TestHold_UnaVezPorTest(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	libre := func() bool {
		t.Helper()
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		var ok bool
		if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, Key).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		if ok {
			_, _ = conn.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, Key)
		}
		return ok
	}

	t.Run("holds once", func(t *testing.T) {
		pool, err := sqlx.Connect("postgres", dsn)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = pool.Close() })
		Hold(t, pool)
		Hold(t, pool) // the same test again: no self-deadlock
		t.Run("sub", func(t *testing.T) {
			Hold(t, pool) // a subtest shares its parent's lock
		})
		if libre() {
			t.Error("the lock is not held while the test runs")
		}
	})
	if !libre() {
		t.Error("the lock was not released when the test ended")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(owned) != 0 {
		t.Errorf("owned = %v, want empty after the test", owned)
	}
}
