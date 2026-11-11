// Package texto holds text helpers shared by several modules and free of any
// domain: today, the folding used to compare names and search text without
// case or accents (ADR 0003 § Plegado).
package texto

import "strings"

// tildes maps every accented vowel (and ü) to its plain form. The ñ is kept on
// purpose so that «año» and «ano» stay different (ADR 0003).
var tildes = strings.NewReplacer(
	"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u",
	"à", "a", "è", "e", "ì", "i", "ò", "o", "ù", "u",
)

// Plegar lowercases s and removes its accents, keeping the ñ. It does not trim
// or collapse spaces: callers that compare names normalise them first.
func Plegar(s string) string {
	return tildes.Replace(strings.ToLower(s))
}
