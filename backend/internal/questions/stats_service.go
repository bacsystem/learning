package questions

import (
	"context"

	"github.com/cbaciliod/learning/internal/grading"
)

// RecordIntentos records the outcome of a finished attempt: one entry per
// answered question, applied in a single batch UPSERT. Callers invoke it once
// per finalized attempt (the finalize is guarded so it can only succeed once).
func (s *Service) RecordIntentos(ctx context.Context, intentos []grading.Intento) error {
	if len(intentos) == 0 {
		return nil
	}
	return s.repo.UpsertEstadisticas(ctx, intentos)
}
