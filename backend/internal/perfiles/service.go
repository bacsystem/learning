package perfiles

import (
	"context"
	"errors"
	"fmt"

	"github.com/cbaciliod/learning/internal/catalogs"
)

// Repositorio is the storage the Service needs.
type Repositorio interface {
	PerfilActivo(ctx context.Context, usuarioID int64) (perfilRow, error)
	ProcesosAprobados(ctx context.Context, usuarioID int64) ([]ProcesoOpcion, error)
	EspecialidadDePersona(ctx context.Context, usuarioID int64) (*int64, error)
	NombresAsignaturas(ctx context.Context, ids []int64) (map[int64]string, error)
	Elegir(ctx context.Context, usuarioID int64, par Par, confirmar bool) error
}

// ServicioPesos is catalogs PE6, the single entry point for weights.
type ServicioPesos interface {
	Pesos(ctx context.Context, procesoID, especialidadID int64, modo catalogs.ModoPesos) (catalogs.Pesos, error)
}

// Service holds the profession rules (SIA-E1-04).
type Service struct {
	repo  Repositorio
	pesos ServicioPesos
}

// NewService creates a Service.
func NewService(repo Repositorio, pesos ServicioPesos) *Service {
	return &Service{repo: repo, pesos: pesos}
}

// Opciones returns the selectable procesos and professions and the
// preselection (F1). It is read-only.
func (s *Service) Opciones(ctx context.Context, usuarioID int64) (Opciones, error) {
	procesos, err := s.repo.ProcesosAprobados(ctx, usuarioID)
	if err != nil {
		return Opciones{}, err
	}
	pre, err := s.preseleccion(ctx, usuarioID, procesos)
	if err != nil {
		return Opciones{}, err
	}
	return Opciones{Procesos: procesos, Preseleccion: pre}, nil
}

// preseleccion: the active profile if any; otherwise the persona's
// especialidad, only if it is one of the offered ones (regla 2), with the
// proceso when there is a single one (contract F1); otherwise nil.
func (s *Service) preseleccion(ctx context.Context, usuarioID int64, procesos []ProcesoOpcion) (*Preseleccion, error) {
	perfil, err := s.repo.PerfilActivo(ctx, usuarioID)
	if err == nil {
		return &Preseleccion{ProcesoID: &perfil.ProcesoID, EspecialidadID: &perfil.EspecialidadID}, nil
	}
	if !errors.Is(err, ErrProfesionNoDefinida) {
		return nil, err
	}
	esp, err := s.repo.EspecialidadDePersona(ctx, usuarioID)
	if err != nil || esp == nil || !ofrecida(procesos, *esp) {
		return nil, err
	}
	pre := &Preseleccion{EspecialidadID: esp}
	if len(procesos) == 1 {
		pre.ProcesoID = &procesos[0].ID
	}
	return pre, nil
}

func ofrecida(procesos []ProcesoOpcion, especialidadID int64) bool {
	for _, p := range procesos {
		for _, e := range p.Especialidades {
			if e.ID == especialidadID {
				return true
			}
		}
	}
	return false
}

// Perfil returns the active profile with its weights (F2).
// ErrProfesionNoDefinida: none; ErrInscripcionRequerida: the inscripción is
// no longer approved (the profile is kept but the module is not usable).
func (s *Service) Perfil(ctx context.Context, usuarioID int64) (Perfil, error) {
	p, err := s.repo.PerfilActivo(ctx, usuarioID)
	if err != nil {
		return Perfil{}, err
	}
	if !p.InscripcionVigente {
		return Perfil{}, ErrInscripcionRequerida
	}
	pesos, err := s.pesosDe(ctx, p.ProcesoID, p.EspecialidadID)
	if err != nil {
		return Perfil{}, err
	}
	return Perfil{
		ID: p.ID, Estado: p.Estado, CreatedAt: p.CreatedAt.UTC(), Pesos: pesos,
		Proceso:      Ref{ID: p.ProcesoID, Nombre: p.Proceso},
		Especialidad: Ref{ID: p.EspecialidadID, Nombre: p.Especialidad},
	}, nil
}

// Elegir validates and saves the profession (F3) and returns the profile.
func (s *Service) Elegir(ctx context.Context, usuarioID int64, req ElegirRequest) (Perfil, error) {
	if err := validarElegir(req); err != nil {
		return Perfil{}, err
	}
	par := Par{ProcesoID: req.ProcesoID, EspecialidadID: req.EspecialidadID}
	if err := s.repo.Elegir(ctx, usuarioID, par, req.ConfirmarReinicio); err != nil {
		return Perfil{}, err
	}
	return s.Perfil(ctx, usuarioID)
}

func validarElegir(req ElegirRequest) error {
	campos := map[string]string{}
	if req.ProcesoID <= 0 {
		campos["proceso_id"] = "requerido"
	}
	if req.EspecialidadID <= 0 {
		campos["especialidad_id"] = "requerido"
	}
	if len(campos) > 0 {
		return &ValidacionError{Campos: campos}
	}
	return nil
}

// pesosDe reads the weights from PE6 in ModoEstimable and adds area names.
func (s *Service) pesosDe(ctx context.Context, procesoID, especialidadID int64) (PesosPerfil, error) {
	p, err := s.pesos.Pesos(ctx, procesoID, especialidadID, catalogs.ModoEstimable)
	if err != nil {
		return PesosPerfil{}, fmt.Errorf("pesos del perfil: %w", err)
	}
	ids := make([]int64, len(p.Areas))
	for i, a := range p.Areas {
		ids[i] = a.AsignaturaID
	}
	nombres, err := s.repo.NombresAsignaturas(ctx, ids)
	if err != nil {
		return PesosPerfil{}, err
	}
	areas := make([]AreaPeso, len(p.Areas))
	for i, a := range p.Areas {
		areas[i] = AreaPeso{AsignaturaID: a.AsignaturaID, Nombre: nombres[a.AsignaturaID], PesoPorcentual: a.PesoPorcentual}
	}
	return PesosPerfil{PesosEstimados: p.PesosEstimados, Areas: areas}, nil
}
