package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cbaciliod/learning/internal/locale"
)

// Marker texts that differ between the Spanish and English templates.
const (
	markerRecoveryEN   = "Reset your password"
	markerRecoveryES   = "Restablecer contraseña"
	markerResetDoneEN  = "Password updated"
	markerResetDoneES  = "Contraseña actualizada"
	markerVerifyEN     = "Complete registration"
	markerVerifyES     = "Completar registro"
	markerPendingEN    = "under review"
	markerPendingES    = "en revisión"
	langAttrEN         = `<html lang="en">`
	langAttrES         = `<html lang="es">`
	brandHeaderPattern = ">" + emailBrand + "</p>"
)

// postJSON sends a raw JSON body to a public auth route on a fresh mux, so the
// per-route rate limiter never interferes between cases.
func (f *fixture) postJSON(path, body string) *httptest.ResponseRecorder {
	f.t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	f.mux().ServeHTTP(rr, req)
	return rr
}

// lastEmail returns the newest queued message for to.
func (f *fixture) lastEmail(to string) (subject, body string) {
	f.t.Helper()
	err := f.db.QueryRow(`SELECT subject, body_html FROM email_queue WHERE to_address=$1 ORDER BY id DESC LIMIT 1`, to).Scan(&subject, &body)
	if err != nil {
		f.t.Fatalf("no e-mail queued for %s: %v", to, err)
	}
	return subject, body
}

func assertLocalized(t *testing.T, body, wantMarker, otherMarker, wantLang string) {
	t.Helper()
	for _, want := range []string{wantMarker, wantLang, brandHeaderPattern} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
	if strings.Contains(body, otherMarker) {
		t.Errorf("body contains the other language marker %q", otherMarker)
	}
}

func TestForgotPassword_EmailLanguage(t *testing.T) {
	tests := []struct {
		name       string
		localeJSON string
		wantMarker string
		other      string
		wantLang   string
	}{
		{"en", `,"locale":"en"`, markerRecoveryEN, markerRecoveryES, langAttrEN},
		{"es", `,"locale":"es"`, markerRecoveryES, markerRecoveryEN, langAttrES},
		{"absent", ``, markerRecoveryES, markerRecoveryEN, langAttrES},
		{"unknown", `,"locale":"fr"`, markerRecoveryES, markerRecoveryEN, langAttrES},
		{"regional variant", `,"locale":"en-US"`, markerRecoveryES, markerRecoveryEN, langAttrES},
		{"padded upper case", `,"locale":" EN "`, markerRecoveryEN, markerRecoveryES, langAttrEN},
		{"null", `,"locale":null`, markerRecoveryES, markerRecoveryEN, langAttrES},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.activeUser("fp", "usuario")
			email := f.email("fp")
			rr := f.postJSON("/auth/forgot-password", fmt.Sprintf(`{"email":%q%s}`, email, tt.localeJSON))
			if rr.Code != http.StatusOK {
				t.Fatalf("status %d, body %s", rr.Code, rr.Body)
			}
			_, body := f.lastEmail(email)
			assertLocalized(t, body, tt.wantMarker, tt.other, tt.wantLang)
		})
	}
}

func TestForgotPassword_SubjectFollowsLocale(t *testing.T) {
	f := newFixture(t)
	f.activeUser("fps", "usuario")
	email := f.email("fps")
	f.postJSON("/auth/forgot-password", fmt.Sprintf(`{"email":%q,"locale":"en"}`, email))
	if subject, _ := f.lastEmail(email); subject != "Password recovery" {
		t.Errorf("subject = %q", subject)
	}
}

// The response must not depend on the locale (anti-enumeration).
func TestForgotPassword_LocaleDoesNotChangeResponse(t *testing.T) {
	f := newFixture(t)
	f.activeUser("fpr", "usuario")
	email := f.email("fpr")
	base := f.postJSON("/auth/forgot-password", fmt.Sprintf(`{"email":%q}`, email))
	for _, loc := range []string{"en", "fr", ""} {
		rr := f.postJSON("/auth/forgot-password", fmt.Sprintf(`{"email":%q,"locale":%q}`, email, loc))
		if rr.Code != base.Code || rr.Body.String() != base.Body.String() {
			t.Errorf("locale %q: %d %s, want %d %s", loc, rr.Code, rr.Body, base.Code, base.Body)
		}
	}
}

func TestForgotPassword_NonStringLocaleIsSyntaxError(t *testing.T) {
	f := newFixture(t)
	rr := f.postJSON("/auth/forgot-password", `{"email":"a@example.com","locale":123}`)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status %d, want 400", rr.Code)
	}
}

func TestResetPassword_EmailLanguage(t *testing.T) {
	tests := []struct {
		name       string
		localeJSON string
		wantMarker string
		other      string
		wantLang   string
	}{
		{"en", `,"locale":"en"`, markerResetDoneEN, markerResetDoneES, langAttrEN},
		{"absent", ``, markerResetDoneES, markerResetDoneEN, langAttrES},
		{"unknown", `,"locale":"fr"`, markerResetDoneES, markerResetDoneEN, langAttrES},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			uid, _ := f.activeUser("rp", "usuario")
			token := "tok-locale-" + f.suffix
			f.resetToken(uid, token, time.Now().Add(time.Hour), false)
			rr := f.postJSON("/auth/reset-password",
				fmt.Sprintf(`{"token":%q,"password":"NewPassword1!","confirm_password":"NewPassword1!"%s}`, token, tt.localeJSON))
			if rr.Code != http.StatusOK {
				t.Fatalf("status %d, body %s", rr.Code, rr.Body)
			}
			_, body := f.lastEmail(f.email("rp"))
			assertLocalized(t, body, tt.wantMarker, tt.other, tt.wantLang)
		})
	}
}

func TestRegisterAndVerifyEmail_EmailLanguage(t *testing.T) {
	tests := []struct {
		name       string
		localeJSON string
		verifyEN   bool
	}{
		{"en", `,"locale":"en"`, true},
		{"absent", ``, false},
		{"unknown", `,"locale":"fr"`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			email := f.email("rg")
			_, dni := f.persona(email)

			rr := f.postJSON("/auth/register", fmt.Sprintf(`{"dni":%q,"email":%q%s}`, dni, email, tt.localeJSON))
			if rr.Code != http.StatusAccepted {
				t.Fatalf("register status %d, body %s", rr.Code, rr.Body)
			}
			wantMarker, other, lang := markerVerifyES, markerVerifyEN, langAttrES
			if tt.verifyEN {
				wantMarker, other, lang = markerVerifyEN, markerVerifyES, langAttrEN
			}
			_, body := f.lastEmail(email)
			assertLocalized(t, body, wantMarker, other, lang)

			token := f.lastVerifyToken(email)
			rr = f.postJSON("/auth/verify-email", fmt.Sprintf(
				`{"token":%q,"username":%q,"password":"Password123!","confirm_password":"Password123!"%s}`,
				token, f.username("rg"), tt.localeJSON))
			if rr.Code != http.StatusCreated {
				t.Fatalf("verify status %d, body %s", rr.Code, rr.Body)
			}
			wantMarker, other, lang = markerPendingES, markerPendingEN, langAttrES
			if tt.verifyEN {
				wantMarker, other, lang = markerPendingEN, markerPendingES, langAttrEN
			}
			_, body = f.lastEmail(email)
			assertLocalized(t, body, wantMarker, other, lang)
		})
	}
}

// E-mails triggered by an administrator are always Spanish, with the brand.
func TestAdminEmails_StaySpanishWithBrand(t *testing.T) {
	for name, render := range map[string]func() (string, string){
		"activated": func() (string, string) { return emailActivado("Ana", "Perez", "ana", "https://x/login") },
		"rejected":  func() (string, string) { return emailRechazado("Ana", "Perez") },
		"inactive":  func() (string, string) { return emailInactivo("Ana", "Perez") },
		"changed":   func() (string, string) { return emailCorreoCambiado() },
	} {
		t.Run(name, func(t *testing.T) {
			_, body := render()
			if !strings.Contains(body, langAttrES) || !strings.Contains(body, brandHeaderPattern) {
				t.Error("admin e-mail must be Spanish and signed Capacitate")
			}
		})
	}
}

// Every user-facing text of a template exists in both languages: the body in
// one language contains none of the texts of the other, and the subjects
// differ. It walks the copy structs, so a new field is covered automatically.
func TestEmailTemplates_LanguagesDoNotLeak(t *testing.T) {
	type rendered struct {
		subject, body string
		texts         []string
	}
	templates := map[string]func(loc string) rendered{
		"recovery": func(loc string) rendered {
			sub, body := emailOlvidePassword(loc, "https://x/r")
			return rendered{sub, body, textsOf(olvidePasswordCopy(loc))}
		},
		"reset done": func(loc string) rendered {
			sub, body := emailPasswordRestablecida(loc, "https://x/l")
			return rendered{sub, body, textsOf(passwordRestablecidaCopy(loc))}
		},
		"verify": func(loc string) rendered {
			sub, body := emailVerificarRegistro(loc, "Ana", "Perez", "https://x/v")
			return rendered{sub, body, textsOf(verificarRegistroCopy(loc))}
		},
		"registration pending": func(loc string) rendered {
			sub, body := emailRegistroPendiente(loc, "Ana", "Perez", "ana")
			return rendered{sub, body, textsOf(registroPendienteCopy(loc))}
		},
	}
	shared := map[string][]string{
		locale.ES: {buttonFallback(locale.ES), layoutAutoNotice(locale.ES)},
		locale.EN: {buttonFallback(locale.EN), layoutAutoNotice(locale.EN)},
	}
	for name, render := range templates {
		t.Run(name, func(t *testing.T) {
			es, en := render(locale.ES), render(locale.EN)
			if es.subject == en.subject {
				t.Errorf("subject is the same in both languages: %q", es.subject)
			}
			for _, c := range []struct {
				body, other string
				foreign     []string
			}{
				{en.body, "es", append(es.texts, shared[locale.ES]...)},
				{es.body, "en", append(en.texts, shared[locale.EN]...)},
			} {
				for _, text := range c.foreign {
					if strings.Contains(c.body, text) {
						t.Errorf("body contains the %s text %q", c.other, text)
					}
				}
			}
		})
	}
}

// textsOf returns the fixed fragments of every string field of a copy struct
// (the part before a %s verb for greetings), skipping the e-mail subject.
func textsOf(texts any) []string {
	v := reflect.ValueOf(texts)
	var out []string
	for i := 0; i < v.NumField(); i++ {
		if v.Type().Field(i).Name == "subject" {
			continue
		}
		text, _, _ := strings.Cut(v.Field(i).String(), "%s")
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}
