// Package pii centralises how encrypted personal data read from the database
// is turned back into plaintext.
package pii

import (
	"crypto/cipher"

	"github.com/cbaciliod/learning/internal/crypto"
	"github.com/rs/zerolog/log"
)

// Decrypt returns the plaintext of an encrypted column value.
//
// One behaviour for every caller: an empty value stays empty, and a value that
// cannot be decrypted (wrong key, corrupt or legacy plaintext row, missing
// cipher) yields "" plus a warning. The ciphertext is never returned — some
// callers serve public endpoints — and the log carries only the column name,
// never the value.
func Decrypt(gcm cipher.AEAD, s, field string) string {
	if s == "" {
		return ""
	}
	if gcm == nil {
		log.Warn().Str("field", field).Msg("pii: no cipher configured; value withheld")
		return ""
	}
	plain, err := crypto.Decrypt(gcm, s)
	if err != nil {
		log.Warn().Str("field", field).Msg("pii: decrypt failed; value withheld")
		return ""
	}
	return plain
}
