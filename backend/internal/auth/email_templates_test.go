package auth

import (
	"strings"
	"testing"
)

// H130: names and usernames come from the database (CSV import, admin edits)
// and must never be interpreted as HTML in any e-mail template.
func TestEmailTemplates_EscapeUserData(t *testing.T) {
	const evil = `<a href=x>clic</a>`
	cases := map[string]func() (string, string){
		"activado":  func() (string, string) { return emailActivado(evil, evil, evil, "https://app.test/auth/sign-in") },
		"rechazado": func() (string, string) { return emailRechazado(evil, evil) },
		"inactivo":  func() (string, string) { return emailInactivo(evil, evil) },
		"registroPendiente": func() (string, string) {
			return emailRegistroPendiente(evil, evil, evil)
		},
		"verificarRegistro": func() (string, string) {
			return emailVerificarRegistro(evil, evil, "https://app.test/auth/verify-email?token=abc")
		},
	}
	for name, render := range cases {
		t.Run(name, func(t *testing.T) {
			_, body := render()
			if strings.Contains(body, "<a href=x>") {
				t.Fatalf("user data rendered as HTML:\n%s", body)
			}
			if !strings.Contains(body, "&lt;a href=x&gt;") {
				t.Fatal("escaped user data missing from the body")
			}
		})
	}
}

// URLs go into an href attribute and the visible text: they are escaped too.
func TestEmailTemplates_EscapeURL(t *testing.T) {
	_, body := emailOlvidePassword(`https://app.test/x?a=1&b="><script>`)
	if strings.Contains(body, `"><script>`) {
		t.Fatalf("URL not escaped:\n%s", body)
	}
}
