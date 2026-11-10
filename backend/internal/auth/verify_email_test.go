package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// ── Registro en dos pasos: register {dni,email} → verify {token,credenciales} ─

var verifyLinkRe = regexp.MustCompile(`/auth/verify-email\?token=([A-Za-z0-9_-]+)`)

// verifyEmails returns the tokens of the verification e-mails queued for to
// (oldest first).
func (f *fixture) verifyEmails(to string) []string {
	f.t.Helper()
	var bodies []string
	if err := f.db.Select(&bodies, `SELECT body_html FROM email_queue WHERE to_address=$1 ORDER BY id`, to); err != nil {
		f.t.Fatal(err)
	}
	var tokens []string
	for _, b := range bodies {
		if m := verifyLinkRe.FindStringSubmatch(b); m != nil {
			tokens = append(tokens, m[1])
		}
	}
	return tokens
}

// lastVerifyToken returns the token of the newest verification e-mail sent to to.
func (f *fixture) lastVerifyToken(to string) string {
	f.t.Helper()
	tokens := f.verifyEmails(to)
	if len(tokens) == 0 {
		f.t.Fatalf("no verification e-mail queued for %s", to)
	}
	return tokens[len(tokens)-1]
}

func (f *fixture) countPendientes(personaID int64) int {
	f.t.Helper()
	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM registros_pendientes WHERE persona_id=$1 AND consumed_at IS NULL`, personaID).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

func (f *fixture) register(dni, email string) error {
	f.t.Helper()
	return f.svc.Register(context.Background(), RegisterRequest{DNI: dni, Email: email}, "127.0.0.1", "test")
}

// verify completes a registration with username and password "Password123!".
func (f *fixture) verify(token, username string) error {
	return f.svc.VerifyEmail(context.Background(), VerifyEmailRequest{
		Token: token, Username: username, Password: "Password123!", ConfirmPassword: "Password123!",
	}, "127.0.0.1", "test")
}

func (f *fixture) deleteUsuarios(personaID int64) {
	f.t.Helper()
	for _, q := range []string{
		`DELETE FROM audit_log WHERE usuario_id IN (SELECT id FROM usuarios WHERE persona_id=$1)`,
		`DELETE FROM audit_log WHERE entidad='usuarios' AND entidad_id IN (SELECT id FROM usuarios WHERE persona_id=$1)`,
		`DELETE FROM usuarios WHERE persona_id=$1`,
	} {
		if _, err := f.db.Exec(q, personaID); err != nil {
			f.t.Fatalf("%v (%s)", err, q)
		}
	}
}

// Register stores no credentials and creates no usuario; verify sets them.
func TestRegister_StoresNoCredentials_VerifySetsThem(t *testing.T) {
	f := newFixture(t)
	email := f.email("owner")
	pid, dni := f.persona(email)
	if err := f.register(dni, email); err != nil {
		t.Fatalf("register: %v", err)
	}
	if n := f.countUsuarios(pid); n != 0 {
		t.Fatalf("register must not create the usuario, got %d", n)
	}
	if n := f.countPendientes(pid); n != 1 {
		t.Fatalf("want 1 pending registration, got %d", n)
	}
	var cols int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns
		WHERE table_name='registros_pendientes' AND column_name IN ('username','password_hash')`).Scan(&cols); err != nil {
		t.Fatal(err)
	}
	if cols != 0 {
		t.Fatal("registros_pendientes must not hold credentials")
	}
	token := f.lastVerifyToken(email)
	var raw, hashed int
	_ = f.db.QueryRow(`SELECT COUNT(*) FILTER (WHERE token_hash=$1), COUNT(*) FILTER (WHERE token_hash=$2)
		FROM registros_pendientes`, token, hashToken(token)).Scan(&raw, &hashed)
	if raw != 0 || hashed != 1 {
		t.Fatalf("token must be stored only as its hash (raw=%d hashed=%d)", raw, hashed)
	}

	username := f.username("own")
	if err := f.verify(token, username); err != nil {
		t.Fatalf("verify: %v", err)
	}
	var estado, got string
	if err := f.db.QueryRow(`SELECT estado, username FROM usuarios WHERE persona_id=$1`, pid).Scan(&estado, &got); err != nil {
		t.Fatal(err)
	}
	if estado != "pendiente" || got != username {
		t.Fatalf("got estado=%q username=%q", estado, got)
	}
	if _, err := f.db.Exec(`UPDATE usuarios SET estado='activo' WHERE persona_id=$1`, pid); err != nil {
		t.Fatal(err)
	}
	f.login(username) // the password chosen at verify time works
	if n := f.countPendientes(pid); n != 0 {
		t.Fatal("pending registration not consumed")
	}
}

func TestVerifyEmail_TokenSingleUse_Concurrent(t *testing.T) {
	f := newFixture(t)
	email := f.email("owner")
	pid, dni := f.persona(email)
	if err := f.register(dni, email); err != nil {
		t.Fatal(err)
	}
	token := f.lastVerifyToken(email)

	const n = 6
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = f.verify(token, f.username(fmt.Sprintf("u%d", i)))
		}(i)
	}
	close(start)
	wg.Wait()
	ok := 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, ErrInvalidVerifyToken):
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if ok != 1 || f.countUsuarios(pid) != 1 {
		t.Fatalf("want exactly one successful verification, got ok=%d usuarios=%d", ok, f.countUsuarios(pid))
	}
}

// The token itself is single use, independently of the persona's usuario: if
// the usuario is later removed (e.g. by an admin), the old link must not
// recreate it.
func TestVerifyEmail_UsedToken_RejectedEvenWithoutUsuario(t *testing.T) {
	f := newFixture(t)
	email := f.email("owner")
	pid, dni := f.persona(email)
	if err := f.register(dni, email); err != nil {
		t.Fatal(err)
	}
	token := f.lastVerifyToken(email)
	if err := f.verify(token, f.username("own")); err != nil {
		t.Fatal(err)
	}
	f.deleteUsuarios(pid)
	if err := f.verify(token, f.username("again")); !errors.Is(err, ErrInvalidVerifyToken) {
		t.Fatalf("used token: want ErrInvalidVerifyToken, got %v", err)
	}
	if f.countUsuarios(pid) != 0 {
		t.Fatal("a used token recreated the usuario")
	}
}

func TestVerifyEmail_ExpiredToken_Rejected(t *testing.T) {
	f := newFixture(t)
	email := f.email("owner")
	pid, dni := f.persona(email)
	if err := f.register(dni, email); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(`UPDATE registros_pendientes SET expires_at=NOW()-INTERVAL '1 minute' WHERE persona_id=$1`, pid); err != nil {
		t.Fatal(err)
	}
	if err := f.verify(f.lastVerifyToken(email), f.username("own")); !errors.Is(err, ErrInvalidVerifyToken) {
		t.Fatalf("want ErrInvalidVerifyToken, got %v", err)
	}
	if f.countUsuarios(pid) != 0 {
		t.Fatal("expired token created a usuario")
	}
}

func TestVerifyEmail_ExpiresIn24h(t *testing.T) {
	f := newFixture(t)
	email := f.email("owner")
	pid, dni := f.persona(email)
	if err := f.register(dni, email); err != nil {
		t.Fatal(err)
	}
	var d float64
	if err := f.db.QueryRow(`SELECT EXTRACT(EPOCH FROM expires_at-created_at) FROM registros_pendientes WHERE persona_id=$1`, pid).Scan(&d); err != nil {
		t.Fatal(err)
	}
	if got := time.Duration(d) * time.Second; got < 23*time.Hour || got > 25*time.Hour {
		t.Fatalf("want ~24h expiry, got %v", got)
	}
}

// A second register for the same persona invalidates the first token: only
// the newest link works.
func TestRegister_SecondRequestInvalidatesFirstToken(t *testing.T) {
	f := newFixture(t)
	email := f.email("owner")
	pid, dni := f.persona(email)
	if err := f.register(dni, email); err != nil {
		t.Fatal(err)
	}
	first := f.lastVerifyToken(email)
	if err := f.register(dni, email); err != nil {
		t.Fatal(err)
	}
	second := f.lastVerifyToken(email)
	if first == second {
		t.Fatal("second registration must issue a new token")
	}
	if n := f.countPendientes(pid); n != 1 {
		t.Fatalf("want 1 active pending registration, got %d", n)
	}
	if err := f.verify(first, f.username("att")); !errors.Is(err, ErrInvalidVerifyToken) {
		t.Fatalf("old token: want ErrInvalidVerifyToken, got %v", err)
	}
	if err := f.verify(second, f.username("own")); err != nil {
		t.Fatalf("new token: %v", err)
	}
}

// An attacker who knows the DNI and the e-mail on file can register as often
// as the caps allow, but never has credentials of their own waiting: every
// link goes to the owner's inbox, and the account gets the credentials the
// owner types when opening it.
func TestRegister_AttackerCannotLeaveLiveCredentials(t *testing.T) {
	f := newFixture(t)
	email := f.email("owner")
	pid, dni := f.persona(email)
	if err := f.svc.Register(context.Background(), RegisterRequest{DNI: dni, Email: strings.ToUpper(email)}, "203.0.113.1", "attacker"); err != nil {
		t.Fatal(err)
	}
	if got := f.verifyEmails(email); len(got) != 1 {
		t.Fatalf("the link must go to the address on file, got %d e-mails", len(got))
	}
	if f.countUsuarios(pid) != 0 {
		t.Fatal("register must not create an account")
	}
	if err := f.verify(f.lastVerifyToken(email), f.username("own")); err != nil {
		t.Fatal(err)
	}
	var username string
	if err := f.db.QueryRow(`SELECT username FROM usuarios WHERE persona_id=$1`, pid).Scan(&username); err != nil {
		t.Fatal(err)
	}
	if username != f.username("own") {
		t.Fatalf("account got %q, want the owner's choice", username)
	}
}

// Expired pending rows of a persona are deleted when a new one is created.
func TestRegister_CleansExpiredPendingLazily(t *testing.T) {
	f := newFixture(t)
	email := f.email("owner")
	pid, dni := f.persona(email)
	if _, err := f.db.Exec(`INSERT INTO registros_pendientes (persona_id,email,email_hmac,token_hash,expires_at,created_at,consumed_at)
		VALUES ($1,'x','x',$2,NOW()-INTERVAL '2 days',NOW()-INTERVAL '3 days',NOW()-INTERVAL '3 days')`, pid, "old-"+f.suffix); err != nil {
		t.Fatal(err)
	}
	if err := f.register(dni, email); err != nil {
		t.Fatal(err)
	}
	var n, old int
	if err := f.db.QueryRow(`SELECT COUNT(*), COUNT(*) FILTER (WHERE token_hash=$2) FROM registros_pendientes WHERE persona_id=$1`,
		pid, "old-"+f.suffix).Scan(&n, &old); err != nil {
		t.Fatal(err)
	}
	if n != 1 || old != 0 {
		t.Fatalf("expired rows not cleaned: %d rows, %d old", n, old)
	}
}

// Username taken between register and verify → 409, and the token stays
// usable so the user can retry with another username.
func TestVerifyEmail_UsernameTaken_409_TokenStillUsable(t *testing.T) {
	f := newFixture(t)
	email := f.email("owner")
	pid, dni := f.persona(email)
	if err := f.register(dni, email); err != nil {
		t.Fatal(err)
	}
	taken := f.username("dup")
	other, _ := f.persona(f.email("other"))
	f.user(other, taken, "Password123!", "usuario", "activo")
	token := f.lastVerifyToken(email)

	post := func(username string) (int, string) {
		body := fmt.Sprintf(`{"token":%q,"username":%q,"password":"Password123!","confirm_password":"Password123!"}`, token, username)
		w := httptest.NewRecorder()
		f.mux().ServeHTTP(w, jsonReq(http.MethodPost, "/auth/verify-email", strings.NewReader(body)))
		return w.Code, w.Body.String()
	}
	if c, b := post(taken); c != http.StatusConflict || !strings.Contains(b, "El nombre de usuario ya está en uso") {
		t.Fatalf("want 409 username taken, got %d %s", c, b)
	}
	if f.countUsuarios(pid) != 0 || f.countPendientes(pid) != 1 {
		t.Fatal("a failed verification must not create the usuario nor consume the token")
	}
	if c, b := post(f.username("own")); c != http.StatusCreated {
		t.Fatalf("retry with another username: want 201, got %d %s", c, b)
	}
}

// The verification e-mail goes to the address on file, and — with no e-mail
// on file — to the submitted (normalized) address, which is only written to
// the persona once the link is used.
func TestRegister_VerificationEmailRecipient(t *testing.T) {
	t.Run("email on file", func(t *testing.T) {
		f := newFixture(t)
		onFile := f.email("owner")
		_, dni := f.persona(onFile)
		if err := f.register(dni, strings.ToUpper(onFile)); err != nil {
			t.Fatal(err)
		}
		if len(f.verifyEmails(onFile)) != 1 {
			t.Fatalf("verification e-mail not sent to the address on file %s", onFile)
		}
		if len(f.verifyEmails(strings.ToUpper(onFile))) != 0 {
			t.Fatal("verification e-mail sent to the submitted spelling, not the address on file")
		}
	})
	t.Run("no email on file", func(t *testing.T) {
		f := newFixture(t)
		pid, dni := f.persona("")
		submitted := "New@" + strings.ToUpper(f.domain)
		if err := f.register(dni, submitted); err != nil {
			t.Fatal(err)
		}
		if f.personaEmailHMAC(pid) != "" {
			t.Fatal("persona e-mail must not be written before verification")
		}
		if err := f.verify(f.lastVerifyToken(f.email("new")), f.username("new")); err != nil {
			t.Fatal(err)
		}
		prof, err := f.repo.GetPersonaProfile(context.Background(), pid)
		if err != nil {
			t.Fatal(err)
		}
		if prof.Email != f.email("new") || f.personaEmailHMAC(pid) != f.repo.emailHMAC(f.email("new")) {
			t.Fatalf("persona e-mail after verify = %q", prof.Email)
		}
	})
}

// The stored pending e-mail is encrypted, never in clear.
func TestRegister_PendingEmailEncrypted(t *testing.T) {
	f := newFixture(t)
	pid, dni := f.persona("")
	if err := f.register(dni, f.email("new")); err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := f.db.QueryRow(`SELECT email FROM registros_pendientes WHERE persona_id=$1`, pid).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == "" || strings.Contains(stored, f.domain) {
		t.Fatalf("pending e-mail must be stored encrypted, got %q", stored)
	}
}

// HTTP: register answers 202 with one generic body for success and every
// business rejection; verify answers 201 / 400 / 422.
func TestHandlerRegister_202Generic_AndVerifyFlow(t *testing.T) {
	f := newFixture(t)
	email := f.email("owner")
	pid, dni := f.persona(email)
	mux := f.mux()
	post := func(path, body, remote string) (int, string) {
		req := jsonReq(http.MethodPost, path, strings.NewReader(body))
		req.RemoteAddr = remote
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		return w.Code, w.Body.String()
	}
	reg := func(dni, email string) string { return fmt.Sprintf(`{"dni":%q,"email":%q}`, dni, email) }
	cOK, bOK := post("/auth/register", reg(dni, email), "198.51.100.20:1")
	cBad, bBad := post("/auth/register", reg(dni, f.email("attacker")), "198.51.100.21:1")
	cUnk, bUnk := post("/auth/register", reg("00000001", email), "198.51.100.22:1")
	if cOK != http.StatusAccepted || !strings.Contains(bOK, "Si los datos son correctos") {
		t.Fatalf("success: want 202 generic, got %d %s", cOK, bOK)
	}
	if cBad != cOK || bBad != bOK || cUnk != cOK || bUnk != bOK {
		t.Fatalf("responses distinguishable: ok=%d %s mismatch=%d %s unknown=%d %s", cOK, bOK, cBad, bBad, cUnk, bUnk)
	}

	ver := func(token, username string) string {
		return fmt.Sprintf(`{"token":%q,"username":%q,"password":"Password123!","confirm_password":"Password123!"}`, token, username)
	}
	if c, b := post("/auth/verify-email", ver("nope-"+f.suffix, f.username("own")), "198.51.100.23:1"); c != http.StatusBadRequest {
		t.Fatalf("bad token: want 400, got %d %s", c, b)
	}
	if c, b := post("/auth/verify-email", `{"token":"x","username":"ab","password":"short","confirm_password":"short"}`, "198.51.100.23:1"); c != http.StatusUnprocessableEntity {
		t.Fatalf("invalid credentials shape: want 422, got %d %s", c, b)
	}
	tok := f.lastVerifyToken(email)
	if c, b := post("/auth/verify-email", ver(tok, f.username("own")), "198.51.100.23:1"); c != http.StatusCreated || !strings.Contains(b, "pendiente de activación") {
		t.Fatalf("verify: want 201 pendiente, got %d %s", c, b)
	}
	if f.countUsuarios(pid) != 1 {
		t.Fatal("usuario not created by verify")
	}
	if c, _ := post("/auth/verify-email", ver(tok, f.username("x")), "198.51.100.24:1"); c != http.StatusBadRequest {
		t.Fatalf("reused token: want 400, got %d", c)
	}
}

// Verify attempts are limited per token (all outcomes count).
func TestHandlerVerifyEmail_PerTokenLimit(t *testing.T) {
	f := newFixture(t)
	h := NewHandler(f.svc)
	token := "probe-" + f.suffix
	t.Cleanup(func() {
		_, _ = f.db.Exec(`DELETE FROM auth_intentos_fallidos WHERE clave=$1`, "verify_tok:"+hashToken(token))
	})
	do := func(i int) int {
		body := fmt.Sprintf(`{"token":%q,"username":%q,"password":"Password123!","confirm_password":"Password123!"}`, token, f.username(fmt.Sprintf("p%d", i)))
		req := jsonReq(http.MethodPost, "/auth/verify-email", strings.NewReader(body))
		req.RemoteAddr = fmt.Sprintf("203.0.113.%d:1", i+1) // different IPs
		w := httptest.NewRecorder()
		h.VerifyEmail(w, req)
		return w.Code
	}
	for i := 0; i < verifyAttemptsPerToken; i++ {
		if c := do(i); c != http.StatusBadRequest {
			t.Fatalf("attempt %d: want 400, got %d", i+1, c)
		}
	}
	if c := do(verifyAttemptsPerToken); c != http.StatusTooManyRequests {
		t.Fatalf("over the per-token limit: want 429, got %d", c)
	}
}

// check-dni keeps answering 404 for a persona with a usuario, and 200 for a
// persona with only a pending registration (the owner can register again).
func TestHandlerCheckDNI_PendingVsRegistered(t *testing.T) {
	f := newFixture(t)
	email := f.email("owner")
	_, dni := f.persona(email)
	if err := f.register(dni, email); err != nil {
		t.Fatal(err)
	}
	ip := f.testIP()
	get := func() int { return f.checkDNIFrom(f.mux(), ip, dni).Code }
	if c := get(); c != http.StatusOK {
		t.Fatalf("pending only: want 200, got %d", c)
	}
	if err := f.verify(f.lastVerifyToken(email), f.username("own")); err != nil {
		t.Fatal(err)
	}
	if c := get(); c != http.StatusNotFound {
		t.Fatalf("with usuario: want 404, got %d", c)
	}
}
