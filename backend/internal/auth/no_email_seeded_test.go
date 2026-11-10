package auth

import (
	"context"
	"testing"
)

// H119: a persona without e-mail can be stored in two shapes: empty columns
// (fixture.persona("")) or, as cmd/seed and the CSV import write it,
// email = Encrypt("") and email_hmac = HMAC(""). Both must be treated as
// "no e-mail on file" everywhere in auth.

type noEmailShape struct {
	name, tag string
	mk        func() (int64, string)
}

func noEmailShapes(f *fixture) []noEmailShape {
	return []noEmailShape{
		{"columnas vacías", "va", func() (int64, string) { return f.persona("") }},
		{"cifrado de vacío", "ci", func() (int64, string) { return f.seededPersona("") }},
	}
}

func TestRegisterVerify_PersonaWithoutEmail_BothShapes(t *testing.T) {
	f := newFixture(t)
	for _, sh := range noEmailShapes(f) {
		t.Run(sh.name, func(t *testing.T) {
			pid, dni := sh.mk()
			email := f.email("ne" + sh.tag)
			if err := f.register(dni, email); err != nil {
				t.Fatalf("register: %v", err)
			}
			if err := f.verify(f.lastVerifyToken(email), f.username("ne"+sh.tag)); err != nil {
				t.Fatalf("verify: %v", err)
			}
			if got := f.personaEmailHMAC(pid); got != f.repo.emailHMAC(email) {
				t.Fatal("the verified e-mail was not written to the persona")
			}
			p, err := f.repo.GetPersonaProfile(context.Background(), pid)
			if err != nil || p.Email != email {
				t.Fatalf("profile e-mail = %q, %v", p.Email, err)
			}
		})
	}
}

// Many personas without e-mail share email_hmac = HMAC(""): that must never
// read as "this address belongs to another persona" for any of them.
func TestRegisterVerify_ManySeededPersonasWithoutEmail_NoConflict(t *testing.T) {
	f := newFixture(t)
	for i, local := range []string{"uno", "dos", "tres"} {
		pid, dni := f.seededPersona("")
		email := f.email(local)
		if err := f.register(dni, email); err != nil {
			t.Fatalf("persona %d register: %v", i, err)
		}
		if err := f.verify(f.lastVerifyToken(email), f.username("m"+local)); err != nil {
			t.Fatalf("persona %d verify: %v", i, err)
		}
		if f.countUsuarios(pid) != 1 {
			t.Fatalf("persona %d: usuario not created", i)
		}
	}
}

func TestCheckDNI_SeededPersonaWithoutEmail_NoHint(t *testing.T) {
	f := newFixture(t)
	_, dni := f.seededPersona("")
	res, err := f.svc.CheckDNI(context.Background(), dni)
	if err != nil {
		t.Fatal(err)
	}
	if res.EmailMasked != "" {
		t.Fatalf("want no e-mail hint, got %q", res.EmailMasked)
	}
}
