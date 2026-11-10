package pii

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/crypto"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestDecrypt(t *testing.T) {
	gcm, err := crypto.NewCipher("0000000000000000000000000000000000000000000000000000000000000001")
	if err != nil {
		t.Fatal(err)
	}
	enc, _ := crypto.Encrypt(gcm, "Juan")

	var buf bytes.Buffer
	prev := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = prev })

	if got := Decrypt(gcm, enc, "nombres"); got != "Juan" {
		t.Fatalf("roundtrip = %q", got)
	}
	if got := Decrypt(gcm, "", "nombres"); got != "" || buf.Len() != 0 {
		t.Fatalf("empty input: got %q, log %q", got, buf.String())
	}
	if got := Decrypt(gcm, "garbage-ciphertext", "nombres"); got != "" {
		t.Fatalf("undecryptable value must be withheld, got %q", got)
	}
	if !strings.Contains(buf.String(), `"field":"nombres"`) || strings.Contains(buf.String(), "garbage") {
		t.Fatalf("warning must name the field and never the value: %q", buf.String())
	}
	if got := Decrypt(nil, enc, "nombres"); got != "" {
		t.Fatalf("nil cipher must withhold the value, got %q", got)
	}
}
