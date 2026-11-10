package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Validation errors are answered with a generic Spanish message per field,
// never the raw validator text ("Key: 'RegisterRequest.Password' Error:…").
func TestValidationErrors_SpanishPerField_422(t *testing.T) {
	h, _ := testSetup(t)
	cases := []struct {
		name    string
		handler http.HandlerFunc
		body    string
		want    string
	}{
		{"register dni", h.Register, `{"dni":"123","email":"a@b.co"}`, validationMessages["dni"]},
		{"register email", h.Register, `{"dni":"12345678","email":"bad"}`, validationMessages["email"]},
		{"verify password", h.VerifyEmail, `{"token":"t","username":"abcd","password":"short","confirm_password":"short"}`, validationMessages["password"]},
		{"verify confirm", h.VerifyEmail, `{"token":"t","username":"abcd","password":"Password123!","confirm_password":"x"}`, validationMessages["confirm_password"]},
		{"verify username", h.VerifyEmail, `{"token":"t","username":"a b","password":"Password123!","confirm_password":"Password123!"}`, validationMessages["username"]},
		{"login", h.Login, `{"username":"","password":"x"}`, validationMessages["username"]},
		{"forgot", h.ForgotPassword, `{"email":"nope"}`, validationMessages["email"]},
		{"reset", h.ResetPassword, `{"token":"t","password":"Password123!","confirm_password":"other"}`, validationMessages["confirm_password"]},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		c.handler(w, jsonReq(http.MethodPost, "/", strings.NewReader(c.body)))
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: want 422, got %d", c.name, w.Code)
			continue
		}
		var got map[string]string
		_ = json.Unmarshal(w.Body.Bytes(), &got)
		if strings.Contains(w.Body.String(), "Key:") || strings.Contains(w.Body.String(), "Error:Field") {
			t.Errorf("%s: raw validator text leaked: %s", c.name, w.Body.String())
		}
		if c.want == "" || got["error"] != c.want {
			t.Errorf("%s: error = %q, want %q", c.name, got["error"], c.want)
		}
	}
}
