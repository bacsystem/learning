package practices

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cbaciliod/learning/internal/grading"
	"github.com/rs/zerolog/log"
)

// statsTimeout bounds the synchronous statistics update after a finalize.
const statsTimeout = 5 * time.Second

// QuestionsService is the interface the practices service requires from the questions module.
type QuestionsService interface {
	GetProcesoInstitucionID(ctx context.Context, procesoID int64) (int64, error)
	GenerateSet(ctx context.Context, req generateSetRequest) ([]preguntaExamenItem, error)
	RecordIntentos(ctx context.Context, intentos []grading.Intento) error
}

// generateSetRequest mirrors questions.GenerateSetRequest to avoid import cycle.
type generateSetRequest struct {
	ProcesoID      int64
	GradoID        *int64
	EspecialidadID *int64
	Asignaturas    []asignaturaSetItem
}

// asignaturaSetItem mirrors questions.AsignaturaSetItem.
type asignaturaSetItem struct {
	AsignaturaID    int64
	NumeroPreguntas int
}

// preguntaExamenItem mirrors questions.PreguntaExamenItem.
type preguntaExamenItem struct {
	ID           int64
	Enunciado    string
	Tipo         string
	Asignatura   string
	AsignaturaID int64 // asignatura it was selected for (H110)
	Respuestas   []respuestaExamen
	Orden        int
}

// respuestaExamen mirrors questions.RespuestaExamen.
type respuestaExamen struct {
	ID    int64
	Texto string
	Orden int
}

// Service implements the business logic for the practices module.
type Service struct {
	repo *Repository
	qSvc QuestionsService
}

// NewService creates a new practices Service.
func NewService(repo *Repository, qSvc QuestionsService) *Service {
	return &Service{repo: repo, qSvc: qSvc}
}

// ── CreatePractica ────────────────────────────────────────────────────────────

// CreatePractica validates, checks access and conflicts, generates questions,
// and creates a practice. Same access rule as simulacros (H12): the proceso's
// nivel must match the user's and the user needs an approved inscripción.
func (s *Service) CreatePractica(ctx context.Context, req CreatePracticaRequest, usuarioID int64, userNivel string, gradoID, especialidadID int64) (PracticaEnCursoResponse, error) {
	if err := ValidateCreate(req); err != nil {
		return PracticaEnCursoResponse{}, err
	}

	procesoNivel, err := s.repo.GetProcesoNivel(ctx, req.ProcesoID)
	if err != nil {
		return PracticaEnCursoResponse{}, err
	}
	if procesoNivel != "" && procesoNivel != userNivel {
		return PracticaEnCursoResponse{}, ErrNivelNoPermitido
	}
	hasIns, err := s.repo.HasInscripcionAprobada(ctx, usuarioID, req.ProcesoID)
	if err != nil {
		return PracticaEnCursoResponse{}, err
	}
	if !hasIns {
		return PracticaEnCursoResponse{}, ErrInscripcionRequired
	}

	// Fast path; the unique index (049) is the real guard against races.
	existing, expirado, err := s.repo.GetPracticaEnCurso(ctx, usuarioID)
	if err != nil {
		return PracticaEnCursoResponse{}, err
	}
	if existing != nil {
		if !expirado {
			return PracticaEnCursoResponse{}, ErrConflict{PracticaID: *existing}
		}
		if err := s.finalizarExpirada(ctx, *existing, usuarioID); err != nil {
			return PracticaEnCursoResponse{}, err
		}
	}

	institucionID, err := s.qSvc.GetProcesoInstitucionID(ctx, req.ProcesoID)
	if err != nil {
		return PracticaEnCursoResponse{}, err
	}

	asigItems := make([]asignaturaSetItem, len(req.Asignaturas))
	for i, a := range req.Asignaturas {
		asigItems[i] = asignaturaSetItem{AsignaturaID: a.AsignaturaID, NumeroPreguntas: a.NumeroPreguntas}
	}
	genReq := generateSetRequest{ProcesoID: req.ProcesoID, Asignaturas: asigItems}
	if gradoID > 0 {
		genReq.GradoID = &gradoID
	}
	if especialidadID > 0 {
		genReq.EspecialidadID = &especialidadID
	}
	preguntas, err := s.qSvc.GenerateSet(ctx, genReq)
	if err != nil {
		return PracticaEnCursoResponse{}, err
	}

	id, err := s.repo.CreatePractica(ctx, req, usuarioID, institucionID, preguntas)
	if errors.Is(err, errEnCursoDuplicado) {
		other, _, gErr := s.repo.GetPracticaEnCurso(ctx, usuarioID)
		if gErr != nil {
			return PracticaEnCursoResponse{}, gErr
		}
		if other != nil {
			return PracticaEnCursoResponse{}, ErrConflict{PracticaID: *other}
		}
		return PracticaEnCursoResponse{}, ErrConflict{}
	}
	if err != nil {
		return PracticaEnCursoResponse{}, err
	}
	// Re-read so the response has exactly the order later GETs return (H58).
	return s.repo.GetPractica(ctx, id, usuarioID)
}

// ── GetPractica & ToggleRespuesta ─────────────────────────────────────────────

// GetPractica returns the practice for the authenticated user. A timed
// práctica whose limit is over is finalized here (lazy expiry) and reported as
// ExpiradoError so the client can go straight to the result (H82).
func (s *Service) GetPractica(ctx context.Context, id, usuarioID int64) (PracticaEnCursoResponse, error) {
	resp, err := s.repo.GetPractica(ctx, id, usuarioID)
	if errors.Is(err, errExpirado) {
		if err := s.finalizarExpirada(ctx, id, usuarioID); err != nil {
			return PracticaEnCursoResponse{}, err
		}
		return PracticaEnCursoResponse{}, ExpiradoError{ResultadoID: id}
	}
	return resp, err
}

// ToggleRespuesta selects or deselects an answer for a question.
func (s *Service) ToggleRespuesta(ctx context.Context, practicaID, preguntaID, respuestaID, usuarioID int64) ([]int64, error) {
	return s.repo.ToggleRespuesta(ctx, practicaID, preguntaID, respuestaID, usuarioID)
}

// ── FinalizarPractica ─────────────────────────────────────────────────────────

// FinalizarPractica finalizes or abandons a practice session.
func (s *Service) FinalizarPractica(ctx context.Context, id, usuarioID int64, req FinalizarPracticaRequest) (PracticaResultadoResponse, error) {
	if err := s.finalizar(ctx, id, usuarioID, req.Abandonar); err != nil {
		return PracticaResultadoResponse{}, err
	}
	if req.Abandonar {
		meta, err := getMeta(ctx, s.repo.db, id, usuarioID, false)
		if err != nil {
			return PracticaResultadoResponse{}, err
		}
		return PracticaResultadoResponse{ID: id, Estado: "abandonada", Proceso: meta.ProcesoNombre}, nil
	}
	return s.repo.GetResultado(ctx, id, usuarioID)
}

// finalizar closes the práctica and records statistics once (the repo
// guarantees a single successful finalize).
func (s *Service) finalizar(ctx context.Context, id, usuarioID int64, abandonar bool) error {
	intentos, err := s.repo.FinalizarPractica(ctx, id, usuarioID, abandonar)
	if err != nil {
		return err
	}
	if !abandonar {
		s.recordIntentos(ctx, id, intentos)
	}
	return nil
}

// finalizarExpirada grades a práctica whose time is over; losing the race
// against another finalize is fine.
func (s *Service) finalizarExpirada(ctx context.Context, id, usuarioID int64) error {
	if err := s.finalizar(ctx, id, usuarioID, false); err != nil && !errors.Is(err, ErrPracticaEstado) {
		return err
	}
	return nil
}

// recordIntentos updates question statistics in one batch (one attempt per
// answered question). Failures are logged and do not fail the finalize.
func (s *Service) recordIntentos(ctx context.Context, practicaID int64, intentos []grading.Intento) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), statsTimeout)
	defer cancel()
	if err := s.qSvc.RecordIntentos(ctx, intentos); err != nil {
		log.Error().Err(err).Int64("practica_id", practicaID).Msg("practices: failed to record estadisticas")
	}
}

// ── ValidarPregunta ───────────────────────────────────────────────────────────

// ValidarPregunta validates the selected answers for a question in inmediato mode.
func (s *Service) ValidarPregunta(ctx context.Context, practicaID, preguntaID, usuarioID int64) (ValidarPreguntaResponse, error) {
	return s.repo.ValidarPregunta(ctx, practicaID, preguntaID, usuarioID)
}

// ── Results & Revision ────────────────────────────────────────────────────────

// GetResultado returns the results for a completed practice.
func (s *Service) GetResultado(ctx context.Context, id, usuarioID int64) (PracticaResultadoResponse, error) {
	return s.repo.GetResultado(ctx, id, usuarioID)
}

// GetRevision returns the full revision for a completed practice.
func (s *Service) GetRevision(ctx context.Context, id, usuarioID int64) (RevisionPracticaResponse, error) {
	return s.repo.GetRevision(ctx, id, usuarioID)
}

// ── History & Deletion ────────────────────────────────────────────────────────

// ListHistorial returns paginated history of completed/abandoned practices.
func (s *Service) ListHistorial(ctx context.Context, usuarioID int64, page, perPage int, estado string) (HistorialResponse, error) {
	return s.repo.ListHistorial(ctx, usuarioID, page, perPage, estado)
}

// DeletePractica soft-deletes a single practice.
func (s *Service) DeletePractica(ctx context.Context, id, usuarioID int64) error {
	return s.repo.DeletePractica(ctx, id, usuarioID)
}

// BatchDeletePracticas soft-deletes multiple practices. Duplicated ids are
// collapsed and at most MaxBatchDelete distinct ids are accepted (H59).
func (s *Service) BatchDeletePracticas(ctx context.Context, ids []int64, usuarioID int64) error {
	seen := map[int64]bool{}
	uniq := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return fmt.Errorf("%w: id inválido", ErrValidation)
		}
		if !seen[id] {
			seen[id] = true
			uniq = append(uniq, id)
		}
	}
	if len(uniq) > MaxBatchDelete {
		return fmt.Errorf("%w: como máximo %d prácticas por solicitud", ErrLimiteExcedido, MaxBatchDelete)
	}
	return s.repo.BatchDeletePracticas(ctx, uniq, usuarioID)
}
