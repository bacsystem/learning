package practices

import (
	"errors"
	"fmt"
)

// ErrNotFound is returned when a practice is not found.
var ErrNotFound = errors.New("Práctica no encontrada.")

// ErrForbidden is returned by the batch delete when some ids are not the user's
// own (a single someone-else's práctica is ErrNotFound, H137).
var ErrForbidden = errors.New("Acceso denegado.")

// ErrPracticaEstado is returned when the practice is not in the expected state.
var ErrPracticaEstado = errors.New("La práctica no está en el estado requerido.")

// ErrValidation is returned when request validation fails.
var ErrValidation = errors.New("Error de validación.")

// ErrInsufficientQuestions is returned when the question bank can't satisfy the request.
var ErrInsufficientQuestions = errors.New("Preguntas insuficientes para generar la práctica.")

// ErrNivelNoPermitido is returned when the user's nivel doesn't match the proceso (403).
var ErrNivelNoPermitido = errors.New("No tienes acceso a este proceso.")

// ErrInscripcionRequired is returned when the user doesn't have an approved inscripción (403).
var ErrInscripcionRequired = errors.New("Debes tener una inscripción aprobada para crear una práctica.")

// ErrLimiteExcedido is returned when a request exceeds a size cap (422).
var ErrLimiteExcedido = errors.New("Se excedió el máximo permitido.")

// ErrTiempoAgotado is returned when an answer arrives after the time limit (409).
var ErrTiempoAgotado = errors.New("El tiempo de la práctica se agotó.")

// ErrPreguntaValidada is returned when a question already validated in
// inmediato mode is answered again (409): its key has been revealed (H74).
var ErrPreguntaValidada = errors.New("La pregunta ya fue comprobada; su respuesta no puede cambiarse.")

// ExpiradoError reports that the práctica's time ran out and it was
// finalized: the client should go to its result (409 tiempo_agotado with
// resultado_id). It matches ErrTiempoAgotado with errors.Is.
type ExpiradoError struct {
	ResultadoID int64
}

func (e ExpiradoError) Error() string { return ErrTiempoAgotado.Error() }

// Unwrap makes errors.Is(err, ErrTiempoAgotado) true.
func (e ExpiradoError) Unwrap() error { return ErrTiempoAgotado }

// errExpirado is an internal signal: the práctica is en_curso but its time is over.
var errExpirado = errors.New("práctica expirada")

// errEnCursoDuplicado is an internal signal: the unique en_curso index rejected the insert.
var errEnCursoDuplicado = errors.New("práctica en curso duplicada")

// ErrConflict signals that the user already has an en_curso practice.
// PracticaID carries the conflicting practice's ID for the 409 body.
type ErrConflict struct {
	PracticaID int64
}

func (e ErrConflict) Error() string {
	return fmt.Sprintf("practica_en_curso: id=%d", e.PracticaID)
}
