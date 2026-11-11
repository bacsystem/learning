package catalogs

import (
	"context"
	"fmt"
)

// ResumenPesos returns the weights summary of a proceso (PE1).
func (s *Service) ResumenPesos(ctx context.Context, procesoID int64) (ResumenPesos, error) {
	return s.repo.ResumenPesos(ctx, procesoID)
}

// ConfiguracionPesos returns the weights of (proceso, especialidad) (PE2).
func (s *Service) ConfiguracionPesos(ctx context.Context, procesoID, especialidadID int64) (ConfiguracionPesos, error) {
	return s.repo.ConfiguracionPesos(ctx, procesoID, especialidadID)
}

// GuardarPesos validates and replaces the whole configuration of (proceso,
// especialidad) (PE3). The shape is checked here; the repository then checks
// the references and, still under the lock, runs rules 3–5, so the first
// error follows the contract's order.
func (s *Service) GuardarPesos(ctx context.Context, procesoID, especialidadID int64, req GuardarPesosRequest, aud AuditorPesos) (ConfiguracionPesos, error) {
	if err := validarAreasPesos(req.Areas); err != nil {
		return ConfiguracionPesos{}, err
	}
	reglas := func() error { return reglasDeConjunto(req.Areas) }
	if err := s.repo.GuardarPesos(ctx, procesoID, especialidadID, req.Areas, reglas, aud); err != nil {
		return ConfiguracionPesos{}, err
	}
	return s.repo.ConfiguracionPesos(ctx, procesoID, especialidadID)
}

// Pesos is the single entry point for the consumers of weights (contract PE6;
// E1-03 reglas 8–9). With a saved set of (proceso, especialidad) it returns
// its rows. Without one, ModoOficial answers ErrFormatoNoConfigurado and
// ModoEstimable splits 100 % equally among the areas with servable questions
// (possibly none), flagged as estimated.
func (s *Service) Pesos(ctx context.Context, procesoID, especialidadID int64, modo ModoPesos) (Pesos, error) {
	conjunto, err := s.repo.ConjuntoDeEspecialidad(ctx, procesoID, especialidadID)
	if err != nil {
		return Pesos{}, err
	}
	if len(conjunto) > 0 {
		return Pesos{Areas: pesosDeConjunto(conjunto)}, nil
	}
	if modo == ModoOficial {
		return Pesos{}, ErrFormatoNoConfigurado
	}
	ids, err := s.repo.AreasServibles(ctx, procesoID, especialidadID)
	if err != nil {
		return Pesos{}, fmt.Errorf("pesos estimados: %w", err)
	}
	return Pesos{Areas: pesosIguales(ids), PesosEstimados: true}, nil
}

func pesosDeConjunto(filas []AreaPeso) []PesoArea {
	areas := make([]PesoArea, len(filas))
	for i, f := range filas {
		n := f.NumeroPreguntas
		areas[i] = PesoArea{AsignaturaID: f.AsignaturaID, PesoPorcentual: f.PesoPorcentual,
			NumeroPreguntas: &n, Coeficiente: f.Coeficiente}
	}
	return areas
}
