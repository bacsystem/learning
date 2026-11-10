package catalogs

import (
	"context"
	"fmt"
)

// ValidateSO3Grado checks that the grado for a pase_efectividad proceso has
// abreviatura = "SO 3°". It is a no-op for other tipos.
func ValidateSO3Grado(ctx context.Context, repo *Repository, tipo string, gradoID *int64) error {
	if tipo != "pase_efectividad" {
		return nil
	}
	if gradoID == nil {
		return fmt.Errorf("%w: el tipo pase_efectividad requiere un grado_id", ErrValidation)
	}
	abrev, err := repo.FindGradoAbreviatura(ctx, *gradoID)
	if err != nil {
		return err // ErrNotFound only for a missing grado; DB errors propagate
	}
	if abrev != "SO 3°" {
		return fmt.Errorf("%w: el grado para pase_efectividad debe ser \"SO 3°\" (abreviatura), pero se recibió %q", ErrValidation, abrev)
	}
	return nil
}
