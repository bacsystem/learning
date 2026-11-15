package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// storedIdioma reads usuarios.idioma straight from the table.
func (f *fixture) storedIdioma(userID int64) string {
	f.t.Helper()
	var idioma string
	if err := f.db.QueryRow(`SELECT idioma FROM usuarios WHERE id=$1`, userID).Scan(&idioma); err != nil {
		f.t.Fatal(err)
	}
	return idioma
}

// do sends a JSON request through the real mux with the bearer.
func (f *fixture) do(method, path, body, bearer string) *httptest.ResponseRecorder {
	f.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rr := httptest.NewRecorder()
	f.mux().ServeHTTP(rr, req)
	return rr
}

// PLT-11 regla 2: the usuario is created by verify-email, which fixes the
// language from its optional locale (absent or unknown → es).
func TestVerifyEmail_StoresLocaleAsIdioma(t *testing.T) {
	tests := []struct {
		name   string
		locale string
		want   string
	}{
		{"english", "en", "en"},
		{"english with spaces and case", " EN ", "en"},
		{"spanish", "es", "es"},
		{"absent", "", "es"},
		{"unknown", "fr", "es"},
		{"regional", "en-US", "es"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			email := f.email("owner")
			pid, dni := f.persona(email)
			if err := f.register(dni, email); err != nil {
				t.Fatalf("register: %v", err)
			}
			err := f.svc.VerifyEmail(context.Background(), VerifyEmailRequest{
				Token: f.lastVerifyToken(email), Username: f.username("idi"),
				Password: "Password123!", ConfirmPassword: "Password123!", Locale: tc.locale,
			}, "127.0.0.1", "test")
			if err != nil {
				t.Fatalf("verify: %v", err)
			}
			var idioma string
			if err := f.db.QueryRow(`SELECT idioma FROM usuarios WHERE persona_id=$1`, pid).Scan(&idioma); err != nil {
				t.Fatal(err)
			}
			if idioma != tc.want {
				t.Fatalf("idioma = %q, want %q", idioma, tc.want)
			}
		})
	}
}

// register, forgot-password and reset-password are public: their locale only
// picks the e-mail language and never touches the stored one.
func TestPublicEndpoints_DoNotChangeStoredIdioma(t *testing.T) {
	f := newFixture(t)
	email := f.email("owner")
	pid, dni := f.persona(email)
	uid := f.user(pid, f.username("pub"), "Password123!", "usuario", "activo")

	if rr := f.do(http.MethodPost, "/auth/forgot-password", `{"email":"`+email+`","locale":"en"}`, ""); rr.Code != http.StatusOK {
		t.Fatalf("forgot-password: %d %s", rr.Code, rr.Body)
	}
	_ = f.svc.Register(context.Background(), RegisterRequest{DNI: dni, Email: email, Locale: "en"}, "127.0.0.1", "test")
	if got := f.storedIdioma(uid); got != "es" {
		t.Fatalf("stored idioma changed to %q", got)
	}
}

func TestSetIdioma(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
		wantStored string
	}{
		{"to english", `{"idioma":"en"}`, http.StatusOK, "", "en"},
		{"to spanish (idempotent)", `{"idioma":"es"}`, http.StatusOK, "", "es"},
		{"unsupported language", `{"idioma":"fr"}`, http.StatusUnprocessableEntity, "idioma_invalido", "es"},
		{"uppercase is not normalized", `{"idioma":"EN"}`, http.StatusUnprocessableEntity, "idioma_invalido", "es"},
		{"regional", `{"idioma":"en-US"}`, http.StatusUnprocessableEntity, "idioma_invalido", "es"},
		{"empty", `{"idioma":""}`, http.StatusUnprocessableEntity, "idioma_invalido", "es"},
		{"missing field", `{}`, http.StatusUnprocessableEntity, "idioma_invalido", "es"},
		{"unknown field", `{"idioma":"en","id":1}`, http.StatusBadRequest, "cuerpo_invalido", "es"},
		{"not a string", `{"idioma":1}`, http.StatusBadRequest, "cuerpo_invalido", "es"},
		{"malformed json", `{"idioma":`, http.StatusBadRequest, "cuerpo_invalido", "es"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			uid, username := f.activeUser("idi", "usuario")
			rr := f.do(http.MethodPut, "/auth/idioma", tc.body, f.login(username))
			if rr.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rr.Code, tc.wantStatus, rr.Body)
			}
			var resp map[string]string
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatal(err)
			}
			if tc.wantCode == "" && resp["idioma"] != tc.wantStored {
				t.Fatalf("body idioma = %q, want %q", resp["idioma"], tc.wantStored)
			}
			if tc.wantCode != "" && resp["error"] != tc.wantCode {
				t.Fatalf("error = %q, want %q", resp["error"], tc.wantCode)
			}
			if got := f.storedIdioma(uid); got != tc.wantStored {
				t.Fatalf("stored idioma = %q, want %q", got, tc.wantStored)
			}
		})
	}
}

func TestSetIdioma_RequiresSession(t *testing.T) {
	f := newFixture(t)
	if rr := f.do(http.MethodPut, "/auth/idioma", `{"idioma":"en"}`, ""); rr.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: want 401, got %d %s", rr.Code, rr.Body)
	}
}

// Only the caller's own row changes: there is no id in the route or body.
func TestSetIdioma_OnlyOwnRow(t *testing.T) {
	f := newFixture(t)
	_, caller := f.activeUser("a", "usuario")
	otherID, _ := f.activeUser("b", "usuario")
	if rr := f.do(http.MethodPut, "/auth/idioma", `{"idioma":"en"}`, f.login(caller)); rr.Code != http.StatusOK {
		t.Fatalf("status %d %s", rr.Code, rr.Body)
	}
	if got := f.storedIdioma(otherID); got != "es" {
		t.Fatalf("other user's idioma = %q, want es", got)
	}
}

// The saved language comes back from /auth/me and from the login user, and
// survives closing and opening a session.
func TestIdioma_InMeAndLogin(t *testing.T) {
	f := newFixture(t)
	_, username := f.activeUser("idi", "usuario")
	token := f.login(username)

	read := func(rr *httptest.ResponseRecorder) string {
		t.Helper()
		var p UserProfile
		if err := json.Unmarshal(rr.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p.Idioma
	}
	if got := read(f.do(http.MethodGet, "/auth/me", "", token)); got != "es" {
		t.Fatalf("default /auth/me idioma = %q, want es", got)
	}
	if rr := f.do(http.MethodPut, "/auth/idioma", `{"idioma":"en"}`, token); rr.Code != http.StatusOK {
		t.Fatalf("status %d %s", rr.Code, rr.Body)
	}
	if got := read(f.do(http.MethodGet, "/auth/me", "", token)); got != "en" {
		t.Fatalf("/auth/me idioma = %q, want en", got)
	}
	resp, err := f.svc.Login(context.Background(), LoginRequest{Username: username, Password: "Password123!"}, "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	if resp.User.Idioma != "en" {
		t.Fatalf("login user idioma = %q, want en", resp.User.Idioma)
	}
}

// The CHECK is the last line of defence for any writer other than the API.
func TestUsuariosIdiomaCheck(t *testing.T) {
	f := newFixture(t)
	uid, _ := f.activeUser("chk", "usuario")
	if _, err := f.db.Exec(`UPDATE usuarios SET idioma='fr' WHERE id=$1`, uid); err == nil {
		t.Fatal("expected usuarios_idioma_check to reject 'fr'")
	}
	if _, err := f.db.Exec(`UPDATE usuarios SET idioma=NULL WHERE id=$1`, uid); err == nil {
		t.Fatal("expected NOT NULL to reject NULL")
	}
}
