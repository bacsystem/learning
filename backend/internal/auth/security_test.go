package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	internalmw "github.com/cbaciliod/learning/internal/middleware"
)

// ── H1: register must never touch an existing account's persona ─────────────

func TestRegister_PersonaWithAccount_DoesNotOverwriteEmail(t *testing.T) {
	f := newFixture(t)
	victimEmail := f.email("victim")
	pid, dni := f.persona(victimEmail)
	f.user(pid, f.username("victim"), "Password123!", "usuario", "activo")
	before := f.personaEmailHMAC(pid)

	err := f.svc.Register(context.Background(), RegisterRequest{
		DNI: dni, Email: f.email("attacker"),
	}, "127.0.0.1", "test")
	if !errors.Is(err, ErrPersonaAlreadyRegistered) {
		t.Fatalf("want ErrPersonaAlreadyRegistered, got %v", err)
	}
	if after := f.personaEmailHMAC(pid); after != before {
		t.Fatal("persona email_hmac was overwritten by a failed registration")
	}
	if n := f.countUsuarios(pid); n != 1 {
		t.Fatalf("want 1 usuario, got %d", n)
	}
}

// The HTTP response for an already-registered DNI must be identical to the
// one for an unknown DNI (no enumeration, no 500).
func TestHandlerRegister_AlreadyRegisteredLooksLikeUnknownDNI(t *testing.T) {
	f := newFixture(t)
	pid, dni := f.persona(f.email("victim"))
	f.user(pid, f.username("victim"), "Password123!", "usuario", "activo")
	mux := f.mux()

	do := func(dni string) (int, string) {
		body := fmt.Sprintf(`{"dni":%q,"email":%q,"username":%q,"password":"Password123!","confirm_password":"Password123!"}`,
			dni, f.email("victim"), f.username("other"))
		req := jsonReq(http.MethodPost, "/auth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		return w.Code, w.Body.String()
	}
	codeReg, bodyReg := do(dni)
	codeUnk, bodyUnk := do("00000001")
	if codeReg != codeUnk || bodyReg != bodyUnk {
		t.Fatalf("registered DNI distinguishable: %d %s vs %d %s", codeReg, bodyReg, codeUnk, bodyUnk)
	}
	if codeReg != http.StatusAccepted {
		t.Fatalf("want 202, got %d", codeReg)
	}
}

// Two concurrent registrations for the same DNI: both are accepted (no 500 on
// the partial unique index) and exactly one pending registration remains; its
// token creates exactly one usuario.
func TestRegister_ConcurrentSameDNI_OneWins(t *testing.T) {
	f := newFixture(t)
	pid, dni := f.persona("")
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = f.svc.Register(context.Background(), RegisterRequest{
				DNI: dni, Email: f.email(fmt.Sprintf("c%d", i)),
			}, "127.0.0.1", "test")
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}
	if n := f.countPendientes(pid); n != 1 {
		t.Fatalf("want exactly one pending registration, got %d", n)
	}
	ok := 0
	for i := range errs {
		for _, tok := range f.verifyEmails(f.email(fmt.Sprintf("c%d", i))) {
			if f.verify(tok, f.username(fmt.Sprintf("c%d", i))) == nil {
				ok++
			}
		}
	}
	if ok != 1 || f.countUsuarios(pid) != 1 {
		t.Fatalf("want exactly one registration, got ok=%d usuarios=%d", ok, f.countUsuarios(pid))
	}
}

// ── H4: check-dni masks the e-mail; register requires the e-mail on file ─────

func TestMaskEmail(t *testing.T) {
	cases := map[string]string{
		"juan.perez@gmail.com":   "j***@g***.com",
		"a@b.pe":                 "a***@b***.pe",
		"x@mail.ejercito.mil.pe": "x***@m***.pe",
		"":                       "",
		"noatsign":               "",
	}
	for in, want := range cases {
		if got := maskEmail(in); got != want {
			t.Errorf("maskEmail(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHandlerCheckDNI_ReturnsMaskedEmailOnly(t *testing.T) {
	f := newFixture(t)
	email := f.email("juan.perez")
	_, dni := f.persona(email)

	w := f.checkDNIFrom(f.mux(), f.testIP(), dni)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), email) || strings.Contains(w.Body.String(), "juan.perez") {
		t.Fatalf("check-dni leaks the plaintext email: %s", w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["email_masked"] != "j***@t***.test" {
		t.Fatalf("unexpected body: %v", got)
	}
	if _, ok := got["email"]; ok {
		t.Fatal("response must not contain the email field")
	}
}

func TestHandlerCheckDNI_NoEmailOnFile(t *testing.T) {
	f := newFixture(t)
	_, dni := f.persona("")
	res, err := f.svc.CheckDNI(context.Background(), dni)
	if err != nil {
		t.Fatal(err)
	}
	if res.EmailMasked != "" {
		t.Fatalf("want empty email_masked, got %+v", res)
	}
}

func TestRegister_EmailMismatch_Rejected(t *testing.T) {
	f := newFixture(t)
	pid, dni := f.persona(f.email("owner"))
	before := f.personaEmailHMAC(pid)
	err := f.svc.Register(context.Background(), RegisterRequest{
		DNI: dni, Email: f.email("attacker"),
	}, "127.0.0.1", "test")
	if !errors.Is(err, ErrEmailMismatch) {
		t.Fatalf("want ErrEmailMismatch, got %v", err)
	}
	if f.countUsuarios(pid) != 0 || f.countPendientes(pid) != 0 || f.personaEmailHMAC(pid) != before {
		t.Fatal("mismatched registration must not create a user/pending registration nor change the email")
	}
}

func TestHandlerRegister_EmailMismatch_GenericResponse(t *testing.T) {
	f := newFixture(t)
	_, dni := f.persona(f.email("owner"))
	body := fmt.Sprintf(`{"dni":%q,"email":%q,"username":%q,"password":"Password123!","confirm_password":"Password123!"}`,
		dni, f.email("attacker"), f.username("att"))
	req := jsonReq(http.MethodPost, "/auth/register", strings.NewReader(body))
	w := httptest.NewRecorder()
	f.mux().ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d %s", w.Code, w.Body.String())
	}
	// No oracle: a wrong e-mail must look exactly like an unknown DNI.
	body2 := fmt.Sprintf(`{"dni":"00000001","email":%q,"username":%q,"password":"Password123!","confirm_password":"Password123!"}`,
		f.email("attacker"), f.username("att"))
	w2 := httptest.NewRecorder()
	f.mux().ServeHTTP(w2, jsonReq(http.MethodPost, "/auth/register", strings.NewReader(body2)))
	if w2.Code != w.Code || w2.Body.String() != w.Body.String() {
		t.Fatalf("email mismatch distinguishable from unknown DNI: %d %s vs %d %s", w.Code, w.Body.String(), w2.Code, w2.Body.String())
	}
	if strings.Contains(w.Body.String(), "coincide") {
		t.Fatalf("response reveals the e-mail check: %s", w.Body.String())
	}
}

func TestRegister_EmailMatchesCaseInsensitive_OK(t *testing.T) {
	f := newFixture(t)
	pid, dni := f.persona("Owner@" + strings.ToUpper(f.domain))
	err := f.svc.Register(context.Background(), RegisterRequest{
		DNI: dni, Email: f.email("owner"),
	}, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if f.countPendientes(pid) != 1 {
		t.Fatal("pending registration not created")
	}
}

func TestRegister_NoEmailOnFile_StoresNormalizedSubmittedEmail(t *testing.T) {
	f := newFixture(t)
	pid, dni := f.persona("")
	submitted := "New@" + strings.ToUpper(f.domain)
	if err := f.svc.Register(context.Background(), RegisterRequest{
		DNI: dni, Email: submitted,
	}, "127.0.0.1", "test"); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := f.verify(f.lastVerifyToken(f.email("new")), f.username("new")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	prof, err := f.repo.GetPersonaProfile(context.Background(), pid)
	if err != nil {
		t.Fatal(err)
	}
	if prof.Email != f.email("new") {
		t.Errorf("stored email %q, want normalized %q", prof.Email, f.email("new"))
	}
	if f.personaEmailHMAC(pid) != f.repo.emailHMAC(f.email("new")) {
		t.Error("email_hmac not computed from the normalized email")
	}
}

// ── H2: role/estado come from the DB on every request ────────────────────────

func TestRequireAdmin_DemotedAdminLosesAccess(t *testing.T) {
	f := newFixture(t)
	uid, username := f.activeUser("adm", "admin")
	token := f.login(username)
	mux := f.mux()

	get := func() int {
		req := httptest.NewRequest(http.MethodGet, "/admin/config/sesiones", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		return w.Code
	}
	if c := get(); c != http.StatusOK {
		t.Fatalf("admin before demotion: want 200, got %d", c)
	}
	if _, err := f.db.Exec(`UPDATE usuarios SET rol='usuario' WHERE id=$1`, uid); err != nil {
		t.Fatal(err)
	}
	if c := get(); c != http.StatusForbidden {
		t.Fatalf("demoted admin: want 403, got %d", c)
	}
	if _, err := f.db.Exec(`UPDATE usuarios SET estado='inactivo' WHERE id=$1`, uid); err != nil {
		t.Fatal(err)
	}
	if c := get(); c != http.StatusUnauthorized {
		t.Fatalf("inactive user: want 401, got %d", c)
	}
}

// ── H14: reset tokens are single use and expire ──────────────────────────────

func (f *fixture) resetToken(userID int64, token string, expires time.Time, usado bool) {
	f.t.Helper()
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
	if _, err := f.db.Exec(`INSERT INTO tokens_recuperacion (usuario_id, token_hash, expires_at, usado) VALUES ($1,$2,$3,$4)`,
		userID, hash, expires, usado); err != nil {
		f.t.Fatal(err)
	}
}

func TestResetPassword_ConcurrentSameToken_ExactlyOneSucceeds(t *testing.T) {
	f := newFixture(t)
	uid, _ := f.activeUser("rst", "usuario")
	token := "tok-concurrent-" + f.suffix
	f.resetToken(uid, token, time.Now().Add(time.Hour), false)

	const n = 5
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = f.svc.ResetPassword(context.Background(), token, fmt.Sprintf("NewPassword%d!", i), "127.0.0.1")
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		} else if !errors.Is(err, ErrInvalidToken) {
			t.Errorf("unexpected error: %v", err)
		}
	}
	if ok != 1 {
		t.Fatalf("want exactly 1 successful reset, got %d", ok)
	}
}

func TestResetPassword_UsedToken_Rejected(t *testing.T) {
	f := newFixture(t)
	uid, _ := f.activeUser("used", "usuario")
	token := "tok-used-" + f.suffix
	f.resetToken(uid, token, time.Now().Add(time.Hour), true)
	if err := f.svc.ResetPassword(context.Background(), token, "NewPassword1!", "127.0.0.1"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("want ErrInvalidToken for used token, got %v", err)
	}
}

func TestResetPassword_ExpiredToken_Rejected(t *testing.T) {
	f := newFixture(t)
	uid, _ := f.activeUser("exp", "usuario")
	token := "tok-expired-" + f.suffix
	f.resetToken(uid, token, time.Now().Add(-time.Minute), false)
	if err := f.svc.ResetPassword(context.Background(), token, "NewPassword1!", "127.0.0.1"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("want ErrInvalidToken for expired token, got %v", err)
	}
}

func TestResetPassword_ValidToken_ChangesPasswordAndRevokesSessions(t *testing.T) {
	f := newFixture(t)
	uid, username := f.activeUser("ok", "usuario")
	oldToken := f.login(username)
	token := "tok-ok-" + f.suffix
	f.resetToken(uid, token, time.Now().Add(time.Hour), false)
	if err := f.svc.ResetPassword(context.Background(), token, "BrandNewPass1!", "127.0.0.1"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(oldToken)))
	if s, err := f.repo.FindActiveSesion(context.Background(), hash); err != nil || s != nil {
		t.Fatalf("old session must be revoked, got %+v %v", s, err)
	}
	if _, err := f.svc.Login(context.Background(), LoginRequest{Username: username, Password: "BrandNewPass1!"}, "127.0.0.1", "t"); err != nil {
		t.Fatalf("login with new password: %v", err)
	}
	if err := f.svc.ResetPassword(context.Background(), token, "Another1234!", "127.0.0.1"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("token reuse: want ErrInvalidToken, got %v", err)
	}
}

// ── H15: no self-deactivation, never zero active admins ──────────────────────

func TestUpdateEstado_CannotDeactivateSelf(t *testing.T) {
	f := newFixture(t)
	uid, _ := f.activeUser("self", "admin")
	for _, estado := range []string{"inactivo", "rechazado"} {
		_, err := f.svc.UpdateEstado(context.Background(), uid, UpdateUsuarioEstadoRequest{Estado: estado}, uid, "")
		if !errors.Is(err, ErrCannotChangeOwnEstado) {
			t.Fatalf("%s self: want ErrCannotChangeOwnEstado, got %v", estado, err)
		}
	}
	var estado string
	_ = f.db.QueryRow(`SELECT estado FROM usuarios WHERE id=$1`, uid).Scan(&estado)
	if estado != "activo" {
		t.Fatalf("estado changed to %s", estado)
	}
}

// The concurrent last-admin test lives in last_admin_test.go: it runs on an
// isolated schema (never skips) and holds the first transaction after the
// lock so the race is deterministic.

func TestHandlerUpdateEstado_Self_409(t *testing.T) {
	f := newFixture(t)
	uid, username := f.activeUser("adm409", "admin")
	token := f.login(username)
	req := jsonReq(http.MethodPatch, fmt.Sprintf("/admin/usuarios/%d/estado", uid),
		strings.NewReader(`{"estado":"inactivo","observacion":""}`))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	f.mux().ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d %s", w.Code, w.Body.String())
	}
}

// ── H16: bcrypt limit is 72 bytes, not runes ─────────────────────────────────

func TestValidation_PasswordOver72Bytes_Rejected(t *testing.T) {
	pw := strings.Repeat("ñ", 40) // 40 runes, 80 bytes
	reg := VerifyEmailRequest{Token: "x", Username: "abcd", Password: pw, ConfirmPassword: pw}
	if err := validate.Struct(reg); err == nil {
		t.Error("verify-email: 80-byte password passed validation")
	}
	rst := ResetPasswordRequest{Token: "x", Password: pw, ConfirmPassword: pw}
	if err := validate.Struct(rst); err == nil {
		t.Error("reset: 80-byte password passed validation")
	}
	ok := strings.Repeat("a", 72)
	if err := validate.Struct(ResetPasswordRequest{Token: "x", Password: ok, ConfirmPassword: ok}); err != nil {
		t.Errorf("72-byte password rejected: %v", err)
	}
}

func TestHandlerVerifyEmail_PasswordOver72Bytes_422(t *testing.T) {
	// Use a real, live token so the request would reach bcrypt (and fail
	// with 500) if validation let the password through.
	f := newFixture(t)
	pid, dni := f.persona(f.email("pw"))
	if err := f.register(dni, f.email("pw")); err != nil {
		t.Fatal(err)
	}
	pw := strings.Repeat("ñ", 40)
	body := fmt.Sprintf(`{"token":%q,"username":%q,"password":%q,"confirm_password":%q}`,
		f.lastVerifyToken(f.email("pw")), f.username("pw"), pw, pw)
	w := httptest.NewRecorder()
	NewHandler(f.svc).VerifyEmail(w, jsonReq(http.MethodPost, "/auth/verify-email", strings.NewReader(body)))
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d %s", w.Code, w.Body.String())
	}
	if f.countUsuarios(pid) != 0 {
		t.Fatal("user must not be created")
	}
}

// ── H17: oversized JSON body → 413 from the handler ──────────────────────────

func TestHandlerLogin_OversizedBody_413(t *testing.T) {
	h, _ := testSetup(t)
	big := `{"username":"` + strings.Repeat("a", 2<<20) + `","password":"x"}`
	req := jsonReq(http.MethodPost, "/auth/login", strings.NewReader(big))
	w := httptest.NewRecorder()
	req.Body = http.MaxBytesReader(w, req.Body, 1<<20)
	h.Login(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d", w.Code)
	}
}

// ── H18: login must fail if the session cannot be stored ─────────────────────

func TestLogin_SessionInsertFails_ReturnsError(t *testing.T) {
	f := newFixture(t)
	_, username := f.activeUser("ses", "usuario")
	// sesiones.ip is varchar(45): a longer value makes the INSERT fail.
	_, err := f.svc.Login(context.Background(), LoginRequest{Username: username, Password: "Password123!"},
		strings.Repeat("9", 60), "test")
	if err == nil {
		t.Fatal("login returned a token although the session was not stored")
	}
}

// ── H48 / H47: brute-force lockout (seeded user) ─────────────────────────────

func TestLogin_BruteForce_Lockout(t *testing.T) {
	f := newFixture(t)
	_, username := f.activeUser("bf", "usuario")
	var err error
	for i := 0; i < maxLoginAttempts; i++ {
		_, err = f.svc.Login(context.Background(), LoginRequest{Username: username, Password: "wrong"}, "127.0.0.1", "t")
	}
	if !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("after %d failures want ErrAccountLocked, got %v", maxLoginAttempts, err)
	}
	// Correct password is refused while locked.
	if _, err := f.svc.Login(context.Background(), LoginRequest{Username: username, Password: "Password123!"}, "127.0.0.1", "t"); !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("locked account: want ErrAccountLocked, got %v", err)
	}
}

func TestLogin_ConcurrentFailures_LockAccount(t *testing.T) {
	f := newFixture(t)
	uid, username := f.activeUser("cbf", "usuario")
	var wg sync.WaitGroup
	for i := 0; i < 2*maxLoginAttempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = f.svc.Login(context.Background(), LoginRequest{Username: username, Password: "wrong"}, "127.0.0.1", "t")
		}()
	}
	wg.Wait()
	var locked bool
	_ = f.db.QueryRow(`SELECT bloqueado_hasta IS NOT NULL AND bloqueado_hasta > NOW() FROM usuarios WHERE id=$1`, uid).Scan(&locked)
	if !locked {
		t.Fatal("account not locked after concurrent failed attempts")
	}
}

func TestHandlerLogin_BruteForce_Lockout429(t *testing.T) {
	f := newFixture(t)
	_, username := f.activeUser("hbf", "usuario")
	h := NewHandler(f.svc)
	code := 0
	for i := 0; i < maxLoginAttempts; i++ {
		body := fmt.Sprintf(`{"username":%q,"password":"wrong"}`, username)
		w := httptest.NewRecorder()
		h.Login(w, jsonReq(http.MethodPost, "/auth/login", bytes.NewBufferString(body)))
		code = w.Code
	}
	if code != http.StatusTooManyRequests {
		t.Fatalf("want 429 after %d failures, got %d", maxLoginAttempts, code)
	}
}

// ── H49: e-mail normalization and deterministic forgot-password ──────────────

func TestEmailHMAC_Normalizes(t *testing.T) {
	repo := testRepo(t)
	if repo.emailHMAC("  Juan@Example.COM ") != repo.emailHMAC("juan@example.com") {
		t.Fatal("emailHMAC must normalize case and surrounding spaces")
	}
}

func (f *fixture) tokensFor(userID int64) int {
	f.t.Helper()
	var n int
	_ = f.db.QueryRow(`SELECT COUNT(*) FROM tokens_recuperacion WHERE usuario_id=$1`, userID).Scan(&n)
	return n
}

func TestForgotPassword_PicksTheActiveUser(t *testing.T) {
	f := newFixture(t)
	email := f.email("shared")
	p1, _ := f.persona(email)
	p2, _ := f.persona(email)
	pending := f.user(p1, f.username("pend"), "Password123!", "usuario", "pendiente")
	active := f.user(p2, f.username("act"), "Password123!", "usuario", "activo")
	if err := f.svc.ForgotPassword(context.Background(), strings.ToUpper(email), "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if f.tokensFor(active) != 1 || f.tokensFor(pending) != 0 {
		t.Fatalf("want token only for the active user, got active=%d pending=%d", f.tokensFor(active), f.tokensFor(pending))
	}
}

func TestForgotPassword_AmbiguousEmail_SendsNothing(t *testing.T) {
	f := newFixture(t)
	email := f.email("dup")
	p1, _ := f.persona(email)
	p2, _ := f.persona(email)
	u1 := f.user(p1, f.username("dupa"), "Password123!", "usuario", "activo")
	u2 := f.user(p2, f.username("dupb"), "Password123!", "usuario", "activo")
	if err := f.svc.ForgotPassword(context.Background(), email, "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if f.tokensFor(u1)+f.tokensFor(u2) != 0 {
		t.Fatal("ambiguous email must not issue any reset token")
	}
	var queued int
	_ = f.db.QueryRow(`SELECT COUNT(*) FROM email_queue WHERE to_address=$1`, email).Scan(&queued)
	if queued != 0 {
		t.Fatal("ambiguous email must not enqueue any message")
	}
}

// ── H50: config parse errors propagate ───────────────────────────────────────

func TestGetMaxSesiones_InvalidValue_ReturnsError(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()
	var orig string
	hadRow := repo.db.QueryRow(`SELECT valor FROM configuracion_sistema WHERE clave='max_sesiones'`).Scan(&orig) == nil
	t.Cleanup(func() {
		if hadRow {
			if _, err := repo.db.Exec(`UPDATE configuracion_sistema SET valor=$1 WHERE clave='max_sesiones'`, orig); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		} else {
			if _, err := repo.db.Exec(`DELETE FROM configuracion_sistema WHERE clave='max_sesiones'`); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
	})
	if _, err := repo.db.Exec(`INSERT INTO configuracion_sistema (clave, valor) VALUES ('max_sesiones','abc')
		ON CONFLICT (clave) DO UPDATE SET valor='abc'`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetMaxSesiones(ctx); err == nil {
		t.Fatal("want error for non-numeric max_sesiones")
	}
}

// Failed-registration limit per (DNI, client IP): an attacker guessing the
// e-mail for a DNI from one IP is blocked after registerFailuresPerDNIAndIP
// failures, while the real owner — same DNI, different IP, correct e-mail —
// can still register. Accepted attempts count too (H128).
func TestHandlerRegister_FailureLimitPerDNIAndIP(t *testing.T) {
	f := newFixture(t)
	_, dni := f.persona(f.email("owner"))
	h := NewHandler(f.svc)

	do := func(email, username, remote string) (int, string) {
		body := fmt.Sprintf(`{"dni":%q,"email":%q,"username":%q,"password":"Password123!","confirm_password":"Password123!"}`,
			dni, email, username)
		req := jsonReq(http.MethodPost, "/auth/register", strings.NewReader(body))
		req.RemoteAddr = remote
		w := httptest.NewRecorder()
		h.Register(w, req)
		return w.Code, w.Body.String()
	}

	const attacker = "203.0.113.66:1"
	for i := 0; i < registerFailuresPerDNIAndIP; i++ {
		if c, b := do(f.email(fmt.Sprintf("guess%d", i)), f.username("att"), attacker); c != http.StatusAccepted {
			t.Fatalf("guess %d: want 202, got %d %s", i+1, c, b)
		}
	}
	if c, _ := do(f.email("guessX"), f.username("att"), attacker); c != http.StatusTooManyRequests {
		t.Fatalf("attacker after %d failures: want 429, got %d", registerFailuresPerDNIAndIP, c)
	}
	// Even the correct e-mail is refused from the blocked (DNI, IP) pair.
	if c, _ := do(f.email("owner"), f.username("att"), attacker); c != http.StatusTooManyRequests {
		t.Fatalf("blocked pair: want 429, got %d", c)
	}
	// The owner, from another IP, is not affected.
	if c, b := do(f.email("owner"), f.username("own"), "198.51.100.7:1"); c != http.StatusAccepted {
		t.Fatalf("owner from another IP: want 202, got %d %s", c, b)
	}
}

// H128: an accepted registration is charged to both counters like a
// rejected one (see TestHandlerRegister_LimiterChargesSuccessLikeFailure);
// it used to be refunded, which made the limiter an oracle for the e-mail.
func TestHandlerRegister_SuccessIsChargedLikeFailure(t *testing.T) {
	f := newFixture(t)
	h := NewHandler(f.svc)
	_, dni := f.persona(f.email("ok"))
	body := fmt.Sprintf(`{"dni":%q,"email":%q}`, dni, f.email("ok"))
	req := jsonReq(http.MethodPost, "/auth/register", strings.NewReader(body))
	req.RemoteAddr = "198.51.100.8:1"
	w := httptest.NewRecorder()
	h.Register(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d %s", w.Code, w.Body.String())
	}
	if n := f.countPendientesByDNI(dni); n != 1 {
		t.Fatalf("registration not stored: %d pending", n)
	}
	pairKey, dniKey := h.registerFailureKeys(dni, "198.51.100.8")
	if n, err := h.registerFailures.Count(context.Background(), pairKey); err != nil || n != 1 {
		t.Fatalf("an accepted registration must be charged to the (dni, ip) counter (count=%d, err=%v)", n, err)
	}
	if n, err := h.registerFailuresPerDNI.Count(context.Background(), dniKey); err != nil || n != 1 {
		t.Fatalf("an accepted registration must be charged to the per-DNI counter (count=%d, err=%v)", n, err)
	}
}

// Global per-DNI failure limit: failures spread over many IPs are bounded.
func TestHandlerRegister_GlobalPerDNIFailureLimit(t *testing.T) {
	f := newFixture(t)
	_, dni := f.persona(f.email("owner"))
	h := NewHandler(f.svc)
	const limit = 3
	h.registerFailuresPerDNI = internalmw.NewDBLimiter(f.db.DB, "reg_dni:", limit, registerFailurePerDNIWindow)

	do := func(email, remote string) int {
		body := fmt.Sprintf(`{"dni":%q,"email":%q,"username":%q,"password":"Password123!","confirm_password":"Password123!"}`,
			dni, email, f.username("att"))
		req := jsonReq(http.MethodPost, "/auth/register", strings.NewReader(body))
		req.RemoteAddr = remote
		w := httptest.NewRecorder()
		h.Register(w, req)
		return w.Code
	}
	for i := 0; i < limit; i++ { // one failure from each of several IPs
		if c := do(f.email(fmt.Sprintf("g%d", i)), fmt.Sprintf("203.0.113.%d:1", i+1)); c != http.StatusAccepted {
			t.Fatalf("failure %d: want 202, got %d", i+1, c)
		}
	}
	if c := do(f.email("gX"), "203.0.113.200:1"); c != http.StatusTooManyRequests {
		t.Fatalf("fresh IP after %d global failures: want 429, got %d", limit, c)
	}
	// The blocked global attempt must not have consumed the fresh IP's pair quota.
	pairKey, _ := h.registerFailureKeys(dni, "203.0.113.200")
	if n, err := h.registerFailures.Count(context.Background(), pairKey); err != nil || n != 0 {
		t.Fatalf("pair quota leaked when the global limiter rejected (count=%d, err=%v)", n, err)
	}
}

// TOCTOU: concurrent failing registrations for the same (DNI, IP) can never
// get past the limiter more than registerFailuresPerDNIAndIP times.
func TestHandlerRegister_ConcurrentFailuresBoundedByLimit(t *testing.T) {
	f := newFixture(t)
	_, dni := f.persona(f.email("owner"))
	h := NewHandler(f.svc)

	const n = 3 * registerFailuresPerDNIAndIP
	var wg sync.WaitGroup
	var mu sync.Mutex
	passed := 0
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := fmt.Sprintf(`{"dni":%q,"email":%q,"username":%q,"password":"Password123!","confirm_password":"Password123!"}`,
				dni, f.email(fmt.Sprintf("c%d", i)), f.username("att"))
			req := jsonReq(http.MethodPost, "/auth/register", strings.NewReader(body))
			req.RemoteAddr = "203.0.113.77:1"
			w := httptest.NewRecorder()
			h.Register(w, req)
			if w.Code != http.StatusTooManyRequests {
				mu.Lock()
				passed++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if passed > registerFailuresPerDNIAndIP {
		t.Fatalf("%d concurrent attempts got past the limiter, max allowed %d", passed, registerFailuresPerDNIAndIP)
	}
}

// The atomic UPDATE in ResetPasswordWithToken is the authority (the service's
// pre-check is only an optimisation): it must reject used and expired tokens
// on its own.
func TestRepository_ResetPasswordWithToken_RejectsUsedAndExpired(t *testing.T) {
	f := newFixture(t)
	uid, _ := f.activeUser("rpw", "usuario")
	cases := map[string]struct {
		expires time.Time
		usado   bool
	}{
		"used":    {time.Now().Add(time.Hour), true},
		"expired": {time.Now().Add(-time.Minute), false},
	}
	for name, c := range cases {
		token := "tok-repo-" + name + "-" + f.suffix
		f.resetToken(uid, token, c.expires, c.usado)
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
		if _, err := f.repo.ResetPasswordWithToken(context.Background(), hash, "x"); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("%s token: want ErrInvalidToken, got %v", name, err)
		}
	}
}
