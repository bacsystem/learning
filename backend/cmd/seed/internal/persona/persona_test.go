package persona

import (
	"encoding/hex"
	"testing"

	"github.com/cbaciliod/learning/internal/crypto"
)

const testKey = "0101010101010101010101010101010101010101010101010101010101010101"

func TestNormalizeEmail(t *testing.T) {
	if got := NormalizeEmail("  Juan.Perez@Mail.COM \t"); got != "juan.perez@mail.com" {
		t.Fatalf("got %q", got)
	}
}

func TestEncryptHMACsUseNormalizedValues(t *testing.T) {
	gcm, err := crypto.NewCipher(testKey)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := hex.DecodeString(testKey)
	e, err := Encrypt(gcm, raw, Plain{DNI: " 12345678 ", Email: " A@B.Com ", Nombres: "N"})
	if err != nil {
		t.Fatal(err)
	}
	if e.DNIHMAC != crypto.HMAC(raw, "12345678") {
		t.Fatal("dni_hmac must be computed over the trimmed DNI")
	}
	if e.EmailHMAC != crypto.HMAC(raw, "a@b.com") {
		t.Fatal("email_hmac must be computed over the lowercased, trimmed email")
	}
	for name, ct := range map[string]string{"dni": e.DNI, "email": e.Email, "nombres": e.Nombres} {
		if ct == "" {
			t.Fatalf("%s ciphertext empty", name)
		}
	}
	if pt, _ := crypto.Decrypt(gcm, e.Email); pt != "a@b.com" {
		t.Fatalf("stored email must be the normalized one, got %q", pt)
	}
}

func TestEncryptRejectsEmptyDNI(t *testing.T) {
	gcm, _ := crypto.NewCipher(testKey)
	raw, _ := hex.DecodeString(testKey)
	if _, err := Encrypt(gcm, raw, Plain{DNI: "  "}); err == nil {
		t.Fatal("expected error for empty DNI")
	}
}
