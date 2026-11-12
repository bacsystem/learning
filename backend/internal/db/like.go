package db

import "strings"

// likeEscaper turns the LIKE metacharacters into literals for the escape
// character `\`. Backslash goes first in the pairs so the escapes it adds are
// not escaped again.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// EscapeLike makes s a literal for LIKE/ILIKE … ESCAPE '\'.
func EscapeLike(s string) string {
	return likeEscaper.Replace(s)
}
