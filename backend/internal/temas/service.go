package temas

import (
	"context"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// Evento is one audit_log row of a tema mutation.
type Evento struct {
	Accion    string
	Entidad   string
	EntidadID int64
	Detalle   map[string]any
}

// Auditor writes ev inside the mutation's transaction; its error rolls the
// mutation back. The handler builds it with the actor, IP and user agent.
type Auditor func(ctx context.Context, tx sqlx.ExecerContext, ev Evento) error

// Repositorio is the storage the Service needs. Mutations enforce, under the
// asignatura lock, the invariants that depend on other rows (references,
// unique name, active limit, current order).
type Repositorio interface {
	Listar(ctx context.Context, f Filtro) ([]Tema, int, error)
	Obtener(ctx context.Context, id int64) (Tema, error)
	ListarActivos(ctx context.Context, asignaturaID, especialidadID int64) ([]TemaActivo, error)
	Crear(ctx context.Context, asignaturaID int64, d DatosTema, aud Auditor) (int64, error)
	Editar(ctx context.Context, id int64, d DatosTema, aud Auditor) error
	CambiarEstado(ctx context.Context, id int64, estado string, aud Auditor) error
	Reordenar(ctx context.Context, asignaturaID int64, ids []int64, aud Auditor) ([]PosicionTema, error)
}

// Service holds the tema rules (SIA-E1-01, SIA-E1-02).
type Service struct {
	repo Repositorio
}

// NewService creates a Service over repo.
func NewService(repo Repositorio) *Service {
	return &Service{repo: repo}
}

// Listar returns one page of temas (T1).
func (s *Service) Listar(ctx context.Context, f Filtro) (ListaTemas, error) {
	data, total, err := s.repo.Listar(ctx, f)
	if err != nil {
		return ListaTemas{}, err
	}
	return ListaTemas{Data: data, Total: total, Page: f.Page, PerPage: f.PerPage}, nil
}

// Obtener returns one tema (T2).
func (s *Service) Obtener(ctx context.Context, id int64) (Tema, error) {
	return s.repo.Obtener(ctx, id)
}

// ListarActivos returns the active temas of an asignatura (T7).
func (s *Service) ListarActivos(ctx context.Context, asignaturaID, especialidadID int64) ([]TemaActivo, error) {
	return s.repo.ListarActivos(ctx, asignaturaID, especialidadID)
}

// Crear validates and creates an active tema (T3).
func (s *Service) Crear(ctx context.Context, req CrearTemaRequest, aud Auditor) (Tema, error) {
	d, err := validarCrear(req)
	if err != nil {
		return Tema{}, err
	}
	id, err := s.repo.Crear(ctx, req.AsignaturaID, d, aud)
	if err != nil {
		return Tema{}, err
	}
	return s.releer(ctx, id)
}

// Editar validates and replaces the editable fields of a tema (T4).
func (s *Service) Editar(ctx context.Context, id int64, req EditarTemaRequest, aud Auditor) (Tema, error) {
	d, err := validarEditar(req)
	if err != nil {
		return Tema{}, err
	}
	if err := s.repo.Editar(ctx, id, d, aud); err != nil {
		return Tema{}, err
	}
	return s.releer(ctx, id)
}

// CambiarEstado archives or reactivates a tema (T5).
func (s *Service) CambiarEstado(ctx context.Context, id int64, req CambiarEstadoRequest, aud Auditor) (Tema, error) {
	if err := validarEstado(req.Estado); err != nil {
		return Tema{}, err
	}
	if err := s.repo.CambiarEstado(ctx, id, req.Estado, aud); err != nil {
		return Tema{}, err
	}
	return s.releer(ctx, id)
}

// Reordenar saves the new order of the active temas of an asignatura (T6).
func (s *Service) Reordenar(ctx context.Context, asignaturaID int64, ids []int64, aud Auditor) ([]PosicionTema, error) {
	if err := validarOrden(ids); err != nil {
		return nil, err
	}
	return s.repo.Reordenar(ctx, asignaturaID, ids, aud)
}

// releer reads back a tema the same request just committed. Temas are never
// deleted, so ErrNoEncontrado here is an internal error, not a 404.
func (s *Service) releer(ctx context.Context, id int64) (Tema, error) {
	t, err := s.repo.Obtener(ctx, id)
	if errors.Is(err, ErrNoEncontrado) {
		return Tema{}, fmt.Errorf("releer tema %d tras confirmar: el tema no existe", id)
	}
	return t, err
}
