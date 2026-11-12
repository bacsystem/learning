package db

import "strings"

// likeEscaper turns the LIKE metacharacters into literals for the escape
// character `\`. The replacer scans the input once, so the escapes it adds are
// never escaped again.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// EscapeLike makes s a literal for LIKE/ILIKE … ESCAPE '\'.
func EscapeLike(s string) string {
	return likeEscaper.Replace(s)
}
