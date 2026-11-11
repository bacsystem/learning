// Package texto holds text helpers shared by several modules and free of any
// domain: the folding used to compare names and search text without case or
// accents (ADR 0003 § Plegado) and the excerpt shown for a passage.
package texto

import (
	"strings"
	"unicode/utf8"
)

// Combining marks of the decomposed (NFD) forms that Plegar folds.
const (
	agudo      = "\u0301"
	grave      = "\u0300"
	dieresis   = "\u0308"
	virgulilla = "\u0303"
)

// tildes maps every accented vowel (and ü) to its plain form, whether it comes
// composed (NFC, one code point) or decomposed (NFD, vowel + combining mark).
// It runs after ToLower, which lowers the base letter and leaves the marks,
// so lowercase entries are enough. The ñ is kept on purpose so that «año» and
// «ano» stay different: a decomposed n + U+0303 becomes the composed ñ.
var tildes = strings.NewReplacer(
	// NFC
	"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u",
	"à", "a", "è", "e", "ì", "i", "ò", "o", "ù", "u",
	// NFD
	"a"+agudo, "a", "e"+agudo, "e", "i"+agudo, "i", "o"+agudo, "o", "u"+agudo, "u",
	"a"+grave, "a", "e"+grave, "e", "i"+grave, "i", "o"+grave, "o", "u"+grave, "u",
	"u"+dieresis, "u",
	"n"+virgulilla, "ñ",
)

// Plegar lowercases s and removes its accents, keeping the ñ, for both NFC
// and NFD input. It does not trim or collapse spaces: callers that compare
// names normalise them first.
func Plegar(s string) string {
	return tildes.Replace(strings.ToLower(s))
}

// MaxExtracto is the length, in runes, at which Extracto cuts.
const MaxExtracto = 300

const puntosSuspensivos = "…"

// Extracto returns the excerpt of a passage: every run of white space
// (line breaks included) becomes one space, the ends are trimmed, and what
// goes past MaxExtracto runes is cut and ends with «…». It is plain text.
func Extracto(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= MaxExtracto {
		return s
	}
	return string([]rune(s)[:MaxExtracto]) + puntosSuspensivos
}
