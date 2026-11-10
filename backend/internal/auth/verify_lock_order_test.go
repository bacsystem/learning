package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lib/pq"
)

// H105: an admin e-mail change of a persona must not deadlock with the
// verification of that persona's pending registration. Verify locks the
// registros_pendientes row first and the personas row later; the admin
// change deletes that pending row (cutOldEmailChannel) after locking the
// persona. The verification is held right after its step 1 (test seam
// afterPendingLock) until the admin change is waiting on a lock, then let
// go: with inconsistent lock orders Postgres aborts one of them with 40P01.
func TestAdminEmailChange_ConcurrentVerify_NoDeadlock(t *testing.T) {
	cases := []struct {
		name   string
		change func(f *fixture, pid int64, email string) error
	}{
		{"UpdatePersonaEmail", func(f *fixture, pid int64, email string) error {
			_, err := f.repo.UpdatePersonaEmail(context.Background(), pid, email, nil)
			return err
		}},
		{"UpdatePersona", func(f *fixture, pid int64, email string) error {
			_, err := f.repo.UpdatePersona(context.Background(), pid, UpdatePersonaRequest{
				Nombres: "Juan", ApellidoPaterno: "Perez", ApellidoMaterno: "Lopez",
				Email: email, Telefono: "999999999", Estado: "activo",
				GradoID: f.gradoID, EspecialidadID: f.espID, InstitucionID: f.instID,
			}, nil)
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			oldEmail := f.email("old" + tc.name)
			pid, dni := f.persona(oldEmail)
			if err := f.register(dni, oldEmail); err != nil {
				t.Fatal(err)
			}
			token := f.lastVerifyToken(oldEmail)

			paused := make(chan struct{})
			resume := make(chan struct{})
			f.repo.afterPendingLock = func() {
				close(paused)
				<-resume
			}
			t.Cleanup(func() { f.repo.afterPendingLock = nil })

			verifyErr := make(chan error, 1)
			go func() { verifyErr <- f.verify(token, f.username("v")) }()
			select {
			case <-paused:
			case <-time.After(10 * time.Second):
				t.Fatal("verification never reached step 1")
			}

			adminErr := make(chan error, 1)
			go func() { adminErr <- tc.change(f, pid, f.email("new"+tc.name)) }()
			f.waitForLockWaiter("registros_pendientes")
			close(resume)

			for _, ch := range []struct {
				who string
				c   chan error
			}{{"verify", verifyErr}, {"admin change", adminErr}} {
				select {
				case err := <-ch.c:
					var pqErr *pq.Error
					if errors.As(err, &pqErr) && pqErr.Code == "40P01" {
						t.Fatalf("%s: deadlock (40P01): %v", ch.who, err)
					}
					if err != nil {
						t.Fatalf("%s: %v", ch.who, err)
					}
				case <-time.After(15 * time.Second):
					t.Fatalf("%s did not finish", ch.who)
				}
			}
		})
	}
}

// waitForLockWaiter blocks until some backend of this database is waiting
// on a lock in a statement mentioning table, or fails after 10 s.
func (f *fixture) waitForLockWaiter(table string) {
	f.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var n int
		if err := f.db.QueryRow(`SELECT COUNT(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'
			  AND query ILIKE '%'||$1||'%'`, table).Scan(&n); err != nil {
			f.t.Fatal(err)
		}
		if n > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	f.t.Fatalf("no transaction waiting on a lock over %s", table)
}
