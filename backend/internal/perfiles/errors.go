package perfiles

import "errors"

// Domain errors; the handler maps them to the codes of contract §3 (F1–F3).
var (
	ErrValidacion            = errors.New("Datos inválidos.")
	ErrInscripcionRequerida  = errors.New("Necesitas una inscripción aprobada en el proceso.")
	ErrEspecialidadInvalida  = errors.New("La profesión no existe, está inactiva o no corresponde al proceso.")
	ErrProfesionNoDefinida   = errors.New("Aún no elegiste tu profesión.")
	ErrConfirmacionRequerida = errors.New("Cambiar de profesión reinicia tu dominio; confírmalo para continuar.")
	ErrEstadoInvalido        = errors.New("Tu profesión cambió mientras guardabas; vuelve a intentarlo.")
)

// ValidacionError lists the broken fields (field → rule); it matches ErrValidacion.
type ValidacionError struct {
	Campos map[string]string
}

func (e *ValidacionError) Error() string { return ErrValidacion.Error() }
func (e *ValidacionError) Unwrap() error { return ErrValidacion }

// ConfirmacionError is ErrConfirmacionRequerida with the current pair, which
// the UI shows in its confirmation dialog.
type ConfirmacionError struct {
	Actual Par
}

func (e *ConfirmacionError) Error() string { return ErrConfirmacionRequerida.Error() }
func (e *ConfirmacionError) Unwrap() error { return ErrConfirmacionRequerida }
