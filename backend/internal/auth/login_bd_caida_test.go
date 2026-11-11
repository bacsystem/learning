package auth

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/cbaciliod/learning/internal/crypto"
)

// handlerSinBD is a Handler over a closed pool: every query fails as with
// the database down.
func handlerSinBD(t *testing.T) (*Handler, *Service) {
	t.Helper()
	db := testDB(t)
	_ = db.Close()
	gcm, err := crypto.NewCipher(testHexKey)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(NewRepository(db, gcm, testRawKey(t)), testJWTSecret, "http://localhost:3000")
	return NewHandler(svc), svc
}

func login(h *Handler, username string) *httptest.ResponseRecorder {
	req := jsonReq(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"username":"`+username+`","password":"Password123!"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Login(w, req)
	return w
}

// #133: with the database down login answers 503, never «Credenciales
// inválidas.»; an unknown user with the database up keeps its 401.
func TestLogin_BDCaida(t *testing.T) {
	sinBD, _ := handlerSinBD(t)
	conBD, _ := testSetup(t)
	cases := []struct {
		name     string
		h        *Handler
		wantCode int
		wantBody string
	}{
		{"database down → 503", sinBD, http.StatusServiceUnavailable, `{"error":"Servicio no disponible."}`},
		{"unknown user → 401", conBD, http.StatusUnauthorized, `{"error":"Credenciales inválidas."}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := login(c.h, "no_existe_"+t.Name()[len(t.Name())-3:])
			if body := strings.TrimSpace(w.Body.String()); w.Code != c.wantCode || body != c.wantBody {
				t.Errorf("got %d %s, want %d %s", w.Code, body, c.wantCode, c.wantBody)
			}
		})
	}
}

func TestService_Login_BDCaidaEsServicioNoDisponible(t *testing.T) {
	_, svc := handlerSinBD(t)
	_, err := svc.Login(context.Background(), LoginRequest{Username: "alguien", Password: "Password123!"}, "127.0.0.1", "test")
	if !errors.Is(err, ErrServicioNoDisponible) || errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("got %v, want ErrServicioNoDisponible and not ErrInvalidCredentials", err)
	}
}

// The 503 log carries the op only: no username and no driver error.
func TestLogin_BDCaidaNoRegistraDatos(t *testing.T) {
	var buf bytes.Buffer
	anterior := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = anterior })

	h, _ := handlerSinBD(t)
	usuario := "usuario_secreto_x9"
	if w := login(h, usuario); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503", w.Code)
	}
	if out := buf.String(); !strings.Contains(out, "auth.login") || strings.Contains(out, usuario) || strings.Contains(out, "sql:") {
		t.Errorf("log %q: want the op only", out)
	}
}

// A wrong password of an existing user keeps its 401 (contract: unchanged).
func TestLogin_ContrasenaMalaSigue401(t *testing.T) {
	f := newFixture(t)
	_, username := f.activeUser("mala", "usuario")
	req := jsonReq(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"username":"`+username+`","password":"Otra-Clave-123!"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	f.mux().ServeHTTP(w, req)
	if body := strings.TrimSpace(w.Body.String()); w.Code != http.StatusUnauthorized || body != `{"error":"Credenciales inválidas."}` {
		t.Errorf("got %d %s, want 401 Credenciales inválidas.", w.Code, body)
	}
}

// If storing the session fails, login is ErrServicioNoDisponible (503) with
// no token, and the user's older sessions are not evicted (one transaction).
func TestLogin_SesionNoGuardadaNoBorraLasAnteriores(t *testing.T) {
	f := newFixture(t)
	uid, username := f.activeUser("ses503", "usuario")
	maxSes, err := f.repo.GetMaxSesiones(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for range maxSes {
		f.login(username)
	}
	contar := func() int {
		var n int
		if err := f.db.Get(&n, `SELECT COUNT(*) FROM sesiones WHERE usuario_id=$1`, uid); err != nil {
			t.Fatal(err)
		}
		return n
	}
	antes := contar()

	// sesiones.ip is varchar(45): a longer value makes the INSERT fail.
	resp, err := f.svc.Login(context.Background(), LoginRequest{Username: username, Password: "Password123!"}, strings.Repeat("9", 60), "test")
	if resp != nil || !errors.Is(err, ErrServicioNoDisponible) {
		t.Fatalf("got resp %v, err %v; want no token and ErrServicioNoDisponible", resp, err)
	}
	if despues := contar(); despues != antes {
		t.Errorf("sessions %d → %d: an older session was evicted for a login that failed", antes, despues)
	}
}
