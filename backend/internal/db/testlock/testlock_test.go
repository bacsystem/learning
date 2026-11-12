package testlock

import (
	"os"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func enUso(root string) bool {
	mu.Lock()
	defer mu.Unlock()
	return owned[root]
}

// Hold is taken once per test: calling it again in the same test, or from its
// subtests, does not block on itself (a second pg_advisory_lock from another
// connection would wait forever), and the test releases it when it ends. The
// database lock may be held by other packages' tests while this one waits for
// its turn, so only this process's bookkeeping is asserted.
func TestHold_UnaVezPorTest(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	var raiz string
	t.Run("holds once", func(t *testing.T) {
		raiz, _, _ = strings.Cut(t.Name(), "/")
		pool, err := sqlx.Connect("postgres", dsn)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = pool.Close() })
		Hold(t, pool)
		Hold(t, pool) // the same test again: no self-deadlock
		if !enUso(raiz) {
			t.Error("the test does not hold the lock after Hold")
		}
		t.Run("sub", func(t *testing.T) {
			Hold(t, pool) // a subtest shares its parent's lock
			if !enUso(raiz) {
				t.Error("the subtest released or lost its parent's lock")
			}
		})
		if !enUso(raiz) {
			t.Error("a finished subtest released its parent's lock")
		}
	})
	if enUso(raiz) {
		t.Error("the lock was not released when the test ended")
	}
}
