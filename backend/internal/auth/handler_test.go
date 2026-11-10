package auth

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/cbaciliod/learning/internal/crypto"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

const testHexKey = "0000000000000000000000000000000000000000000000000000000000000001"

// testDB returns a live DB connection or skips the test.
func testDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Fatalf("db connect: %v", err)
	}
	// The Postgres server is shared: bound each test's pool and release it.
	db.SetMaxOpenConns(10)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// jsonReq is httptest.NewRequest with Content-Type: application/json, which
// decodeJSON requires (415 otherwise).
func jsonReq(method, target string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, target, body)
	r.Header.Set("Content-Type", "application/json")
	return r
}

func testRawKey(t *testing.T) []byte {
	t.Helper()
	key, err := hex.DecodeString(testHexKey)
	if err != nil {
		t.Fatalf("raw key: %v", err)
	}
	return key
}

func testSetup(t *testing.T) (*Handler, *Service) {
	t.Helper()
	db := testDB(t)
	gcm, err := crypto.NewCipher(testHexKey)
	if err != nil {
		t.Fatalf("gcm: %v", err)
	}
	_ = fmt.Sprintf // keep fmt import used via testHexKey formatting if needed
	repo := NewRepository(db, gcm, testRawKey(t))
	svc := NewService(repo, "test-secret-32-bytes-exactly-ok!", "http://localhost:3000")
	h := NewHandler(svc)
	return h, svc
}

// ── Register ─────────────────────────────────────────────────────────────────

func TestHandler_Register_ValidationError(t *testing.T) {
	h, _ := testSetup(t)
	body := `{"dni":"123","email":"bad","username":"x","password":"short","confirm_password":"short"}`
	req := jsonReq(http.MethodPost, "/auth/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Register(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422, got %d", w.Code)
	}
}

func TestHandler_Register_InvalidJSON(t *testing.T) {
	h, _ := testSetup(t)
	req := jsonReq(http.MethodPost, "/auth/register", bytes.NewBufferString("{bad"))
	w := httptest.NewRecorder()
	h.Register(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// ── Login ─────────────────────────────────────────────────────────────────────

func TestHandler_Login_InvalidCredentials(t *testing.T) {
	h, _ := testSetup(t)
	body := `{"username":"nonexistent","password":"wrongpassword"}`
	req := jsonReq(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Login(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestHandler_Login_ValidationError(t *testing.T) {
	h, _ := testSetup(t)
	body := `{"username":"","password":""}`
	req := jsonReq(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	h.Login(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422, got %d", w.Code)
	}
}

// ── Logout ─────────────────────────────────────────────────────────────────

func TestHandler_Logout_RequiresAuth(t *testing.T) {
	_, svc := testSetup(t)
	mux := http.NewServeMux()
	Mount(mux, svc, "test-secret-32-bytes-exactly-ok!", noSessions)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

// ── Me ────────────────────────────────────────────────────────────────────────

func TestHandler_Me_RequiresAuth(t *testing.T) {
	_, svc := testSetup(t)
	mux := http.NewServeMux()
	Mount(mux, svc, "test-secret-32-bytes-exactly-ok!", noSessions)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

// ── ForgotPassword ────────────────────────────────────────────────────────────

func TestHandler_ForgotPassword_AlwaysReturns200(t *testing.T) {
	h, _ := testSetup(t)
	body := `{"email":"nonexistent@example.com"}`
	req := jsonReq(http.MethodPost, "/auth/forgot-password", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ForgotPassword(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestHandler_ForgotPassword_NoEmailEnumeration(t *testing.T) {
	h, _ := testSetup(t)
	do := func(email string) int {
		body := fmt.Sprintf(`{"email":%q}`, email)
		req := jsonReq(http.MethodPost, "/auth/forgot-password", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ForgotPassword(w, req)
		return w.Code
	}
	if do("unknown@example.com") != http.StatusOK {
		t.Error("unknown email should return 200")
	}
}

// ── ResetPassword ─────────────────────────────────────────────────────────────

func TestHandler_ResetPassword_InvalidToken(t *testing.T) {
	h, _ := testSetup(t)
	body := `{"token":"invalid-token","password":"newpassword123","confirm_password":"newpassword123"}`
	req := jsonReq(http.MethodPost, "/auth/reset-password", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ResetPassword(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestHandler_ResetPassword_PasswordMismatch(t *testing.T) {
	h, _ := testSetup(t)
	body := `{"token":"some-token","password":"newpassword123","confirm_password":"different123"}`
	req := jsonReq(http.MethodPost, "/auth/reset-password", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ResetPassword(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422, got %d", w.Code)
	}
}

// Brute-force lockout with a seeded user: see TestLogin_BruteForce_Lockout and
// TestHandlerLogin_BruteForce_Lockout429 in security_test.go.
