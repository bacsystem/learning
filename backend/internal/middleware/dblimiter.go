package middleware

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"
)

// DBLimiter is a fixed-window counter stored in Postgres (table
// auth_intentos_fallidos, migration 056), so the quota is shared by every
// replica and survives restarts. Use it to count only some outcomes (e.g.
// failed attempts): TryAcquire reserves one unit atomically before the
// attempt and Release refunds it when the outcome should not count.
//
// Windows are fixed and aligned to the Postgres clock (ventana_inicio =
// NOW() truncated to a multiple of the window), the same for all replicas
// (tests can fix the instant with ConReloj).
// As with any fixed window, up to 2×limit events can fit around a boundary;
// that is accepted in exchange for a single-statement atomic reservation.
//
// Keys are stored as given: callers must pass derived values (HMACs), never
// raw PII such as a DNI or an IP address.
type DBLimiter struct {
	db     *sql.DB
	prefix string
	limit  int
	window time.Duration

	// ahora, when set, gives the instant whose window counts (tests only,
	// see ConReloj); nil uses now() of Postgres.
	ahora func() time.Time

	sweepMu   sync.Mutex
	lastSweep time.Time
}

// dbLimiterSweepEvery bounds how often one process deletes expired windows.
const dbLimiterSweepEvery = time.Minute

// dbLimiterSweepMargin is how long an expired window is kept before the
// sweep deletes it. It is a constant so the sweep can use the expira_en index;
// it must stay above the test window (ventanaPrueba, 15 min), so a window that
// a test still counts is never swept.
const dbLimiterSweepMargin = time.Hour

// NewDBLimiter returns a limiter allowing limit events per window per key.
// prefix namespaces the keys of this limiter (several share the table).
func NewDBLimiter(db *sql.DB, prefix string, limit int, window time.Duration) *DBLimiter {
	return &DBLimiter{db: db, prefix: prefix, limit: limit, window: window}
}

// ConReloj makes the limiter compute its window from ahora instead of now() of
// Postgres, and returns l. Only for tests: in production the window comes from
// now() of Postgres so that every replica shares one clock. A test fixes ahora
// at its real start instant, so requests that cross a window boundary while it
// runs still count in one window. Release uses the reserved window, and the
// sweep keeps every row dbLimiterSweepMargin after it expires, so a test that
// crosses a real boundary does not lose its fixed window.
func (l *DBLimiter) ConReloj(ahora func() time.Time) *DBLimiter {
	l.ahora = ahora
	return l
}

// instante is the window instant for the SQL: nil (NULL → now() of Postgres)
// unless a test clock is set.
func (l *DBLimiter) instante() any {
	if l.ahora == nil {
		return nil
	}
	return l.ahora()
}

// Reservation identifies the unit taken by TryAcquire so that Release refunds
// the same window even if a new one started in between.
type Reservation struct {
	key    string
	window time.Time
	count  int
}

// Count is the key's count in the window right after this reservation, read
// atomically by the reserving statement (1 = first unit of the window). Use
// it instead of a separate Count call, which would race with other callers.
func (r Reservation) Count() int { return r.count }

// TryAcquire atomically reserves one unit of key's quota in the current
// window. It returns ok=false (counting nothing) when the quota is used up.
// The INSERT … ON CONFLICT DO UPDATE … WHERE intentos < limit is one
// statement: the row lock serializes concurrent callers, so the limit cannot
// be exceeded by racing requests, across processes too.
func (l *DBLimiter) TryAcquire(ctx context.Context, key string) (res Reservation, ok bool, err error) {
	l.maybeSweep(ctx)
	return l.tryAcquire(ctx, l.db, key)
}

// TryAcquireTx is TryAcquire inside tx: the unit counts only if tx commits,
// and the key's row stays locked until tx ends, so a concurrent reservation
// of the same key waits for this transaction's outcome instead of counting
// on top of a unit that may still be rolled back.
//
// It does not sweep: sweeping needs a second pool connection, and a caller
// holding tx (and row locks other transactions wait on) must never block on
// the pool. Call MaybeSweep before opening tx.
func (l *DBLimiter) TryAcquireTx(ctx context.Context, tx *sql.Tx, key string) (res Reservation, ok bool, err error) {
	return l.tryAcquire(ctx, tx, key)
}

// MaybeSweep runs the lazy sweep TryAcquire does; for TryAcquireTx callers.
func (l *DBLimiter) MaybeSweep(ctx context.Context) { l.maybeSweep(ctx) }

type rowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (l *DBLimiter) tryAcquire(ctx context.Context, q rowQuerier, key string) (res Reservation, ok bool, err error) {
	res.key = l.prefix + key
	err = q.QueryRowContext(ctx, `
		WITH w AS (SELECT to_timestamp(floor(extract(epoch FROM COALESCE($4::timestamptz, now())) / $2::float8) * $2::float8) AS inicio)
		INSERT INTO auth_intentos_fallidos (clave, ventana_inicio, intentos, expira_en)
		SELECT $1, w.inicio, 1, w.inicio + make_interval(secs => $2::float8) FROM w
		ON CONFLICT (clave, ventana_inicio) DO UPDATE
			SET intentos = auth_intentos_fallidos.intentos + 1
			WHERE auth_intentos_fallidos.intentos < $3
		RETURNING ventana_inicio, intentos`,
		res.key, l.window.Seconds(), l.limit, l.instante()).Scan(&res.window, &res.count)
	if errors.Is(err, sql.ErrNoRows) {
		return Reservation{}, false, nil
	}
	if err != nil {
		return Reservation{}, false, err
	}
	return res, true, nil
}

// Release refunds a unit reserved with TryAcquire (no-op for a zero
// Reservation). It is detached from ctx cancellation: a client that hangs up
// must not leave its unit counted.
func (l *DBLimiter) Release(ctx context.Context, res Reservation) error {
	if res.key == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, err := l.db.ExecContext(ctx, `
		UPDATE auth_intentos_fallidos SET intentos = intentos - 1
		WHERE clave = $1 AND ventana_inicio = $2 AND intentos > 0`, res.key, res.window)
	return err
}

// Count returns how many units of key are used in the current window.
func (l *DBLimiter) Count(ctx context.Context, key string) (int, error) {
	var n int
	err := l.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(intentos), 0) FROM auth_intentos_fallidos
		WHERE clave = $1 AND ventana_inicio = to_timestamp(floor(extract(epoch FROM COALESCE($3::timestamptz, now())) / $2::float8) * $2::float8)`,
		l.prefix+key, l.window.Seconds(), l.instante()).Scan(&n)
	return n, err
}

// maybeSweep deletes windows of every limiter expired more than dbLimiterSweepMargin ago, at most once per
// dbLimiterSweepEvery per process (lazy cleanup, no cron). Errors are ignored:
// stale rows never affect correctness, only table size.
func (l *DBLimiter) maybeSweep(ctx context.Context) {
	l.sweepMu.Lock()
	if time.Since(l.lastSweep) < dbLimiterSweepEvery {
		l.sweepMu.Unlock()
		return
	}
	l.lastSweep = time.Now()
	l.sweepMu.Unlock()
	// Rows are kept dbLimiterSweepMargin after they expire. Production never
	// reads an expired window, so the margin only delays the cleanup; it keeps
	// a test whose clock is fixed (ConReloj) from losing its window when the
	// test crosses a real boundary and any limiter sweeps (#139). The bound
	// is a constant on expira_en so idx_auth_intentos_fallidos_expira is used.
	_, _ = l.db.ExecContext(ctx, `
		DELETE FROM auth_intentos_fallidos WHERE expira_en < now() - make_interval(secs => $1)`,
		dbLimiterSweepMargin.Seconds())
}
