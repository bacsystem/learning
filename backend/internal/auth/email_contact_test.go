package auth

import (
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/locale"
)

// QA #275: the "request received" e-mail gives a real contact, in both
// languages, and promises no fixed response time.
func TestEmailRegistroPendiente_ContactAndNoFixedDeadline(t *testing.T) {
	const contact = "soporte@example.test"
	tests := []struct {
		loc         string
		wantLine    string
		forbidden   []string
		wantContact bool
	}{
		{locale.ES, "escribe a", []string{"1 a 2", "días hábiles", "área de administración"}, true},
		{locale.EN, "write to", []string{"1 to 2", "business days", "administration area"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.loc, func(t *testing.T) {
			_, body := emailRegistroPendiente(tt.loc, contact, "Ana", "Perez", "ana")
			if !strings.Contains(body, `href="mailto:`+contact+`"`) || !strings.Contains(body, tt.wantLine) {
				t.Errorf("contact sentence missing:\n%s", body)
			}
			for _, bad := range tt.forbidden {
				if strings.Contains(body, bad) {
					t.Errorf("body contains %q", bad)
				}
			}
		})
	}
}

func TestEmailRegistroPendiente_NoContactOmitsSentence(t *testing.T) {
	for _, loc := range []string{locale.ES, locale.EN} {
		_, body := emailRegistroPendiente(loc, "", "Ana", "Perez", "ana")
		if strings.Contains(body, "mailto:") {
			t.Errorf("%s: unexpected mailto without a contact", loc)
		}
	}
}

// verify-email queues the "request received" e-mail with the service's contact.
func TestVerifyEmail_QueuesContactFromService(t *testing.T) {
	f := newFixture(t)
	WithContactEmail("soporte@example.test")(f.svc)
	email := f.email("owner")
	_, dni := f.persona(email)
	if err := f.register(dni, email); err != nil {
		t.Fatal(err)
	}
	if err := f.verify(f.lastVerifyToken(email), f.username("ct")); err != nil {
		t.Fatal(err)
	}
	_, body := f.lastEmail(email)
	if !strings.Contains(body, `mailto:soporte@example.test`) {
		t.Fatalf("contact missing:\n%s", body)
	}
}

func TestEmailRegistroPendiente_EscapesContact(t *testing.T) {
	_, body := emailRegistroPendiente(locale.ES, `x"><script>@e.test`, "Ana", "Perez", "ana")
	if strings.Contains(body, "<script>") {
		t.Fatal("contact rendered as HTML")
	}
}
