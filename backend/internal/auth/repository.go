package auth

import (
	"context"
	"crypto/cipher"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/crypto"
	internalmw "github.com/cbaciliod/learning/internal/middleware"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/rs/zerolog/log"
)

type Repository struct {
	db     *sqlx.DB
	gcm    cipher.AEAD
	rawKey []byte

	// afterAdminLock is a test-only seam, nil in production: UpdateEstado
	// calls it right after locking the active admin rows, so a test can hold
	// one transaction there and prove a concurrent one waits on the lock.
	afterAdminLock func()
	// afterPendingLock is a test-only seam, nil in production:
	// VerifyPendingRegistration calls it right after locking the pending
	// registration row (step 1), so a test can hold it there.
	afterPendingLock func()
}

func NewRepository(db *sqlx.DB, gcm cipher.AEAD, rawKey []byte) *Repository {
	return &Repository{db: db, gcm: gcm, rawKey: rawKey}
}

// ── helpers ─────────────────────────────────────────────────────────────────

func (r *Repository) encrypt(s string) (string, error) { return crypto.Encrypt(r.gcm, s) }
func (r *Repository) decrypt(s string) (string, error) { return crypto.Decrypt(r.gcm, s) }
func (r *Repository) dniHMAC(plainDNI string) string   { return crypto.HMAC(r.rawKey, plainDNI) }

// ipHMAC keys per-IP limiter counters without storing the IP in clear. The
// IP is first normalized with middleware.LimiterIP (IPv6 by /64), so every
// limiter keyed by IP shares one granularity. The "ip:" domain prefix keeps
// it from colliding with DNI HMACs. Only for limiter keys: audit rows keep
// the real IP.
func (r *Repository) ipHMAC(ip string) string {
	return crypto.HMAC(r.rawKey, "ip:"+internalmw.LimiterIP(ip))
}

// NormalizeEmail is the canonical form used for storage and for email_hmac:
// trimmed and lower-cased.
func NormalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// emailHMAC returns the lookup HMAC of the NORMALIZED email.
func (r *Repository) emailHMAC(plainEmail string) string {
	return crypto.HMAC(r.rawKey, NormalizeEmail(plainEmail))
}

// rawEmailHMAC hashes the email exactly as given. Rows written before e-mail
// normalization (see cmd/rehash-emails) were hashed this way.
func (r *Repository) rawEmailHMAC(plainEmail string) string { return crypto.HMAC(r.rawKey, plainEmail) }

// emailHMACCandidates returns the HMACs an e-mail may be stored under: the
// normalized one and, until cmd/rehash-emails has been applied, the legacy
// un-normalized one (trimmed input as typed).
func (r *Repository) emailHMACCandidates(email string) []string {
	norm := r.emailHMAC(email)
	legacy := r.rawEmailHMAC(strings.TrimSpace(email))
	if legacy == norm {
		return []string{norm}
	}
	return []string{norm, legacy}
}

// emailOnFile decrypts the e-mail column of a persona and returns it trimmed;
// "" means "no e-mail on file". A persona without e-mail is stored either
// with an empty column or, as cmd/seed and the CSV import write it, as
// Encrypt("") (with email_hmac = HMAC(""), shared by all of them): the
// decision is always taken on the DECRYPTED value, never on the ciphertext or
// on email_hmac. A value that cannot be decrypted is an error: treating it as
// "no e-mail" would let anyone register that DNI with an arbitrary address.
func (r *Repository) emailOnFile(enc string) (string, error) {
	if enc == "" {
		return "", nil
	}
	plain, err := r.decrypt(enc)
	if err != nil {
		return "", fmt.Errorf("decrypt persona email: %w", err)
	}
	return strings.TrimSpace(plain), nil
}

// isUniqueViolation reports whether err is a Postgres unique violation on the
// given constraint ("" matches any constraint).
func isUniqueViolation(err error, constraint string) bool {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) || pqErr.Code != "23505" {
		return false
	}
	return constraint == "" || pqErr.Constraint == constraint
}

// maskDNI returns the last 4 digits of a DNI with leading asterisks, e.g. "****5678".
func maskDNI(dni string) string {
	if len(dni) <= 4 {
		return dni
	}
	return "****" + dni[len(dni)-4:]
}

// ── Personas ─────────────────────────────────────────────────────────────────

type personaRow struct {
	ID             int64  `db:"id"`
	DNI            string `db:"dni"`
	DNIHMAC        string `db:"dni_hmac"`
	Nombres        string `db:"nombres"`
	ApellidoP      string `db:"apellido_paterno"`
	ApellidoM      string `db:"apellido_materno"`
	GradoID        int64  `db:"grado_id"`
	EspecialidadID int64  `db:"especialidad_id"`
	InstitucionID  int64  `db:"institucion_id"`
	Telefono       string `db:"telefono"`
	Email          string `db:"email"`
	EmailHMAC      string `db:"email_hmac"`
}

// FindPersonaByDNI looks up a persona by plaintext DNI using the deterministic HMAC index.
func (r *Repository) FindPersonaByDNI(ctx context.Context, dni string) (*personaRow, error) {
	var row personaRow
	err := r.db.GetContext(ctx, &row, `
		SELECT id, dni, dni_hmac, nombres, apellido_paterno, apellido_materno,
		       grado_id, especialidad_id, institucion_id, telefono, email
		FROM personas WHERE dni_hmac = $1`,
		r.dniHMAC(dni))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, sql.ErrNoRows
	}
	return &row, err
}

// BatchInsert inserts multiple personas, skipping DNI duplicates via dni_hmac,
// in one transaction: a row that fails is rolled back to its savepoint and
// reported in errs, the others are kept. auditFn (nil: no audit) builds the
// audit row from the counts; it is written in the same transaction, and its
// failure rolls the whole batch back (err).
func (r *Repository) BatchInsert(ctx context.Context, items []BatchPersonaItem, auditFn func(inserted, duplicates int) AuditEntry) (inserted, duplicates int, errs []string, err error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, 0, nil, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit
	for _, item := range items {
		dniHMAC := r.dniHMAC(item.DNI)
		encDNI, _ := r.encrypt(item.DNI)
		encNombres, _ := r.encrypt(item.Nombres)
		encAP, _ := r.encrypt(item.ApellidoPaterno)
		encAM, _ := r.encrypt(item.ApellidoMaterno)
		encTel, _ := r.encrypt(item.Telefono)
		encEmail, _ := r.encrypt(NormalizeEmail(item.Email))
		emailHMAC := r.emailHMAC(item.Email)

		if _, err := tx.ExecContext(ctx, `SAVEPOINT batch_item`); err != nil {
			return 0, 0, nil, err
		}
		res, err := tx.ExecContext(ctx, `
			INSERT INTO personas (dni,dni_hmac,nombres,apellido_paterno,apellido_materno,
				grado_id,especialidad_id,institucion_id,telefono,email,email_hmac)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT (dni_hmac) DO NOTHING`,
			encDNI, dniHMAC, encNombres, encAP, encAM,
			item.GradoID, item.EspecialidadID, item.InstitucionID, encTel, encEmail, emailHMAC,
		)
		if err != nil {
			errs = append(errs, fmt.Sprintf("DNI %s: %v", item.DNI, err))
			if _, err := tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT batch_item`); err != nil {
				return 0, 0, nil, err
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `RELEASE SAVEPOINT batch_item`); err != nil {
			return 0, 0, nil, err
		}
		n, _ := res.RowsAffected()
		if n > 0 {
			inserted++
		} else {
			duplicates++
		}
	}
	if auditFn != nil {
		if err := r.auditTx(ctx, tx, auditFn(inserted, duplicates)); err != nil {
			return 0, 0, nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, nil, err
	}
	return inserted, duplicates, errs, nil
}

// ── Usuarios ─────────────────────────────────────────────────────────────────

type usuarioRow struct {
	ID             int64         `db:"id"`
	PersonaID      int64         `db:"persona_id"`
	Username       string        `db:"username"`
	PasswordHash   string        `db:"password_hash"`
	Rol            string        `db:"rol"`
	Estado         string        `db:"estado"`
	LoginIntentos  int           `db:"login_intentos"`
	BloqueadoHasta sql.NullTime  `db:"bloqueado_hasta"`
	ActivatedAt    sql.NullTime  `db:"activated_at"`
	ActivatedBy    sql.NullInt64 `db:"activated_by"`
	TourCompletado bool          `db:"tour_completado"`
	CreatedAt      time.Time     `db:"created_at"`
	UpdatedAt      time.Time     `db:"updated_at"`
}

// ── Registros pendientes (verificación de e-mail) ───────────────────────────

// PendingRegistration is what POST /auth/register stores until the e-mailed
// link is used. It holds NO credentials: username and password are chosen at
// verification time. Email is the (plaintext, normalized) address the link
// was sent to; it is stored encrypted.
type PendingRegistration struct {
	PersonaID int64
	Email     string
	TokenHash string
	ExpiresAt time.Time
}

// SavePendingRegistration creates or REPLACES the persona's unconsumed pending
// registration and enqueues the verification e-mail, in one transaction.
// The upsert on the partial unique index (persona_id WHERE consumed_at IS
// NULL) overwrites the previous token, so a newer request always invalidates
// older ones, also under concurrency. Expired rows of the persona are deleted
// first (lazy cleanup, no cron).
func (r *Repository) SavePendingRegistration(ctx context.Context, p PendingRegistration, subject, bodyHTML string) error {
	norm := NormalizeEmail(p.Email)
	encEmail, err := r.encrypt(norm)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM registros_pendientes WHERE persona_id=$1 AND expires_at <= NOW()`, p.PersonaID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO registros_pendientes (persona_id,email,email_hmac,token_hash,expires_at)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (persona_id) WHERE consumed_at IS NULL DO UPDATE SET
			email=EXCLUDED.email, email_hmac=EXCLUDED.email_hmac,
			token_hash=EXCLUDED.token_hash, expires_at=EXCLUDED.expires_at, created_at=NOW()`,
		p.PersonaID, encEmail, r.emailHMAC(norm), p.TokenHash, p.ExpiresAt); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO email_queue (to_address,subject,body_html) VALUES ($1,$2,$3)`, p.Email, subject, bodyHTML); err != nil {
		return err
	}
	return tx.Commit()
}

// EmailClaimedByOther reports whether email (normalized or legacy HMAC)
// already belongs to a persona other than personaID. Registering it for a
// persona with no e-mail on file would make two personas share it, and
// forgot-password refuses ambiguous e-mails, silently breaking the owner's
// recovery. Only a pre-check: VerifyPendingRegistration re-checks under a
// lock. Pending registrations are NOT considered (they are unverified and
// must not be able to squat an address).
func (r *Repository) EmailClaimedByOther(ctx context.Context, email string, personaID int64) (bool, error) {
	var claimed bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM personas WHERE email_hmac = ANY($1) AND id <> $2)`,
		pq.Array(r.emailHMACCandidates(email)), personaID).Scan(&claimed)
	return claimed, err
}

// VerifiedRegistration is what VerifyPendingRegistration created.
type VerifiedRegistration struct {
	UserID    int64
	PersonaID int64
}

// VerifyPendingRegistration creates the usuario (estado 'pendiente') with the
// credentials chosen now, for the pending registration of tokenHash, and
// consumes it — all in one transaction:
//
//  1. SELECT … FOR UPDATE the live (unconsumed, unexpired) row: concurrent
//     verifications of the same token queue on the row lock, and once the
//     winner commits the row no longer matches, so they get
//     ErrInvalidVerifyToken.
//  2. Re-check the e-mail under pg_advisory_xact_lock(hashtext(email_hmac)),
//     which serializes verifications carrying the same address: it must not
//     belong to another persona (when it will be written) nor to another
//     persona's account. If the persona has an e-mail on file it must still
//     be the one the link was sent to.
//  3. INSERT the usuario, write the persona e-mail if it had none, and mark
//     the row consumed.
//
// Any error rolls everything back, so a failed attempt (e.g.
// ErrUsernameTaken) leaves the token usable. Errors: ErrInvalidVerifyToken
// (unknown/used/expired token, the persona already has a usuario or its
// e-mail changed), ErrUsernameTaken, ErrEmailTaken.
func (r *Repository) VerifyPendingRegistration(ctx context.Context, tokenHash, username, passwordHash string) (VerifiedRegistration, error) {
	var out VerifiedRegistration
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	var pendingID int64
	var encPending string
	err = tx.QueryRowContext(ctx, `
		SELECT id, persona_id, email FROM registros_pendientes
		WHERE token_hash=$1 AND consumed_at IS NULL AND expires_at > NOW()
		FOR UPDATE`, tokenHash).Scan(&pendingID, &out.PersonaID, &encPending)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrInvalidVerifyToken
	}
	if err != nil {
		return out, err
	}
	if r.afterPendingLock != nil {
		r.afterPendingLock()
	}
	plain, err := r.decrypt(encPending)
	if err != nil {
		return out, fmt.Errorf("decrypt pending email: %w", err)
	}
	norm := NormalizeEmail(plain)

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, r.emailHMAC(norm)); err != nil {
		return out, err
	}
	var encOnFile string
	if err := tx.QueryRowContext(ctx, `SELECT email FROM personas WHERE id=$1 FOR UPDATE`, out.PersonaID).Scan(&encOnFile); err != nil {
		return out, err
	}
	onFile, err := r.emailOnFile(encOnFile)
	if err != nil {
		return out, err
	}
	writeEmail := onFile == ""
	if !writeEmail && NormalizeEmail(onFile) != norm {
		return out, ErrInvalidVerifyToken // the link went to an address no longer on file
	}
	// Persona-level conflict only matters when we are about to write the
	// address; an account-level conflict always does.
	var claimed bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM personas p
		WHERE p.email_hmac = ANY($1) AND p.id <> $2
		  AND ($3 OR EXISTS(SELECT 1 FROM usuarios u WHERE u.persona_id = p.id)))`,
		pq.Array(r.emailHMACCandidates(norm)), out.PersonaID, writeEmail).Scan(&claimed); err != nil {
		return out, err
	}
	if claimed {
		return out, ErrEmailTaken
	}

	err = tx.QueryRowContext(ctx,
		`INSERT INTO usuarios (persona_id,username,password_hash) VALUES ($1,$2,$3) RETURNING id`,
		out.PersonaID, username, passwordHash,
	).Scan(&out.UserID)
	switch {
	case isUniqueViolation(err, "usuarios_persona_id_key"):
		return out, ErrInvalidVerifyToken
	case isUniqueViolation(err, "usuarios_username_key"):
		return out, ErrUsernameTaken
	case err != nil:
		return out, err
	}

	if writeEmail {
		enc, err := r.encrypt(norm)
		if err != nil {
			return out, err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE personas SET email=$1, email_hmac=$2, updated_at=NOW() WHERE id=$3`,
			enc, r.emailHMAC(norm), out.PersonaID); err != nil {
			return out, err
		}
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE registros_pendientes SET consumed_at=NOW() WHERE id=$1`, pendingID); err != nil {
		return out, err
	}
	if err := tx.Commit(); err != nil {
		return out, err
	}
	return out, nil
}

func (r *Repository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM usuarios WHERE username=$1`, username).Scan(&n)
	return n > 0, err
}

// ExistsByPersonaID returns true if the persona already has a registered user account.
func (r *Repository) ExistsByPersonaID(ctx context.Context, personaID int64) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM usuarios WHERE persona_id=$1`, personaID).Scan(&n)
	return n > 0, err
}

// ExistsByEmail checks if an email is already linked to a registered user via email_hmac index.
func (r *Repository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var personaID int64
	err := r.db.QueryRowContext(ctx,
		`SELECT p.id FROM personas p JOIN usuarios u ON u.persona_id = p.id WHERE p.email_hmac = ANY($1) LIMIT 1`,
		pq.Array(r.emailHMACCandidates(email)),
	).Scan(&personaID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (r *Repository) FindByID(ctx context.Context, id int64) (*usuarioRow, error) {
	var u usuarioRow
	err := r.db.GetContext(ctx, &u, `SELECT * FROM usuarios WHERE id=$1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &u, err
}

func (r *Repository) FindByUsername(ctx context.Context, username string) (*usuarioRow, error) {
	var u usuarioRow
	err := r.db.GetContext(ctx, &u, `SELECT * FROM usuarios WHERE username=$1`, username)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &u, err
}

// RegisterFailedLogin atomically increments login_intentos and, when the new
// count reaches maxAttempts, locks the account for lockout. Concurrent
// attempts are serialized by the row lock of the UPDATE, so the threshold
// cannot be skipped by racing requests. When a previous lock has already
// expired the series starts over (this failure is the first one and the
// stale bloqueado_hasta is cleared): otherwise a single failure after every
// lock would re-lock the account forever (H129).
func (r *Repository) RegisterFailedLogin(ctx context.Context, id int64, maxAttempts int, lockout time.Duration) (attempts int, locked bool, err error) {
	// In an UPDATE every right-hand side sees the OLD row, so the new count is
	// spelled out in both assignments.
	err = r.db.QueryRowContext(ctx, `
		UPDATE usuarios SET
			login_intentos  = CASE WHEN bloqueado_hasta <= NOW() THEN 1 ELSE login_intentos + 1 END,
			bloqueado_hasta = CASE
			                    WHEN (CASE WHEN bloqueado_hasta <= NOW() THEN 1 ELSE login_intentos + 1 END) >= $2
			                      THEN NOW() + ($3 * INTERVAL '1 second')
			                    WHEN bloqueado_hasta <= NOW() THEN NULL
			                    ELSE bloqueado_hasta END,
			updated_at = NOW()
		WHERE id = $1
		RETURNING login_intentos`, id, maxAttempts, lockout.Seconds()).Scan(&attempts)
	return attempts, err == nil && attempts >= maxAttempts, err
}

func (r *Repository) ResetLoginIntentos(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE usuarios SET login_intentos=0, bloqueado_hasta=NULL, updated_at=NOW() WHERE id=$1`, id)
	return err
}

// UpdateEstado changes a usuario's estado. Moving an active admin out of
// "activo" is refused with ErrLastAdmin when no other active admin would
// remain. The check and the update run in one transaction that first locks
// every active admin row (in id order, so concurrent calls cannot deadlock):
// two admins deactivating each other at the same time are serialized and the
// second one sees the first one's change. audit (nil: no audit) is written in
// the same transaction: an audit failure rolls the change back.
func (r *Repository) UpdateEstado(ctx context.Context, id int64, estado string, adminID int64, audit *AuditEntry) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	if estado != "activo" {
		var activeAdmins []int64
		if err := tx.SelectContext(ctx, &activeAdmins,
			`SELECT id FROM usuarios WHERE rol='admin' AND estado='activo' ORDER BY id FOR UPDATE`); err != nil {
			return err
		}
		if r.afterAdminLock != nil {
			r.afterAdminLock()
		}
		targetIsActiveAdmin := false
		for _, a := range activeAdmins {
			if a == id {
				targetIsActiveAdmin = true
				break
			}
		}
		if targetIsActiveAdmin && len(activeAdmins) <= 1 {
			return ErrLastAdmin
		}
	}

	var res sql.Result
	if estado == "activo" {
		res, err = tx.ExecContext(ctx,
			`UPDATE usuarios SET estado=$1, activated_at=NOW(), activated_by=$2, updated_at=NOW() WHERE id=$3`,
			estado, adminID, id)
	} else {
		res, err = tx.ExecContext(ctx,
			`UPDATE usuarios SET estado=$1, updated_at=NOW() WHERE id=$2`, estado, id)
	}
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrUserNotFound
	}
	if audit != nil {
		if err := r.auditTx(ctx, tx, *audit); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ── Sesiones ─────────────────────────────────────────────────────────────────

func (r *Repository) InvalidateSesionByHash(ctx context.Context, tokenHash string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sesiones WHERE token_hash=$1`, tokenHash)
	return err
}

func (r *Repository) InvalidateAllSesiones(ctx context.Context, usuarioID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sesiones WHERE usuario_id=$1`, usuarioID)
	return err
}

// FindActiveSesion refreshes last_seen_at of the non-expired session for
// tokenHash and returns it with the usuario's CURRENT rol and estado. It
// returns (nil, nil) when there is no such session. It is the SessionChecker
// used by RequireAuth.
func (r *Repository) FindActiveSesion(ctx context.Context, tokenHash string) (*Session, error) {
	var s Session
	err := r.db.QueryRowContext(ctx, `
		UPDATE sesiones s SET last_seen_at=NOW()
		FROM usuarios u
		WHERE s.token_hash=$1 AND s.expires_at > NOW() AND u.id = s.usuario_id
		RETURNING u.id, u.rol, u.estado`,
		tokenHash).Scan(&s.UserID, &s.Rol, &s.Estado)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repository) CountSesionesActivas(ctx context.Context, usuarioID int64) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM sesiones WHERE usuario_id=$1 AND expires_at > NOW()`, usuarioID)
	return count, err
}

// LoginClaims are the raw ids and nivel of the JWT claims of a login.
type LoginClaims struct {
	GradoID        int64  `db:"grado_id"`
	EspecialidadID int64  `db:"especialidad_id"`
	InstitucionID  int64  `db:"institucion_id"`
	Nivel          string `db:"nivel"`
}

// LoadLoginClaims reads the claims of userID from its persona and grado.
func (r *Repository) LoadLoginClaims(ctx context.Context, userID int64) (LoginClaims, error) {
	var c LoginClaims
	err := r.db.GetContext(ctx, &c,
		`SELECT p.grado_id, p.especialidad_id, p.institucion_id, g.nivel
		 FROM personas p
		 JOIN usuarios u ON u.persona_id=p.id
		 JOIN grados g ON g.id=p.grado_id
		 WHERE u.id=$1`, userID)
	return c, err
}

// AbrirSesion stores the session of a new login and evicts the user's oldest
// sessions so that at most maxSesiones remain, in one transaction: if the
// INSERT fails, no older session has been removed.
func (r *Repository) AbrirSesion(ctx context.Context, usuarioID int64, maxSesiones int, tokenHash, ip, userAgent string, expiresAt time.Time) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin session tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM sesiones
		WHERE usuario_id=$1
		AND id NOT IN (
			SELECT id FROM sesiones
			WHERE usuario_id=$1
			ORDER BY created_at DESC
			LIMIT $2
		)`, usuarioID, maxSesiones-1); err != nil {
		return fmt.Errorf("evict sessions: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO sesiones (usuario_id,token_hash,ip,user_agent,expires_at) VALUES ($1,$2,$3,$4,$5)`,
		usuarioID, tokenHash, ip, userAgent, expiresAt); err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return tx.Commit()
}

// ── Usuarios online ──────────────────────────────────────────────────────────

type UsuarioOnlineItem struct {
	ID             int64     `db:"id"              json:"id"`
	Username       string    `db:"username"        json:"username"`
	NombreCompleto string    `db:"nombre_completo" json:"nombre_completo"`
	Rol            string    `db:"rol"             json:"rol"`
	IP             string    `db:"ip"              json:"ip"`
	LastSeenAt     time.Time `db:"last_seen_at"    json:"last_seen_at"`
}

// ListUsuariosOnline returns users with an active session seen in the last 5 minutes.
func (r *Repository) ListUsuariosOnline(ctx context.Context) ([]UsuarioOnlineItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT ON (s.usuario_id)
			u.id,
			u.username,
			p.nombres,
			p.apellido_paterno,
			u.rol,
			COALESCE(s.ip, '') AS ip,
			s.last_seen_at
		FROM sesiones s
		JOIN usuarios u ON u.id = s.usuario_id
		JOIN personas p ON p.id = u.persona_id
		WHERE s.last_seen_at > NOW() - INTERVAL '5 minutes'
		  AND s.expires_at > NOW()
		ORDER BY s.usuario_id, s.last_seen_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []UsuarioOnlineItem
	for rows.Next() {
		var item UsuarioOnlineItem
		var nombres, apellidoPaterno string
		if err := rows.Scan(&item.ID, &item.Username, &nombres, &apellidoPaterno, &item.Rol, &item.IP, &item.LastSeenAt); err != nil {
			return nil, err
		}
		n, _ := r.decrypt(nombres)
		a, _ := r.decrypt(apellidoPaterno)
		item.NombreCompleto = n + " " + a
		result = append(result, item)
	}
	if result == nil {
		result = []UsuarioOnlineItem{}
	}
	return result, rows.Err()
}

type SesionHistorialItem struct {
	IP         string    `db:"ip"          json:"ip"`
	UserAgent  string    `db:"user_agent"  json:"user_agent"`
	CreatedAt  time.Time `db:"created_at"  json:"created_at"`
	LastSeenAt time.Time `db:"last_seen_at" json:"last_seen_at"`
	ExpiresAt  time.Time `db:"expires_at"  json:"expires_at"`
	Activa     bool      `db:"activa"       json:"activa"`
}

func (r *Repository) ListSesionesUsuario(ctx context.Context, usuarioID int64) ([]SesionHistorialItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			COALESCE(ip, '') AS ip,
			COALESCE(user_agent, '') AS user_agent,
			created_at,
			last_seen_at,
			expires_at,
			(expires_at > NOW() AND last_seen_at > NOW() - INTERVAL '5 minutes') AS activa
		FROM sesiones
		WHERE usuario_id = $1
		ORDER BY last_seen_at DESC
		LIMIT 1
	`, usuarioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []SesionHistorialItem
	for rows.Next() {
		var item SesionHistorialItem
		if err := rows.Scan(&item.IP, &item.UserAgent, &item.CreatedAt, &item.LastSeenAt, &item.ExpiresAt, &item.Activa); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if result == nil {
		result = []SesionHistorialItem{}
	}
	return result, rows.Err()
}

func (r *Repository) GetMaxSesiones(ctx context.Context) (int, error) {
	var valor string
	err := r.db.GetContext(ctx, &valor,
		`SELECT valor FROM configuracion_sistema WHERE clave='max_sesiones'`)
	if errors.Is(err, sql.ErrNoRows) {
		return 2, nil // clave ausente → default
	}
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(valor))
	if err != nil {
		return 0, fmt.Errorf("configuracion_sistema.max_sesiones=%q no es un entero: %w", valor, err)
	}
	if n < 1 {
		return 1, nil
	}
	return n, nil
}

// SetMaxSesiones stores max_sesiones and writes audit (nil: none) in the same
// transaction.
func (r *Repository) SetMaxSesiones(ctx context.Context, value int, audit *AuditEntry) error {
	return r.withAuditTx(ctx, audit, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, `
		INSERT INTO configuracion_sistema (clave, valor, updated_at)
		VALUES ('max_sesiones', $1, NOW())
		ON CONFLICT (clave) DO UPDATE SET valor=EXCLUDED.valor, updated_at=NOW()`,
			fmt.Sprintf("%d", value))
		return err
	})
}

// GetRecursos devuelve el JSON crudo de la clave 'recursos'. Si no existe,
// devuelve "[]" para que el handler lo trate como lista vacía.
func (r *Repository) GetRecursos(ctx context.Context) (string, error) {
	var valor string
	err := r.db.GetContext(ctx, &valor,
		`SELECT valor FROM configuracion_sistema WHERE clave='recursos'`)
	if errors.Is(err, sql.ErrNoRows) {
		return "[]", nil // clave ausente → lista vacía
	}
	if err != nil {
		return "", err
	}
	return valor, nil
}

// SetRecursos guarda (upsert) el JSON de la lista de recursos y escribe audit
// (nil: ninguno) en la misma transacción.
func (r *Repository) SetRecursos(ctx context.Context, valor string, audit *AuditEntry) error {
	return r.withAuditTx(ctx, audit, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, `
		INSERT INTO configuracion_sistema (clave, valor, updated_at)
		VALUES ('recursos', $1, NOW())
		ON CONFLICT (clave) DO UPDATE SET valor=EXCLUDED.valor, updated_at=NOW()`,
			valor)
		return err
	})
}

// ── Tokens Recuperacion ───────────────────────────────────────────────────────

func (r *Repository) InvalidatePreviosTokens(ctx context.Context, usuarioID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE tokens_recuperacion SET usado=TRUE WHERE usuario_id=$1 AND usado=FALSE`, usuarioID)
	return err
}

func (r *Repository) CreateToken(ctx context.Context, usuarioID int64, tokenHash string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO tokens_recuperacion (usuario_id,token_hash,expires_at) VALUES ($1,$2,$3)`,
		usuarioID, tokenHash, expiresAt)
	return err
}

// ResetPasswordWithToken atomically consumes a valid (unused, unexpired)
// recovery token, sets the new password hash and revokes every session of the
// usuario, all in one transaction. The conditional UPDATE … RETURNING makes
// the token single-use even under concurrent requests. Returns ErrInvalidToken
// when the token does not exist, was already used or has expired.
func (r *Repository) ResetPasswordWithToken(ctx context.Context, tokenHash, passwordHash string) (int64, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	var userID int64
	err = tx.QueryRowContext(ctx, `
		UPDATE tokens_recuperacion SET usado=TRUE
		WHERE token_hash=$1 AND usado=FALSE AND expires_at > NOW()
		RETURNING usuario_id`, tokenHash).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrInvalidToken
	}
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE usuarios SET password_hash=$1, updated_at=NOW() WHERE id=$2`, passwordHash, userID); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sesiones WHERE usuario_id=$1`, userID); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return userID, nil
}

// ResetTokenLooksValid reports whether an unused, unexpired token exists.
// Only an optimisation: ResetPasswordWithToken re-checks atomically.
func (r *Repository) ResetTokenLooksValid(ctx context.Context, tokenHash string) (bool, error) {
	var ok bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM tokens_recuperacion WHERE token_hash=$1 AND usado=FALSE AND expires_at > NOW())`,
		tokenHash).Scan(&ok)
	return ok, err
}

// FindActiveUserIDsByEmail returns the ids of usuarios in estado "activo"
// whose persona has this e-mail (normalized or legacy HMAC), ordered by id.
func (r *Repository) FindActiveUserIDsByEmail(ctx context.Context, email string) ([]int64, error) {
	var ids []int64
	err := r.db.SelectContext(ctx, &ids, `
		SELECT u.id FROM usuarios u
		JOIN personas p ON p.id = u.persona_id
		WHERE p.email_hmac = ANY($1) AND u.estado = 'activo'
		ORDER BY u.id`, pq.Array(r.emailHMACCandidates(email)))
	return ids, err
}

// ── Audit Log ─────────────────────────────────────────────────────────────────

// likeEscaper escapes LIKE metacharacters so a prefix matches literally.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// ClearLimiterCounters deletes the DBLimiter rows (auth_intentos_fallidos)
// whose key is one of keys or starts with one of prefixes, in every window,
// and returns how many rows were deleted. Keys/prefixes are full stored keys
// (limiter prefix included) and must be derived values, never raw PII.
// auditFn (nil: no audit) builds the audit row from the count; it is written
// in the same transaction and its failure keeps the counters.
func (r *Repository) ClearLimiterCounters(ctx context.Context, keys, prefixes []string, auditFn func(deleted int64) AuditEntry) (int64, error) {
	patterns := make([]string, len(prefixes))
	for i, p := range prefixes {
		patterns[i] = likeEscaper.Replace(p) + "%"
	}
	var n int64
	err := r.withAuditTx(ctx, nil, func(tx *sqlx.Tx) error {
		res, err := tx.ExecContext(ctx,
			`DELETE FROM auth_intentos_fallidos WHERE clave = ANY($1) OR clave LIKE ANY($2)`,
			pq.Array(keys), pq.Array(patterns))
		if err != nil {
			return err
		}
		if n, err = res.RowsAffected(); err != nil {
			return err
		}
		if auditFn != nil {
			return r.auditTx(ctx, tx, auditFn(n))
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}

type AuditEntry struct {
	UsuarioID *int64
	Accion    string
	Entidad   string
	EntidadID *int64
	Detalle   []byte
	IP        string
	UserAgent string
}

// WriteAudit inserts e on its own, after the fact, and only logs a failure.
// Use it for events that are not a transactional mutation (login, logout,
// register…); admin mutations use auditTx inside their transaction.
func (r *Repository) WriteAudit(ctx context.Context, e AuditEntry) {
	// Detached from the request context (see audit.Logger.Log).
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), audit.WriteTimeout)
	defer cancel()
	if err := insertAudit(ctx, r.db, e); err != nil {
		log.Error().Err(err).Str("accion", e.Accion).Msg("audit_log write failed")
	}
}

// auditTx inserts e inside the caller's transaction and returns the error, so
// the mutation it records rolls back when it cannot be audited (H127).
func (r *Repository) auditTx(ctx context.Context, tx sqlx.ExecerContext, e AuditEntry) error {
	if err := insertAudit(ctx, tx, e); err != nil {
		return fmt.Errorf("audit_log %s/%s: %w", e.Accion, e.Entidad, err)
	}
	return nil
}

// withAuditTx runs fn and then writes audit (nil: none) in one transaction.
func (r *Repository) withAuditTx(ctx context.Context, audit *AuditEntry, fn func(tx *sqlx.Tx) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit
	if err := fn(tx); err != nil {
		return err
	}
	if audit != nil {
		if err := r.auditTx(ctx, tx, *audit); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func insertAudit(ctx context.Context, ex sqlx.ExecerContext, e AuditEntry) error {
	ip := audit.ValidIP(e.IP) // NULL for empty/invalid values (column is inet)
	// Pass true nil interface (not nil []byte inside interface) so pq sends SQL NULL for jsonb.
	var detalle interface{}
	if e.Detalle != nil {
		detalle = e.Detalle
	}
	_, err := ex.ExecContext(ctx,
		`INSERT INTO audit_log (usuario_id,accion,entidad,entidad_id,detalle,ip,user_agent)
		 VALUES ($1,$2,$3,$4,$5,$6::inet,$7)`,
		e.UsuarioID, e.Accion, e.Entidad, e.EntidadID, detalle, ip, e.UserAgent)
	return err
}

// ── Notification Queues ───────────────────────────────────────────────────────

func (r *Repository) EnqueueEmail(ctx context.Context, to, subject, bodyHTML string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO email_queue (to_address,subject,body_html) VALUES ($1,$2,$3)`,
		to, subject, bodyHTML)
	return err
}

// ListPersonas returns all personas with decrypted PII, joined with catalog names.
func (r *Repository) ListPersonas(ctx context.Context) ([]PersonaListItem, error) {
	rows, err := r.db.QueryxContext(ctx, `
		SELECT p.id, p.dni, p.nombres, p.apellido_paterno, p.apellido_materno,
		       p.email, p.telefono, p.estado,
		       p.grado_id, g.nombre AS grado,
		       p.especialidad_id, e.nombre AS especialidad,
		       p.institucion_id, i.nombre AS institucion,
		       (u.id IS NOT NULL) AS tiene_usuario
		FROM personas p
		JOIN grados        g ON g.id = p.grado_id
		JOIN especialidades e ON e.id = p.especialidad_id
		JOIN instituciones  i ON i.id = p.institucion_id
		LEFT JOIN usuarios  u ON u.persona_id = p.id
		ORDER BY p.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []PersonaListItem
	for rows.Next() {
		var row struct {
			ID             int64  `db:"id"`
			DNI            string `db:"dni"`
			Nombres        string `db:"nombres"`
			ApellidoP      string `db:"apellido_paterno"`
			ApellidoM      string `db:"apellido_materno"`
			Email          string `db:"email"`
			Telefono       string `db:"telefono"`
			Estado         string `db:"estado"`
			GradoID        int64  `db:"grado_id"`
			Grado          string `db:"grado"`
			EspecialidadID int64  `db:"especialidad_id"`
			Especialidad   string `db:"especialidad"`
			InstitucionID  int64  `db:"institucion_id"`
			Institucion    string `db:"institucion"`
			TieneUsuario   bool   `db:"tiene_usuario"`
		}
		if err := rows.StructScan(&row); err != nil {
			continue
		}
		dni, _ := r.decrypt(row.DNI)
		nombres, _ := r.decrypt(row.Nombres)
		ap, _ := r.decrypt(row.ApellidoP)
		am, _ := r.decrypt(row.ApellidoM)
		email, _ := r.decrypt(row.Email)
		telefono, _ := r.decrypt(row.Telefono)
		result = append(result, PersonaListItem{
			ID:             row.ID,
			DNI:            dni,
			NombreCompleto: nombres + " " + ap + " " + am,
			Nombres:        nombres,
			ApellidoP:      ap,
			ApellidoM:      am,
			Email:          email,
			Telefono:       telefono,
			Estado:         row.Estado,
			GradoID:        row.GradoID,
			Grado:          row.Grado,
			EspecialidadID: row.EspecialidadID,
			Especialidad:   row.Especialidad,
			InstitucionID:  row.InstitucionID,
			Institucion:    row.Institucion,
			TieneUsuario:   row.TieneUsuario,
		})
	}
	return result, rows.Err()
}

// GetPersona returns a single persona by ID with decrypted PII.
func (r *Repository) GetPersona(ctx context.Context, id int64) (*PersonaListItem, error) {
	var row struct {
		ID             int64  `db:"id"`
		DNI            string `db:"dni"`
		Nombres        string `db:"nombres"`
		ApellidoP      string `db:"apellido_paterno"`
		ApellidoM      string `db:"apellido_materno"`
		Email          string `db:"email"`
		Telefono       string `db:"telefono"`
		Estado         string `db:"estado"`
		GradoID        int64  `db:"grado_id"`
		Grado          string `db:"grado"`
		EspecialidadID int64  `db:"especialidad_id"`
		Especialidad   string `db:"especialidad"`
		InstitucionID  int64  `db:"institucion_id"`
		Institucion    string `db:"institucion"`
		TieneUsuario   bool   `db:"tiene_usuario"`
	}
	err := r.db.GetContext(ctx, &row, `
		SELECT p.id, p.dni, p.nombres, p.apellido_paterno, p.apellido_materno,
		       p.email, p.telefono, p.estado,
		       p.grado_id, g.nombre AS grado,
		       p.especialidad_id, e.nombre AS especialidad,
		       p.institucion_id, i.nombre AS institucion,
		       (u.id IS NOT NULL) AS tiene_usuario
		FROM personas p
		JOIN grados        g ON g.id = p.grado_id
		JOIN especialidades e ON e.id = p.especialidad_id
		JOIN instituciones  i ON i.id = p.institucion_id
		LEFT JOIN usuarios  u ON u.persona_id = p.id
		WHERE p.id = $1`, id)
	if err != nil {
		return nil, err
	}
	dni, _ := r.decrypt(row.DNI)
	nombres, _ := r.decrypt(row.Nombres)
	ap, _ := r.decrypt(row.ApellidoP)
	am, _ := r.decrypt(row.ApellidoM)
	email, _ := r.decrypt(row.Email)
	telefono, _ := r.decrypt(row.Telefono)
	return &PersonaListItem{
		ID:             row.ID,
		DNI:            dni,
		NombreCompleto: nombres + " " + ap + " " + am,
		Nombres:        nombres,
		ApellidoP:      ap,
		ApellidoM:      am,
		Email:          email,
		Telefono:       telefono,
		Estado:         row.Estado,
		GradoID:        row.GradoID,
		Grado:          row.Grado,
		EspecialidadID: row.EspecialidadID,
		Especialidad:   row.Especialidad,
		InstitucionID:  row.InstitucionID,
		Institucion:    row.Institucion,
		TieneUsuario:   row.TieneUsuario,
	}, nil
}

// lockEmailForPersona serializes writers of the same address (the advisory
// lock also taken by VerifyPendingRegistration) and returns ErrEmailTaken
// when norm (normalized or legacy HMAC) already belongs to a persona other
// than personaID — and with it, possibly, to that persona's account.
func (r *Repository) lockEmailForPersona(ctx context.Context, tx *sqlx.Tx, norm string, personaID int64) error {
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, r.emailHMAC(norm)); err != nil {
		return err
	}
	var claimed bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM personas WHERE email_hmac = ANY($1) AND id <> $2)`,
		pq.Array(r.emailHMACCandidates(norm)), personaID).Scan(&claimed); err != nil {
		return err
	}
	if claimed {
		return ErrEmailTaken
	}
	return nil
}

// PersonaUpdate reports what UpdatePersona changed: the names of the
// changed fields (no values) and whether one of them is carried in the JWT
// claims (grado, especialidad, institución), so the caller must revoke the
// usuario's sessions.
type PersonaUpdate struct {
	Campos        []string
	ClaimsChanged bool
	// EmailChanged: the e-mail on file changed, and with it the old recovery
	// channel was cut (see cutOldEmailChannel).
	EmailChanged bool
	// SesionesRevocadas: the sessions of the persona's usuario were revoked
	// (by the e-mail change, or because a claim changed).
	SesionesRevocadas bool
}

// lockPendingRegistrations locks the unconsumed registros_pendientes rows of
// personaID (the ones cutOldEmailChannel may delete). Admin e-mail changes
// call it before the per-address advisory lock and the personas row, the
// order VerifyPendingRegistration uses (pending row, advisory lock,
// persona): taking the persona first and deleting the pending row later
// deadlocks (40P01) against a verification of that persona (H105).
func (r *Repository) lockPendingRegistrations(ctx context.Context, tx *sqlx.Tx, personaID int64) error {
	_, err := tx.ExecContext(ctx, `SELECT id FROM registros_pendientes
		WHERE persona_id=$1 AND consumed_at IS NULL ORDER BY id FOR UPDATE`, personaID)
	return err
}

// cutOldEmailChannel runs in the transaction that replaced the e-mail of
// personaID (oldEnc is the previous, encrypted value): unused password-reset
// tokens and unconsumed pending registrations (both sent to the old address)
// are invalidated, the sessions of the persona's usuario are revoked, and the
// old address is told the recovery e-mail changed (without the new one).
func (r *Repository) cutOldEmailChannel(ctx context.Context, tx *sqlx.Tx, personaID int64, oldEnc string) error {
	for _, q := range []string{
		`UPDATE tokens_recuperacion SET usado=TRUE
		 WHERE usado=FALSE AND usuario_id IN (SELECT id FROM usuarios WHERE persona_id=$1)`,
		`DELETE FROM registros_pendientes WHERE persona_id=$1 AND consumed_at IS NULL`,
		`DELETE FROM sesiones WHERE usuario_id IN (SELECT id FROM usuarios WHERE persona_id=$1)`,
	} {
		if _, err := tx.ExecContext(ctx, q, personaID); err != nil {
			return err
		}
	}
	old, err := r.emailOnFile(oldEnc)
	if err != nil {
		// The change itself must not be blocked by an undecryptable old value.
		log.Error().Err(err).Int64("persona_id", personaID).Msg("email change: decrypt old e-mail failed; no notice sent")
		return nil
	}
	if old == "" {
		return nil // there was no old address to notify
	}
	sub, bdy := emailCorreoCambiado()
	_, err = tx.ExecContext(ctx,
		`INSERT INTO email_queue (to_address,subject,body_html) VALUES ($1,$2,$3)`, NormalizeEmail(old), sub, bdy)
	return err
}

// UpdatePersona re-encrypts and updates all mutable fields of a persona in
// one transaction: the row is locked, the e-mail must not belong to another
// persona (ErrEmailTaken, under the per-address advisory lock) and the
// changed fields are reported. When grado, especialidad or institución change
// the sessions of the persona's usuario are revoked (those values are read
// from the JWT claims). auditFn (nil: no audit) builds the audit row from the
// outcome; it is written in the same transaction and its failure rolls
// everything back. Errors: sql.ErrNoRows (unknown id), ErrEmailTaken.
func (r *Repository) UpdatePersona(ctx context.Context, id int64, req UpdatePersonaRequest, auditFn func(PersonaUpdate) AuditEntry) (PersonaUpdate, error) {
	var out PersonaUpdate
	norm := NormalizeEmail(req.Email)
	enc := map[string]string{}
	for k, v := range map[string]string{
		"nombres": req.Nombres, "apellido_paterno": req.ApellidoPaterno, "apellido_materno": req.ApellidoMaterno,
		"email": norm, "telefono": req.Telefono,
	} {
		e, err := r.encrypt(v)
		if err != nil {
			return out, err
		}
		enc[k] = e
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	// Pending registrations, then the advisory lock, then the persona row:
	// the same order as VerifyPendingRegistration (see
	// lockPendingRegistrations), so the two cannot deadlock.
	if err := r.lockPendingRegistrations(ctx, tx, id); err != nil {
		return out, err
	}
	if err := r.lockEmailForPersona(ctx, tx, norm, id); err != nil {
		return out, err
	}
	var old struct {
		Nombres        string `db:"nombres"`
		ApellidoP      string `db:"apellido_paterno"`
		ApellidoM      string `db:"apellido_materno"`
		Email          string `db:"email"`
		EmailHMAC      string `db:"email_hmac"`
		Telefono       string `db:"telefono"`
		GradoID        int64  `db:"grado_id"`
		EspecialidadID int64  `db:"especialidad_id"`
		InstitucionID  int64  `db:"institucion_id"`
		Estado         string `db:"estado"`
	}
	if err := tx.GetContext(ctx, &old, `
		SELECT nombres, apellido_paterno, apellido_materno, email, COALESCE(email_hmac,'') AS email_hmac, telefono,
		       grado_id, especialidad_id, institucion_id, estado
		FROM personas WHERE id=$1 FOR UPDATE`, id); err != nil {
		return out, err
	}

	dec := func(s string) string { v, _ := r.decrypt(s); return v }
	changed := func(name string, differs bool) {
		if differs {
			out.Campos = append(out.Campos, name)
		}
	}
	changed("nombres", dec(old.Nombres) != req.Nombres)
	changed("apellido_paterno", dec(old.ApellidoP) != req.ApellidoPaterno)
	changed("apellido_materno", dec(old.ApellidoM) != req.ApellidoMaterno)
	out.EmailChanged = old.EmailHMAC != r.emailHMAC(norm)
	changed("email", out.EmailChanged)
	changed("telefono", dec(old.Telefono) != req.Telefono)
	changed("estado", old.Estado != req.Estado)
	for _, c := range []struct {
		name     string
		old, new int64
	}{
		{"grado_id", old.GradoID, req.GradoID},
		{"especialidad_id", old.EspecialidadID, req.EspecialidadID},
		{"institucion_id", old.InstitucionID, req.InstitucionID},
	} {
		if c.old != c.new {
			out.Campos = append(out.Campos, c.name)
			out.ClaimsChanged = true
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE personas SET
			nombres=$1, apellido_paterno=$2, apellido_materno=$3,
			email=$4, email_hmac=$5, telefono=$6,
			grado_id=$7, especialidad_id=$8, institucion_id=$9,
			estado=$10, updated_at=NOW()
		WHERE id=$11`,
		enc["nombres"], enc["apellido_paterno"], enc["apellido_materno"],
		enc["email"], r.emailHMAC(norm), enc["telefono"],
		req.GradoID, req.EspecialidadID, req.InstitucionID,
		req.Estado, id); err != nil {
		return out, err
	}
	if out.EmailChanged {
		if err := r.cutOldEmailChannel(ctx, tx, id, old.Email); err != nil {
			return out, err
		}
		out.SesionesRevocadas = true
	} else if out.ClaimsChanged {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM sesiones WHERE usuario_id IN (SELECT id FROM usuarios WHERE persona_id=$1)`, id); err != nil {
			return out, err
		}
		var hasUser bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM usuarios WHERE persona_id=$1)`, id).Scan(&hasUser); err != nil {
			return out, err
		}
		out.SesionesRevocadas = hasUser
	}
	if auditFn != nil {
		if err := r.auditTx(ctx, tx, auditFn(out)); err != nil {
			return out, err
		}
	}
	return out, tx.Commit()
}

// UpdatePersonaEmail sets the encrypted (normalized) e-mail and its HMAC of
// a persona, refusing an address that belongs to another persona
// (ErrEmailTaken), in one transaction under the per-address advisory lock.
// When the address actually changes the old recovery channel is cut in the
// same transaction (cutOldEmailChannel) and changed=true. auditFn (nil: no
// audit) builds the audit row from changed; it is written in the same
// transaction and its failure rolls the change back. Errors: sql.ErrNoRows
// (unknown persona), ErrEmailTaken.
func (r *Repository) UpdatePersonaEmail(ctx context.Context, personaID int64, plainEmail string, auditFn func(changed bool) AuditEntry) (changed bool, err error) {
	norm := NormalizeEmail(plainEmail)
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit
	// Same lock order as UpdatePersona and VerifyPendingRegistration.
	if err := r.lockPendingRegistrations(ctx, tx, personaID); err != nil {
		return false, err
	}
	if err := r.lockEmailForPersona(ctx, tx, norm, personaID); err != nil {
		return false, err
	}
	var oldEnc, oldHMAC string
	if err := tx.QueryRowContext(ctx,
		`SELECT email, COALESCE(email_hmac,'') FROM personas WHERE id=$1 FOR UPDATE`, personaID).Scan(&oldEnc, &oldHMAC); err != nil {
		return false, err
	}
	if err := r.updatePersonaEmail(ctx, tx, personaID, norm); err != nil {
		return false, err
	}
	changed = oldHMAC != r.emailHMAC(norm)
	if changed {
		if err := r.cutOldEmailChannel(ctx, tx, personaID, oldEnc); err != nil {
			return false, err
		}
	}
	if auditFn != nil {
		if err := r.auditTx(ctx, tx, auditFn(changed)); err != nil {
			return false, err
		}
	}
	return changed, tx.Commit()
}

func (r *Repository) updatePersonaEmail(ctx context.Context, exec sqlx.ExecerContext, personaID int64, plainEmail string) error {
	norm := NormalizeEmail(plainEmail)
	encEmail, err := r.encrypt(norm)
	if err != nil {
		return err
	}
	_, err = exec.ExecContext(ctx,
		`UPDATE personas SET email=$1, email_hmac=$2, updated_at=NOW() WHERE id=$3`,
		encEmail, r.emailHMAC(norm), personaID)
	return err
}

// PersonaProfile holds decrypted personal data for registration checks.
type PersonaProfile struct {
	Nombres         string
	ApellidoPaterno string
	ApellidoMaterno string
	Email           string
}

func (r *Repository) GetPersonaProfile(ctx context.Context, personaID int64) (PersonaProfile, error) {
	var row personaRow
	if err := r.db.GetContext(ctx, &row,
		`SELECT nombres, apellido_paterno, apellido_materno, email FROM personas WHERE id=$1`, personaID); err != nil {
		return PersonaProfile{}, err
	}
	p := PersonaProfile{}
	p.Nombres, _ = r.decrypt(row.Nombres)
	p.ApellidoPaterno, _ = r.decrypt(row.ApellidoP)
	p.ApellidoMaterno, _ = r.decrypt(row.ApellidoM)
	email, err := r.emailOnFile(row.Email) // "" = no e-mail on file
	if err != nil {
		return PersonaProfile{}, err
	}
	p.Email = email
	return p, nil
}

// GetUserProfileByID returns a full UserProfile joining with catalog tables.
func (r *Repository) GetUserProfileByID(ctx context.Context, userID int64) (*UserProfile, error) {
	var row struct {
		usuarioRow
		Nombres         string `db:"nombres"`
		ApellidoPaterno string `db:"apellido_paterno"`
		ApellidoMaterno string `db:"apellido_materno"`
		GradoID         int64  `db:"grado_id"`
		Grado           string `db:"grado"`
		Nivel           string `db:"nivel"`
		Especialidad    string `db:"especialidad"`
		Institucion     string `db:"institucion"`
		TourCompletado  bool   `db:"tour_completado"`
	}
	err := r.db.GetContext(ctx, &row, `
		SELECT u.id, u.username, u.rol, u.estado, u.tour_completado,
		       p.nombres, p.apellido_paterno, p.apellido_materno,
		       p.grado_id, g.nombre AS grado, g.nivel AS nivel,
		       e.nombre AS especialidad, i.nombre AS institucion
		FROM usuarios u
		JOIN personas     p ON p.id = u.persona_id
		JOIN grados       g ON g.id = p.grado_id
		JOIN especialidades e ON e.id = p.especialidad_id
		JOIN instituciones  i ON i.id = p.institucion_id
		WHERE u.id = $1`, userID)
	if err != nil {
		return nil, err
	}
	nombres, _ := r.decrypt(row.Nombres)
	ap, _ := r.decrypt(row.ApellidoPaterno)
	am, _ := r.decrypt(row.ApellidoMaterno)

	return &UserProfile{
		ID:              row.usuarioRow.ID,
		Username:        row.usuarioRow.Username,
		Nombres:         nombres,
		ApellidoPaterno: ap,
		ApellidoMaterno: am,
		Rol:             row.usuarioRow.Rol,
		Estado:          row.usuarioRow.Estado,
		GradoID:         row.GradoID,
		Grado:           row.Grado,
		Nivel:           row.Nivel,
		Especialidad:    row.Especialidad,
		Institucion:     row.Institucion,
		TourCompletado:  row.TourCompletado,
	}, nil
}

// ResetTourByAdmin clears tour_completado of usuario userID and writes audit
// in the same transaction. Errors: ErrUserNotFound.
func (r *Repository) ResetTourByAdmin(ctx context.Context, userID int64, audit AuditEntry) error {
	return r.withAuditTx(ctx, &audit, func(tx *sqlx.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE usuarios SET tour_completado=false WHERE id=$1`, userID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrUserNotFound
		}
		return nil
	})
}

func (r *Repository) SetTourCompletado(ctx context.Context, userID int64, completado bool) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE usuarios SET tour_completado=$1 WHERE id=$2`, completado, userID)
	return err
}

// ListUsuarios returns all users filtered by estado (empty = all), with decrypted PII.
func (r *Repository) ListUsuarios(ctx context.Context, estado string) ([]UsuarioPendiente, error) {
	query := `
		SELECT u.id, u.username, u.estado,
		       p.nombres, p.apellido_paterno, p.apellido_materno,
		       p.email, p.dni,
		       g.nombre AS grado, e.nombre AS especialidad,
		       u.created_at
		FROM usuarios u
		JOIN personas      p ON p.id = u.persona_id
		JOIN grados        g ON g.id = p.grado_id
		JOIN especialidades e ON e.id = p.especialidad_id`
	args := []any{}
	if estado != "" {
		query += " WHERE u.estado = $1"
		args = append(args, estado)
	}
	query += " ORDER BY u.created_at DESC"

	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []UsuarioPendiente
	for rows.Next() {
		var row struct {
			ID              int64     `db:"id"`
			Username        string    `db:"username"`
			Estado          string    `db:"estado"`
			Nombres         string    `db:"nombres"`
			ApellidoPaterno string    `db:"apellido_paterno"`
			ApellidoMaterno string    `db:"apellido_materno"`
			Email           string    `db:"email"`
			DNI             string    `db:"dni"`
			Grado           string    `db:"grado"`
			Especialidad    string    `db:"especialidad"`
			CreatedAt       time.Time `db:"created_at"`
		}
		if err := rows.StructScan(&row); err != nil {
			continue
		}
		nombres, _ := r.decrypt(row.Nombres)
		ap, _ := r.decrypt(row.ApellidoPaterno)
		am, _ := r.decrypt(row.ApellidoMaterno)
		email, _ := r.decrypt(row.Email)
		dni, _ := r.decrypt(row.DNI)

		result = append(result, UsuarioPendiente{
			ID:             row.ID,
			Username:       row.Username,
			NombreCompleto: nombres + " " + ap + " " + am,
			DNI:            maskDNI(dni),
			Email:          email,
			Grado:          row.Grado,
			Especialidad:   row.Especialidad,
			Estado:         row.Estado,
			FechaSolicitud: row.CreatedAt.Format("2006-01-02"),
		})
	}
	return result, rows.Err()
}

// GetAdminEmails returns emails of all active admins (for notifications).
func (r *Repository) GetAdminEmails(ctx context.Context) ([]string, error) {
	var encEmails []string
	err := r.db.SelectContext(ctx, &encEmails, `
		SELECT p.email
		FROM usuarios u
		JOIN personas p ON p.id = u.persona_id
		WHERE u.rol='admin' AND u.estado='activo'`)
	if err != nil {
		return nil, err
	}
	emails := make([]string, 0, len(encEmails))
	for _, enc := range encEmails {
		if plain, err := r.emailOnFile(enc); err == nil && plain != "" {
			emails = append(emails, plain)
		}
	}
	return emails, nil
}
