package auth

import (
	"context"
	"errors"
	"testing"
)

// H129: once bloqueado_hasta has passed, the failure counter starts over. A
// single failed login after the lock expired must not lock the account again
// (otherwise one wrong guess every 15 minutes keeps any account locked).
func TestLogin_FailureAfterLockExpired_DoesNotRelock(t *testing.T) {
	f := newFixture(t)
	uid, username := f.activeUser("lx", "usuario")
	ctx := context.Background()
	wrong := LoginRequest{Username: username, Password: "wrong"}
	var err error
	for i := 0; i < maxLoginAttempts; i++ {
		_, err = f.svc.Login(ctx, wrong, "127.0.0.1", "t")
	}
	if !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("setup: want ErrAccountLocked, got %v", err)
	}
	if _, err := f.db.Exec(`UPDATE usuarios SET bloqueado_hasta = NOW() - INTERVAL '1 second' WHERE id=$1`, uid); err != nil {
		t.Fatal(err)
	}

	if _, err := f.svc.Login(ctx, wrong, "127.0.0.1", "t"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("first failure after the lock expired: want ErrInvalidCredentials, got %v", err)
	}
	var intentos int
	var locked bool
	if err := f.db.QueryRow(`SELECT login_intentos, COALESCE(bloqueado_hasta > NOW(), false) FROM usuarios WHERE id=$1`, uid).
		Scan(&intentos, &locked); err != nil {
		t.Fatal(err)
	}
	if locked || intentos != 1 {
		t.Fatalf("after the lock expired one failure must count as the first: intentos=%d locked=%v", intentos, locked)
	}
	// The correct password works again.
	f.login(username)

	// And a full new series still locks.
	for i := 0; i < maxLoginAttempts; i++ {
		_, err = f.svc.Login(ctx, wrong, "127.0.0.1", "t")
	}
	if !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("a new series of %d failures must lock again, got %v", maxLoginAttempts, err)
	}
}
