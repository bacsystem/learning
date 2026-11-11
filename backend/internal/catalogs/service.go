package catalogs

import (
	"context"
	"errors"
	"fmt"

	"github.com/cbaciliod/learning/internal/audit"
)

// Service implements the business logic for catalog entities.
type Service struct {
	repo *Repository
}

// NewService creates a new catalogs Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ── Instituciones ─────────────────────────────────────────────────────────────

func (s *Service) ListInstituciones(ctx context.Context, q ListQuery) ([]InstitucionResponse, error) {
	return s.repo.ListInstituciones(ctx, q)
}

func (s *Service) GetInstitucion(ctx context.Context, id int64) (*InstitucionResponse, error) {
	return s.repo.GetInstitucion(ctx, id)
}

func (s *Service) CreateInstitucion(ctx context.Context, req CreateInstitucionRequest) (*InstitucionResponse, error) {
	result, err := s.repo.CreateInstitucion(ctx, req)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("%w: Ya existe una institución con esas siglas.", ErrConflict)
		}
		return nil, err
	}
	return result, nil
}

func (s *Service) UpdateInstitucion(ctx context.Context, id int64, req UpdateInstitucionRequest) (*InstitucionResponse, error) {
	result, err := s.repo.UpdateInstitucion(ctx, id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("%w: Ya existe una institución con esas siglas.", ErrConflict)
		}
		return nil, err
	}
	return result, nil
}

func (s *Service) UpdateInstitucionEstado(ctx context.Context, id int64, estado string, auditFn audit.TxFunc) error {
	return s.repo.UpdateInstitucionEstado(ctx, id, estado, auditFn)
}

// ── Grados ────────────────────────────────────────────────────────────────────

func (s *Service) ListGrados(ctx context.Context, q ListQuery) ([]GradoResponse, error) {
	return s.repo.ListGrados(ctx, q)
}

func (s *Service) GetGrado(ctx context.Context, id int64) (*GradoResponse, error) {
	return s.repo.GetGrado(ctx, id)
}

func (s *Service) CreateGrado(ctx context.Context, req CreateGradoRequest) (*GradoResponse, error) {
	result, err := s.repo.CreateGrado(ctx, req)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("%w: Ya existe un grado con ese nombre u orden para esta institución.", ErrConflict)
		}
		if isFKViolation(err) {
			return nil, invalidRef("la institución indicada no existe.")
		}
		return nil, err
	}
	return result, nil
}

func (s *Service) UpdateGrado(ctx context.Context, id int64, req CreateGradoRequest) (*GradoResponse, error) {
	result, err := s.repo.UpdateGrado(ctx, id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("%w: Ya existe un grado con ese nombre u orden para esta institución.", ErrConflict)
		}
		if isFKViolation(err) {
			return nil, invalidRef("la institución indicada no existe.")
		}
		return nil, err
	}
	return result, nil
}

func (s *Service) UpdateGradoEstado(ctx context.Context, id int64, estado string) error {
	if estado == "inactivo" {
		count, err := s.repo.CountActiveUsersByGrado(ctx, id)
		if err != nil {
			return err // logged by mapErr
		}
		if count > 0 {
			return fmt.Errorf("%w: Existen usuarios activos vinculados a este grado.", ErrConflict)
		}
	}
	return s.repo.UpdateGradoEstado(ctx, id, estado)
}

// ── Especialidades ────────────────────────────────────────────────────────────

func (s *Service) ListEspecialidades(ctx context.Context, q ListQuery) ([]EspecialidadResponse, error) {
	return s.repo.ListEspecialidades(ctx, q)
}

func (s *Service) GetEspecialidad(ctx context.Context, id int64) (*EspecialidadResponse, error) {
	return s.repo.GetEspecialidad(ctx, id)
}

func (s *Service) CreateEspecialidad(ctx context.Context, req CreateEspecialidadRequest) (*EspecialidadResponse, error) {
	result, err := s.repo.CreateEspecialidad(ctx, req)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("%w: Ya existe una especialidad con ese código para esta institución.", ErrConflict)
		}
		if isFKViolation(err) {
			return nil, invalidRef("la institución indicada no existe.")
		}
		return nil, err
	}
	return result, nil
}

func (s *Service) UpdateEspecialidad(ctx context.Context, id int64, req CreateEspecialidadRequest) (*EspecialidadResponse, error) {
	result, err := s.repo.UpdateEspecialidad(ctx, id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("%w: Ya existe una especialidad con ese código para esta institución.", ErrConflict)
		}
		if isFKViolation(err) {
			return nil, invalidRef("la institución indicada no existe.")
		}
		return nil, err
	}
	return result, nil
}

func (s *Service) UpdateEspecialidadEstado(ctx context.Context, id int64, estado string) error {
	if estado == "inactivo" {
		count, err := s.repo.CountActiveUsersByEspecialidad(ctx, id)
		if err != nil {
			return err // logged by mapErr
		}
		if count > 0 {
			return fmt.Errorf("%w: Existen usuarios activos vinculados a esta especialidad.", ErrConflict)
		}
	}
	return s.repo.UpdateEspecialidadEstado(ctx, id, estado)
}

// ── Asignaturas ───────────────────────────────────────────────────────────────

func (s *Service) ListAsignaturas(ctx context.Context, q ListQuery) ([]AsignaturaResponse, error) {
	return s.repo.ListAsignaturas(ctx, q)
}

func (s *Service) GetAsignatura(ctx context.Context, id int64) (*AsignaturaResponse, error) {
	return s.repo.GetAsignatura(ctx, id)
}

func (s *Service) CreateAsignatura(ctx context.Context, req CreateAsignaturaRequest) (*AsignaturaResponse, error) {
	result, err := s.repo.CreateAsignatura(ctx, req)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("%w: Ya existe una asignatura con ese código.", ErrConflict)
		}
		return nil, err
	}
	return result, nil
}

func (s *Service) UpdateAsignatura(ctx context.Context, id int64, req CreateAsignaturaRequest) (*AsignaturaResponse, error) {
	result, err := s.repo.UpdateAsignatura(ctx, id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("%w: Ya existe una asignatura con ese código.", ErrConflict)
		}
		return nil, err
	}
	return result, nil
}

func (s *Service) UpdateAsignaturaEstado(ctx context.Context, id int64, estado string) error {
	return s.repo.UpdateAsignaturaEstado(ctx, id, estado)
}

// ── Procesos ──────────────────────────────────────────────────────────────────

func (s *Service) ListProcesos(ctx context.Context, q ListQuery) ([]ProcesoResponse, error) {
	return s.repo.ListProcesos(ctx, q)
}

func (s *Service) GetProceso(ctx context.Context, id int64) (*ProcesoDetailResponse, error) {
	return s.repo.GetProceso(ctx, id)
}

func (s *Service) CreateProceso(ctx context.Context, req CreateProcesoRequest) (*ProcesoResponse, error) {
	if err := ValidateSO3Grado(ctx, s.repo, req.Tipo, req.GradoID); err != nil {
		return nil, err
	}
	result, err := s.repo.CreateProceso(ctx, req)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("%w: Ya existe un proceso con ese nombre para esta institución.", ErrConflict)
		}
		if isFKViolation(err) {
			return nil, invalidRef("la institución o el grado indicados no existen.")
		}
		return nil, err
	}
	return result, nil
}

func (s *Service) UpdateProceso(ctx context.Context, id int64, req CreateProcesoRequest) (*ProcesoResponse, error) {
	if err := ValidateSO3Grado(ctx, s.repo, req.Tipo, req.GradoID); err != nil {
		return nil, err
	}
	result, err := s.repo.UpdateProceso(ctx, id, req)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("%w: Ya existe un proceso con ese nombre para esta institución.", ErrConflict)
		}
		if isFKViolation(err) {
			return nil, invalidRef("la institución o el grado indicados no existen.")
		}
		return nil, err
	}
	return result, nil
}

func (s *Service) UpdateProcesoEstado(ctx context.Context, id int64, estado string, auditFn audit.TxFunc) error {
	return s.repo.UpdateProcesoEstado(ctx, id, estado, auditFn)
}

// ── Proceso-Asignaturas ───────────────────────────────────────────────────────

// ListProcesoAsignaturas lists the set that applies to the user's especialidad.
func (s *Service) ListProcesoAsignaturas(ctx context.Context, procesoID, gradoID, especialidadID int64) ([]ProcesoAsignaturaItem, error) {
	return s.repo.ListProcesoAsignaturas(ctx, procesoID, gradoID, especialidadID)
}

// ListProcesoAsignaturasCompatibilidad lists only the compatibility rows.
func (s *Service) ListProcesoAsignaturasCompatibilidad(ctx context.Context, procesoID, gradoID, especialidadID int64) ([]ProcesoAsignaturaItem, error) {
	return s.repo.ListProcesoAsignaturasCompatibilidad(ctx, procesoID, gradoID, especialidadID)
}

func (s *Service) AsignarAsignatura(ctx context.Context, procesoID int64, req AsignarAsignaturaRequest, auditFn audit.TxFunc) (*ProcesoAsignaturaItem, error) {
	result, err := s.repo.AsignarAsignatura(ctx, procesoID, req, auditFn)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrPesoDuplicado
		}
		if isFKViolation(err) {
			return nil, invalidRef("la asignatura indicada no existe.")
		}
		return nil, err
	}
	return result, nil
}

func (s *Service) UpdateProcesoAsignatura(ctx context.Context, procesoID, asignaturaID int64, req UpdateProcesoAsignaturaRequest, auditFn audit.TxFunc) error {
	return s.repo.UpdateProcesoAsignatura(ctx, procesoID, asignaturaID, req, auditFn)
}

func (s *Service) DeleteProcesoAsignatura(ctx context.Context, procesoID, asignaturaID int64, auditFn audit.TxFunc) error {
	return s.repo.DeleteProcesoAsignatura(ctx, procesoID, asignaturaID, auditFn)
}
