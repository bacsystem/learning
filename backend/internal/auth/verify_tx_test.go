package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// ── H77 / H1: the persona e-mail is written in the verify transaction ───────

// Personas without e-mail on file, each with its own pending registration,
// race to verify with the SAME username. Exactly one wins; the losers fail
// on the usuario insert (ErrUsernameTaken) and their transaction must leave
// nothing behind: only the winner's persona ends up with an e-mail, and only
// one usuario exists. Writing the e-mail outside the transaction (or before
// the usuario insert with the pool) would leave the losers' e-mails written
// or block on the persona row lock (caught by the timeout).
func TestVerifyEmail_ConcurrentSameUsername_EmailOnlyForWinner(t *testing.T) {
	f := newFixture(t)
	const n = 6
	f.db.SetMaxIdleConns(n + 2)
	for round := 0; round < 4; round++ {
		pids := make([]int64, n)
		emails := make([]string, n)
		tokens := make([]string, n)
		for i := range pids {
			pid, dni := f.persona("")
			pids[i] = pid
			emails[i] = f.email(fmt.Sprintf("r%dp%d", round, i))
			if err := f.register(dni, emails[i]); err != nil {
				t.Fatal(err)
			}
			tokens[i] = f.lastVerifyToken(emails[i])
		}
		username := f.username(fmt.Sprintf("w%d", round))

		var wg sync.WaitGroup
		start := make(chan struct{})
		errs := make([]error, n)
		for i := range tokens {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				<-start
				errs[i] = f.svc.VerifyEmail(ctx, VerifyEmailRequest{
					Token: tokens[i], Username: username, Password: "Password123!", ConfirmPassword: "Password123!",
				}, "127.0.0.1", "test")
			}(i)
		}
		close(start)
		wg.Wait()

		winner := -1
		for i, err := range errs {
			switch {
			case err == nil:
				if winner != -1 {
					t.Fatalf("round %d: two winners (%d and %d)", round, winner, i)
				}
				winner = i
			case errors.Is(err, ErrUsernameTaken):
			default:
				t.Fatalf("round %d: verification %d: unexpected error %v", round, i, err)
			}
		}
		if winner == -1 {
			t.Fatalf("round %d: no winner (%v)", round, errs)
		}
		usuarios := 0
		for i, pid := range pids {
			usuarios += f.countUsuarios(pid)
			got := f.personaEmailHMAC(pid)
			if i == winner && got != f.repo.emailHMAC(emails[i]) {
				t.Fatalf("round %d: winner's persona email_hmac %q, want its e-mail", round, got)
			}
			if i != winner && got != "" {
				t.Fatalf("round %d: loser %d's persona got an e-mail written (email_hmac %q)", round, i, got)
			}
		}
		if usuarios != 1 {
			t.Fatalf("round %d: want exactly one usuario, got %d", round, usuarios)
		}
	}
}
