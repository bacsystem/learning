package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeClock is a virtual clock: work takes no virtual time, sleeping advances
// it. With it, the virtual duration of a request is exactly what padSince
// adds, independently of the real time the DB work takes.
type fakeClock struct {
	mu     sync.Mutex
	cur    time.Time
	slept  []time.Duration
	onNow  func(call int) time.Duration // optional extra advance per now() call
	nCalls int
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nCalls++
	if c.onNow != nil {
		c.cur = c.cur.Add(c.onNow(c.nCalls))
	}
	return c.cur
}

func (c *fakeClock) sleep(_ context.Context, d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.slept = append(c.slept, d)
	c.cur = c.cur.Add(d)
}

func (f *fixture) timedHandler(min time.Duration, clk *fakeClock) *Handler {
	h := NewHandler(f.svc)
	h.registerMinDuration = min
	h.now = clk.now
	h.sleep = clk.sleep
	return h
}

// Unknown DNI, e-mail mismatch and success all answer the generic 202 after
// the same minimum duration, measured from request start.
func TestHandlerRegister_GenericOutcomesPaddedToSameMinimum(t *testing.T) {
	f := newFixture(t)
	email := f.email("owner")
	_, dni := f.persona(email)
	const min = 400 * time.Millisecond
	cases := []struct{ name, dni, email string }{
		{"not found", "00000001", email},
		{"mismatch", dni, f.email("attacker")},
		{"success", dni, email},
	}
	for i, c := range cases {
		clk := &fakeClock{cur: time.Unix(1_700_000_000, 0)}
		h := f.timedHandler(min, clk)
		body := fmt.Sprintf(`{"dni":%q,"email":%q}`, c.dni, c.email)
		req := jsonReq(http.MethodPost, "/auth/register", strings.NewReader(body))
		req.RemoteAddr = fmt.Sprintf("198.51.100.%d:1", 60+i)
		w := httptest.NewRecorder()
		start := clk.cur
		h.Register(w, req)
		if w.Code != http.StatusAccepted {
			t.Fatalf("%s: want 202, got %d %s", c.name, w.Code, w.Body.String())
		}
		if len(clk.slept) != 1 || clk.slept[0] != min {
			t.Fatalf("%s: want one pad of %v, got %v", c.name, min, clk.slept)
		}
		if got := clk.cur.Sub(start); got != min {
			t.Fatalf("%s: response after %v (virtual), want %v", c.name, got, min)
		}
	}
	if len(f.verifyEmails(email)) != 1 {
		t.Fatal("success case did not issue the link")
	}
}

// Work that already took longer than the minimum is not delayed further;
// shape validation errors are not padded.
func TestHandlerRegister_PaddingOnlyTopsUp(t *testing.T) {
	f := newFixture(t)
	const min = 400 * time.Millisecond
	// Every now() after the first (request start) jumps 1s: the work "took" longer.
	clk := &fakeClock{cur: time.Unix(1_700_000_000, 0), onNow: func(n int) time.Duration {
		if n > 1 {
			return time.Second
		}
		return 0
	}}
	h := f.timedHandler(min, clk)
	req := jsonReq(http.MethodPost, "/auth/register", strings.NewReader(`{"dni":"00000001","email":"x@`+f.domain+`"}`))
	req.RemoteAddr = "198.51.100.70:1"
	w := httptest.NewRecorder()
	h.Register(w, req)
	if w.Code != http.StatusAccepted || len(clk.slept) != 0 {
		t.Fatalf("slow path: want 202 without extra sleep, got %d slept=%v", w.Code, clk.slept)
	}

	clk2 := &fakeClock{cur: time.Unix(1_700_000_000, 0)}
	h2 := f.timedHandler(min, clk2)
	w2 := httptest.NewRecorder()
	h2.Register(w2, jsonReq(http.MethodPost, "/auth/register", strings.NewReader(`{"dni":"123","email":"bad"}`)))
	if w2.Code != http.StatusUnprocessableEntity || len(clk2.slept) != 0 {
		t.Fatalf("validation error: want 422 unpadded, got %d slept=%v", w2.Code, clk2.slept)
	}
}

// The real sleeper returns as soon as the request context is cancelled.
func TestSleepCtx_RespectsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	sleepCtx(ctx, time.Minute)
	if time.Since(start) > time.Second {
		t.Fatal("sleepCtx ignored the cancelled context")
	}
}
