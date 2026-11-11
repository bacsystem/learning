package simulations

import (
	"context"
	"errors"
	"time"

	"github.com/cbaciliod/learning/internal/grading"
	"github.com/rs/zerolog/log"
)

// statsTimeout bounds the synchronous statistics update after a finalize.
const statsTimeout = 5 * time.Second

// QuestionsService is the interface the simulations service requires from the questions module.
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

// Service implements the business logic for the simulations module.
type Service struct {
	repo *Repository
	qSvc QuestionsService
}

// NewService creates a new simulations Service.
func NewService(repo *Repository, qSvc QuestionsService) *Service {
	return &Service{repo: repo, qSvc: qSvc}
}

// ── CreateSimulacro ───────────────────────────────────────────────────────────

// CreateSimulacro validates, checks for conflicts, loads config, generates questions, and creates a simulacro.
func (s *Service) CreateSimulacro(ctx context.Context, req CreateSimulacroRequest, usuarioID int64, userNivel string, gradoID, especialidadID int64) (SimulacroEnCursoResponse, error) {
	if err := ValidateCreate(req); err != nil {
		return SimulacroEnCursoResponse{}, err
	}

	// Validate user's nivel matches the proceso's nivel
	procesoNivel, err := s.repo.GetProcesoNivel(ctx, req.ProcesoID)
	if err != nil {
		return SimulacroEnCursoResponse{}, err
	}
	if procesoNivel != "" && procesoNivel != userNivel {
		return SimulacroEnCursoResponse{}, ErrNivelNoPermitido
	}

	// Validate user has an approved inscripcion for this proceso
	hasIns, err := s.repo.HasInscripcionAprobada(ctx, usuarioID, req.ProcesoID)
	if err != nil {
		return SimulacroEnCursoResponse{}, err
	}
	if !hasIns {
		return SimulacroEnCursoResponse{}, ErrInscripcionRequired
	}

	// Fast path for the common conflict; the unique index (049) is the real guard.
	existing, err := s.GetEnCurso(ctx, usuarioID)
	if err != nil {
		return SimulacroEnCursoResponse{}, err
	}
	if existing != nil {
		return SimulacroEnCursoResponse{}, ErrConflict{SimulacroID: *existing}
	}

	cfg, err := s.repo.GetConfiguracion(ctx, req.ProcesoID)
	if err != nil {
		return SimulacroEnCursoResponse{}, err
	}
	if cfg == nil {
		return SimulacroEnCursoResponse{}, ErrNoConfig
	}

	all, err := s.repo.GetProcesoAsignaturas(ctx, req.ProcesoID, especialidadID)
	if err != nil {
		return SimulacroEnCursoResponse{}, err
	}
	var asignaturas []simulacroAsignaturaRow
	asigItems := []asignaturaSetItem{}
	for _, a := range all {
		if a.NumeroPreguntas > 0 {
			asignaturas = append(asignaturas, a)
			asigItems = append(asigItems, asignaturaSetItem{AsignaturaID: a.AsignaturaID, NumeroPreguntas: a.NumeroPreguntas})
		}
	}
	if len(asignaturas) == 0 {
		return SimulacroEnCursoResponse{}, ErrNoConfig
	}

	institucionID, err := s.qSvc.GetProcesoInstitucionID(ctx, req.ProcesoID)
	if err != nil {
		return SimulacroEnCursoResponse{}, err
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
		return SimulacroEnCursoResponse{}, err
	}

	id, err := s.repo.CreateSimulacro(ctx, req.ProcesoID, usuarioID, institucionID, *cfg, asignaturas, preguntas)
	if errors.Is(err, errEnCursoDuplicado) {
		other, _, gErr := s.repo.GetSimulacroEnCurso(ctx, usuarioID)
		if gErr != nil {
			return SimulacroEnCursoResponse{}, gErr
		}
		if other != nil {
			return SimulacroEnCursoResponse{}, ErrConflict{SimulacroID: *other}
		}
		return SimulacroEnCursoResponse{}, ErrConflict{}
	}
	if err != nil {
		return SimulacroEnCursoResponse{}, err
	}
	// Re-read so the response has exactly the order later GETs will return.
	return s.repo.GetSimulacro(ctx, id, usuarioID)
}

// ── GetSimulacro ──────────────────────────────────────────────────────────────

// GetSimulacro returns the active simulacro for the authenticated user. A
// simulacro whose time is over is finalized here (lazy expiry) and reported as
// ExpiradoError so the client can go straight to the result (H82).
func (s *Service) GetSimulacro(ctx context.Context, id, usuarioID int64) (SimulacroEnCursoResponse, error) {
	resp, err := s.repo.GetSimulacro(ctx, id, usuarioID)
	if errors.Is(err, errExpirado) {
		if err := s.finalizarExpirado(ctx, id, usuarioID); err != nil {
			return SimulacroEnCursoResponse{}, err
		}
		return SimulacroEnCursoResponse{}, ExpiradoError{ResultadoID: id}
	}
	return resp, err
}

// ── Respuestas ────────────────────────────────────────────────────────────────

// ResponderPregunta saves one answer ('unica': replaces; 'multiple': toggles)
// and returns the question's current selection. Does NOT return correctness.
func (s *Service) ResponderPregunta(ctx context.Context, simulacroID int64, req ResponderSimulacroRequest, usuarioID int64) ([]int64, error) {
	return s.repo.ResponderPregunta(ctx, simulacroID, req.PreguntaID, req.RespuestaID, usuarioID)
}

// SetRespuestas replaces the whole selection of a question.
func (s *Service) SetRespuestas(ctx context.Context, simulacroID, preguntaID int64, ids []int64, usuarioID int64) ([]int64, error) {
	return s.repo.SetRespuestas(ctx, simulacroID, preguntaID, ids, usuarioID)
}

// ── FinalizarSimulacro ────────────────────────────────────────────────────────

// FinalizarSimulacro finalizes a simulacro and returns the result.
func (s *Service) FinalizarSimulacro(ctx context.Context, id, usuarioID int64) (SimulacroResultadoResponse, error) {
	if err := s.finalizar(ctx, id, usuarioID); err != nil {
		return SimulacroResultadoResponse{}, err
	}
	return s.repo.GetResultado(ctx, id, usuarioID)
}

// finalizar grades the simulacro and records question statistics. The repo
// guarantees only one finalize can succeed, so statistics are recorded once.
func (s *Service) finalizar(ctx context.Context, id, usuarioID int64) error {
	intentos, err := s.repo.FinalizarSimulacro(ctx, id, usuarioID)
	if err != nil {
		return err
	}
	s.recordIntentos(ctx, id, intentos)
	return nil
}

// finalizarExpirado finalizes a simulacro whose time is over. Losing the race
// against another finalize is fine.
func (s *Service) finalizarExpirado(ctx context.Context, id, usuarioID int64) error {
	if err := s.finalizar(ctx, id, usuarioID); err != nil && !errors.Is(err, ErrSimulacroEstado) {
		return err
	}
	return nil
}

// recordIntentos updates question statistics in one batch (one attempt per
// answered question). A failure is logged and does not fail the finalize.
func (s *Service) recordIntentos(ctx context.Context, simulacroID int64, intentos []grading.Intento) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), statsTimeout)
	defer cancel()
	if err := s.qSvc.RecordIntentos(ctx, intentos); err != nil {
		log.Error().Err(err).Int64("simulacro_id", simulacroID).Msg("simulations: failed to record estadisticas")
	}
}

// ── GetResultado ──────────────────────────────────────────────────────────────

// GetResultado returns the result for a completed simulacro.
func (s *Service) GetResultado(ctx context.Context, id, usuarioID int64) (SimulacroResultadoResponse, error) {
	return s.repo.GetResultado(ctx, id, usuarioID)
}

// ── GetRevision ───────────────────────────────────────────────────────────────

// GetRevision returns the full revision for a completed simulacro.
func (s *Service) GetRevision(ctx context.Context, id, usuarioID int64) (RevisionSimulacroResponse, error) {
	return s.repo.GetRevision(ctx, id, usuarioID)
}

// ── GetHistorial ──────────────────────────────────────────────────────────────

// GetHistorial returns paginated history of completed simulacros.
func (s *Service) GetHistorial(ctx context.Context, usuarioID int64, page, perPage int, filtros HistorialFiltros) (HistorialSimulacroResponse, error) {
	return s.repo.GetHistorial(ctx, usuarioID, page, perPage, filtros)
}

// GetEnCurso returns the active simulacro ID if any. An en_curso simulacro
// whose time is over is finalized first (lazy expiry) so it never stays
// en_curso forever nor blocks the user from starting a new one.
func (s *Service) GetEnCurso(ctx context.Context, usuarioID int64) (*int64, error) {
	id, expirado, err := s.repo.GetSimulacroEnCurso(ctx, usuarioID)
	if err != nil || id == nil {
		return nil, err
	}
	if !expirado {
		return id, nil
	}
	if err := s.finalizarExpirado(ctx, *id, usuarioID); err != nil {
		return nil, err
	}
	return nil, nil
}

// ── Configuracion Examen ──────────────────────────────────────────────────────

// CreateConfiguracion creates a new exam configuration for a proceso.
// Absent nota_minima / nota_maxima take the defaults (55 / 100).
func (s *Service) CreateConfiguracion(ctx context.Context, req CreateConfiguracionRequest) (ConfiguracionResponse, error) {
	notaMinima, notaMaxima := DefaultNotaMinima, DefaultNotaMaxima
	if req.NotaMinima != nil {
		notaMinima = *req.NotaMinima
	}
	if req.NotaMaxima != nil {
		notaMaxima = *req.NotaMaxima
	}
	if err := validateConfiguracion(req.TiempoTotalSeg, req.TotalPreguntas, notaMinima, notaMaxima, "activo"); err != nil {
		return ConfiguracionResponse{}, err
	}
	return s.repo.CreateConfiguracion(ctx, req, notaMinima, notaMaxima)
}

// GetConfiguracionByProceso returns the active config for a proceso with asignaturas filtered by grado/especialidad.
func (s *Service) GetConfiguracionByProceso(ctx context.Context, procesoID, gradoID, especialidadID int64) (*ConfiguracionResponse, error) {
	return s.repo.GetConfiguracionDetalle(ctx, procesoID, gradoID, especialidadID)
}

// ListConfiguraciones returns all active configurations.
func (s *Service) ListConfiguraciones(ctx context.Context) (ConfiguracionListResponse, error) {
	data, err := s.repo.ListConfiguraciones(ctx)
	if err != nil {
		return ConfiguracionListResponse{}, err
	}
	return ConfiguracionListResponse{Data: data}, nil
}

// UpdateConfiguracion updates an existing configuracion_examen by id. Absent
// fields keep their current value; the merged result is validated.
func (s *Service) UpdateConfiguracion(ctx context.Context, id int64, req UpdateConfiguracionRequest) (ConfiguracionResponse, error) {
	cur, err := s.repo.GetConfiguracionByID(ctx, id)
	if err != nil {
		return ConfiguracionResponse{}, err
	}
	if req.TiempoTotalSeg != nil {
		cur.TiempoTotalSeg = *req.TiempoTotalSeg
	}
	if req.TotalPreguntas != nil {
		cur.TotalPreguntas = *req.TotalPreguntas
	}
	if req.NotaMinima != nil {
		cur.NotaMinima = *req.NotaMinima
	}
	if req.NotaMaxima != nil {
		cur.NotaMaxima = *req.NotaMaxima
	}
	if req.Descripcion != nil {
		cur.Descripcion = *req.Descripcion
	}
	if req.Estado != nil {
		cur.Estado = *req.Estado
	}
	if err := validateConfiguracion(cur.TiempoTotalSeg, cur.TotalPreguntas, cur.NotaMinima, cur.NotaMaxima, cur.Estado); err != nil {
		return ConfiguracionResponse{}, err
	}
	return s.repo.UpdateConfiguracion(ctx, cur)
}
