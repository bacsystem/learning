package salud

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cbaciliod/learning/internal/auth"
	"github.com/cbaciliod/learning/internal/jwt"
)

const secretoPrueba = "test-secret-32-bytes-exactly-ok!"

// GET /admin/salud is admin only and inherits the auth answers, including the
// 503 of a session check that fails (Base de datos totalmente caída).
func TestMount_Acceso(t *testing.T) {
	r := &relojFijo{t: inicio}
	svc := NewService([]Comprobacion{ComprobarAPI("1.4.0", inicio, r.ahora)}, r.ahora)
	token := func(rol string) string {
		tok, _, err := jwt.Issue(secretoPrueba, 7, rol, "suboficial", 1, 2, 3)
		if err != nil {
			t.Fatal(err)
		}
		return "Bearer " + tok
	}
	sesion := func(rol string, err error) auth.SessionChecker {
		return func(context.Context, string) (*auth.Session, error) {
			if err != nil {
				return nil, err
			}
			return &auth.Session{UserID: 7, Rol: rol, Estado: "activo"}, nil
		}
	}
	cases := []struct {
		name    string
		authz   string
		checker auth.SessionChecker
		want    int
	}{
		{"admin gets 200", token("admin"), sesion("admin", nil), http.StatusOK},
		{"user role gets 403", token("usuario"), sesion("usuario", nil), http.StatusForbidden},
		{"no session gets 401", "", sesion("admin", nil), http.StatusUnauthorized},
		{"session check failing gets 503", token("admin"), sesion("", errors.New("db down")), http.StatusServiceUnavailable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mux := http.NewServeMux()
			Mount(mux, svc, secretoPrueba, c.checker)
			req := httptest.NewRequest(http.MethodGet, "/admin/salud", nil)
			if c.authz != "" {
				req.Header.Set("Authorization", c.authz)
			}
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != c.want {
				t.Errorf("got %d, want %d (%s)", w.Code, c.want, w.Body.String())
			}
		})
	}
}
