package crypto

import (
	"strings"
	"testing"
)

const testKey = "0000000000000000000000000000000000000000000000000000000000000000"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	gcm, err := NewCipher(testKey)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	plain := "12345678"
	ct, err := Encrypt(gcm, plain)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if ct == plain {
		t.Fatal("ciphertext must not equal plaintext")
	}
	got, err := Decrypt(gcm, ct)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if got != plain {
		t.Fatalf("want %q got %q", plain, got)
	}
}

func TestEncryptProducesUniqueNonces(t *testing.T) {
	gcm, _ := NewCipher(testKey)
	ct1, _ := Encrypt(gcm, "same")
	ct2, _ := Encrypt(gcm, "same")
	if ct1 == ct2 {
		t.Fatal("two encryptions of same plaintext must differ (random nonce)")
	}
}

func TestInvalidKey(t *testing.T) {
	_, err := NewCipher("tooshort")
	if err == nil || !strings.Contains(err.Error(), "32 bytes") {
		t.Fatalf("expected invalid-key error, got %v", err)
	}
}
