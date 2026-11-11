package auth

import (
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/cbaciliod/learning/internal/httpx"
	internalmw "github.com/cbaciliod/learning/internal/middleware"
	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
)

var validate = newValidator()

// newValidator returns the package validator with custom rules:
//   - field names in errors are the JSON tag names;
//   - maxbytes=N: string length in BYTES ≤ N (bcrypt silently/explicitly
//     limits passwords to 72 bytes, while max=N counts runes).
func newValidator() *validator.Validate {
	v := validator.New()
	// Report fields by their JSON name (see validationMessages).
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			return f.Name
		}
		return name
	})
	if err := v.RegisterValidation("maxbytes", func(fl validator.FieldLevel) bool {
		n, err := strconv.Atoi(fl.Param())
		return err == nil && len(fl.Field().String()) <= n
	}); err != nil {
		panic(err)
	}
	return v
}

// maxJSONBody caps every JSON body decoded by this package, whatever the
// global middleware allowed (its multipart ceiling is chosen from the
// client-sent Content-Type and only meant for upload routes).
const maxJSONBody = internalmw.DefaultJSONBodyLimit

// decodeJSON decodes the request body into v. The body must be declared
// application/json (415 otherwise) and is read through its own
// http.MaxBytesReader of maxJSONBody. On failure it writes the error response
// (415, 413 when the body exceeded the limit, 400 otherwise) and returns false.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		jsonError(w, http.StatusUnsupportedMediaType, "El cuerpo de la solicitud debe ser JSON (Content-Type: application/json).")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			jsonError(w, http.StatusRequestEntityTooLarge, "El cuerpo de la solicitud es demasiado grande.")
		} else {
			jsonError(w, http.StatusBadRequest, "Cuerpo de solicitud inválido.")
		}
		return false
	}
	return true
}

type Handler struct {
	svc *Service
	// Registration-attempt limiters, on top of the per-IP request limiter.
	// Keys use dni_hmac and an HMAC of the client IP, never the raw DNI/IP.
	// Every attempt with a business outcome counts, accepted or rejected
	// alike: each one reserves a unit atomically with TryAcquire (no
	// check-then-record race) and only internal errors (500) refund it.
	// Refunding successes would make the N-th attempt answer 202 for the
	// right e-mail and 429 for a wrong one — an oracle for the e-mail on file
	// behind the generic 202 (H128).
	//
	//   - registerFailures: per (dni_hmac, client IP) — 5 / 15 min. Stops a
	//     single source quickly without letting others burn the owner's quota.
	//   - registerFailuresPerDNI: per dni_hmac, all IPs — 30 / 1 h. Bounds a
	//     distributed attacker guessing the e-mail for one DNI.
	//
	// Deliberate tradeoff: an attacker can still lock the real owner out of
	// registering for up to an hour, but that now takes >= 30 failures spread
	// over >= 6 IPs. Knowing a DNI and its e-mail no longer creates an account
	// (the link goes to the e-mail on file, see Service.Register). The
	// counters live in Postgres (auth_intentos_fallidos): shared by every
	// replica and kept across restarts.
	registerFailures       *internalmw.DBLimiter
	registerFailuresPerDNI *internalmw.DBLimiter
	// verifyAttempts: POST /auth/verify-email attempts per token (sha256),
	// all outcomes counted — bounds username probing with one link.
	verifyAttempts *internalmw.DBLimiter
	// check-dni limiters (see acquireCheckDNI):
	//   - checkDNIPair: per (dni_hmac, ip), every check counts.
	//   - checkDNIPerIP: per ip_hmac, distinct DNIs — bounds enumeration.
	//   - checkDNIPerDNIAll: per dni_hmac, distinct sources.
	checkDNIPair      *internalmw.DBLimiter
	checkDNIPerDNIAll *internalmw.DBLimiter
	checkDNIPerIP     *internalmw.DBLimiter

	// Timing padding of POST /auth/register: every outcome answered with the
	// generic 202 (accepted, business rejection, over the issue cap) is
	// delayed until registerMinDuration after the request started, so the
	// response time does not reveal which branch ran (an unknown DNI returns
	// fast, a success writes and enqueues). now/sleep are injectable for tests.
	registerMinDuration time.Duration
	now                 func() time.Time
	sleep               func(ctx context.Context, d time.Duration)
}

const (
	registerFailuresPerDNIAndIP = 5
	registerFailureWindow       = 15 * time.Minute
	registerFailuresPerDNI      = 30
	registerFailurePerDNIWindow = time.Hour
	// GET /auth/check-dni (public, answers with masked data for a DNI
	// without account), persisted like the register limiters: every check
	// per (dni_hmac, ip), distinct DNIs per IP and distinct sources per DNI.
	checkDNIPerDNIAndIP    = 5
	checkDNIPerDNI         = 30
	checkDNIPerDNIWindow   = time.Hour
	checkDNIDistinctPerIP  = 5
	checkDNIWindow         = 15 * time.Minute
	checkDNIPairPrefix     = "chk_dni_ip:"
	checkDNIPerDNIPrefix   = "chk_dni:"
	checkDNIPerIPPrefix    = "chk_ip:"
	registerPairPrefix     = "reg_dni_ip:"
	registerDNIPrefix      = "reg_dni:"
	verifyAttemptsPerToken = 10
	verifyAttemptWindow    = time.Hour
	// registerMinDuration: fixed minimum duration of every generic register
	// response (see Handler.registerMinDuration).
	registerMinDuration = 400 * time.Millisecond
)

// msgRegisterAccepted is the single 202 response of POST /auth/register,
// both when a verification e-mail was sent and for every business rejection
// (unknown DNI, DNI already registered, e-mail not matching the one on file,
// username/e-mail in use): the endpoint must not act as an oracle for any of
// them. Only request-shape validation errors (422) and 429/500 differ.
const msgRegisterAccepted = "Si los datos son correctos, te enviamos un correo para confirmar tu registro."

// msgVerifyUsernameTaken: the username was taken between register and verify.
const msgVerifyUsernameTaken = "El nombre de usuario ya está en uso. Elige otro."

// msgVerifyEmailTaken: the e-mail was attached to another persona between
// register and verify.
const msgVerifyEmailTaken = "El correo ya está asociado a otra persona. Contacta al administrador."

// SetTrustedProxies configures the trusted proxies used to resolve the real
// client IP. Kept for backward compatibility; it delegates to
// middleware.SetTrustedProxies, which auth, audit and the rate limiter share.
func SetTrustedProxies(list string) { internalmw.SetTrustedProxies(list) }

// clientIP extracts the real client IP (X-Forwarded-For only from trusted proxies).
func clientIP(r *http.Request) string { return internalmw.ClientIP(r) }

func NewHandler(svc *Service) *Handler {
	return &Handler{
		svc:                    svc,
		registerFailures:       internalmw.NewDBLimiter(svc.repo.db.DB, registerPairPrefix, registerFailuresPerDNIAndIP, registerFailureWindow),
		registerFailuresPerDNI: internalmw.NewDBLimiter(svc.repo.db.DB, registerDNIPrefix, registerFailuresPerDNI, registerFailurePerDNIWindow),
		verifyAttempts:         internalmw.NewDBLimiter(svc.repo.db.DB, "verify_tok:", verifyAttemptsPerToken, verifyAttemptWindow),
		checkDNIPair:           internalmw.NewDBLimiter(svc.repo.db.DB, checkDNIPairPrefix, checkDNIPerDNIAndIP, checkDNIWindow),
		checkDNIPerDNIAll:      internalmw.NewDBLimiter(svc.repo.db.DB, checkDNIPerDNIPrefix, checkDNIPerDNI, checkDNIPerDNIWindow),
		checkDNIPerIP:          internalmw.NewDBLimiter(svc.repo.db.DB, checkDNIPerIPPrefix, checkDNIDistinctPerIP, checkDNIWindow),
		registerMinDuration:    registerMinDuration,
		now:                    time.Now,
		sleep:                  sleepCtx,
	}
}

// registerFailureKeys returns the (dni, ip) and dni counter keys: HMACs only.
func (h *Handler) registerFailureKeys(dni, ip string) (pairKey, dniKey string) {
	dniKey = h.svc.repo.dniHMAC(dni)
	return dniKey + "|" + h.svc.repo.ipHMAC(ip), dniKey
}

func (h *Handler) CheckDNI(w http.ResponseWriter, r *http.Request) {
	dni := r.URL.Query().Get("dni")
	if !isDNI(dni) {
		jsonError(w, http.StatusBadRequest, "El DNI debe tener exactamente 8 dígitos.")
		return
	}
	ctx := r.Context()
	ipKey := h.svc.repo.ipHMAC(clientIP(r))
	dniKey := h.svc.repo.dniHMAC(dni)
	ok, err := h.acquireCheckDNI(ctx, dniKey, ipKey)
	if err != nil {
		log.Error().Err(err).Msg("check_dni: limiter")
		jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}
	if !ok {
		jsonError(w, http.StatusTooManyRequests, "Demasiadas consultas de DNI. Intenta más tarde.")
		return
	}
	result, err := h.svc.CheckDNI(ctx, dni)
	if err != nil {
		switch {
		case errors.Is(err, ErrDNINotFound):
			// Respuesta genérica: no revelar si el DNI existe en la base de personas
			jsonError(w, http.StatusNotFound, "DNI no encontrado o no disponible para registro.")
		case errors.Is(err, ErrPersonaAlreadyRegistered):
			// Misma respuesta que "no encontrado" para no enumerar personas registradas
			jsonError(w, http.StatusNotFound, "DNI no encontrado o no disponible para registro.")
		default:
			jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
		}
		return
	}
	jsonOK(w, http.StatusOK, result)
}

// acquireCheckDNI reserves one check of dniKey from ipKey (an HMAC of the
// LimiterIP-normalized client IP). ok=false means 429.
//
//   - The (dni, ip) pair is always charged; its reservation returns the
//     post-increment count atomically, so "first check of this DNI from this
//     source in the window" (count 1) is decided without a separate read.
//   - Only a first check charges the per-IP limiter (distinct DNIs per
//     source) and the per-DNI limiter (distinct sources per DNI), both or
//     neither: when one refuses, every unit taken is undone. A source that
//     already holds a check of this DNI passes even when the per-DNI cap is
//     reached, so other sources cannot lock the owner out of a check that
//     already succeeded, and one source hammering a DNI costs the per-DNI
//     quota one unit.
//   - The three reservations run in one transaction (H101). The pair's row
//     stays locked until it commits or rolls back, so a concurrent check of
//     the same pair waits for that outcome: it sees count > 1 only when the
//     first check was granted (committed), never while that first check may
//     still be refused and undone.
func (h *Handler) acquireCheckDNI(ctx context.Context, dniKey, ipKey string) (ok bool, err error) {
	h.checkDNIPair.MaybeSweep(ctx) // all limiters share the table
	tx, err := h.svc.repo.db.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() {
		if !ok {
			_ = tx.Rollback()
		}
	}()
	pair, granted, err := h.checkDNIPair.TryAcquireTx(ctx, tx, dniKey+"|"+ipKey)
	if err != nil || !granted {
		return false, err
	}
	if pair.Count() == 1 {
		if _, granted, err := h.checkDNIPerIP.TryAcquireTx(ctx, tx, ipKey); err != nil || !granted {
			return false, err
		}
		if _, granted, err := h.checkDNIPerDNIAll.TryAcquireTx(ctx, tx, dniKey); err != nil || !granted {
			return false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func isDNI(s string) bool {
	if len(s) != 8 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (h *Handler) CheckUsername(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if len(username) < 4 || len(username) > 30 {
		jsonError(w, http.StatusBadRequest, "Username inválido.")
		return
	}
	taken, err := h.svc.CheckUsername(r.Context(), username)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}
	jsonOK(w, http.StatusOK, map[string]bool{"available": !taken})
}

// sleepCtx waits d or until ctx is done, whichever comes first.
func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
	case <-ctx.Done():
	}
}

// padSince sleeps until registerMinDuration has elapsed since start; it adds
// nothing when the work already took longer.
func (h *Handler) padSince(ctx context.Context, start time.Time) {
	if rem := h.registerMinDuration - h.now().Sub(start); rem > 0 {
		h.sleep(ctx, rem)
	}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	start := h.now()
	var req RegisterRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonValidationError(w, err)
		return
	}

	ip := clientIP(r)
	ua := r.UserAgent()

	ctx := r.Context()
	pairKey, dniKey := h.registerFailureKeys(req.DNI, ip)
	pairRes, ok, err := h.registerFailures.TryAcquire(ctx, pairKey)
	if err != nil {
		log.Error().Err(err).Msg("register: failure limiter")
		jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}
	if !ok {
		jsonError(w, http.StatusTooManyRequests, "Demasiados intentos de registro fallidos. Intenta más tarde.")
		return
	}
	release := func(l *internalmw.DBLimiter, res internalmw.Reservation) {
		if err := l.Release(ctx, res); err != nil {
			log.Error().Err(err).Msg("register: failure limiter refund")
		}
	}
	dniRes, ok, err := h.registerFailuresPerDNI.TryAcquire(ctx, dniKey)
	if err != nil || !ok {
		release(h.registerFailures, pairRes)
		if err != nil {
			log.Error().Err(err).Msg("register: failure limiter")
			jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
			return
		}
		jsonError(w, http.StatusTooManyRequests, "Demasiados intentos de registro fallidos. Intenta más tarde.")
		return
	}
	// refund returns both reservations after an internal error, which says
	// nothing about the submitted data.
	refund := func() {
		release(h.registerFailures, pairRes)
		release(h.registerFailuresPerDNI, dniRes)
	}

	if err := h.svc.Register(ctx, req, ip, ua); err != nil {
		switch {
		case errors.Is(err, ErrDNINotFound), errors.Is(err, ErrPersonaAlreadyRegistered),
			errors.Is(err, ErrEmailMismatch), errors.Is(err, ErrEmailTaken):
			// Every business rejection gets exactly the success response
			// (see msgRegisterAccepted), and keeps its units like a success.
			h.padSince(ctx, start)
			jsonOK(w, http.StatusAccepted, map[string]string{"message": msgRegisterAccepted})
		default:
			refund()
			jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
		}
		return
	}

	// Accepted: the units are kept, exactly as for a rejection (H128).
	h.padSince(ctx, start)
	jsonOK(w, http.StatusAccepted, map[string]string{"message": msgRegisterAccepted})
}

// VerifyEmail takes the token e-mailed by Register plus the chosen
// credentials and creates the (pending-activation) usuario. Rate-limited per
// IP (route) and per token (verifyAttempts, every attempt counts).
func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req VerifyEmailRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonValidationError(w, err)
		return
	}
	if _, ok, err := h.verifyAttempts.TryAcquire(r.Context(), hashToken(req.Token)); err != nil {
		log.Error().Err(err).Msg("verify_email: attempt limiter")
		jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	} else if !ok {
		jsonError(w, http.StatusTooManyRequests, "Demasiados intentos con este enlace. Intenta más tarde.")
		return
	}
	if err := h.svc.VerifyEmail(r.Context(), req, clientIP(r), r.UserAgent()); err != nil {
		switch {
		case errors.Is(err, ErrInvalidVerifyToken):
			jsonError(w, http.StatusBadRequest, ErrInvalidVerifyToken.Error())
		case errors.Is(err, ErrUsernameTaken):
			jsonError(w, http.StatusConflict, msgVerifyUsernameTaken)
		case errors.Is(err, ErrEmailTaken):
			// The link proves control of the address, so naming the
			// conflict here is not an oracle for the register endpoint.
			jsonError(w, http.StatusConflict, msgVerifyEmailTaken)
		default:
			jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
		}
		return
	}
	jsonOK(w, http.StatusCreated, map[string]string{"message": "Registro exitoso. Tu cuenta está pendiente de activación."})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonValidationError(w, err)
		return
	}

	ip := clientIP(r)
	ua := r.UserAgent()

	resp, err := h.svc.Login(r.Context(), req, ip, ua)
	if err != nil {
		switch {
		case errors.Is(err, ErrAccountLocked):
			jsonError(w, http.StatusTooManyRequests, err.Error())
		case errors.Is(err, ErrAccountPending):
			jsonError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, ErrAccountRejected), errors.Is(err, ErrAccountInactive):
			jsonError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, ErrInvalidCredentials):
			jsonError(w, http.StatusUnauthorized, ErrInvalidCredentials.Error())
		case errors.Is(err, ErrServicioNoDisponible):
			// The driver error may carry connection details: log the op only.
			log.Error().Str("op", "auth.login").Msg("login unavailable: database error")
			jsonError(w, http.StatusServiceUnavailable, ErrServicioNoDisponible.Error())
		default:
			httpx.InternalError(w, "auth.login", err)
		}
		return
	}

	jsonOK(w, http.StatusOK, resp)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		jsonError(w, http.StatusUnauthorized, "No autenticado.")
		return
	}

	tokenStr := r.Header.Get("Authorization")
	if len(tokenStr) > 7 {
		tokenStr = tokenStr[7:]
	}

	_ = h.svc.Logout(r.Context(), claims.UserID, tokenStr, clientIP(r))
	jsonOK(w, http.StatusOK, map[string]string{"message": "Sesión cerrada."})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		jsonError(w, http.StatusUnauthorized, "No autenticado.")
		return
	}
	profile, err := h.svc.repo.GetUserProfileByID(r.Context(), claims.UserID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}
	jsonOK(w, http.StatusOK, profile)
}

func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonValidationError(w, err)
		return
	}

	_ = h.svc.ForgotPassword(r.Context(), req.Email, clientIP(r))
	jsonOK(w, http.StatusOK, map[string]string{"message": "Si el email está registrado, recibirás un enlace para restablecer tu contraseña."})
}

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req ResetPasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := validate.Struct(req); err != nil {
		jsonValidationError(w, err)
		return
	}

	if err := h.svc.ResetPassword(r.Context(), req.Token, req.Password, clientIP(r)); err != nil {
		if errors.Is(err, ErrInvalidToken) {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		jsonError(w, http.StatusInternalServerError, "Error interno del servidor.")
		return
	}

	jsonOK(w, http.StatusOK, map[string]string{"message": "Contraseña restablecida exitosamente."})
}

func (h *Handler) CompleteTour(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		jsonError(w, http.StatusUnauthorized, "No autenticado.")
		return
	}
	if err := h.svc.SetTourCompletado(r.Context(), claims.UserID, true); err != nil {
		jsonError(w, http.StatusInternalServerError, "Error interno.")
		return
	}
	jsonOK(w, http.StatusOK, map[string]bool{"tour_completado": true})
}

func (h *Handler) ResetTour(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromContext(r.Context())
	if claims == nil {
		jsonError(w, http.StatusUnauthorized, "No autenticado.")
		return
	}
	if err := h.svc.SetTourCompletado(r.Context(), claims.UserID, false); err != nil {
		jsonError(w, http.StatusInternalServerError, "Error interno.")
		return
	}
	jsonOK(w, http.StatusOK, map[string]bool{"tour_completado": false})
}
