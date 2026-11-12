package db

import "testing"

func TestEscapeLike(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"plain text", "ascenso", "ascenso"},
		{"percent", "100%", `100\%`},
		{"underscore", "a_b", `a\_b`},
		{"backslash", `a\b`, `a\\b`},
		{"backslash before percent", `\%`, `\\\%`},
		{"all metacharacters", `%_\`, `\%\_\\`},
		{"accents untouched", "año ñandú", "año ñandú"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EscapeLike(tt.in); got != tt.want {
				t.Errorf("EscapeLike(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
