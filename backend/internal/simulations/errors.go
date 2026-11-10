package simulations

import (
	"errors"
	"fmt"
)

// ErrNotFound is returned when a simulacro is not found.
var ErrNotFound = errors.New("Simulacro no encontrado.")

// ErrForbidden is returned when a user tries to access another user's simulacro.
var ErrForbidden = errors.New("Acceso denegado.")

// ErrSimulacroEstado is returned when the simulacro is not in the expected state.
var ErrSimulacroEstado = errors.New("El simulacro no está en el estado requerido.")

// ErrNoConfig is returned when there is no active configuracion_examen for the proceso.
var ErrNoConfig = errors.New("No hay configuración de examen activa para este proceso.")

// ErrInsufficientQuestions is returned when the question bank can't satisfy the request.
var ErrInsufficientQuestions = errors.New("Preguntas insuficientes para generar el simulacro.")

// ErrNivelNoPermitido is returned when the user's nivel doesn't match the proceso.
var ErrNivelNoPermitido = errors.New("No tienes acceso a este proceso.")

// ErrInscripcionRequired is returned when the user doesn't have an approved inscripcion.
var ErrInscripcionRequired = errors.New("Debes tener una inscripción aprobada para crear un simulacro.")

// ErrValidation is returned when the request is well-formed but its values are not acceptable (422).
var ErrValidation = errors.New("Datos inválidos.")

// ErrTiempoAgotado is returned when an answer arrives after the time limit (409).
var ErrTiempoAgotado = errors.New("El tiempo del simulacro se agotó.")

// ExpiradoError reports that the simulacro's time ran out and it was
// finalized: the client should go to its result (409 tiempo_agotado with
// resultado_id). It matches ErrTiempoAgotado with errors.Is.
type ExpiradoError struct {
	ResultadoID int64
}

func (e ExpiradoError) Error() string { return ErrTiempoAgotado.Error() }

// Unwrap makes errors.Is(err, ErrTiempoAgotado) true.
func (e ExpiradoError) Unwrap() error { return ErrTiempoAgotado }

// errExpirado is an internal signal: the simulacro is still en_curso but its
// time is over, so it must be finalized lazily.
var errExpirado = errors.New("simulacro expirado")

// errEnCursoDuplicado is an internal signal: the unique index on en_curso
// simulacros rejected the insert (a concurrent request won).
var errEnCursoDuplicado = errors.New("simulacro en curso duplicado")

// ErrConflict signals that the user already has an en_curso simulacro.
// SimulacroID carries the conflicting simulacro's ID for the 409 body.
type ErrConflict struct {
	SimulacroID int64
}

func (e ErrConflict) Error() string {
	return fmt.Sprintf("simulacro_en_curso: id=%d", e.SimulacroID)
}
