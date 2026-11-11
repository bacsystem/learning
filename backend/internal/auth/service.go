package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	jwtpkg "github.com/cbaciliod/learning/internal/jwt"
	internalmw "github.com/cbaciliod/learning/internal/middleware"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

// bcryptCost is a variable only so the package tests can lower it
// (bcrypt_cost_test.go): production code never changes it.
var bcryptCost = 12

const (
	maxLoginAttempts  = 5
	lockoutDuration   = 15 * time.Minute
	resetTokenExpiry  = time.Hour
	verifyTokenExpiry = 24 * time.Hour
	// Accepted registrations (verification e-mails issued) per persona per
	// hour. Not refunded: bounds link invalidation and inbox flooding by
	// someone who knows the DNI and the e-mail on file.
	registerIssuesPerPersona = 3
	registerIssueWindow      = time.Hour
	registerIssuesPrefix     = "reg_emitidos_dni:"
)

type Service struct {
	repo            *Repository
	jwtSecret       string
	frontendBaseURL string
	// registerIssues caps accepted registrations per persona (dni_hmac), in
	// the shared Postgres counter table; see registerIssuesPerPersona.
	registerIssues *internalmw.DBLimiter
	// relojLimites is nil in production; tests fix it (fijarRelojLimites).
	relojLimites func() time.Time
}

func NewService(repo *Repository, jwtSecret string, frontendBaseURL string) *Service {
	return &Service{
		repo: repo, jwtSecret: jwtSecret, frontendBaseURL: frontendBaseURL,
		registerIssues: internalmw.NewDBLimiter(repo.db.DB, registerIssuesPrefix, registerIssuesPerPersona, registerIssueWindow),
	}
}

// fijarRelojLimites makes every DBLimiter of this service, and of the handlers
// built on it afterwards, count in the window of ahora. Only for tests (see
// DBLimiter.ConReloj); production keeps now() of Postgres.
func (s *Service) fijarRelojLimites(ahora func() time.Time) {
	s.relojLimites = ahora
	s.registerIssues.ConReloj(ahora)
}

// ── Check DNI ────────────────────────────────────────────────────────────────

// CheckDNIResult is returned by the PUBLIC endpoint GET /auth/check-dni. It
// must never contain the plaintext e-mail nor full names: only the first
// given name, the initial of the first surname ("Juan", "P.") and a masked
// e-mail hint ("" when there is no e-mail on file).
type CheckDNIResult struct {
	Nombres         string `json:"nombres"`
	ApellidoPaterno string `json:"apellido_paterno"`
	EmailMasked     string `json:"email_masked"`
}

func (s *Service) CheckDNI(ctx context.Context, dni string) (*CheckDNIResult, error) {
	persona, err := s.repo.FindPersonaByDNI(ctx, dni)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDNINotFound
	}
	if err != nil {
		return nil, err
	}
	already, err := s.repo.ExistsByPersonaID(ctx, persona.ID)
	if err != nil {
		return nil, err
	}
	if already {
		return nil, ErrPersonaAlreadyRegistered
	}
	profile, err := s.repo.GetPersonaProfile(ctx, persona.ID)
	if err != nil {
		return nil, err
	}
	nombre, inicial := maskNames(profile.Nombres, profile.ApellidoPaterno)
	return &CheckDNIResult{
		Nombres:         nombre,
		ApellidoPaterno: inicial,
		EmailMasked:     maskEmail(profile.Email),
	}, nil
}

// maskNames reduces names for the public check-dni response: the first given
// name and the initial of the first surname followed by a dot
// ("Juan Carlos", "Pérez" → "Juan", "P."). Empty inputs stay empty.
func maskNames(nombres, apellidoPaterno string) (nombre, inicial string) {
	if f := strings.Fields(nombres); len(f) > 0 {
		nombre = f[0]
	}
	for _, r := range strings.TrimSpace(apellidoPaterno) {
		inicial = string(r) + "."
		break
	}
	return nombre, inicial
}

// maskEmail hides an e-mail address keeping just enough to be recognised by
// its owner: "juan.perez@gmail.com" → "j***@g***.com". Only the first
// character of the local part and of the domain, and the TLD, are kept.
// Returns "" for an empty or malformed address.
func maskEmail(email string) string {
	at := strings.LastIndexByte(email, '@')
	if at < 1 || at == len(email)-1 {
		return ""
	}
	local, domain := email[:at], email[at+1:]
	firstRune := func(s string) string {
		for _, r := range s {
			return string(r)
		}
		return ""
	}
	masked := firstRune(local) + "***@" + firstRune(domain) + "***"
	if dot := strings.LastIndexByte(domain, '.'); dot > 0 && dot < len(domain)-1 {
		masked += domain[dot:]
	}
	return masked
}

// ── Check Username ────────────────────────────────────────────────────────────

func (s *Service) CheckUsername(ctx context.Context, username string) (bool, error) {
	return s.repo.ExistsByUsername(ctx, username)
}

// ── Batch Import ─────────────────────────────────────────────────────────────

// BatchImport inserts the personas and audits the batch in one transaction:
// if the audit row cannot be written nothing is inserted.
func (s *Service) BatchImport(ctx context.Context, items []BatchPersonaItem, adminID int64, ip string) (*BatchPersonasResponse, error) {
	inserted, duplicates, errs, err := s.repo.BatchInsert(ctx, items, func(inserted, duplicates int) AuditEntry {
		return AuditEntry{
			UsuarioID: &adminID,
			Accion:    "batch_import",
			Entidad:   "personas",
			Detalle:   auditJSON(map[string]any{"inserted": inserted, "duplicates": duplicates}),
			IP:        ip,
		}
	})
	if err != nil {
		return nil, err
	}
	return &BatchPersonasResponse{Inserted: inserted, Duplicates: duplicates, Errors: errs}, nil
}

// ── Register ─────────────────────────────────────────────────────────────────

// Register validates the identity of a pre-loaded persona (DNI + e-mail) and,
// when it is acceptable, stores a PENDING registration and e-mails a link.
// No credentials are collected here: username and password are chosen in
// VerifyEmail, by whoever controls the inbox the link was sent to. Knowing a
// DNI and its e-mail on file therefore never leaves attacker-chosen
// credentials waiting to be activated.
//
// Security rules:
//   - A persona that already has a usuario: ErrPersonaAlreadyRegistered.
//   - If the persona has an e-mail on file, the submitted e-mail must match it
//     (normalized comparison); otherwise ErrEmailMismatch. The link goes to
//     the address ON FILE. With no e-mail on file the link goes to the
//     submitted one, which must not belong to another persona
//     (ErrEmailTaken) and is written to the persona only at verification.
//   - A new registration for the same persona replaces (invalidates) any
//     previous unconsumed one; accepted (re)issues are capped per persona.
//
// The handler renders every business error and success identically.
func (s *Service) Register(ctx context.Context, req RegisterRequest, ip, ua string) error {
	email := NormalizeEmail(req.Email)

	persona, err := s.repo.FindPersonaByDNI(ctx, req.DNI)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrDNINotFound
	}
	if err != nil {
		return err
	}

	already, err := s.repo.ExistsByPersonaID(ctx, persona.ID)
	if err != nil {
		return err
	}
	if already {
		return ErrPersonaAlreadyRegistered
	}

	profile, err := s.repo.GetPersonaProfile(ctx, persona.ID)
	if err != nil {
		return err
	}
	sendTo := profile.Email
	if profile.Email != "" {
		if !hmac.Equal([]byte(s.repo.emailHMAC(profile.Email)), []byte(s.repo.emailHMAC(email))) {
			return ErrEmailMismatch
		}
	} else {
		sendTo = email
		// The persona has no e-mail on file: the submitted one must not belong
		// to another persona (see EmailClaimedByOther); verify re-checks it.
		claimed, err := s.repo.EmailClaimedByOther(ctx, email, persona.ID)
		if err != nil {
			return err
		}
		if claimed {
			return ErrEmailTaken
		}
	}

	existsEmail, err := s.repo.ExistsByEmail(ctx, email)
	if err != nil {
		return err
	}
	if existsEmail {
		return ErrEmailTaken
	}

	// Cap accepted (re)issues per persona. Past the cap nothing is stored or
	// sent, but the caller gets the same generic outcome (no oracle) and the
	// last link issued keeps working.
	issue, ok, err := s.registerIssues.TryAcquire(ctx, persona.DNIHMAC)
	if err != nil {
		return err
	}
	if !ok {
		log.Warn().Int64("persona_id", persona.ID).Msg("register: accepted registrations cap reached; nothing issued")
		return nil
	}

	token, err := newVerifyToken()
	if err != nil {
		_ = s.registerIssues.Release(ctx, issue)
		return err
	}
	link := fmt.Sprintf("%s/auth/verify-email?token=%s", s.frontendBaseURL, token)
	sub, bdy := emailVerificarRegistro(profile.Nombres, profile.ApellidoPaterno, link)
	if err := s.repo.SavePendingRegistration(ctx, PendingRegistration{
		PersonaID: persona.ID,
		Email:     sendTo,
		TokenHash: hashToken(token),
		ExpiresAt: time.Now().Add(verifyTokenExpiry),
	}, sub, bdy); err != nil {
		_ = s.registerIssues.Release(ctx, issue) // nothing was issued
		return err
	}

	s.repo.WriteAudit(ctx, AuditEntry{
		Accion:    "register_pendiente",
		Entidad:   "personas",
		EntidadID: &persona.ID,
		IP:        ip,
		UserAgent: ua,
	})
	return nil
}

// ResetRegistroLimites clears, for the persona of dni, every persisted
// registration counter keyed by its dni_hmac: register failures (per DNI and
// per DNI+IP), check-dni counts (per DNI and per DNI+IP) and the accepted
// registrations cap. It is the admin way out when an attacker keeps the
// owner's windows exhausted. The audit row carries the persona id, never the
// DNI. The counters are cleared and audited in one transaction. Errors:
// ErrDNINotFound.
func (s *Service) ResetRegistroLimites(ctx context.Context, dni string, adminID int64, ip string) error {
	persona, err := s.repo.FindPersonaByDNI(ctx, dni)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrDNINotFound
	}
	if err != nil {
		return err
	}
	_, err = s.repo.ClearLimiterCounters(ctx,
		[]string{registerDNIPrefix + persona.DNIHMAC, checkDNIPerDNIPrefix + persona.DNIHMAC, registerIssuesPrefix + persona.DNIHMAC},
		[]string{registerPairPrefix + persona.DNIHMAC + "|", checkDNIPairPrefix + persona.DNIHMAC + "|"},
		func(n int64) AuditEntry {
			return AuditEntry{
				UsuarioID: &adminID,
				Accion:    "reset_limites_registro",
				Entidad:   "personas",
				EntidadID: &persona.ID,
				Detalle:   auditJSON(map[string]any{"contadores_borrados": n}),
				IP:        ip,
			}
		})
	return err
}

// newVerifyToken returns 32 random bytes, base64url-encoded (no padding).
func newVerifyToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashToken is the SHA-256 hex digest under which one-time tokens are stored.
func hashToken(token string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(token))) }

// VerifyEmail sets the credentials and creates the usuario (estado
// 'pendiente'; an admin activates it) for the registration of the e-mailed
// token, consuming the token atomically and only on success.
// Errors: ErrInvalidVerifyToken, ErrUsernameTaken, ErrEmailTaken.
func (s *Service) VerifyEmail(ctx context.Context, req VerifyEmailRequest, ip, ua string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		return err
	}
	res, err := s.repo.VerifyPendingRegistration(ctx, hashToken(req.Token), req.Username, string(hash))
	if err != nil {
		return err
	}

	// Notify user: registration pending admin activation.
	if profile, err := s.repo.GetPersonaProfile(ctx, res.PersonaID); err != nil {
		log.Error().Err(err).Int64("usuario_id", res.UserID).Msg("verify_email: load profile failed")
	} else if profile.Email != "" {
		sub, bdy := emailRegistroPendiente(profile.Nombres, profile.ApellidoPaterno, req.Username)
		if err := s.repo.EnqueueEmail(ctx, profile.Email, sub, bdy); err != nil {
			log.Error().Err(err).Int64("usuario_id", res.UserID).Msg("verify_email: enqueue email failed")
		}
	}

	s.repo.WriteAudit(ctx, AuditEntry{
		UsuarioID: &res.UserID,
		Accion:    "register",
		Entidad:   "usuarios",
		EntidadID: &res.UserID,
		IP:        ip,
		UserAgent: ua,
	})
	return nil
}

// ── Login ─────────────────────────────────────────────────────────────────────

func (s *Service) Login(ctx context.Context, req LoginRequest, ip, ua string) (*LoginResponse, error) {
	u, err := s.repo.FindByUsername(ctx, req.Username)
	if err != nil || u == nil {
		return nil, ErrInvalidCredentials
	}

	// Check lockout
	if u.BloqueadoHasta.Valid && time.Now().Before(u.BloqueadoHasta.Time) {
		return nil, ErrAccountLocked
	}

	// Validate password
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		_, locked, err := s.repo.RegisterFailedLogin(ctx, u.ID, maxLoginAttempts, lockoutDuration)
		if err != nil {
			return nil, err
		}
		if locked {
			s.repo.WriteAudit(ctx, AuditEntry{UsuarioID: &u.ID, Accion: "lockout", Entidad: "usuarios", EntidadID: &u.ID, IP: ip})
			return nil, ErrAccountLocked
		}
		s.repo.WriteAudit(ctx, AuditEntry{UsuarioID: &u.ID, Accion: "login_failed", Entidad: "usuarios", EntidadID: &u.ID, IP: ip})
		return nil, ErrInvalidCredentials
	}

	// Check account estado
	switch u.Estado {
	case "pendiente":
		return nil, ErrAccountPending
	case "rechazado":
		return nil, ErrAccountRejected
	case "inactivo":
		return nil, ErrAccountInactive
	}

	// Reset brute-force counter
	if err := s.repo.ResetLoginIntentos(ctx, u.ID); err != nil {
		return nil, err
	}

	// Fetch profile for claims
	profile, err := s.repo.GetUserProfileByID(ctx, u.ID)
	if err != nil {
		return nil, err
	}

	// Raw IDs and nivel from grados for JWT claims
	var rawIDs struct {
		GradoID        int64  `db:"grado_id"`
		EspecialidadID int64  `db:"especialidad_id"`
		InstitucionID  int64  `db:"institucion_id"`
		Nivel          string `db:"nivel"`
	}
	if err := s.repo.db.GetContext(ctx, &rawIDs,
		`SELECT p.grado_id, p.especialidad_id, p.institucion_id, g.nivel
		 FROM personas p
		 JOIN usuarios u ON u.persona_id=p.id
		 JOIN grados g ON g.id=p.grado_id
		 WHERE u.id=$1`, u.ID); err != nil {
		return nil, fmt.Errorf("load claims: %w", err)
	}

	token, expiresAt, err := jwtpkg.Issue(s.jwtSecret, u.ID, u.Rol, rawIDs.Nivel, rawIDs.GradoID, rawIDs.EspecialidadID, rawIDs.InstitucionID)
	if err != nil {
		return nil, err
	}

	// Enforce session limit: evict oldest sessions before creating new one
	maxSes, err := s.repo.GetMaxSesiones(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.repo.DeleteOldestSesiones(ctx, u.ID, maxSes-1); err != nil {
		return nil, err
	}

	// Store session (hash of JWT). Without it every request with this token
	// would be rejected, so a failure here must fail the login.
	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
	if err := s.repo.CreateSesion(ctx, u.ID, tokenHash, ip, ua, time.Unix(expiresAt, 0)); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	s.repo.WriteAudit(ctx, AuditEntry{UsuarioID: &u.ID, Accion: "login", Entidad: "usuarios", EntidadID: &u.ID, IP: ip})

	return &LoginResponse{Token: token, ExpiresAt: expiresAt, User: *profile}, nil
}

// ── Logout ────────────────────────────────────────────────────────────────────

func (s *Service) Logout(ctx context.Context, userID int64, tokenStr, ip string) error {
	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(tokenStr)))
	_ = s.repo.InvalidateSesionByHash(ctx, tokenHash)
	s.repo.WriteAudit(ctx, AuditEntry{UsuarioID: &userID, Accion: "logout", Entidad: "sesiones", IP: ip})
	return nil
}

// ── Admin: Update Estado ──────────────────────────────────────────────────────

// UpdateEstado changes a usuario's estado. An admin cannot deactivate or
// reject their own account (ErrCannotChangeOwnEstado) and the last active
// admin can never be removed (ErrLastAdmin, enforced atomically in the repo).
// The change and its audit row (with the admin's ip) commit together.
func (s *Service) UpdateEstado(ctx context.Context, targetID int64, req UpdateUsuarioEstadoRequest, adminID int64, ip string) (*UpdateUsuarioEstadoResponse, error) {
	if targetID == adminID && req.Estado != "activo" {
		return nil, ErrCannotChangeOwnEstado
	}

	u, err := s.repo.FindByID(ctx, targetID)
	if err != nil || u == nil {
		return nil, ErrUserNotFound
	}

	if err := s.repo.UpdateEstado(ctx, targetID, req.Estado, adminID, &AuditEntry{
		UsuarioID: &adminID,
		Accion:    req.Estado,
		Entidad:   "usuarios",
		EntidadID: &targetID,
		Detalle:   auditJSON(map[string]any{"observacion": req.Observacion}),
		IP:        ip,
	}); err != nil {
		return nil, err
	}

	if req.Estado == "rechazado" || req.Estado == "inactivo" {
		if err := s.repo.InvalidateAllSesiones(ctx, targetID); err != nil {
			// RequireAuth re-reads estado per request, so access is cut anyway.
			log.Error().Err(err).Int64("usuario_id", targetID).Msg("update_estado: revoke sessions failed")
		}
	}

	// Notify user via email
	profile, err := s.repo.GetUserProfileByID(ctx, targetID)
	if err != nil {
		log.Error().Err(err).Int64("usuario_id", targetID).Msg("update_estado: load profile failed; no e-mail sent")
	} else {
		var subject, body string
		loginURL := s.frontendBaseURL + "/auth/sign-in"
		switch req.Estado {
		case "activo":
			subject, body = emailActivado(profile.Nombres, profile.ApellidoPaterno, profile.Username, loginURL)
		case "rechazado":
			subject, body = emailRechazado(profile.Nombres, profile.ApellidoPaterno)
		case "inactivo":
			subject, body = emailInactivo(profile.Nombres, profile.ApellidoPaterno)
		}
		if subject != "" {
			s.notifyUsuario(ctx, targetID, "update_estado", subject, body)
		}
	}

	var updatedAt string
	if err := s.repo.db.QueryRowContext(ctx, `SELECT updated_at FROM usuarios WHERE id=$1`, targetID).Scan(&updatedAt); err != nil {
		// The change is committed and audited: report it, without updated_at.
		log.Error().Err(err).Int64("usuario_id", targetID).Msg("update_estado: read updated_at failed")
	}
	return &UpdateUsuarioEstadoResponse{ID: targetID, Estado: req.Estado, UpdatedAt: updatedAt}, nil
}

// notifyUsuario enqueues an e-mail to the address on file of usuario userID.
// Failures (lookup, decrypt, enqueue) are logged, never returned: the state
// change that triggered the notice is already committed. op tags the log.
func (s *Service) notifyUsuario(ctx context.Context, userID int64, op, subject, body string) {
	var encEmail string
	if err := s.repo.db.QueryRowContext(ctx,
		`SELECT p.email FROM personas p JOIN usuarios u ON u.persona_id=p.id WHERE u.id=$1`, userID,
	).Scan(&encEmail); err != nil {
		log.Error().Err(err).Int64("usuario_id", userID).Str("op", op).Msg("notify: load e-mail failed")
		return
	}
	email, err := s.repo.emailOnFile(encEmail)
	if err != nil {
		log.Error().Err(err).Int64("usuario_id", userID).Str("op", op).Msg("notify: decrypt e-mail failed")
		return
	}
	if email == "" {
		return // no e-mail on file
	}
	if err := s.repo.EnqueueEmail(ctx, email, subject, body); err != nil {
		log.Error().Err(err).Int64("usuario_id", userID).Str("op", op).Msg("notify: enqueue e-mail failed")
	}
}

// ── Admin: persona / e-mail edits ────────────────────────────────────────────

// AdminUpdateUsuarioEmail changes the e-mail on file (the recovery channel)
// of usuario targetID's persona; a real change invalidates the old channel
// (reset tokens, pending registrations, sessions; notice to the old address).
// Errors: ErrUserNotFound, ErrEmailTaken.
// Audited with ids only (no e-mail in detalle), in the change's transaction.
func (s *Service) AdminUpdateUsuarioEmail(ctx context.Context, targetID int64, email string, adminID int64, ip string) error {
	var personaID int64
	if err := s.repo.db.QueryRowContext(ctx, `SELECT persona_id FROM usuarios WHERE id=$1`, targetID).Scan(&personaID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUserNotFound
		}
		return err
	}
	_, err := s.repo.UpdatePersonaEmail(ctx, personaID, email, func(changed bool) AuditEntry {
		return AuditEntry{
			UsuarioID: &adminID,
			Accion:    "update_email",
			Entidad:   "personas",
			EntidadID: &personaID,
			Detalle: auditJSON(map[string]any{
				"usuario_id": targetID, "campos": []string{"email"},
				"canal_anterior_invalidado": changed, "sesiones_revocadas": changed,
			}),
			IP: ip,
		}
	})
	return err
}

// AdminUpdatePersona updates a persona (see Repository.UpdatePersona). When
// grado, especialidad or institución change, the sessions of the persona's
// usuario are revoked: those values are read from the JWT claims for up to
// 24 h. Audited with the changed field names only, in the same transaction.
// Errors: sql.ErrNoRows, ErrEmailTaken.
func (s *Service) AdminUpdatePersona(ctx context.Context, personaID int64, req UpdatePersonaRequest, adminID int64, ip string) error {
	_, err := s.repo.UpdatePersona(ctx, personaID, req, func(upd PersonaUpdate) AuditEntry {
		campos := upd.Campos
		if campos == nil {
			campos = []string{}
		}
		return AuditEntry{
			UsuarioID: &adminID,
			Accion:    "update_persona",
			Entidad:   "personas",
			EntidadID: &personaID,
			Detalle: auditJSON(map[string]any{
				"campos": campos, "sesiones_revocadas": upd.SesionesRevocadas, "canal_anterior_invalidado": upd.EmailChanged,
			}),
			IP: ip,
		}
	})
	return err
}

// ── Password Recovery ─────────────────────────────────────────────────────────

// ForgotPassword issues a reset link when the e-mail belongs to exactly one
// ACTIVE usuario. It always returns nil (anti-enumeration). If several active
// usuarios share the e-mail nothing is sent: picking one would be arbitrary
// and could hand one person's account to another.
func (s *Service) ForgotPassword(ctx context.Context, email, ip string) error {
	email = NormalizeEmail(email)
	ids, err := s.repo.FindActiveUserIDsByEmail(ctx, email)
	if err != nil {
		log.Error().Err(err).Msg("forgot_password: lookup failed")
		return nil
	}
	switch {
	case len(ids) == 0:
		return nil // don't reveal whether the e-mail exists
	case len(ids) > 1:
		log.Warn().Int("matches", len(ids)).Msg("forgot_password: e-mail shared by several active usuarios; no reset link sent")
		return nil
	}
	userID := ids[0]

	if err := s.repo.InvalidatePreviosTokens(ctx, userID); err != nil {
		log.Error().Err(err).Int64("usuario_id", userID).Msg("forgot_password: invalidate previous tokens failed")
		return nil
	}

	token := uuid.New().String()
	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
	if err := s.repo.CreateToken(ctx, userID, tokenHash, time.Now().Add(resetTokenExpiry)); err != nil {
		log.Error().Err(err).Int64("usuario_id", userID).Msg("forgot_password: create token failed")
		return nil
	}

	resetLink := fmt.Sprintf("%s/auth/password-new?token=%s", s.frontendBaseURL, token)
	sub, bdy := emailOlvidePassword(resetLink)
	if err := s.repo.EnqueueEmail(ctx, email, sub, bdy); err != nil {
		log.Error().Err(err).Int64("usuario_id", userID).Msg("forgot_password: enqueue email failed")
	}

	s.repo.WriteAudit(ctx, AuditEntry{UsuarioID: &userID, Accion: "forgot_password", Entidad: "tokens_recuperacion", IP: ip})
	return nil
}

// ResetPassword consumes a recovery token (single use, atomically) and sets
// the new password, revoking all sessions of the usuario.
func (s *Service) ResetPassword(ctx context.Context, token, newPassword, ip string) error {
	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))

	// Cheap pre-check so invalid tokens don't cost a bcrypt hash. It is NOT
	// the authority: the token is consumed atomically below.
	valid, err := s.repo.ResetTokenLooksValid(ctx, tokenHash)
	if err != nil {
		return err
	}
	if !valid {
		return ErrInvalidToken
	}

	// Hash before the transaction (slow) so the token-consuming tx stays short.
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return err
	}

	userID, err := s.repo.ResetPasswordWithToken(ctx, tokenHash, string(hash))
	if err != nil {
		return err
	}

	// Notify user: password changed
	sub, bdy := emailPasswordRestablecida(s.frontendBaseURL + "/auth/sign-in")
	s.notifyUsuario(ctx, userID, "reset_password", sub, bdy)

	s.repo.WriteAudit(ctx, AuditEntry{UsuarioID: &userID, Accion: "reset_password", Entidad: "usuarios", EntidadID: &userID, IP: ip})
	return nil
}

func (s *Service) ListUsuariosOnline(ctx context.Context) ([]UsuarioOnlineItem, error) {
	return s.repo.ListUsuariosOnline(ctx)
}

func (s *Service) ListSesionesUsuario(ctx context.Context, usuarioID int64) ([]SesionHistorialItem, error) {
	return s.repo.ListSesionesUsuario(ctx, usuarioID)
}

func (s *Service) SetTourCompletado(ctx context.Context, userID int64, completado bool) error {
	return s.repo.SetTourCompletado(ctx, userID, completado)
}
