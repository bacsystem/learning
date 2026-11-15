// Package locale resolves the language of user-facing text (e-mails today,
// notifications later) from a client-supplied value.
package locale

import "strings"

// Supported languages. Keep the list closed: other values fall back to ES.
const (
	ES = "es"
	EN = "en"
)

// IsSupported reports whether raw is exactly a supported language code. Unlike
// Normalize it does not trim or fold case: it validates a value the client is
// choosing, where a wrong one must be rejected rather than guessed.
func IsSupported(raw string) bool { return raw == ES || raw == EN }

// Normalize returns EN or ES. Surrounding spaces and case are ignored; an
// empty, unknown or regional value ("en-US", "fr") resolves to ES, the default
// language. It never fails, so an unrecognised locale cannot reject a request.
func Normalize(raw string) string {
	if strings.ToLower(strings.TrimSpace(raw)) == EN {
		return EN
	}
	return ES
}
