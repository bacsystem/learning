package catalogs

import (
	"errors"
	"fmt"

	"github.com/cbaciliod/learning/internal/db/pgerr"
)

var (
	ErrNotFound   = errors.New("Registro no encontrado.")
	ErrConflict   = errors.New("Ya existe un registro con esos datos.")
	ErrValidation = errors.New("Error de validación.")
	ErrInternal   = errors.New("Error interno del servidor.")
	// ErrInvalidRef: the request references a row that does not exist (FK).
	ErrInvalidRef = errors.New("Referencia inválida.")
)

// isFKViolation / isUniqueViolation classify PostgreSQL errors (shared
// implementation in internal/db/pgerr).
var (
	isFKViolation     = pgerr.IsFKViolation
	isUniqueViolation = pgerr.IsUniqueViolation
)

// invalidRef wraps an FK violation into ErrInvalidRef with a message.
func invalidRef(msg string) error { return fmt.Errorf("%w: %s", ErrInvalidRef, msg) }
