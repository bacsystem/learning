// Package testlock holds the session-level advisory lock that integration
// tests take while they create rows other packages' tests assert on globally
// (e.g. the number of active admins), and that serializes the integration
// tests of all packages on the one shared test database. It is a leaf package
// so every test can import it, including internal tests of packages that
// internal/db/dbtest itself imports (auth).
package testlock

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/jmoiron/sqlx"
)

// Key is the pg_advisory_lock key shared by dbtest.Open, questionstest.DB, the
// auth test fixtures and the seed/admin tests: 0x6c6561726e, "learn" in ASCII.
const Key int64 = 0x6c6561726e

var (
	mu    sync.Mutex
	owned = map[string]bool{} // root test name → its lock is already held
)

// Hold takes the lock for the whole test that t belongs to (a subtest shares
// its parent's) and releases it when that test ends. A test that opens more
// than one fixture, in a loop or through several helpers, calls Hold again
// without blocking on itself: the lock is taken once per test.
//
// db's connection pool must outlive the lock: register db.Close with
// t.Cleanup before calling Hold, so the lock is released first.
func Hold(t testing.TB, db *sqlx.DB) {
	t.Helper()
	root, _, _ := strings.Cut(t.Name(), "/")
	mu.Lock()
	if owned[root] {
		mu.Unlock()
		return
	}
	owned[root] = true
	mu.Unlock()

	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		release(root)
		t.Fatalf("db conn: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, Key); err != nil {
		_ = conn.Close()
		release(root)
		t.Fatalf("advisory lock: %v", err)
	}
	t.Cleanup(func() {
		_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, Key)
		_ = conn.Close()
		release(root)
	})
}

func release(root string) {
	mu.Lock()
	delete(owned, root)
	mu.Unlock()
}
