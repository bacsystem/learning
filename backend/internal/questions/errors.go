package questions

import (
	"errors"

	"github.com/lib/pq"
)

var (
	// ErrNotFound is returned when a requested record does not exist.
	ErrNotFound = errors.New("Registro no encontrado.")
	// ErrConflict is returned when a unique constraint is violated.
	ErrConflict = errors.New("Ya existe un registro con esos datos.")
	// ErrValidation is returned when business-rule validation fails.
	ErrValidation = errors.New("Error de validación.")
	// ErrInsufficientQuestions is returned when generate-set can't satisfy the request.
	ErrInsufficientQuestions = errors.New("Preguntas insuficientes para generar el conjunto.")
	// ErrLimiteExcedido is returned when a request asks for more items than allowed.
	ErrLimiteExcedido = errors.New("Se excedió el máximo permitido.")
)

const msgAlternativaConIntentos = "no se puede eliminar una alternativa que ya tiene intentos registrados"

// msgClaveConIntentos is the 409 message when an edit changes the key of a
// question already used to grade attempts (H133).
const msgClaveConIntentos = "no se puede cambiar la respuesta correcta de una pregunta que ya calificó intentos; crea una pregunta nueva"

// isAlternativaConIntentos returns true when err is the RESTRICT foreign key
// violation raised by deleting an alternative referenced by an attempt (048).
func isAlternativaConIntentos(err error) bool {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) || pqErr.Code != "23503" {
		return false
	}
	switch pqErr.Constraint {
	case "practica_respuestas_pregunta_respuesta_fkey", "simulacro_sel_pregunta_respuesta_fkey":
		return true
	}
	return false
}

// isUniqueViolation returns true when err is a PostgreSQL unique constraint violation (code 23505).
func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}

// isForeignKeyViolation returns true for a 23503 that is not the alternative
// RESTRICT above: a grado_id or especialidad_id that does not exist. Etiquetas
// are checked before the insert (validarClasificacion), so they never get here.
func isForeignKeyViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23503" && !isAlternativaConIntentos(err)
}
