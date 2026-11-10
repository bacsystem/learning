package simulations

import (
	"fmt"
	"time"
)

const (
	// DefaultNotaMinima is the passing grade when the admin does not set one (0-100 scale).
	DefaultNotaMinima = 55.0
	// DefaultNotaMaxima is the top of the grading scale when not set.
	DefaultNotaMaxima = 100.0
	// EscalaMaxima is the largest nota_maxima accepted (the scale is 0-100 since migration 045).
	EscalaMaxima = 100.0
	// MinTiempoTotalSeg is the minimum duration of a simulacro.
	MinTiempoTotalSeg = 60

	// GraciaRespuesta tolerates answers in flight right after the time limit
	// (network latency, the client timer firing at 0). Answers later than
	// fecha_inicio + tiempo_limite + GraciaRespuesta are rejected / discarded.
	GraciaRespuesta = 5 * time.Second
)

// ValidateCreate validates a CreateSimulacroRequest.
func ValidateCreate(req CreateSimulacroRequest) error {
	if req.ProcesoID <= 0 {
		return fmt.Errorf("%w: proceso_id es requerido", ErrValidation)
	}
	return nil
}

// validateConfiguracion checks the ranges of an exam configuration.
func validateConfiguracion(tiempo, total int, notaMinima, notaMaxima float64, estado string) error {
	switch {
	case tiempo < MinTiempoTotalSeg:
		return fmt.Errorf("%w: tiempo_total_seg debe ser al menos %d", ErrValidation, MinTiempoTotalSeg)
	case total < 1:
		return fmt.Errorf("%w: total_preguntas debe ser al menos 1", ErrValidation)
	case notaMaxima <= 0 || notaMaxima > EscalaMaxima:
		return fmt.Errorf("%w: nota_maxima debe estar entre 0 y %.0f", ErrValidation, EscalaMaxima)
	case notaMinima < 0 || notaMinima > notaMaxima:
		return fmt.Errorf("%w: nota_minima debe estar entre 0 y nota_maxima", ErrValidation)
	case estado != "activo" && estado != "inactivo":
		return fmt.Errorf("%w: estado debe ser 'activo' o 'inactivo'", ErrValidation)
	}
	return nil
}
