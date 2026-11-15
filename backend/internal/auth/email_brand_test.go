package auth

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cbaciliod/learning/internal/correo"
)

func TestEmailTemplates_Brand(t *testing.T) {
	templates := map[string]func() (string, string){
		"recovery":      func() (string, string) { return emailOlvidePassword("es", "https://x/reset") },
		"reset done":    func() (string, string) { return emailPasswordRestablecida("es", "https://x/login") },
		"verify":        func() (string, string) { return emailVerificarRegistro("es", "Ana", "Perez", "https://x/v") },
		"registration":  func() (string, string) { return emailRegistroPendiente("es", "Ana", "Perez", "ana") },
		"activated":     func() (string, string) { return emailActivado("Ana", "Perez", "ana", "https://x/login") },
		"rejected":      func() (string, string) { return emailRechazado("Ana", "Perez") },
		"inactive":      func() (string, string) { return emailInactivo("Ana", "Perez") },
		"email changed": func() (string, string) { return emailCorreoCambiado() },
	}
	footer := fmt.Sprintf("© %d %s", time.Now().Year(), correo.Brand)
	for name, render := range templates {
		t.Run(name, func(t *testing.T) {
			_, body := render()
			if !strings.Contains(body, ">"+correo.Brand+"</p>") {
				t.Error("header does not carry the brand")
			}
			if !strings.Contains(body, footer) {
				t.Errorf("footer %q missing", footer)
			}
			for _, gone := range []string{"Sistema de Capacitación", "Ejército del Perú", "tu unidad", "your unit"} {
				if strings.Contains(body, gone) {
					t.Errorf("body still contains %q", gone)
				}
			}
		})
	}
}
