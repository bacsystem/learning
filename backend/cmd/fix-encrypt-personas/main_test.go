package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"reflect"
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/crypto"
)

const testHexKey = "0303030303030303030303030303030303030303030303030303030303030303"

func setup(t *testing.T) ([]byte, func(string) string) {
	t.Helper()
	gcm, err := crypto.NewCipher(testHexKey)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := hex.DecodeString(testHexKey)
	enc := func(s string) string {
		v, err := crypto.Encrypt(gcm, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	return raw, enc
}

func cols(u update) []string {
	var n []string
	for _, c := range u.Columns {
		n = append(n, c.Name)
	}
	return n
}

func TestPlanPerFieldNoDoubleEncryption(t *testing.T) {
	raw, enc := setup(t)
	gcm, _ := crypto.NewCipher(testHexKey)
	rows := []personaRow{
		// plaintext DNI, everything else already encrypted, hmacs stale
		{ID: 1, DNI: "11111111", Nombres: enc("ANA"), ApellidoPaterno: enc("P"), ApellidoMaterno: enc("M"),
			Telefono: enc("999"), Email: enc("ana@x.com"), EmailHMAC: crypto.HMAC(raw, "ana@x.com")},
		// fully correct row: no update
		{ID: 2, DNI: enc("22222222"), DNIHMAC: crypto.HMAC(raw, "22222222"), Nombres: enc("B"), ApellidoPaterno: enc("P"),
			ApellidoMaterno: enc("M"), Telefono: enc(""), Email: enc("b@x.com"), EmailHMAC: crypto.HMAC(raw, "b@x.com")},
		// fully plaintext row, mixed-case email
		{ID: 3, DNI: "33333333", Nombres: "C", ApellidoPaterno: "P", ApellidoMaterno: "M", Telefono: "", Email: " C@X.Com "},
	}
	rep, err := plan(gcm, raw, rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Updates) != 2 || rep.Updates[0].ID != 1 || rep.Updates[1].ID != 3 {
		t.Fatalf("updates = %+v", rep.Updates)
	}
	if got, want := cols(rep.Updates[0]), []string{"dni", "dni_hmac"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("row 1 must only touch dni + dni_hmac (no double encryption), got %v", got)
	}
	u3 := rep.Updates[1]
	if got, want := cols(u3), []string{"dni", "nombres", "apellido_paterno", "apellido_materno", "telefono", "email", "dni_hmac", "email_hmac"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("row 3 cols = %v", got)
	}
	for _, c := range u3.Columns {
		switch c.Name {
		case "email":
			if pt, _ := crypto.Decrypt(gcm, c.Value); pt != "c@x.com" {
				t.Fatalf("email must be stored normalized, got %q", pt)
			}
		case "email_hmac":
			if c.Value != crypto.HMAC(raw, "c@x.com") {
				t.Fatal("email_hmac must use normalized email")
			}
		case "dni_hmac":
			if c.Value != crypto.HMAC(raw, "33333333") {
				t.Fatal("bad dni_hmac")
			}
		}
	}
}

func TestPlanDetectsDNIHMACCollisions(t *testing.T) {
	raw, enc := setup(t)
	gcm, _ := crypto.NewCipher(testHexKey)
	rows := []personaRow{
		{ID: 10, DNI: enc("44444444"), DNIHMAC: crypto.HMAC(raw, "44444444"), Nombres: enc("x"), ApellidoPaterno: enc("x"),
			ApellidoMaterno: enc("x"), Telefono: enc("x"), Email: enc("x@x"), EmailHMAC: crypto.HMAC(raw, "x@x")},
		{ID: 11, DNI: "44444444", Nombres: "x", ApellidoPaterno: "x", ApellidoMaterno: "x", Telefono: "x", Email: "y@y"},
		{ID: 12, DNI: "55555555", Nombres: "x", ApellidoPaterno: "x", ApellidoMaterno: "x", Telefono: "x", Email: "z@z"},
	}
	rep, err := plan(gcm, raw, rows)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rep.Collisions, [][]int64{{10, 11}}) {
		t.Fatalf("collisions = %v", rep.Collisions)
	}
	if len(rep.Updates) != 1 || rep.Updates[0].ID != 12 {
		t.Fatalf("only id 12 should be updated, got %+v", rep.Updates)
	}
}

func TestRenderWrapsInTransactionAndHasNoPlaintext(t *testing.T) {
	raw, _ := setup(t)
	gcm, _ := crypto.NewCipher(testHexKey)
	rows := []personaRow{{ID: 7, DNI: "77777777", Nombres: "SECRETO", ApellidoPaterno: "P", ApellidoMaterno: "M", Telefono: "987654321", Email: "secret@x.com"}}
	rep, err := plan(gcm, raw, rows)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := render(&buf, rep.Updates); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "BEGIN;\n") || !strings.HasSuffix(out, "COMMIT;\n") {
		t.Fatalf("SQL must be wrapped in BEGIN/COMMIT:\n%s", out)
	}
	for _, pii := range []string{"77777777", "SECRETO", "987654321", "secret@x.com"} {
		if strings.Contains(out, pii) {
			t.Fatalf("output leaks %q", pii)
		}
	}
	if !strings.Contains(out, "WHERE id=7\n") {
		t.Fatalf("missing WHERE id:\n%s", out)
	}
}

// H121: filas cifradas con la clave A y plan con la clave B. El valor tiene
// forma de ciphertext pero no descifra: hay que abortar, no re-cifrarlo.
func TestPlanAbortsOnWrongKey(t *testing.T) {
	_, encA := setup(t)
	const otherKey = "0404040404040404040404040404040404040404040404040404040404040404"
	gcmB, err := crypto.NewCipher(otherKey)
	if err != nil {
		t.Fatal(err)
	}
	rawB, _ := hex.DecodeString(otherKey)
	rows := []personaRow{{ID: 42, DNI: encA("12345678"), Nombres: encA("ANA"), ApellidoPaterno: encA("P"),
		ApellidoMaterno: encA("M"), Telefono: encA("999"), Email: encA("ana@x.com")}}
	rep, err := plan(gcmB, rawB, rows)
	if err == nil {
		t.Fatalf("plan con la clave equivocada debe fallar, generó %d update(s)", len(rep.Updates))
	}
	msg := err.Error()
	if !strings.Contains(msg, "id=42") || !strings.Contains(msg, "dni") || !strings.Contains(msg, "ENCRYPTION_KEY") {
		t.Fatalf("el error debe nombrar persona, campo y la clave: %q", msg)
	}
	if strings.Contains(msg, rows[0].DNI) {
		t.Fatalf("el error no debe incluir el valor: %q", msg)
	}
}

// H121: texto plano realista (incluido un DNI de 8 dígitos, que es base64
// válido de 6 bytes) se sigue planificando para cifrar.
func TestPlanStillEncryptsRealPlaintext(t *testing.T) {
	raw, _ := setup(t)
	gcm, _ := crypto.NewCipher(testHexKey)
	rows := []personaRow{{ID: 5, DNI: "12345678", Nombres: "MARIA JOSE", ApellidoPaterno: "QUISPE",
		ApellidoMaterno: "MAMANI", Telefono: "987654321", Email: "maria@x.com"},
		{ID: 6, DNI: "87654321", Nombres: "ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKL", ApellidoPaterno: "",
			ApellidoMaterno: "AB==", Telefono: "", Email: ""}}
	rep, err := plan(gcm, raw, rows)
	if err != nil {
		t.Fatalf("texto plano no debe abortar: %v", err)
	}
	if len(rep.Updates) != 2 {
		t.Fatalf("updates = %+v", rep.Updates)
	}
	for _, u := range rep.Updates {
		if len(u.Encrypted) != 6 {
			t.Fatalf("persona %d: todos los campos deben cifrarse, got %v", u.ID, u.Encrypted)
		}
	}
}

// H121: umbral exacto de looksLikeCiphertext (nonce 12 + tag 16 = 28 bytes).
func TestLooksLikeCiphertextThreshold(t *testing.T) {
	gcm, _ := crypto.NewCipher(testHexKey)
	b64 := func(n int) string { return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{'a'}, n)) }
	cases := map[string]bool{
		b64(27): false, b64(28): true, "12345678": false, "ana@x.com": false, "": false,
		"MARIA JOSE": false, b64(28)[:len(b64(28))-1]: false, // sin padding: no es base64 estricto
	}
	for in, want := range cases {
		if got := looksLikeCiphertext(gcm, in); got != want {
			t.Errorf("looksLikeCiphertext(%q) = %v, want %v", in, got, want)
		}
	}
}

// H146: cada UPDATE se protege contra cambios posteriores a la lectura y el
// script aborta (y revierte) si alguno no afecta exactamente una fila.
func TestRenderGuardsAgainstConcurrentChanges(t *testing.T) {
	raw, _ := setup(t)
	gcm, _ := crypto.NewCipher(testHexKey)
	rows := []personaRow{{ID: 7, DNI: "77777777", DNIHMAC: "legacy:7", Nombres: "N", ApellidoPaterno: "P",
		ApellidoMaterno: "M", Telefono: "1", Email: "a@x.com", EmailHMAC: "o'ld", UpdatedAt: "2026-11-09 10:00:00.123456+00"}}
	rep, err := plan(gcm, raw, rows)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := render(&buf, rep.Updates); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"WHERE id=7",
		"AND updated_at='2026-11-09 10:00:00.123456+00'::timestamptz",
		"AND dni_hmac='legacy:7'",
		"AND email_hmac='o''ld'", // escapado
		"GET DIAGNOSTICS",
		"RAISE EXCEPTION",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("falta %q en:\n%s", want, out)
		}
	}
}
