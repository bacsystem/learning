package dbtest

import (
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

// LockWaitTimeout bounds how long BlockedBy waits for a backend to block.
const LockWaitTimeout = 5 * time.Second

// BlockedBy reports whether some backend ends up blocked by the backend pid
// before done delivers a value (the request answered on its own) or the
// timeout expires. A value received from done is sent back so the caller can
// still read it. Tests use it to prove that a request waits for a row lock
// held by another transaction.
func BlockedBy(t *testing.T, db *sqlx.DB, pid int, done chan int) bool {
	t.Helper()
	limit := time.After(LockWaitTimeout)
	for {
		var blocked bool
		if err := db.Get(&blocked,
			`SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid)))`, pid); err != nil {
			t.Fatal(err)
		}
		if blocked {
			return true
		}
		select {
		case v := <-done:
			done <- v
			return false
		case <-limit:
			return false
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// BackendPID returns the backend pid of tx's connection.
func BackendPID(t *testing.T, tx *sqlx.Tx) int {
	t.Helper()
	var pid int
	if err := tx.Get(&pid, `SELECT pg_backend_pid()`); err != nil {
		t.Fatal(err)
	}
	return pid
}
