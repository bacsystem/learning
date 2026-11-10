package notifier

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/rs/zerolog/log"
)

const (
	// maxRetries is the number of delivery attempts before a message is left
	// as 'failed' for good.
	maxRetries = 5
	// backoffBase/backoffMax: next_retry_at = now + min(base·2^attempt, max).
	backoffBase = 30 * time.Second
	backoffMax  = time.Hour
	// maxErrLen truncates last_error.
	maxErrLen = 500
)

// Sender delivers one email. It is an interface so tests need no SMTP server.
// Implementations must honour ctx: the worker gives every message a deadline
// shorter than staleAfter (H69).
type Sender interface {
	Send(ctx context.Context, to, subject, htmlBody string) error
}

// EmailWorker drains email_queue.
//
// Delivery is at-least-once and safe with several replicas: a batch is claimed
// atomically (status 'sending' + claimed_at, FOR UPDATE SKIP LOCKED), so two
// workers never pick the same row. A claim whose result could not be recorded
// (crash, DB error) is reclaimed after staleAfter and counts as an attempt, so
// a message is never resent in a tight loop and gives up after maxRetries.
//
// A replica that is slow on SMTP must not keep sending rows another replica
// already reclaimed (H69): right before each Send the claims of that row and
// of the rest of the batch are renewed with a compare-and-set on claimed_at
// (H131) and the row is skipped if it was lost, and every Send runs under
// sendTimeout (< staleAfter). So no claim of a live worker ages more than one
// sendTimeout plus bookkeeping, whatever the batch size.
type EmailWorker struct {
	db         *sqlx.DB
	sender     Sender
	interval   time.Duration
	batchSize  int
	staleAfter time.Duration
	opTimeout  time.Duration
	// sendTimeout bounds one SMTP delivery end to end; it must stay below
	// staleAfter.
	sendTimeout time.Duration
}

func NewEmailWorker(db *sqlx.DB, host string, port int, user, password, from string) *EmailWorker {
	return newEmailWorker(db, newSMTPSender(host, port, user, password, from))
}

func newEmailWorker(db *sqlx.DB, s Sender) *EmailWorker {
	return &EmailWorker{
		db:          db,
		sender:      s,
		interval:    10 * time.Second,
		batchSize:   20,
		staleAfter:  10 * time.Minute,
		opTimeout:   10 * time.Second,
		sendTimeout: 2 * time.Minute,
	}
}

// Run polls the queue every interval until ctx is cancelled. A batch in
// progress when ctx is cancelled is finished message-by-message: the message
// being sent completes and is recorded, the rest of the claimed batch is
// released back to the queue, and only then does Run return.
func (w *EmailWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.flush(ctx)
		}
	}
}

type emailRow struct {
	ID        int64     `db:"id"`
	ToAddr    string    `db:"to_address"`
	Subject   string    `db:"subject"`
	BodyHTML  string    `db:"body_html"`
	Retry     int       `db:"retry_count"`
	ClaimedAt time.Time `db:"claimed_at"`
}

// dbCtx gives bookkeeping queries a deadline that does not depend on the
// worker's lifetime context, so results are still recorded during shutdown.
func (w *EmailWorker) dbCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), w.opTimeout)
}

func (w *EmailWorker) flush(ctx context.Context) {
	defer func() {
		if p := recover(); p != nil {
			log.Error().Interface("panic", p).Msg("email_worker: flush panicked")
		}
	}()
	rows, err := w.claim(ctx)
	if err != nil {
		log.Error().Err(err).Msg("email_worker: claim failed")
		return
	}
	w.process(ctx, rows)
}

// process sends a claimed batch serially. Right before each send the claims of
// the row about to be sent AND of every row still waiting in the batch are
// renewed (compare-and-set on claimed_at): the tail of a long batch must not go
// stale while this worker is busy with earlier rows (H131), or another replica
// would reclaim it with retry_count+1 without it ever being attempted. Rows
// whose claim was lost are skipped. On shutdown the unsent rest of the batch
// is released.
func (w *EmailWorker) process(ctx context.Context, rows []emailRow) {
	for i := range rows {
		if ctx.Err() != nil {
			w.release(ctx, rows[i:])
			return
		}
		if !w.renew(ctx, rows[i:]) {
			continue
		}
		row := rows[i]
		if err := w.sendSafe(ctx, row); err != nil {
			w.markFailed(ctx, row, err)
			continue
		}
		w.markSent(ctx, row)
	}
}

// claim abandons exhausted stale claims and atomically claims a batch of due
// rows (pending/failed whose next_retry_at passed, or stale 'sending').
func (w *EmailWorker) claim(ctx context.Context) ([]emailRow, error) {
	staleSecs := int64(w.staleAfter / time.Second)
	if _, err := w.db.ExecContext(ctx, `
		UPDATE email_queue
		SET status='failed', retry_count=retry_count+1, claimed_at=NULL, updated_at=NOW(),
		    last_error='claim expired: delivery result unknown, retries exhausted'
		WHERE status='sending' AND claimed_at < NOW() - $1 * INTERVAL '1 second'
		  AND retry_count >= $2 - 1`, staleSecs, maxRetries); err != nil {
		return nil, fmt.Errorf("abandon stale: %w", err)
	}
	var rows []emailRow
	err := w.db.SelectContext(ctx, &rows, `
		UPDATE email_queue q
		SET status='sending', claimed_at=clock_timestamp(), updated_at=NOW(),
		    retry_count = CASE WHEN q.status='sending' THEN q.retry_count+1 ELSE q.retry_count END
		FROM (
			SELECT id FROM email_queue
			WHERE (status IN ('pending','failed') AND retry_count < $1 AND next_retry_at <= NOW())
			   OR (status='sending' AND claimed_at < NOW() - $2 * INTERVAL '1 second')
			ORDER BY created_at, id
			LIMIT $3
			FOR UPDATE SKIP LOCKED
		) c
		WHERE q.id = c.id
		RETURNING q.id, q.to_address, q.subject, q.body_html, q.retry_count, q.claimed_at`,
		maxRetries, staleSecs, w.batchSize)
	return rows, err
}

// renew refreshes, in one statement, the claim of every row in rows that this
// worker still owns, updating their ClaimedAt in place, and reports whether
// rows[0] (the row about to be sent) is still owned. false means another
// replica reclaimed it, or ownership could not be checked: it must not be
// sent. Rows of the tail whose claim was lost keep their old ClaimedAt, so
// every later compare-and-set on them (renew, mark*, release) is a no-op.
func (w *EmailWorker) renew(ctx context.Context, rows []emailRow) bool {
	ids := make([]int64, len(rows))
	claims := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
		claims[i] = r.ClaimedAt.UTC().Format(time.RFC3339Nano)
	}
	c, cancel := w.dbCtx(ctx)
	defer cancel()
	var renewed []struct {
		ID        int64     `db:"id"`
		ClaimedAt time.Time `db:"claimed_at"`
	}
	if err := w.db.SelectContext(c, &renewed, `
		UPDATE email_queue q SET claimed_at=clock_timestamp(), updated_at=NOW()
		FROM unnest($1::bigint[], $2::timestamptz[]) AS c(id, claimed_at)
		WHERE q.id = c.id AND q.status='sending' AND q.claimed_at = c.claimed_at
		RETURNING q.id, q.claimed_at`, pq.Array(ids), pq.Array(claims)); err != nil {
		// Unknown ownership: do not send; the row is retried after staleAfter.
		log.Error().Err(err).Int64("id", rows[0].ID).Msg("email_worker: failed to renew claim")
		return false
	}
	byID := make(map[int64]time.Time, len(renewed))
	for _, r := range renewed {
		byID[r.ID] = r.ClaimedAt
	}
	for i := range rows {
		if t, ok := byID[rows[i].ID]; ok {
			rows[i].ClaimedAt = t
		}
	}
	if _, ok := byID[rows[0].ID]; !ok {
		log.Warn().Int64("id", rows[0].ID).Msg("email_worker: claim lost before sending, skipping")
		return false
	}
	return true
}

// sendSafe sends one message under sendTimeout, turning a panic into an
// error. The deadline is detached from the worker's lifetime ctx so a message
// in flight at shutdown still completes (bounded by sendTimeout).
func (w *EmailWorker) sendSafe(ctx context.Context, row emailRow) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic while sending: %v", p)
		}
	}()
	c, cancel := context.WithTimeout(context.WithoutCancel(ctx), w.sendTimeout)
	defer cancel()
	return w.sender.Send(c, row.ToAddr, row.Subject, row.BodyHTML)
}

// markSent records a delivery. The claim timestamp guards ownership: if the
// claim was reclaimed meanwhile, nothing is overwritten.
func (w *EmailWorker) markSent(ctx context.Context, row emailRow) {
	c, cancel := w.dbCtx(ctx)
	defer cancel()
	res, err := w.db.ExecContext(c, `
		UPDATE email_queue SET status='sent', claimed_at=NULL, last_error=NULL, updated_at=NOW()
		WHERE id=$1 AND status='sending' AND claimed_at=$2`, row.ID, row.ClaimedAt)
	if err != nil {
		// The row stays 'sending' and is retried after staleAfter (counted).
		log.Error().Err(err).Int64("id", row.ID).Msg("email_worker: failed to mark sent")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		log.Warn().Int64("id", row.ID).Msg("email_worker: claim lost before marking sent")
	}
}

// markFailed records a failed attempt with exponential backoff. The SMTP error
// goes to last_error only (it may contain the address); logs carry no PII.
func (w *EmailWorker) markFailed(ctx context.Context, row emailRow, sendErr error) {
	msg := sendErr.Error()
	if len(msg) > maxErrLen {
		msg = msg[:maxErrLen]
	}
	backoff := min(backoffBase<<min(row.Retry, 16), backoffMax)
	log.Warn().Int64("id", row.ID).Int("attempt", row.Retry+1).Dur("backoff", backoff).
		Msg("email_worker: send failed (see email_queue.last_error)")
	c, cancel := w.dbCtx(ctx)
	defer cancel()
	if _, err := w.db.ExecContext(c, `
		UPDATE email_queue
		SET status='failed', retry_count=retry_count+1, last_error=$3, claimed_at=NULL,
		    next_retry_at=NOW() + $4 * INTERVAL '1 second', updated_at=NOW()
		WHERE id=$1 AND status='sending' AND claimed_at=$2`,
		row.ID, row.ClaimedAt, msg, int64(backoff/time.Second)); err != nil {
		log.Error().Err(err).Int64("id", row.ID).Msg("email_worker: failed to record send failure")
	}
}

// release hands claimed-but-unsent rows back to the queue (shutdown). Only
// rows whose claim is still ours are released.
func (w *EmailWorker) release(ctx context.Context, rows []emailRow) {
	ids := make([]int64, len(rows))
	claims := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
		claims[i] = r.ClaimedAt.UTC().Format(time.RFC3339Nano)
	}
	c, cancel := w.dbCtx(ctx)
	defer cancel()
	if _, err := w.db.ExecContext(c, `
		UPDATE email_queue q SET status='pending', claimed_at=NULL, updated_at=NOW()
		FROM unnest($1::bigint[], $2::timestamptz[]) AS c(id, claimed_at)
		WHERE q.id = c.id AND q.status='sending' AND q.claimed_at = c.claimed_at`,
		pq.Array(ids), pq.Array(claims)); err != nil {
		log.Error().Err(err).Int("count", len(ids)).Msg("email_worker: failed to release claims")
	}
}
