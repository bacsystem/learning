package auth

import (
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"testing"
)

// ── check-dni limiter review: atomic "first check", IPv6 /64, distinct sources

// testIP6Prefix returns a random documentation /64 ("2001:db8:x:y:") whose
// persisted counters are deleted on cleanup.
func (f *fixture) testIP6Prefix() string {
	f.t.Helper()
	p := fmt.Sprintf("2001:db8:%x:%x:", rand.Intn(0xffff), rand.Intn(0xffff))
	f.t.Cleanup(func() {
		if _, err := f.db.Exec(`DELETE FROM auth_intentos_fallidos WHERE clave LIKE '%'||$1||'%'`, f.repo.ipHMAC(p+":1")); err != nil {
			f.t.Errorf("cleanup ip counters: %v", err)
		}
	})
	return p
}

// counter returns the units used by a stored limiter key (all windows).
func (f *fixture) counter(key string) int {
	f.t.Helper()
	var n int
	if err := f.db.QueryRow(`SELECT COALESCE(SUM(intentos),0) FROM auth_intentos_fallidos WHERE clave=$1`, key).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

// Concurrent checks of one DNI from one IP charge the distinct-DNI (per-IP)
// and per-DNI limiters exactly once: "first check" is decided from the count
// returned by the same atomic statement that reserved the unit.
func TestHandlerCheckDNI_ConcurrentSameDNIChargesDistinctOnce(t *testing.T) {
	f := newFixture(t)
	mux := f.mux()
	for round := 0; round < 6; round++ {
		ip := f.testIP()
		_, dni := f.persona("")
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < checkDNIPerDNIAndIP; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				f.checkDNIFrom(mux, ip, dni)
			}()
		}
		close(start)
		wg.Wait()
		perIP := f.counter(checkDNIPerIPPrefix + f.repo.ipHMAC(ip))
		perDNI := f.counter(checkDNIPerDNIPrefix + f.repo.dniHMAC(dni))
		if perIP != 1 || perDNI != 1 {
			t.Fatalf("round %d: %d concurrent checks of one DNI from one IP charged per-IP=%d per-DNI=%d, want 1 and 1",
				round, checkDNIPerDNIAndIP, perIP, perDNI)
		}
	}
}

// Concurrent first checks of distinct DNIs from one IP are bounded by the
// per-IP limit.
func TestHandlerCheckDNI_ConcurrentDistinctDNIsBounded(t *testing.T) {
	f := newFixture(t)
	mux := f.mux()
	ip := f.testIP()
	const n = 12
	dnis := make([]string, n)
	for i := range dnis {
		_, dnis[i] = f.persona("")
	}
	var wg sync.WaitGroup
	start := make(chan struct{})
	codes := make([]int, n)
	for i := range dnis {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			codes[i] = f.checkDNIFrom(mux, ip, dnis[i]).Code
		}(i)
	}
	close(start)
	wg.Wait()
	ok := 0
	for _, c := range codes {
		if c == http.StatusOK {
			ok++
		}
	}
	if got := f.counter(checkDNIPerIPPrefix + f.repo.ipHMAC(ip)); ok != checkDNIDistinctPerIP || got != checkDNIDistinctPerIP {
		t.Fatalf("want exactly %d distinct DNIs through and charged, got ok=%d charged=%d (codes %v)",
			checkDNIDistinctPerIP, ok, got, codes)
	}
}

// Limiter keys use the client's /64 for IPv6: rotating addresses inside it
// does not buy more distinct checks; another /64 is independent.
func TestHandlerCheckDNI_IPv6SameSlash64SharesQuota(t *testing.T) {
	f := newFixture(t)
	mux := f.mux()
	p := f.testIP6Prefix()
	for i := 0; i < checkDNIDistinctPerIP; i++ {
		_, dni := f.persona("")
		if w := f.checkDNIFrom(mux, fmt.Sprintf("[%s:%x]", p, i+1), dni); w.Code != http.StatusOK {
			t.Fatalf("check %d: want 200, got %d", i+1, w.Code)
		}
	}
	_, dni := f.persona("")
	if w := f.checkDNIFrom(mux, "["+p+":abcd]", dni); w.Code != http.StatusTooManyRequests {
		t.Fatalf("another address in the same /64: want 429, got %d", w.Code)
	}
	if w := f.checkDNIFrom(mux, "["+f.testIP6Prefix()+":1]", dni); w.Code != http.StatusOK {
		t.Fatalf("another /64: want 200, got %d", w.Code)
	}
}

// One source hammering a DNI does not consume the per-DNI quota of other
// sources: it counts distinct sources only. Many distinct sources do hit the
// cap, but a source already holding a check of that DNI keeps passing.
func TestHandlerCheckDNI_PerDNICountsDistinctSources(t *testing.T) {
	f := newFixture(t)
	mux := f.mux()
	_, dni := f.persona("")
	// Six IPs each exhaust their (dni, ip) pair: 30 checks in total.
	for s := 0; s < 6; s++ {
		ip := f.testIP()
		for i := 0; i < checkDNIPerDNIAndIP; i++ {
			f.checkDNIFrom(mux, ip, dni)
		}
	}
	owner := f.testIP()
	if w := f.checkDNIFrom(mux, owner, dni); w.Code != http.StatusOK {
		t.Fatalf("owner's first check after six sources hammering: want 200, got %d", w.Code)
	}
	// Fill the per-DNI cap with distinct sources (7 used so far).
	for s := 7; s < checkDNIPerDNI; s++ {
		if w := f.checkDNIFrom(mux, f.testIP(), dni); w.Code != http.StatusOK {
			t.Fatalf("distinct source %d: want 200, got %d", s+1, w.Code)
		}
	}
	if w := f.checkDNIFrom(mux, f.testIP(), dni); w.Code != http.StatusTooManyRequests {
		t.Fatalf("source #%d: want 429, got %d", checkDNIPerDNI+1, w.Code)
	}
	if w := f.checkDNIFrom(mux, owner, dni); w.Code != http.StatusOK {
		t.Fatalf("owner already counted: want 200 past the per-DNI cap, got %d", w.Code)
	}
}

// H101: once an IP has used its distinct-DNI quota, concurrent checks of a
// new DNI from it are all refused. The (dni, ip) pair reservation of the
// first request must not let the others pass as "already checked" while
// that first one is refused by the per-IP limiter and refunded.
func TestHandlerCheckDNI_ConcurrentSameDNIAfterPerIPCap_AllRefused(t *testing.T) {
	f := newFixture(t)
	ip := f.testIP()
	for i := 0; i < checkDNIDistinctPerIP; i++ {
		_, dni := f.persona("")
		if w := f.checkDNIFrom(f.mux(), ip, dni); w.Code != http.StatusOK {
			t.Fatalf("setup check %d: want 200, got %d", i+1, w.Code)
		}
	}
	for round := 0; round < 4; round++ {
		mux := f.mux() // fresh in-memory 20/min limiter per round
		_, dni := f.persona("")
		var wg sync.WaitGroup
		start := make(chan struct{})
		codes := make([]int, checkDNIPerDNIAndIP)
		for i := range codes {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				codes[i] = f.checkDNIFrom(mux, ip, dni).Code
			}(i)
		}
		close(start)
		wg.Wait()
		for i, c := range codes {
			if c != http.StatusTooManyRequests {
				t.Fatalf("round %d: request %d got %d, want all 429 (codes %v)", round, i, c, codes)
			}
		}
		perIP := f.counter(checkDNIPerIPPrefix + f.repo.ipHMAC(ip))
		perDNI := f.counter(checkDNIPerDNIPrefix + f.repo.dniHMAC(dni))
		if perIP != checkDNIDistinctPerIP || perDNI != 0 {
			t.Fatalf("round %d: counters per-IP=%d per-DNI=%d, want %d and 0", round, perIP, perDNI, checkDNIDistinctPerIP)
		}
	}
}
