package inscripciones

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/examen"
	"github.com/rs/zerolog/log"
)

// maxObservacion bounds the admin's note (and the motivo of a rejection).
const maxObservacion = 500

var (
	ErrProcesoNotFound     = errors.New("El proceso no existe o no está activo.")
	ErrYaInscrito          = errors.New("Ya estás inscrito en este proceso.")
	ErrInscripcionNotFound = errors.New("Inscripción no encontrada.")
	ErrEstadoInvalido      = errors.New("Estado inválido. Valores permitidos: aprobado, rechazado.")
	// ErrMotivoRequerido: a rejection must say why (the postulante reads it).
	ErrMotivoRequerido = errors.New("El motivo del rechazo es obligatorio.")
	// ErrObservacionLarga: the observación exceeds maxObservacion characters.
	ErrObservacionLarga = errors.New("La observación no puede superar los 500 caracteres.")
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
	repo            *Repository
	frontendBaseURL string // no trailing slash; the e-mails link to it
}

func NewService(repo *Repository, frontendBaseURL string) *Service {
	return &Service{repo: repo, frontendBaseURL: frontendBaseURL}
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

	// Read before creating: a failure after Create would answer 500 for an
	// inscription that exists, and the retry would get ya_inscrito.
	proceso, err := s.repo.ProcesoResumen(ctx, procesoID)
	if err != nil {
		return nil, err
	}
	row, err := s.repo.Create(ctx, usuarioID, procesoID, onReapply)
	if err != nil {
		return nil, err
	}
	return &InscripcionDTO{
		ID:            row.ID,
		ProcesoID:     row.ProcesoID,
		ProcesoNombre: proceso.Nombre,
		ProcesoTipo:   proceso.Tipo,
		ProcesoAnio:   proceso.Anio,
		Estado:        row.Estado,
		Observacion:   row.Observacion,
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

// UpdateEstado decides a pendiente inscripción and tells the postulante, by
// e-mail and in the app, in the same transaction (PLT-12). A rejection needs a
// motivo of 1 to 500 characters; an approval may carry one of up to 500.
func (s *Service) UpdateEstado(ctx context.Context, id int64, estado, observacion string, adminID int64, auditFn audit.TxFunc) error {
	if estado != estadoAprobado && estado != estadoRechazado {
		return ErrEstadoInvalido
	}
	observacion = strings.TrimSpace(observacion)
	if utf8.RuneCountInString(observacion) > maxObservacion {
		return ErrObservacionLarga
	}
	if estado == estadoRechazado && observacion == "" {
		return ErrMotivoRequerido
	}
	return s.repo.UpdateEstado(ctx, id, estado, observacion, adminID, auditFn, s.avisar(adminID))
}

// avisar renders the notice of a decision and writes it inside its
// transaction: the in-app notification always, the e-mail when the persona has
// an address. A failure here reverts the decision; a later SMTP failure does
// not, because the worker retries the queued e-mail.
func (s *Service) avisar(adminID int64) AvisoFunc {
	return func(ctx context.Context, tx execer, d datosAviso) error {
		a := renderAviso(d, s.frontendBaseURL)
		params, err := json.Marshal(a.Parametros)
		if err != nil {
			return fmt.Errorf("aviso: parámetros: %w", err)
		}
		if err := s.repo.insertNotificacion(ctx, tx, adminID, d.UsuarioID, a, params); err != nil {
			return fmt.Errorf("aviso: notificación: %w", err)
		}
		if d.Email == "" {
			log.Warn().Str("op", "inscripciones.aviso").Int64("inscripcion_id", d.InscripcionID).
				Int64("usuario_id", d.UsuarioID).Msg("no e-mail address; only the in-app notice was created")
			return nil
		}
		if err := s.repo.encolarCorreo(ctx, tx, d.Email, a.Asunto, a.CorreoHTML); err != nil {
			return fmt.Errorf("aviso: correo: %w", err)
		}
		return nil
	}
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
