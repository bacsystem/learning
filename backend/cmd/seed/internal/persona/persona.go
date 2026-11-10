// Package persona contiene la normalización y el cifrado de los campos PII de
// personas compartidos por las herramientas de seed (cmd/seed y
// cmd/seed/admin), para que todas calculen dni_hmac/email_hmac igual que auth.
package persona

import (
	"crypto/cipher"
	"fmt"
	"strings"

	"github.com/cbaciliod/learning/internal/crypto"
)

// NormalizeDNI quita espacios alrededor del DNI.
func NormalizeDNI(s string) string { return strings.TrimSpace(s) }

// NormalizeEmail pasa el email a minúsculas y quita espacios, igual que auth
// antes de calcular el HMAC de búsqueda.
func NormalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// Plain son los campos PII en claro.
type Plain struct {
	DNI       string
	Nombres   string
	ApellidoP string
	ApellidoM string
	Telefono  string
	Email     string
}

// Encrypted son los campos listos para INSERT: cifrados + HMACs.
type Encrypted struct {
	DNI       string
	DNIHMAC   string
	Nombres   string
	ApellidoP string
	ApellidoM string
	Telefono  string
	Email     string
	EmailHMAC string
}

// Encrypt normaliza DNI y email, calcula sus HMAC y cifra cada campo. Cualquier
// error de cifrado se devuelve (nunca se guarda un campo vacío en su lugar).
// Los mensajes de error nombran el campo, nunca su valor.
func Encrypt(gcm cipher.AEAD, rawKey []byte, p Plain) (Encrypted, error) {
	dni := NormalizeDNI(p.DNI)
	email := NormalizeEmail(p.Email)
	if dni == "" {
		return Encrypted{}, fmt.Errorf("dni vacío")
	}
	out := Encrypted{
		DNIHMAC:   crypto.HMAC(rawKey, dni),
		EmailHMAC: crypto.HMAC(rawKey, email),
	}
	fields := []struct {
		name string
		in   string
		dst  *string
	}{
		{"dni", dni, &out.DNI},
		{"nombres", p.Nombres, &out.Nombres},
		{"apellido_paterno", p.ApellidoP, &out.ApellidoP},
		{"apellido_materno", p.ApellidoM, &out.ApellidoM},
		{"telefono", p.Telefono, &out.Telefono},
		{"email", email, &out.Email},
	}
	for _, f := range fields {
		v, err := crypto.Encrypt(gcm, f.in)
		if err != nil {
			return Encrypted{}, fmt.Errorf("cifrar %s: %w", f.name, err)
		}
		*f.dst = v
	}
	return out, nil
}
