package testlock

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func conectar(t *testing.T) (*sqlx.DB, string) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, dsn
}

func enUso(root string, key int64) bool {
	mu.Lock()
	defer mu.Unlock()
	return owned[fmt.Sprintf("%s#%d", root, key)]
}

// libre reports whether another session can take the advisory lock now.
func libre(t *testing.T, db *sqlx.DB, key int64) bool {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var ok bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, key).Scan(&ok); err != nil {
		t.Fatal(err)
	}
	if ok {
		_, _ = conn.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, key)
	}
	return ok
}

// hold takes the lock in Postgres (another session cannot get it meanwhile),
// once per test: a second call in the same test, or from its subtests, does
// not wait for itself, and the lock is free when the test ends. It runs on a
// key of its own, so the tests of other packages holding Key cannot make it
// pass or fail by accident.
func TestHold_TomaElLockEnPostgres(t *testing.T) {
	db, dsn := conectar(t)
	key := Key + 1 + int64(os.Getpid())
	var raiz string
	t.Run("holds once", func(t *testing.T) {
		raiz, _, _ = strings.Cut(t.Name(), "/")
		if !libre(t, db, key) {
			t.Fatal("setup: the test key is not free")
		}
		pool, err := sqlx.Connect("postgres", dsn)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = pool.Close() })
		hold(t, pool, key)
		if libre(t, db, key) {
			t.Error("Hold did not take the advisory lock in Postgres")
		}
		hold(t, pool, key) // the same test again: no self-deadlock, nothing else taken
		t.Run("sub", func(t *testing.T) {
			hold(t, pool, key) // a subtest shares its parent's lock
			if !enUso(raiz, key) {
				t.Error("the subtest released or lost its parent's lock")
			}
		})
		if libre(t, db, key) || !enUso(raiz, key) {
			t.Error("a finished subtest released its parent's lock")
		}
	})
	if !libre(t, db, key) {
		t.Error("the advisory lock was not released when the test ended")
	}
	if enUso(raiz, key) {
		t.Error("the bookkeeping still says the lock is held after the test")
	}
}
