package inscripciones

import (
	"context"
	"errors"
	"time"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/examen"
)

var (
	ErrProcesoNotFound     = errors.New("El proceso no existe o no está activo.")
	ErrYaInscrito          = errors.New("Ya estás inscrito en este proceso.")
	ErrInscripcionNotFound = errors.New("Inscripción no encontrada.")
	ErrEstadoInvalido      = errors.New("Estado inválido. Valores permitidos: aprobado, rechazado.")
	// Access errors of the proceso (403), decided by examen.AccesoProceso.
	ErrNivelNoPermitido       = examen.ErrNivelNoPermitido
	ErrInstitucionNoPermitida = examen.ErrInstitucionNoPermitida
	// ErrTransicionInvalida: only pendiente → aprobado|rechazado is allowed;
	// a decided inscription is final.
	ErrTransicionInvalida = errors.New("La inscripción ya fue revisada; solo se pueden aprobar o rechazar inscripciones pendientes.")
	// ErrSinConfigPago: the proceso's year has no configuracion_pago row.
	ErrSinConfigPago = errors.New("El proceso no tiene un monto de inscripción configurado para su año. Contacta al administrador.")
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ── Stats públicos ────────────────────────────────────────────────────────────

type StatsPublicosDTO struct {
	TotalUsuarios   int64 `json:"total_usuarios"`
	TotalSimulacros int64 `json:"total_simulacros"`
	TotalPracticas  int64 `json:"total_practicas"`
	TotalProcesos   int64 `json:"total_procesos"`
}

func (s *Service) GetStatsPublicos(ctx context.Context) (StatsPublicosDTO, error) {
	row, err := s.repo.GetStatsPublicos(ctx)
	if err != nil {
		return StatsPublicosDTO{}, err
	}
	return StatsPublicosDTO{
		TotalUsuarios:   row.TotalUsuarios,
		TotalSimulacros: row.TotalSimulacros,
		TotalPracticas:  row.TotalPracticas,
		TotalProcesos:   row.TotalProcesos,
	}, nil
}

// ── Procesos públicos ─────────────────────────────────────────────────────────

type ProcesoDTO struct {
	ID          int64   `json:"id"`
	Nombre      string  `json:"nombre"`
	Tipo        string  `json:"tipo"`
	GradoNombre string  `json:"grado"`
	Nivel       string  `json:"nivel"`
	Descripcion *string `json:"descripcion,omitempty"`
	Anio        int     `json:"anio"`
}

func (s *Service) ListTodosProcesosActivos(ctx context.Context) ([]ProcesoDTO, error) {
	rows, err := s.repo.ListProcesosActivos(ctx)
	if err != nil {
		return nil, err
	}
	return toProcesoDTOs(rows), nil
}

func toProcesoDTOs(rows []ProcesoRow) []ProcesoDTO {
	out := make([]ProcesoDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, ProcesoDTO{
			ID:          r.ID,
			Nombre:      r.Nombre,
			Tipo:        r.Tipo,
			GradoNombre: r.GradoNombre,
			Nivel:       r.Nivel,
			Descripcion: r.Descripcion,
			Anio:        r.Anio,
		})
	}
	return out
}

// ListProcesosActivos lists the active procesos the user may join: those of
// the institución of their persona that fit their grado's nivel (PLT-06).
func (s *Service) ListProcesosActivos(ctx context.Context, usuarioID int64) ([]ProcesoDTO, error) {
	rows, err := s.repo.ListProcesosElegibles(ctx, usuarioID)
	if err != nil {
		return nil, err
	}
	return toProcesoDTOs(rows), nil
}

// ── Inscripciones de usuario ──────────────────────────────────────────────────

type InscripcionDTO struct {
	ID            int64   `json:"id"`
	ProcesoID     int64   `json:"proceso_id"`
	ProcesoNombre string  `json:"proceso_nombre"`
	ProcesoTipo   string  `json:"proceso_tipo"`
	ProcesoAnio   int     `json:"proceso_anio"`
	Estado        string  `json:"estado"`
	Observacion   *string `json:"observacion,omitempty"`
}

// Inscribir creates the user's inscription, or re-applies a rejected one
// (onReapply audits it); pendiente/aprobado → ErrYaInscrito. The proceso must
// exist, be active and fit the user's institución and nivel, checked in that
// order (PLT-06).
func (s *Service) Inscribir(ctx context.Context, usuarioID, procesoID int64, onReapply ReapplyAuditFunc) (*InscripcionDTO, error) {
	acceso, err := s.repo.AccesoProceso(ctx, usuarioID, procesoID)
	if err != nil {
		return nil, err
	}
	if !acceso.Activo {
		return nil, ErrProcesoNotFound
	}
	if err := acceso.Permitido(); err != nil {
		return nil, err
	}

	row, err := s.repo.Create(ctx, usuarioID, procesoID, onReapply)
	if err != nil {
		return nil, err
	}
	return &InscripcionDTO{
		ID:        row.ID,
		ProcesoID: row.ProcesoID,
		Estado:    row.Estado,
	}, nil
}

func (s *Service) MisInscripciones(ctx context.Context, usuarioID int64) ([]InscripcionDTO, error) {
	rows, err := s.repo.ListByUsuario(ctx, usuarioID)
	if err != nil {
		return nil, err
	}
	out := make([]InscripcionDTO, len(rows))
	for i, r := range rows {
		out[i] = InscripcionDTO{
			ID:            r.ID,
			ProcesoID:     r.ProcesoID,
			ProcesoNombre: r.ProcesoNombre,
			ProcesoTipo:   r.ProcesoTipo,
			ProcesoAnio:   r.ProcesoAnio,
			Estado:        r.Estado,
			Observacion:   r.Observacion,
		}
	}
	return out, nil
}

// ── Admin: inscripciones ──────────────────────────────────────────────────────

type InscripcionAdminDTO struct {
	ID                 int64   `json:"id"`
	UsuarioID          int64   `json:"usuario_id"`
	Username           string  `json:"username"`
	NombreCompleto     string  `json:"nombre_completo"`
	DNI                string  `json:"dni"`
	GradoNombre        string  `json:"grado"`
	EspecialidadNombre string  `json:"especialidad"`
	ProcesoNombre      string  `json:"proceso_nombre"`
	ProcesoTipo        string  `json:"proceso_tipo"`
	ProcesoAnio        int     `json:"proceso_anio"`
	Estado             string  `json:"estado"`
	Observacion        *string `json:"observacion,omitempty"`
	FechaSolicitud     string  `json:"fecha_solicitud"`
	// Set once the inscription was decided: reviewer's username and date.
	RevisadoPor *string `json:"revisado_por,omitempty"`
	RevisadoAt  *string `json:"revisado_at,omitempty"`
}

func (s *Service) ListInscripciones(ctx context.Context, estado string, page, perPage int) ([]InscripcionAdminDTO, error) {
	rows, err := s.repo.ListByEstado(ctx, estado, page, perPage)
	if err != nil {
		return nil, err
	}
	return toAdminDTOs(rows), nil
}

func (s *Service) UpdateEstado(ctx context.Context, id int64, estado, observacion string, adminID int64, auditFn audit.TxFunc) error {
	if estado != "aprobado" && estado != "rechazado" {
		return ErrEstadoInvalido
	}
	return s.repo.UpdateEstado(ctx, id, estado, observacion, adminID, auditFn)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func toAdminDTOs(rows []InscripcionDetail) []InscripcionAdminDTO {
	out := make([]InscripcionAdminDTO, len(rows))
	for i, r := range rows {
		out[i] = InscripcionAdminDTO{
			ID:                 r.ID,
			UsuarioID:          r.UsuarioID,
			Username:           r.Username,
			NombreCompleto:     r.Nombres + " " + r.ApellidoPaterno + " " + r.ApellidoMaterno,
			DNI:                r.DNI,
			GradoNombre:        r.GradoNombre,
			EspecialidadNombre: r.EspecialidadNombre,
			ProcesoNombre:      r.ProcesoNombre,
			ProcesoTipo:        r.ProcesoTipo,
			ProcesoAnio:        r.ProcesoAnio,
			Estado:             r.Estado,
			Observacion:        r.Observacion,
			FechaSolicitud:     r.CreatedAt.Format(time.RFC3339),
			RevisadoPor:        r.RevisadoPor,
		}
		if r.RevisadoAt != nil {
			s := r.RevisadoAt.Format(time.RFC3339)
			out[i].RevisadoAt = &s
		}
	}
	return out
}
