package notifier

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// fakeSender records deliveries; behaviour per address can be overridden.
type fakeSender struct {
	mu    sync.Mutex
	sent  map[string]int
	delay time.Duration
	hook  func(ctx context.Context, to string) error // may panic or block
}

func newFake() *fakeSender { return &fakeSender{sent: map[string]int{}} }

// Send models a real SMTP client: it honours the per-message deadline, so a
// slow or hung server surfaces as a timeout error instead of blocking forever.
func (f *fakeSender) Send(ctx context.Context, to, subject, html string) error {
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return fmt.Errorf("smtp: %w", ctx.Err())
		}
	}
	if f.hook != nil {
		if err := f.hook(ctx, to); err != nil {
			return err
		}
	}
	f.mu.Lock()
	f.sent[to]++
	f.mu.Unlock()
	return nil
}

func (f *fakeSender) count(to string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sent[to]
}

func testWorker(db *sqlx.DB, s Sender) *EmailWorker {
	w := newEmailWorker(db, s)
	w.batchSize = 1000 // never starve on unrelated rows in the shared test DB
	w.interval = 10 * time.Millisecond
	return w
}

func enqueue(t *testing.T, db *sqlx.DB, n int) []string {
	t.Helper()
	u := dbtest.Uniq()
	addrs := make([]string, n)
	for i := range addrs {
		addrs[i] = fmt.Sprintf("%s-%d@notifier.test", u, i)
		if _, err := db.Exec(`INSERT INTO email_queue (to_address, subject, body_html) VALUES ($1,'s','<p>b</p>')`, addrs[i]); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM email_queue WHERE to_address LIKE $1`, u+"-%") })
	return addrs
}

type rowState struct {
	Status      string
	RetryCount  int
	LastError   *string
	NextRetryIn float64 // seconds from now
}

func state(t *testing.T, db *sqlx.DB, to string) rowState {
	t.Helper()
	var s rowState
	if err := db.QueryRow(`SELECT status, retry_count, last_error, EXTRACT(EPOCH FROM next_retry_at - NOW())
		FROM email_queue WHERE to_address=$1`, to).Scan(&s.Status, &s.RetryCount, &s.LastError, &s.NextRetryIn); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestFlush_SendsAndMarksSent(t *testing.T) {
	db := dbtest.Open(t)
	f := newFake()
	addrs := enqueue(t, db, 2)
	testWorker(db, f).flush(context.Background())
	for _, a := range addrs {
		if f.count(a) != 1 || state(t, db, a).Status != "sent" {
			t.Fatalf("%s: sent %d times, status %s", a, f.count(a), state(t, db, a).Status)
		}
	}
	testWorker(db, f).flush(context.Background())
	if f.count(addrs[0]) != 1 {
		t.Fatal("sent row delivered again")
	}
}

// H30: several replicas flushing at once never deliver the same email twice.
func TestFlush_ConcurrentWorkersSendOnce(t *testing.T) {
	db := dbtest.Open(t)
	f := newFake()
	f.delay = 5 * time.Millisecond
	addrs := enqueue(t, db, 12)
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			testWorker(db, f).flush(context.Background())
		}()
	}
	wg.Wait()
	for _, a := range addrs {
		if n := f.count(a); n != 1 {
			t.Fatalf("%s delivered %d times, want 1", a, n)
		}
	}
}

// H30: a send failure is recorded with exponential backoff; retries stop at 5.
func TestFlush_FailureBackoff(t *testing.T) {
	db := dbtest.Open(t)
	f := newFake()
	f.hook = func(context.Context, string) error { return errors.New("smtp: 451 try later") }
	addr := enqueue(t, db, 1)[0]
	w := testWorker(db, f)

	w.flush(context.Background())
	s := state(t, db, addr)
	if s.Status != "failed" || s.RetryCount != 1 || s.LastError == nil || *s.LastError == "" {
		t.Fatalf("after 1st failure: %+v", s)
	}
	first := s.NextRetryIn
	if first < 20 {
		t.Fatalf("next retry in %.0fs, want backoff ≥ 20s", first)
	}
	w.flush(context.Background()) // not due yet
	if s2 := state(t, db, addr); s2.RetryCount != 1 {
		t.Fatalf("retried before next_retry_at: %+v", s2)
	}
	for i := 2; i <= 6; i++ {
		_, _ = db.Exec(`UPDATE email_queue SET next_retry_at=NOW() WHERE to_address=$1`, addr)
		w.flush(context.Background())
		s = state(t, db, addr)
		if i == 2 && s.NextRetryIn < first*1.5 {
			t.Fatalf("backoff not growing: %.0fs then %.0fs", first, s.NextRetryIn)
		}
	}
	if s.RetryCount != maxRetries {
		t.Fatalf("retry_count = %d, want capped at %d", s.RetryCount, maxRetries)
	}
}

// H30: a panic while sending one message does not kill the worker.
func TestFlush_PanicRecovered(t *testing.T) {
	db := dbtest.Open(t)
	f := newFake()
	addrs := enqueue(t, db, 2)
	f.hook = func(_ context.Context, to string) error {
		if to == addrs[0] {
			panic("boom")
		}
		return nil
	}
	testWorker(db, f).flush(context.Background())
	if s := state(t, db, addrs[0]); s.Status != "failed" || s.RetryCount != 1 {
		t.Fatalf("panicking row: %+v", s)
	}
	if state(t, db, addrs[1]).Status != "sent" {
		t.Fatal("other row not processed after panic")
	}
}

// H30: a claim left behind (worker died / could not record the result) is
// retried only after the stale timeout, counts as an attempt, and is abandoned
// after maxRetries instead of being resent forever.
func TestFlush_StaleClaims(t *testing.T) {
	db := dbtest.Open(t)
	f := newFake()
	addrs := enqueue(t, db, 3)
	_, _ = db.Exec(`UPDATE email_queue SET status='sending', claimed_at=NOW()-INTERVAL '1 minute' WHERE to_address=$1`, addrs[0])
	_, _ = db.Exec(`UPDATE email_queue SET status='sending', claimed_at=NOW()-INTERVAL '1 hour' WHERE to_address=$1`, addrs[1])
	_, _ = db.Exec(`UPDATE email_queue SET status='sending', claimed_at=NOW()-INTERVAL '1 hour', retry_count=4 WHERE to_address=$1`, addrs[2])

	testWorker(db, f).flush(context.Background())
	if f.count(addrs[0]) != 0 || state(t, db, addrs[0]).Status != "sending" {
		t.Fatal("fresh claim of another worker was stolen")
	}
	if s := state(t, db, addrs[1]); f.count(addrs[1]) != 1 || s.Status != "sent" || s.RetryCount != 1 {
		t.Fatalf("stale claim not retried once: sent=%d %+v", f.count(addrs[1]), s)
	}
	if s := state(t, db, addrs[2]); f.count(addrs[2]) != 0 || s.Status != "failed" || s.RetryCount != maxRetries {
		t.Fatalf("exhausted stale claim: sent=%d %+v", f.count(addrs[2]), s)
	}
}

// H30: Run returns only after the in-flight message finished and was recorded,
// even though ctx is already cancelled.
func TestRun_WaitsForInFlightOnShutdown(t *testing.T) {
	db := dbtest.Open(t)
	f := newFake()
	addr := enqueue(t, db, 1)[0]
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	f.hook = func(_ context.Context, to string) error {
		if to == addr {
			once.Do(func() { close(started) })
			<-release
		}
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { testWorker(db, f).Run(ctx); close(done) }()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("worker never picked the message")
	}
	cancel()
	select {
	case <-done:
		t.Fatal("Run returned while a message was still being sent")
	case <-time.After(150 * time.Millisecond):
	}
	close(release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after the in-flight message finished")
	}
	if s := state(t, db, addr); s.Status != "sent" {
		t.Fatalf("in-flight message not recorded as sent after shutdown: %+v", s)
	}
}

// claimedAt returns the row's current claimed_at (zero when NULL).
func claimedAt(t *testing.T, db *sqlx.DB, to string) time.Time {
	t.Helper()
	var c *time.Time
	if err := db.QueryRow(`SELECT claimed_at FROM email_queue WHERE to_address=$1`, to).Scan(&c); err != nil {
		t.Fatal(err)
	}
	if c == nil {
		return time.Time{}
	}
	return *c
}

// H69: replica A claims a batch and is stuck on slow SMTP; meanwhile its claims
// go stale and replica B reclaims and sends them. When A resumes it must not
// send the rows it no longer owns.
func TestFlush_SlowReplicaDoesNotResendReclaimedRows(t *testing.T) {
	db := dbtest.Open(t)
	addrs := enqueue(t, db, 3)
	slow := newFake()
	started, resume := make(chan struct{}), make(chan struct{})
	slow.hook = func(_ context.Context, to string) error {
		if to == addrs[0] {
			close(started)
			<-resume
		}
		return nil
	}
	a := testWorker(db, slow)
	a.sendTimeout = time.Minute
	done := make(chan struct{})
	go func() { a.flush(context.Background()); close(done) }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("replica A never started sending")
	}
	// Simulate A's SMTP being slow for longer than staleAfter.
	if _, err := db.Exec(`UPDATE email_queue SET claimed_at = claimed_at - INTERVAL '1 hour'
		WHERE to_address = ANY($1)`, pq.Array(addrs)); err != nil {
		t.Fatal(err)
	}
	fast := newFake()
	testWorker(db, fast).flush(context.Background()) // replica B
	close(resume)
	<-done

	for _, addr := range addrs[1:] {
		if n := slow.count(addr) + fast.count(addr); n != 1 {
			t.Fatalf("%s delivered %d times (A=%d, B=%d), want 1", addr, n, slow.count(addr), fast.count(addr))
		}
		if s := state(t, db, addr); s.Status != "sent" {
			t.Fatalf("%s: %+v", addr, s)
		}
	}
}

// H69/H131: right before each send the claim of the row being sent and the
// claims of the rows still waiting in the batch are renewed: addrs[1] already
// carries a fresh claim while addrs[0] is being sent, and is renewed again
// right before its own send. (The tail-reclaim scenario itself is covered by
// TestFlush_LongBatchTailIsNotReclaimed.)
func TestFlush_RenewsClaimBeforeEachSend(t *testing.T) {
	db := dbtest.Open(t)
	addrs := enqueue(t, db, 2)
	f := newFake()
	var seen, tailDuringFirst time.Time
	f.hook = func(_ context.Context, to string) error {
		if to == addrs[1] {
			seen = claimedAt(t, db, to)
		}
		if to == addrs[0] {
			tailDuringFirst = claimedAt(t, db, addrs[1])
			time.Sleep(20 * time.Millisecond)
		}
		return nil
	}
	w := testWorker(db, f)
	rows, err := w.claim(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var claimed time.Time
	for _, r := range rows {
		if r.ToAddr == addrs[1] {
			claimed = r.ClaimedAt
		}
	}
	w.process(context.Background(), rows)
	if !tailDuringFirst.After(claimed) {
		t.Fatalf("tail claim not renewed before the first send: claimed %v, during first send %v", claimed, tailDuringFirst)
	}
	if !seen.After(tailDuringFirst.Add(10 * time.Millisecond)) {
		t.Fatalf("claim not renewed before its own send: renewed %v, at send %v", tailDuringFirst, seen)
	}
	if s := state(t, db, addrs[1]); s.Status != "sent" {
		t.Fatalf("renewed row not marked sent: %+v", s)
	}
}

// H69/H30: markSent and markFailed only touch rows still owned by this claim.
func TestMarkSentAndFailed_RequireClaimOwnership(t *testing.T) {
	db := dbtest.Open(t)
	addrs := enqueue(t, db, 2)
	w := testWorker(db, newFake())
	rows, err := w.claim(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	mine := map[string]emailRow{}
	for _, r := range rows {
		mine[r.ToAddr] = r
	}
	// Another replica reclaimed both rows meanwhile.
	if _, err := db.Exec(`UPDATE email_queue SET claimed_at = clock_timestamp() + INTERVAL '1 second'
		WHERE to_address = ANY($1)`, pq.Array(addrs)); err != nil {
		t.Fatal(err)
	}
	w.markSent(context.Background(), mine[addrs[0]])
	w.markFailed(context.Background(), mine[addrs[1]], errors.New("boom"))
	for _, a := range addrs {
		if s := state(t, db, a); s.Status != "sending" || s.RetryCount != 0 || s.LastError != nil {
			t.Fatalf("%s overwritten although the claim was lost: %+v", a, s)
		}
	}
	// With the claim still owned both are recorded.
	for _, a := range addrs {
		if _, err := db.Exec(`UPDATE email_queue SET claimed_at=$2 WHERE to_address=$1`, a, mine[a].ClaimedAt); err != nil {
			t.Fatal(err)
		}
	}
	w.markSent(context.Background(), mine[addrs[0]])
	w.markFailed(context.Background(), mine[addrs[1]], errors.New("boom"))
	if s := state(t, db, addrs[0]); s.Status != "sent" {
		t.Fatalf("owned row not marked sent: %+v", s)
	}
	if s := state(t, db, addrs[1]); s.Status != "failed" || s.RetryCount != 1 {
		t.Fatalf("owned row not marked failed: %+v", s)
	}
}

// H30: on shutdown the rest of the claimed batch goes back to the queue
// immediately instead of waiting for the stale timeout; rows reclaimed by
// another replica meanwhile are left alone.
func TestRun_ReleasesUnsentBatchOnShutdown(t *testing.T) {
	db := dbtest.Open(t)
	addrs := enqueue(t, db, 3)
	f := newFake()
	started, resume := make(chan struct{}), make(chan struct{})
	var once sync.Once
	f.hook = func(_ context.Context, to string) error {
		if to == addrs[0] {
			once.Do(func() { close(started) })
			<-resume
		}
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { testWorker(db, f).Run(ctx); close(done) }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("worker never picked the message")
	}
	// addrs[2] is reclaimed by another replica while we are blocked.
	other := time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond)
	if _, err := db.Exec(`UPDATE email_queue SET claimed_at=$2 WHERE to_address=$1`, addrs[2], other); err != nil {
		t.Fatal(err)
	}
	cancel()
	close(resume)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return")
	}
	if s := state(t, db, addrs[0]); s.Status != "sent" {
		t.Fatalf("in-flight row: %+v", s)
	}
	if s := state(t, db, addrs[1]); s.Status != "pending" || f.count(addrs[1]) != 0 || !claimedAt(t, db, addrs[1]).IsZero() {
		t.Fatalf("unsent row not released: sent=%d %+v", f.count(addrs[1]), s)
	}
	if s := state(t, db, addrs[2]); s.Status != "sending" || !claimedAt(t, db, addrs[2]).Equal(other) {
		t.Fatalf("row owned by another replica was released: %+v", s)
	}
}

// H69: a send that hangs is cut by the per-message deadline and recorded as a
// failed attempt, so the claim cannot outlive staleAfter while still sending.
func TestFlush_HungSenderCutByDeadline(t *testing.T) {
	db := dbtest.Open(t)
	addr := enqueue(t, db, 1)[0]
	f := newFake()
	f.delay = time.Hour // a server that never answers
	w := testWorker(db, f)
	w.sendTimeout = 100 * time.Millisecond
	start := time.Now()
	w.flush(context.Background())
	if el := time.Since(start); el > 5*time.Second {
		t.Fatalf("flush took %v", el)
	}
	if s := state(t, db, addr); s.Status != "failed" || s.RetryCount != 1 || s.LastError == nil {
		t.Fatalf("hung send not recorded as failure: %+v", s)
	}
}

// H69: the send deadline is always shorter than the stale timeout.
func TestNewEmailWorker_SendTimeoutBelowStale(t *testing.T) {
	w := NewEmailWorker(nil, "localhost", 25, "", "", "a@b.c")
	if w.sendTimeout <= 0 || w.sendTimeout >= w.staleAfter {
		t.Fatalf("sendTimeout %v must be in (0, staleAfter=%v)", w.sendTimeout, w.staleAfter)
	}
}

// H69: the real SMTP sender is bounded by the context deadline even when the
// server accepts the TCP connection and then never speaks.
func TestSMTPSender_SilentServerTimesOut(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	var mu sync.Mutex
	var conns []net.Conn
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, c) // hold it open, say nothing
			mu.Unlock()
		}
	}()
	defer func() {
		mu.Lock()
		defer mu.Unlock()
		for _, c := range conns {
			c.Close()
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port
	s := newSMTPSender("127.0.0.1", port, "", "", "from@test.local")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := s.Send(ctx, "to@test.local", "s", "<p>b</p>"); err == nil {
		t.Fatal("send to a silent server succeeded")
	}
	if el := time.Since(start); el > 3*time.Second {
		t.Fatalf("send blocked for %v despite a 200ms deadline", el)
	}
	if err := s.Send(context.Background(), "to@test.local", "s", "b"); err == nil {
		t.Fatal("send without deadline must be refused")
	}
}

// H131: the claim of every row still waiting in the batch is renewed before
// each send, not only the claim of the row about to be sent. Otherwise the
// tail of a long batch (batchSize·sendTimeout can exceed staleAfter) goes
// stale while its owner is busy with earlier rows, another replica reclaims
// it with retry_count+1 although it was never attempted, and after maxRetries
// such reclaims the message ends 'failed' without a single delivery attempt.
//
// Real time, scaled down: staleAfter = 1s and each send of replica A takes
// 450ms, so the 4th row of A's batch waits ~1.35s before its turn; replica B
// polls the queue meanwhile.
func TestFlush_LongBatchTailIsNotReclaimed(t *testing.T) {
	db := dbtest.Open(t)
	addrs := enqueue(t, db, 4)
	mine := map[string]bool{}
	for _, a := range addrs {
		mine[a] = true
	}

	slow := newFake()
	slow.hook = func(_ context.Context, to string) error {
		if mine[to] {
			time.Sleep(450 * time.Millisecond)
		}
		return nil
	}
	a := testWorker(db, slow)
	a.staleAfter = time.Second
	a.sendTimeout = 5 * time.Second
	rows, err := a.claim(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	fast := newFake()
	b := testWorker(db, fast)
	b.staleAfter = time.Second
	stop := make(chan struct{})
	bDone := make(chan struct{})
	go func() {
		defer close(bDone)
		for {
			select {
			case <-stop:
				return
			case <-time.After(50 * time.Millisecond):
				b.flush(context.Background())
			}
		}
	}()
	a.process(context.Background(), rows)
	close(stop)
	<-bDone

	for _, addr := range addrs {
		s := state(t, db, addr)
		if fast.count(addr) != 0 || slow.count(addr) != 1 || s.Status != "sent" || s.RetryCount != 0 {
			t.Errorf("%s: A sent %d, B sent %d, %+v; want sent once by its owner A with retry_count 0",
				addr, slow.count(addr), fast.count(addr), s)
		}
	}
}
