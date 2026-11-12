package ia

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// A6, escenario «Logs»: whatever the outcome, no log line and no returned
// error contains the input, the output or the detected personal data.
func TestServicio_LogsSinContenido(t *testing.T) {
	var buf bytes.Buffer
	anterior := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = anterior })

	const (
		textoItem = "Enunciado secreto sobre la cadena de frío"
		email     = "persona.privada@example.com"
		dni       = "45678912"
	)
	in := EntradaClasificar{Items: []ItemTexto{{ID: 1, Texto: textoItem}}, Taxonomia: []OpcionTaxonomia{{AsignaturaID: 3, Nombre: "Salud pública"}}}
	conEmail := EntradaClasificar{Items: []ItemTexto{{ID: 1, Texto: "Contacto " + email}}, Taxonomia: in.Taxonomia}
	conDNI := EntradaClasificar{Items: []ItemTexto{{ID: 1, Texto: "Paciente " + dni}}, Taxonomia: in.Taxonomia}

	ctx := context.Background()
	var errs []error
	for _, c := range []struct {
		prov Proveedor
		in   EntradaClasificar
		reg  Registrador
	}{
		{NewDoble(), in, &registroMemoria{}},
		{NewDoble(SalidaInvalida(2)), in, &registroMemoria{}},
		{NewDoble(Lento()), in, &registroMemoria{}},
		{NewDoble(Transitorio(3)), in, &registroMemoria{}},
		{NewDoble(), conEmail, &registroMemoria{}},
		{NewDoble(), conDNI, &registroMemoria{}},
		{NewDoble(), in, &registroMemoria{err: errors.New("pq: " + textoItem)}}, // even a store error quoting data
		{nil, in, &registroMemoria{}},
	} {
		s, _ := nuevo(c.prov, c.reg)
		_, err := s.Clasificar(ctx, sistema, c.in)
		errs = append(errs, err)
	}
	if buf.Len() == 0 {
		t.Fatal("nothing was logged: the test would prove nothing")
	}
	prohibidos := []string{textoItem, "cadena de frío", email, "example.com", dni, "Salud pública",
		"campo_fuera_del_esquema", "resultados", "confianza"}
	for _, p := range prohibidos {
		if strings.Contains(buf.String(), p) {
			t.Errorf("a log line contains %q:\n%s", p, buf.String())
		}
		for _, err := range errs {
			if err != nil && strings.Contains(err.Error(), p) {
				t.Errorf("an error contains %q: %v", p, err)
			}
		}
	}
}
