package auth

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

const claveFalsa = "Password123!"

// storeFalso is an in-memory loginStore whose step falla fails like a
// database error; the rest answer as for an active user.
type storeFalso struct {
	falla  string
	hash   string
	sesion bool // AbrirSesion succeeded
}

var errBDFalsa = errors.New(`pq: connection refused (host=db.internal user=admin)`)

func (s *storeFalso) fallar(paso string) error {
	if s.falla == paso {
		return errBDFalsa
	}
	return nil
}

func (s *storeFalso) FindByUsername(context.Context, string) (*usuarioRow, error) {
	if err := s.fallar("find user"); err != nil {
		return nil, err
	}
	return &usuarioRow{ID: 7, Username: "ana", PasswordHash: s.hash, Rol: "usuario", Estado: "activo"}, nil
}

func (s *storeFalso) RegisterFailedLogin(context.Context, int64, int, time.Duration) (int, bool, error) {
	return 1, false, s.fallar("register failed login")
}

func (s *storeFalso) ResetLoginIntentos(context.Context, int64) error {
	return s.fallar("reset login attempts")
}

func (s *storeFalso) GetUserProfileByID(context.Context, int64) (*UserProfile, error) {
	if err := s.fallar("load profile"); err != nil {
		return nil, err
	}
	return &UserProfile{}, nil
}

func (s *storeFalso) LoadLoginClaims(context.Context, int64) (LoginClaims, error) {
	return LoginClaims{GradoID: 1, EspecialidadID: 2, InstitucionID: 3, Nivel: "suboficial"}, s.fallar("load claims")
}

func (s *storeFalso) GetMaxSesiones(context.Context) (int, error) { return 2, s.fallar("max sessions") }

func (s *storeFalso) AbrirSesion(context.Context, int64, int, string, string, string, time.Time) error {
	if err := s.fallar("open session"); err != nil {
		return err
	}
	s.sesion = true
	return nil
}

func (s *storeFalso) WriteAudit(context.Context, AuditEntry) {}

// Review H1 of #137: a database error in ANY step of Login is
// ErrServicioNoDisponible (503), never credentials, and no session or token
// is issued.
func TestLogin_ErrorDeBDEnCadaPaso(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte(claveFalsa), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		paso  string
		clave string
	}{
		{"find user", claveFalsa},
		{"register failed login", "otra-clave"}, // only reached with a wrong password
		{"reset login attempts", claveFalsa},
		{"load profile", claveFalsa},
		{"load claims", claveFalsa},
		{"max sessions", claveFalsa},
		{"open session", claveFalsa},
	}
	for _, c := range cases {
		t.Run(c.paso, func(t *testing.T) {
			store := &storeFalso{falla: c.paso, hash: string(hash)}
			svc := &Service{login: store, jwtSecret: testJWTSecret}
			resp, err := svc.Login(context.Background(), LoginRequest{Username: "ana", Password: c.clave}, "127.0.0.1", "test")
			var f *fallaBD
			if resp != nil || !errors.Is(err, ErrServicioNoDisponible) || errors.Is(err, ErrInvalidCredentials) ||
				!errors.As(err, &f) || f.paso != c.paso || !errors.Is(err, errBDFalsa) {
				t.Fatalf("got resp %v, err %v; want ErrServicioNoDisponible at %q with the cause in the chain", resp, err, c.paso)
			}
			if store.sesion && c.paso != "open session" {
				t.Errorf("a session was opened although %q failed", c.paso)
			}
		})
	}

	// Without a failing step the double logs in, so each case fails where it says.
	svc := &Service{login: &storeFalso{hash: string(hash)}, jwtSecret: testJWTSecret}
	if resp, err := svc.Login(context.Background(), LoginRequest{Username: "ana", Password: claveFalsa}, "127.0.0.1", "test"); err != nil || resp.Token == "" {
		t.Fatalf("no failing step: resp %v, err %v; want a token", resp, err)
	}
}

// Review H2 of #137: the 503 log names the failed step, not the cause.
func TestLogin_BDCaidaRegistraElPaso(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte(claveFalsa), bcrypt.MinCost)
	var buf bytes.Buffer
	anterior := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = anterior })

	h, svc := handlerSinBD(t) // NewHandler needs a pool; Login only uses the double
	svc.login = &storeFalso{falla: "load claims", hash: string(hash)}
	if w := login(h, "ana"); w.Code != 503 {
		t.Fatalf("got %d, want 503", w.Code)
	}
	out := buf.String()
	if !strings.Contains(out, `"paso":"load claims"`) || strings.Contains(out, "db.internal") || strings.Contains(out, "ana") {
		t.Errorf("log %q: want the op and the step only", out)
	}
}
