package balotario

import "errors"

// Codes of the API errors (contract E3-01/E3-03 §7). The handler maps each to
// its status; the domain only knows the code.
const (
	CodigoNoEncontrado              = "no_encontrado"
	CodigoTipoFuenteInvalido        = "tipo_fuente_invalido"
	CodigoFuenteNoActiva            = "fuente_no_activa"
	CodigoLimitePreguntas           = "limite_preguntas"
	CodigoVersionDesactualizada     = "version_desactualizada"
	CodigoYaImportado               = "ya_importado"
	CodigoEspecialidadInvalida      = "especialidad_invalida"
	CodigoEspecialidadNoCorresponde = "especialidad_no_corresponde"
	CodigoReferenciaInvalida        = "referencia_invalida"
	CodigoSinValidas                = "sin_validas"
	CodigoValidacion                = "validacion"
)

// ErrNoEncontrado: the fuente does not exist.
var ErrNoEncontrado = errors.New("balotario: fuente no encontrada")

// ReglaError is a rule that refused the request. Codigo is the stable API
// code; Detalle is its optional payload.
type ReglaError struct {
	Codigo  string
	Detalle map[string]any
}

func (e *ReglaError) Error() string { return "regla del balotario incumplida: " + e.Codigo }

func regla(codigo string, detalle map[string]any) *ReglaError {
	return &ReglaError{Codigo: codigo, Detalle: detalle}
}
