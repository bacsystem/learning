package auth

import (
	"fmt"
	"net/http"
	"testing"
)

// H128: the register limiter must charge a correct (DNI, e-mail) pair exactly
// like a wrong one. If success refunded its unit, the N-th request of a series
// would answer 202 for the right e-mail and 429 for a wrong one: an oracle for
// the e-mail on file, behind the generic 202.
func TestHandlerRegister_LimiterChargesSuccessLikeFailure(t *testing.T) {
	f := newFixture(t)
	h := NewHandler(f.svc)
	h.registerMinDuration = 0
	_, dniOK := f.persona(f.email("right"))
	_, dniKO := f.persona(f.email("other"))

	for i := 1; i <= registerFailuresPerDNIAndIP+1; i++ {
		right := f.registerVia(h, dniOK, f.email("right"), "203.0.113.120:1")
		wrong := f.registerVia(h, dniKO, f.email(fmt.Sprintf("guess%d", i)), "203.0.113.121:1")
		if right != wrong {
			t.Fatalf("request %d: correct e-mail got %d, wrong e-mail got %d (observable difference)", i, right, wrong)
		}
		want := http.StatusAccepted
		if i > registerFailuresPerDNIAndIP {
			want = http.StatusTooManyRequests
		}
		if right != want {
			t.Fatalf("request %d: want %d, got %d", i, want, right)
		}
	}
}
