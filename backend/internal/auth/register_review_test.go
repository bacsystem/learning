package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/lib/pq"
)

// ── Review findings: shared e-mails and unlimited re-issues ─────────────────

// A persona with no e-mail on file cannot be registered with an e-mail that
// already belongs to another persona: otherwise both would share the
// email_hmac and forgot-password (which refuses ambiguous e-mails) would stop
// working for the victim.
func TestRegister_NoEmailOnFile_EmailOfAnotherPersona_Rejected(t *testing.T) {
	f := newFixture(t)
	victimUID, _ := f.activeUser("vic", "usuario")
	victimEmail := f.email("vic")
	pid, dni := f.persona("")

	if err := f.register(dni, victimEmail); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("want ErrEmailTaken, got %v", err)
	}
	if f.countPendientes(pid) != 0 {
		t.Fatal("pending registration created with another persona's e-mail")
	}
	if err := f.svc.ForgotPassword(context.Background(), victimEmail, "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if f.tokensFor(victimUID) != 1 {
		t.Fatal("victim's forgot-password no longer issues a reset token")
	}

	// Same when the owner of the e-mail has no usuario yet (it would become
	// ambiguous as soon as the owner registers).
	f.persona(f.email("later"))
	if err := f.register(dni, f.email("later")); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("e-mail of a persona without usuario: want ErrEmailTaken, got %v", err)
	}
	if f.countPendientes(pid) != 0 {
		t.Fatal("pending registration created with another persona's e-mail")
	}
}

// Unverified pending registrations must not squat an address: another
// persona without e-mail can still register with it (conflicts are settled
// at verify time, under the lock).
func TestRegister_OtherPersonasPendingRowsDoNotBlock(t *testing.T) {
	f := newFixture(t)
	shared := f.email("shared")
	p1, dni1 := f.persona("")
	p2, dni2 := f.persona("")
	if err := f.register(dni1, shared); err != nil {
		t.Fatal(err)
	}
	first := f.lastVerifyToken(shared)
	if err := f.register(dni2, shared); err != nil {
		t.Fatalf("another persona's pending row blocked the registration: %v", err)
	}
	if f.countPendientes(p2) != 1 {
		t.Fatal("pending registration not created")
	}
	second := f.lastVerifyToken(shared)
	// The first to verify gets the address; the other is refused.
	if err := f.verify(second, f.username("two")); err != nil {
		t.Fatal(err)
	}
	if err := f.verify(first, f.username("one")); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("second verification with the same e-mail: want ErrEmailTaken, got %v", err)
	}
	if f.countUsuarios(p1) != 0 {
		t.Fatal("both personas ended up with the same e-mail")
	}
}

// Pending registrations carrying the same e-mail verified concurrently:
// exactly one may write it to its persona (advisory lock).
func TestVerifyEmail_ConcurrentSameEmail_ExactlyOneSucceeds(t *testing.T) {
	f := newFixture(t)
	shared := f.email("shared")
	const n = 8
	pids := make([]int64, n)
	tokens := make([]string, n)
	for i := range pids {
		pid, dni := f.persona("")
		pids[i] = pid
		if err := f.register(dni, shared); err != nil {
			t.Fatal(err)
		}
		tokens[i] = f.lastVerifyToken(shared)
	}
	// Warm the pool so the verifications really overlap.
	f.db.SetMaxIdleConns(n + 2)
	var warm sync.WaitGroup
	for i := 0; i < n; i++ {
		warm.Add(1)
		go func() { defer warm.Done(); _, _ = f.db.Exec(`SELECT pg_sleep(0.05)`) }()
	}
	warm.Wait()

	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, n)
	for i := range tokens {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = f.verify(tokens[i], f.username(fmt.Sprintf("s%d", i)))
		}(i)
	}
	close(start)
	wg.Wait()
	ok := 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, ErrEmailTaken):
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	var withEmail int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM personas WHERE id = ANY($1) AND email_hmac=$2`,
		pq.Array(pids), f.repo.emailHMAC(shared)).Scan(&withEmail); err != nil {
		t.Fatal(err)
	}
	usuarios := 0
	for _, pid := range pids {
		usuarios += f.countUsuarios(pid)
	}
	if ok != 1 || withEmail != 1 || usuarios != 1 {
		t.Fatalf("want exactly one verification, got ok=%d personas_with_email=%d usuarios=%d", ok, withEmail, usuarios)
	}
}

// Accepted (re)issues are capped per persona and never refunded: past the
// cap nothing is stored or e-mailed (same generic outcome for the caller),
// so the latest valid link keeps working and the inbox is not flooded.
func TestRegister_AcceptedReissuesCapped(t *testing.T) {
	f := newFixture(t)
	email := f.email("owner")
	pid, dni := f.persona(email)
	for i := 0; i < registerIssuesPerPersona; i++ {
		if err := f.register(dni, email); err != nil {
			t.Fatalf("issue %d: %v", i+1, err)
		}
	}
	last := f.lastVerifyToken(email)
	sent := len(f.verifyEmails(email))
	if err := f.register(dni, email); err != nil {
		t.Fatalf("over the cap: want nil (generic), got %v", err)
	}
	if got := len(f.verifyEmails(email)); got != sent {
		t.Fatalf("over the cap an e-mail was queued (%d → %d)", sent, got)
	}
	if f.lastVerifyToken(email) != last || f.countPendientes(pid) != 1 {
		t.Fatal("over the cap a new pending registration replaced the last one")
	}
	if err := f.verify(last, f.username("own")); err != nil {
		t.Fatalf("last issued token must still work: %v", err)
	}
}
