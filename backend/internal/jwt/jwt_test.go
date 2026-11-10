package jwt

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestIssueAndValidate(t *testing.T) {
	secret := "test-secret-32-bytes-exactly-ok!"
	token, expiresAt, err := Issue(secret, 42, "admin", "suboficial", 1, 2, 3)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if expiresAt <= time.Now().Unix() {
		t.Fatal("expiresAt should be in the future")
	}

	claims, err := Validate(secret, token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if claims.UserID != 42 {
		t.Errorf("got UserID=%d, want 42", claims.UserID)
	}
	if claims.Rol != "admin" {
		t.Errorf("got Rol=%q, want admin", claims.Rol)
	}
	if claims.GradoID != 1 || claims.EspecialidadID != 2 || claims.InstitucionID != 3 {
		t.Errorf("unexpected claim IDs")
	}
	if claims.Nivel != "suboficial" {
		t.Errorf("got Nivel=%q, want suboficial", claims.Nivel)
	}
}

func TestValidate_WrongSecret(t *testing.T) {
	token, _, _ := Issue("secret-a", 1, "docente", "", 0, 0, 0)
	_, err := Validate("secret-b", token)
	if err == nil {
		t.Fatal("expected error with wrong secret")
	}
}

func TestValidate_MalformedToken(t *testing.T) {
	_, err := Validate("secret", "not.a.valid.token")
	if err == nil {
		t.Fatal("expected error for malformed token")
	}
}

func sign(t *testing.T, secret string, method jwt.SigningMethod, claims Claims) string {
	t.Helper()
	tok, err := jwt.NewWithClaims(method, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestValidate_ExpiredToken(t *testing.T) {
	secret := "test-secret-32-bytes-exactly-ok!"
	past := time.Now().Add(-time.Hour)
	tok := sign(t, secret, jwt.SigningMethodHS256, Claims{
		UserID: 1, Rol: "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(past),
			IssuedAt:  jwt.NewNumericDate(past.Add(-time.Hour)),
		},
	})
	if _, err := Validate(secret, tok); err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidate_RejectsNoneAlgorithm(t *testing.T) {
	tok, err := jwt.NewWithClaims(jwt.SigningMethodNone, Claims{UserID: 1, Rol: "admin",
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Validate("secret", tok); err == nil {
		t.Fatal("expected error for alg=none token")
	}
}

func TestIssue_SetsExpiry24h(t *testing.T) {
	_, exp, err := Issue("s", 1, "usuario", "", 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	d := time.Until(time.Unix(exp, 0))
	if d < 23*time.Hour || d > 25*time.Hour {
		t.Fatalf("expiry %v, want ~24h", d)
	}
}

// The issued token itself carries exp (~24h); checking only Issue's return
// value would not notice a token without it.
func TestIssue_TokenCarriesExpClaim(t *testing.T) {
	tok, exp, err := Issue("s", 1, "usuario", "", 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	var c Claims
	if _, _, err := jwt.NewParser().ParseUnverified(tok, &c); err != nil {
		t.Fatal(err)
	}
	if c.ExpiresAt == nil {
		t.Fatal("issued token has no exp claim")
	}
	if c.ExpiresAt.Unix() != exp {
		t.Fatalf("exp claim %d, Issue returned %d", c.ExpiresAt.Unix(), exp)
	}
	if d := time.Until(c.ExpiresAt.Time); d < 23*time.Hour || d > 25*time.Hour {
		t.Fatalf("exp claim %v from now, want ~24h", d)
	}
}

// A validly signed token without exp is rejected.
func TestValidate_RequiresExp(t *testing.T) {
	secret := "test-secret-32-bytes-exactly-ok!"
	tok := sign(t, secret, jwt.SigningMethodHS256, Claims{UserID: 1, Rol: "admin",
		RegisteredClaims: jwt.RegisteredClaims{IssuedAt: jwt.NewNumericDate(time.Now())},
	})
	if _, err := Validate(secret, tok); err == nil {
		t.Fatal("expected error for a token without exp")
	}
}
