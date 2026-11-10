package practices

import (
	"fmt"
	"time"
)

// MaxPreguntasPractica caps the total number of questions of one práctica.
const MaxPreguntasPractica = 200

// MaxBatchDelete caps the number of ids accepted by DELETE /practicas/batch.
const MaxBatchDelete = 100

// GraciaRespuesta tolerates answers in flight right after the time limit of a
// timed práctica; later answers are rejected / discarded.
const GraciaRespuesta = 5 * time.Second

// ValidateCreate validates a CreatePracticaRequest.
func ValidateCreate(req CreatePracticaRequest) error {
	if req.ProcesoID <= 0 {
		return fmt.Errorf("%w: proceso_id es requerido", ErrValidation)
	}
	if len(req.Asignaturas) == 0 {
		return fmt.Errorf("%w: debe especificar al menos una asignatura", ErrValidation)
	}
	seen := map[int64]bool{}
	total := 0
	for _, a := range req.Asignaturas {
		if a.AsignaturaID <= 0 {
			return fmt.Errorf("%w: asignatura_id es requerido", ErrValidation)
		}
		if a.NumeroPreguntas < 1 {
			return fmt.Errorf("%w: numero_preguntas debe ser al menos 1", ErrValidation)
		}
		// A repeated asignatura would draw the same questions twice (H37).
		if seen[a.AsignaturaID] {
			return fmt.Errorf("%w: asignatura_id %d repetida", ErrValidation, a.AsignaturaID)
		}
		seen[a.AsignaturaID] = true
		total += a.NumeroPreguntas
	}
	if total > MaxPreguntasPractica {
		return fmt.Errorf("%w: una práctica admite como máximo %d preguntas", ErrLimiteExcedido, MaxPreguntasPractica)
	}
	switch req.ModoCalificacion {
	case "inmediato", "al_final":
	default:
		return fmt.Errorf("%w: modo_calificacion debe ser 'inmediato' o 'al_final'", ErrValidation)
	}
	if req.TiempoLimiteSeg != nil && *req.TiempoLimiteSeg < 60 {
		return fmt.Errorf("%w: tiempo_limite_seg debe ser al menos 60 segundos", ErrValidation)
	}
	return nil
}
