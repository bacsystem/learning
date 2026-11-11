package questions

import (
	"context"
	"errors"
	"fmt"
)

// Service implements the business logic for the questions module.
type Service struct {
	repo *Repository
}

// NewService creates a new questions Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ── Preguntas ─────────────────────────────────────────────────────────────────

// CreatePregunta validates and creates a new question.
func (s *Service) CreatePregunta(ctx context.Context, req CreatePreguntaRequest) (PreguntaResponse, error) {
	if err := ValidateRespuestas(req.Tipo, req.Respuestas); err != nil {
		return PreguntaResponse{}, err
	}

	instID, err := s.repo.GetProcesoInstitucionID(ctx, req.ProcesoID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return PreguntaResponse{}, fmt.Errorf("%w: proceso_id no encontrado", ErrValidation)
		}
		return PreguntaResponse{}, err
	}

	result, err := s.repo.CreatePregunta(ctx, req, instID)
	if err != nil {
		if isTemaFKViolation(err) {
			return PreguntaResponse{}, ErrTemaInvalido
		}
		if isForeignKeyViolation(err) {
			return PreguntaResponse{}, fmt.Errorf("%w: grado o especialidad no existe", ErrValidation)
		}
		if isUniqueViolation(err) {
			return PreguntaResponse{}, ErrConflict
		}
		return PreguntaResponse{}, err
	}
	return result, nil
}

// ValidarAsignatura checks that the asignatura is assigned to the proceso (H19).
func (s *Service) ValidarAsignatura(ctx context.Context, procesoID, asignaturaID int64) error {
	return validarClasificacion(ctx, s.repo.db, procesoID, asignaturaID, nil)
}

// GetPregunta returns a question by ID.
func (s *Service) GetPregunta(ctx context.Context, id int64) (PreguntaResponse, error) {
	return s.repo.GetPregunta(ctx, id)
}

// ListPreguntas returns a paginated, filtered list of questions.
func (s *Service) ListPreguntas(ctx context.Context, q ListQuery) (PreguntaListResponse, error) {
	return s.repo.ListPreguntas(ctx, q)
}

// UpdatePregunta validates and updates a question.
func (s *Service) UpdatePregunta(ctx context.Context, id int64, req UpdatePreguntaRequest) (PreguntaResponse, error) {
	if err := ValidateRespuestas(req.Tipo, req.Respuestas); err != nil {
		return PreguntaResponse{}, err
	}

	result, err := s.repo.UpdatePregunta(ctx, id, req)
	if err != nil {
		if isAlternativaConIntentos(err) {
			return PreguntaResponse{}, fmt.Errorf("%w: %s", ErrConflict, msgAlternativaConIntentos)
		}
		if isTemaFKViolation(err) {
			return PreguntaResponse{}, ErrTemaInvalido
		}
		if isForeignKeyViolation(err) {
			return PreguntaResponse{}, fmt.Errorf("%w: grado o especialidad no existe", ErrValidation)
		}
		if isUniqueViolation(err) {
			return PreguntaResponse{}, ErrConflict
		}
		return PreguntaResponse{}, err
	}
	return result, nil
}

// CambiarEstado moves a question through its life cycle (E3-07 §3). The body
// rules that need no database run first; the state rules run under the row
// lock, against the state read there.
func (s *Service) CambiarEstado(ctx context.Context, id int64, req CambiarEstadoRequest, aud Auditor) (PreguntaResponse, error) {
	if err := validarCuerpoEstado(req); err != nil {
		return PreguntaResponse{}, err
	}
	decidir := func(b preguntaBloqueada) (cambioEstado, error) { return decidirCambioEstado(b, req) }
	if err := s.repo.CambiarEstado(ctx, id, decidir, aud); err != nil {
		return PreguntaResponse{}, err
	}
	return s.repo.GetPregunta(ctx, id)
}

// ReemplazarCitas replaces the citations of a question (E3-07 §4). It never
// changes the state.
func (s *Service) ReemplazarCitas(ctx context.Context, id int64, req ReemplazarCitasRequest, aud Auditor) (PreguntaResponse, error) {
	pedidos, err := validarPasajeIDs(req)
	if err != nil {
		return PreguntaResponse{}, err
	}
	decidir := func(b preguntaBloqueada, info map[int64]pasajeInfo) (planCitas, error) {
		return decidirCitas(b, pedidos, info)
	}
	if err := s.repo.ReemplazarCitas(ctx, id, pedidos, decidir, aud); err != nil {
		return PreguntaResponse{}, err
	}
	return s.repo.GetPregunta(ctx, id)
}

// DeletePregunta soft-deletes a question.
func (s *Service) DeletePregunta(ctx context.Context, id int64) error {
	return s.repo.DeletePregunta(ctx, id)
}

// ── Etiquetas ─────────────────────────────────────────────────────────────────

// ListEtiquetas returns all tags, optionally filtered.
func (s *Service) ListEtiquetas(ctx context.Context, asignaturaID int64, estado string) ([]EtiquetaResponse, error) {
	return s.repo.ListEtiquetas(ctx, asignaturaID, estado)
}

// CreateEtiqueta creates a new tag.
func (s *Service) CreateEtiqueta(ctx context.Context, req CreateEtiquetaRequest) (EtiquetaResponse, error) {
	result, err := s.repo.CreateEtiqueta(ctx, req)
	if err != nil {
		if isUniqueViolation(err) {
			return EtiquetaResponse{}, fmt.Errorf("%w: ya existe una etiqueta con ese nombre para esta asignatura", ErrConflict)
		}
		return EtiquetaResponse{}, err
	}
	return result, nil
}

// UpdateEtiqueta updates a tag.
func (s *Service) UpdateEtiqueta(ctx context.Context, id int64, req UpdateEtiquetaRequest) (EtiquetaResponse, error) {
	result, err := s.repo.UpdateEtiqueta(ctx, id, req)
	if err != nil {
		if isUniqueViolation(err) {
			return EtiquetaResponse{}, fmt.Errorf("%w: ya existe una etiqueta con ese nombre para esta asignatura", ErrConflict)
		}
		return EtiquetaResponse{}, err
	}
	return result, nil
}

// DeleteEtiqueta deletes a tag.
func (s *Service) DeleteEtiqueta(ctx context.Context, id int64) error {
	return s.repo.DeleteEtiqueta(ctx, id)
}

// ── Generate Set ──────────────────────────────────────────────────────────────

// GetProcesoInstitucionID returns the institucion_id for a given proceso.
func (s *Service) GetProcesoInstitucionID(ctx context.Context, procesoID int64) (int64, error) {
	return s.repo.GetProcesoInstitucionID(ctx, procesoID)
}

// GenerateSet validates and generates a random question set for an exam.
func (s *Service) GenerateSet(ctx context.Context, req GenerateSetRequest) (GenerateSetResponse, error) {
	if req.ProcesoID == 0 {
		return GenerateSetResponse{}, fmt.Errorf("%w: proceso_id es requerido", ErrValidation)
	}
	if len(req.Asignaturas) == 0 {
		return GenerateSetResponse{}, fmt.Errorf("%w: debe especificar al menos una asignatura", ErrValidation)
	}
	seen := map[int64]bool{}
	for _, a := range req.Asignaturas {
		if a.NumeroPreguntas < 1 {
			return GenerateSetResponse{}, fmt.Errorf("%w: numero_preguntas debe ser al menos 1", ErrValidation)
		}
		if seen[a.AsignaturaID] {
			return GenerateSetResponse{}, fmt.Errorf("%w: asignatura_id %d repetida", ErrValidation, a.AsignaturaID)
		}
		seen[a.AsignaturaID] = true
	}

	items, err := s.repo.GenerateSet(ctx, req)
	if err != nil {
		return GenerateSetResponse{}, err
	}

	return GenerateSetResponse{
		Preguntas: items,
		Total:     len(items),
	}, nil
}

// ── Statistics ────────────────────────────────────────────────────────────────

// GetStats returns global question bank statistics.
func (s *Service) GetStats(ctx context.Context) (StatsGlobalesResponse, error) {
	return s.repo.GetGlobalStats(ctx)
}
