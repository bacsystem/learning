package questions

import (
	"errors"
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func citaActiva() []citaFila { return []citaFila{{PasajeID: 1, FuenteID: 1, FuenteActiva: true}} }

func codigoRegla(t *testing.T, err error) string {
	t.Helper()
	var re *ReglaError
	var ve *ValidacionPreguntaError
	switch {
	case err == nil:
		return ""
	case errors.As(err, &re):
		return re.Codigo
	case errors.As(err, &ve):
		return CodigoValidacion
	case errors.Is(err, ErrNotFound):
		return "no_encontrado"
	}
	t.Fatalf("error inesperado: %v", err)
	return ""
}

// Steps 2–3 of E3-07 §3.2.
func TestValidarCuerpoEstado(t *testing.T) {
	tests := []struct {
		name string
		req  CambiarEstadoRequest
		want string
	}{
		{"estado desconocido", CambiarEstadoRequest{Estado: "activo"}, CodigoValidacion},
		{"eliminada va por DELETE", CambiarEstadoRequest{Estado: EstadoEliminada}, CodigoValidacion},
		{"estado vacío", CambiarEstadoRequest{}, CodigoValidacion},
		{"rechazar sin motivo", CambiarEstadoRequest{Estado: EstadoRechazada}, CodigoValidacion},
		{"rechazar con motivo corto", CambiarEstadoRequest{Estado: EstadoRechazada, Motivo: ptr("  abc  ")}, CodigoValidacion},
		{"rechazar con motivo largo", CambiarEstadoRequest{Estado: EstadoRechazada, Motivo: ptr(strings.Repeat("a", MotivoMax+1))}, CodigoValidacion},
		{"rechazar con motivo válido", CambiarEstadoRequest{Estado: EstadoRechazada, Motivo: ptr("Clave incorrecta")}, ""},
		{"rechazar con motivo de 500", CambiarEstadoRequest{Estado: EstadoRechazada, Motivo: ptr(strings.Repeat("ñ", MotivoMax))}, ""},
		{"en_revision sin motivo", CambiarEstadoRequest{Estado: EstadoEnRevision}, ""},
		{"en_revision con motivo largo", CambiarEstadoRequest{Estado: EstadoEnRevision, Motivo: ptr(strings.Repeat("a", MotivoMax+1))}, CodigoValidacion},
		{"borrador ignora el motivo", CambiarEstadoRequest{Estado: EstadoBorrador, Motivo: ptr(strings.Repeat("a", 900))}, ""},
		{"aprobada no valida el motivo", CambiarEstadoRequest{Estado: EstadoAprobada, Motivo: ptr("x")}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := codigoRegla(t, validarCuerpoEstado(tt.req)); got != tt.want {
				t.Errorf("código = %q, want %q", got, tt.want)
			}
		})
	}
}

// Steps 6–11 of E3-07 §3.2, including the fixed order of the rules.
func TestDecidirCambioEstado(t *testing.T) {
	notaValida := ptr("Revisada contra la NTS 2024, pasaje 3.2.")
	tests := []struct {
		name string
		b    preguntaBloqueada
		req  CambiarEstadoRequest
		want string // rule code; "" = allowed
	}{
		// E3-02 §3: one case per row.
		{"borrador → aprobada (manual)", preguntaBloqueada{EstadoBorrador, OrigenManual, citaActiva()}, CambiarEstadoRequest{Estado: EstadoAprobada, Verificado: ptr(true)}, ""},
		{"borrador → rechazada", preguntaBloqueada{Estado: EstadoBorrador}, CambiarEstadoRequest{Estado: EstadoRechazada, Motivo: ptr("Clave incorrecta")}, ""},
		{"borrador → en_revision", preguntaBloqueada{Estado: EstadoBorrador}, CambiarEstadoRequest{Estado: EstadoEnRevision}, CodigoEstadoInvalido},
		{"borrador → borrador", preguntaBloqueada{Estado: EstadoBorrador}, CambiarEstadoRequest{Estado: EstadoBorrador}, CodigoEstadoInvalido},
		{"en_revision → aprobada (revalidar manual)", preguntaBloqueada{EstadoEnRevision, OrigenManual, citaActiva()}, CambiarEstadoRequest{Estado: EstadoAprobada, Verificado: ptr(true)}, ""},
		{"en_revision → rechazada", preguntaBloqueada{Estado: EstadoEnRevision}, CambiarEstadoRequest{Estado: EstadoRechazada, Motivo: ptr("Clave incorrecta")}, ""},
		{"en_revision → borrador", preguntaBloqueada{Estado: EstadoEnRevision}, CambiarEstadoRequest{Estado: EstadoBorrador}, CodigoEstadoInvalido},
		{"rechazada → borrador (reabrir)", preguntaBloqueada{Estado: EstadoRechazada}, CambiarEstadoRequest{Estado: EstadoBorrador}, ""},
		{"rechazada → aprobada", preguntaBloqueada{Estado: EstadoRechazada}, CambiarEstadoRequest{Estado: EstadoAprobada, Verificado: ptr(true)}, CodigoEstadoInvalido},
		{"rechazada → en_revision", preguntaBloqueada{Estado: EstadoRechazada}, CambiarEstadoRequest{Estado: EstadoEnRevision}, CodigoEstadoInvalido},
		{"aprobada → en_revision", preguntaBloqueada{Estado: EstadoAprobada}, CambiarEstadoRequest{Estado: EstadoEnRevision}, ""},
		{"aprobada → rechazada", preguntaBloqueada{Estado: EstadoAprobada}, CambiarEstadoRequest{Estado: EstadoRechazada, Motivo: ptr("Clave incorrecta")}, CodigoEstadoInvalido},
		{"aprobada → aprobada", preguntaBloqueada{EstadoAprobada, OrigenManual, citaActiva()}, CambiarEstadoRequest{Estado: EstadoAprobada, Verificado: ptr(true)}, CodigoEstadoInvalido},
		{"activo → en_revision", preguntaBloqueada{Estado: EstadoActivo}, CambiarEstadoRequest{Estado: EstadoEnRevision}, ""},
		{"activo → aprobada", preguntaBloqueada{EstadoActivo, OrigenManual, citaActiva()}, CambiarEstadoRequest{Estado: EstadoAprobada, Verificado: ptr(true)}, CodigoEstadoInvalido},
		{"eliminada tras el bloqueo es inexistente", preguntaBloqueada{Estado: EstadoEliminada}, CambiarEstadoRequest{Estado: EstadoBorrador}, "no_encontrado"},

		// Steps 7–11.
		{"aprobar balotario: revision_especializada", preguntaBloqueada{EstadoBorrador, OrigenBalotario, citaActiva()}, CambiarEstadoRequest{Estado: EstadoAprobada, Verificado: ptr(true)}, CodigoRevisionEspecializada},
		{"aprobar ia: revision_especializada", preguntaBloqueada{EstadoBorrador, OrigenIA, citaActiva()}, CambiarEstadoRequest{Estado: EstadoAprobada, Verificado: ptr(true)}, CodigoRevisionEspecializada},
		{"aprobar sin cita", preguntaBloqueada{Estado: EstadoBorrador, Origen: OrigenManual}, CambiarEstadoRequest{Estado: EstadoAprobada, Verificado: ptr(true)}, CodigoFaltaCita},
		{"aprobar con cita a fuente retirada", preguntaBloqueada{EstadoBorrador, OrigenManual, []citaFila{{PasajeID: 1, FuenteID: 1}}}, CambiarEstadoRequest{Estado: EstadoAprobada, Verificado: ptr(true)}, CodigoFaltaCita},
		{"aprobar sin casilla", preguntaBloqueada{EstadoBorrador, OrigenManual, citaActiva()}, CambiarEstadoRequest{Estado: EstadoAprobada}, CodigoVerificacionRequerida},
		{"aprobar con casilla en false", preguntaBloqueada{EstadoBorrador, OrigenManual, citaActiva()}, CambiarEstadoRequest{Estado: EstadoAprobada, Verificado: ptr(false)}, CodigoVerificacionRequerida},
		{"aprobar manual con nota larga", preguntaBloqueada{EstadoBorrador, OrigenManual, citaActiva()}, CambiarEstadoRequest{Estado: EstadoAprobada, Verificado: ptr(true), Nota: ptr(strings.Repeat("a", NotaMax+1))}, CodigoValidacion},
		{"revalidar manual sin nota", preguntaBloqueada{EstadoEnRevision, OrigenManual, citaActiva()}, CambiarEstadoRequest{Estado: EstadoAprobada, Verificado: ptr(true)}, ""},
		{"revalidar ia con nota", preguntaBloqueada{EstadoEnRevision, OrigenIA, citaActiva()}, CambiarEstadoRequest{Estado: EstadoAprobada, Verificado: ptr(true), Nota: notaValida}, ""},
		{"revalidar balotario con nota", preguntaBloqueada{EstadoEnRevision, OrigenBalotario, citaActiva()}, CambiarEstadoRequest{Estado: EstadoAprobada, Verificado: ptr(true), Nota: notaValida}, ""},
		{"revalidar ia sin nota", preguntaBloqueada{EstadoEnRevision, OrigenIA, citaActiva()}, CambiarEstadoRequest{Estado: EstadoAprobada, Verificado: ptr(true)}, CodigoNotaObligatoria},
		{"revalidar ia con nota de 9", preguntaBloqueada{EstadoEnRevision, OrigenIA, citaActiva()}, CambiarEstadoRequest{Estado: EstadoAprobada, Verificado: ptr(true), Nota: ptr("  123456789  ")}, CodigoNotaObligatoria},
		{"revalidar ia con nota de 10", preguntaBloqueada{EstadoEnRevision, OrigenIA, citaActiva()}, CambiarEstadoRequest{Estado: EstadoAprobada, Verificado: ptr(true), Nota: ptr("1234567890")}, ""},
		{"revalidar ia con nota de 501", preguntaBloqueada{EstadoEnRevision, OrigenIA, citaActiva()}, CambiarEstadoRequest{Estado: EstadoAprobada, Verificado: ptr(true), Nota: ptr(strings.Repeat("a", NotaMax+1))}, CodigoNotaObligatoria},

		// The order: each scenario gets its own code regardless of the later fields.
		{"orden: estado_invalido antes que cualquier guarda", preguntaBloqueada{Estado: EstadoRechazada, Origen: OrigenIA}, CambiarEstadoRequest{Estado: EstadoAprobada}, CodigoEstadoInvalido},
		{"orden: revision_especializada antes que falta_cita", preguntaBloqueada{Estado: EstadoBorrador, Origen: OrigenIA}, CambiarEstadoRequest{Estado: EstadoAprobada}, CodigoRevisionEspecializada},
		{"orden: falta_cita antes que la casilla", preguntaBloqueada{Estado: EstadoBorrador, Origen: OrigenManual}, CambiarEstadoRequest{Estado: EstadoAprobada}, CodigoFaltaCita},
		{"orden: nota_obligatoria antes que la casilla", preguntaBloqueada{EstadoEnRevision, OrigenIA, citaActiva()}, CambiarEstadoRequest{Estado: EstadoAprobada}, CodigoNotaObligatoria},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decidirCambioEstado(tt.b, tt.req)
			if got := codigoRegla(t, err); got != tt.want {
				t.Errorf("código = %q, want %q (err=%v)", got, tt.want, err)
			}
		})
	}
}

func TestDecidirCambioEstado_Detalle(t *testing.T) {
	aprobar := func(nota *string) cambioEstado {
		c, err := decidirCambioEstado(preguntaBloqueada{EstadoEnRevision, OrigenManual, citaActiva()},
			CambiarEstadoRequest{Estado: EstadoAprobada, Verificado: ptr(true), Nota: nota})
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	if c := aprobar(nil); c.Detalle["nota"] != nil || c.Detalle["verificado"] != true || c.MotivoRechazo != nil {
		t.Errorf("sin nota: %+v", c)
	}
	if c := aprobar(ptr("  ")); c.Detalle["nota"] != nil {
		t.Errorf("nota en blanco debe auditarse como null: %+v", c)
	}
	if c := aprobar(ptr("  buena nota  ")); c.Detalle["nota"] != "buena nota" {
		t.Errorf("la nota se recorta: %+v", c)
	}

	c, err := decidirCambioEstado(preguntaBloqueada{Estado: EstadoBorrador}, CambiarEstadoRequest{Estado: EstadoRechazada, Motivo: ptr("  Clave incorrecta ")})
	if err != nil || c.MotivoRechazo == nil || *c.MotivoRechazo != "Clave incorrecta" || c.Detalle["motivo"] != "Clave incorrecta" {
		t.Errorf("rechazar: %+v, %v", c, err)
	}
	// verificado and nota are ignored for any other target.
	c, err = decidirCambioEstado(preguntaBloqueada{Estado: EstadoRechazada}, CambiarEstadoRequest{Estado: EstadoBorrador, Nota: ptr(strings.Repeat("a", 900)), Motivo: ptr("x")})
	if err != nil || c.MotivoRechazo != nil || c.Detalle["motivo"] != nil {
		t.Errorf("reabrir: %+v, %v", c, err)
	}
	c, err = decidirCambioEstado(preguntaBloqueada{Estado: EstadoAprobada}, CambiarEstadoRequest{Estado: EstadoEnRevision})
	if err != nil || c.Detalle["motivo"] != nil {
		t.Errorf("en_revision sin motivo: %+v, %v", c, err)
	}
}
