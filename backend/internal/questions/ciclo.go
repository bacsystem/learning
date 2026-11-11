package questions

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jmoiron/sqlx"
)

// Codes of the life-cycle rules (contract E3-02 §7, E3-07 §7). The handler
// maps each to its HTTP status; the domain only knows the code.
const (
	CodigoEstadoInvalido        = "estado_invalido"
	CodigoRevisionEspecializada = "revision_especializada"
	CodigoFaltaCita             = "falta_cita"
	CodigoNotaObligatoria       = "nota_obligatoria"
	CodigoVerificacionRequerida = "verificacion_requerida"
	CodigoLimiteCitas           = "limite_citas"
	CodigoFuenteNoActiva        = "fuente_no_activa"
	CodigoUltimaCitaActiva      = "ultima_cita_activa"
	CodigoReferenciaInvalida    = "referencia_invalida"
	CodigoValidacion            = "validacion"
	CodigoClaveOficial          = "clave_oficial"
)

// Limits of the review rules (contract E3-07 §3.2, §4.2).
const (
	MotivoMin    = 5
	MotivoMax    = 500
	NotaMin      = 10
	NotaMax      = 500
	MaxCitas     = 20
	campoEstado  = "estado"
	campoMotivo  = "motivo"
	campoNota    = "nota"
	campoPasajes = "pasaje_ids"
)

// ReglaError is a business rule that refused the request. Codigo is the
// stable API code; Detalle is its optional payload.
type ReglaError struct {
	Codigo  string
	Detalle map[string]any
}

func (e *ReglaError) Error() string { return "regla de preguntas incumplida: " + e.Codigo }

func regla(codigo string, detalle map[string]any) *ReglaError {
	return &ReglaError{Codigo: codigo, Detalle: detalle}
}

// ValidacionPreguntaError lists the broken fields (field → rule); it answers
// 422 validacion.
type ValidacionPreguntaError struct {
	Campos map[string]string
}

func (e *ValidacionPreguntaError) Error() string { return "validación de pregunta fallida" }

// Evento is one audit row, written in the mutation's transaction.
type Evento struct {
	Accion    string
	EntidadID int64
	Detalle   map[string]any
}

// Auditor writes ev in the mutation's transaction; its error rolls the
// mutation back. The caller binds the actor, IP and user agent.
type Auditor func(ctx context.Context, tx sqlx.ExecerContext, ev Evento) error

const (
	accionEstado = "pregunta.estado"
	accionCitas  = "pregunta.citas"
)

// transiciones is the state machine of contract E3-02 §3: from → to.
// activo is the inherited bank: it is never a destination.
var transiciones = map[string]map[string]bool{
	EstadoBorrador:   {EstadoAprobada: true, EstadoRechazada: true},
	EstadoEnRevision: {EstadoAprobada: true, EstadoRechazada: true},
	EstadoRechazada:  {EstadoBorrador: true, EstadoEliminada: true},
	EstadoAprobada:   {EstadoEnRevision: true, EstadoEliminada: true},
	EstadoActivo:     {EstadoEnRevision: true, EstadoEliminada: true},
}

func transicionPermitida(desde, hacia string) bool { return transiciones[desde][hacia] }

// estadosDestino are the values PATCH …/estado accepts (eliminada goes by DELETE).
var estadosDestino = map[string]bool{EstadoAprobada: true, EstadoRechazada: true, EstadoBorrador: true, EstadoEnRevision: true}

// CambiarEstadoRequest is the body of PATCH /preguntas/{id}/estado (E3-07 §3.1).
type CambiarEstadoRequest struct {
	Estado     string  `json:"estado"`
	Motivo     *string `json:"motivo"`
	Verificado *bool   `json:"verificado"`
	Nota       *string `json:"nota"`
}

// citaFila is a current citation with the state of its fuente.
type citaFila struct {
	PasajeID     int64
	FuenteID     int64
	FuenteActiva bool
}

// preguntaBloqueada is what the rules read after the row lock.
type preguntaBloqueada struct {
	Estado string
	Origen string
	Citas  []citaFila
}

// citasActivas counts the citations to a pasaje of an 'activa' fuente.
func (b preguntaBloqueada) citasActivas() int {
	n := 0
	for _, c := range b.Citas {
		if c.FuenteActiva {
			n++
		}
	}
	return n
}

// cambioEstado is the outcome of a validated transition.
type cambioEstado struct {
	Desde, Hacia  string
	MotivoRechazo *string // preguntas.motivo_rechazo; nil clears it
	Detalle       map[string]any
}

// validarCuerpoEstado runs steps 2–3 of E3-07 §3.2, which need no database.
func validarCuerpoEstado(req CambiarEstadoRequest) error {
	if !estadosDestino[req.Estado] {
		return &ValidacionPreguntaError{Campos: map[string]string{campoEstado: "valor no permitido"}}
	}
	motivo := recortar(req.Motivo)
	switch req.Estado {
	case EstadoRechazada:
		if n := utf8.RuneCountInString(motivo); n < MotivoMin || n > MotivoMax {
			return &ValidacionPreguntaError{Campos: map[string]string{campoMotivo: fmt.Sprintf("entre %d y %d caracteres", MotivoMin, MotivoMax)}}
		}
	case EstadoEnRevision:
		if utf8.RuneCountInString(motivo) > MotivoMax {
			return &ValidacionPreguntaError{Campos: map[string]string{campoMotivo: fmt.Sprintf("máximo %d caracteres", MotivoMax)}}
		}
	}
	return nil
}

// decidirCambioEstado applies steps 6–11 of E3-07 §3.2, in order, to the state
// read under the row lock. It returns the first rule that fails.
func decidirCambioEstado(b preguntaBloqueada, req CambiarEstadoRequest) (cambioEstado, error) {
	if !transicionPermitida(b.Estado, req.Estado) {
		return cambioEstado{}, regla(CodigoEstadoInvalido, map[string]any{"estado_actual": b.Estado})
	}
	c := cambioEstado{Desde: b.Estado, Hacia: req.Estado}
	switch req.Estado {
	case EstadoAprobada:
		nota, err := validarAprobacion(b, req)
		if err != nil {
			return cambioEstado{}, err
		}
		c.Detalle = map[string]any{"verificado": true, "nota": nota}
	case EstadoRechazada:
		motivo := recortar(req.Motivo)
		c.MotivoRechazo = &motivo
		c.Detalle = map[string]any{"motivo": motivo}
	case EstadoEnRevision:
		c.Detalle = map[string]any{"motivo": nilSiVacio(recortar(req.Motivo))}
	default: // borrador: reopening clears the rejection reason
		c.Detalle = map[string]any{"motivo": nil}
	}
	return c, nil
}

// validarAprobacion is steps 7–11. It returns the note to audit (nil if none).
func validarAprobacion(b preguntaBloqueada, req CambiarEstadoRequest) (any, error) {
	revalidar := b.Estado == EstadoEnRevision
	if !revalidar && b.Origen != OrigenManual {
		return nil, regla(CodigoRevisionEspecializada, map[string]any{"origen": b.Origen})
	}
	// step 8 (conflicto_clave) is reserved: E3-01/E3-03 activate it.
	if b.citasActivas() < 1 {
		return nil, regla(CodigoFaltaCita, nil)
	}
	nota := recortar(req.Nota)
	n := utf8.RuneCountInString(nota)
	if revalidar && b.Origen != OrigenManual {
		if n < NotaMin || n > NotaMax {
			return nil, regla(CodigoNotaObligatoria, map[string]any{"min": NotaMin, "max": NotaMax})
		}
	} else if n > NotaMax {
		return nil, &ValidacionPreguntaError{Campos: map[string]string{campoNota: fmt.Sprintf("máximo %d caracteres", NotaMax)}}
	}
	if req.Verificado == nil || !*req.Verificado {
		return nil, regla(CodigoVerificacionRequerida, nil)
	}
	return nilSiVacio(nota), nil
}

func recortar(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

// nilSiVacio returns nil for "" so the audit row carries null, not "".
func nilSiVacio(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// motivoEdicion is the audit motive when editing content sends a question to review.
const motivoEdicion = "edición de contenido"

// retroGuardada is the stored feedback of a question.
type retroGuardada struct {
	Explicacion string
	Referencia  *string
}

// contenidoGuardado is what a PUT is compared with (E3-02 §4.5): what is
// stored, not what the client sent before.
type contenidoGuardado struct {
	Estado, Origen, Enunciado, Tipo string
	Retro                           *retroGuardada // nil: no feedback stored
}

// requiereRevision: editing the content of an approved or inherited question
// sends it back to review.
func (c contenidoGuardado) requiereRevision() bool {
	return c.Estado == EstadoAprobada || c.Estado == EstadoActivo
}

// cambiaContenido reports whether req changes the content: statement, type,
// alternatives (already compared by the caller) or feedback. Metadata
// (asignatura, tema, etiquetas, dificultad, grado, especialidad) is not content.
func (c contenidoGuardado) cambiaContenido(req UpdatePreguntaRequest, cambioRespuestas bool) bool {
	return c.Enunciado != req.Enunciado || c.Tipo != req.Tipo || cambioRespuestas || c.cambiaRetro(req.Retroalimentacion)
}

func (c contenidoGuardado) cambiaRetro(nueva *CreateRetroRequest) bool {
	switch {
	case c.Retro == nil && nueva == nil:
		return false
	case c.Retro == nil || nueva == nil:
		return true
	}
	return c.Retro.Explicacion != nueva.Explicacion || valorODefecto(c.Retro.Referencia) != valorODefecto(nueva.Referencia)
}

func valorODefecto(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
