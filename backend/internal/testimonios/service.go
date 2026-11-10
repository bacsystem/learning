package testimonios

import (
	"context"
	"database/sql"
	"errors"
)

// Service holds business logic for testimonios.
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Submit upserts a testimony for the given user.
func (s *Service) Submit(ctx context.Context, usuarioID int64, contenido string, calificacion int) error {
	return s.repo.UpsertTestimonio(ctx, usuarioID, contenido, calificacion)
}

// GetPublicos returns all approved testimonials for public display.
func (s *Service) GetPublicos(ctx context.Context) ([]TestimonioPublicoDTO, error) {
	rows, err := s.repo.GetPublicos(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]TestimonioPublicoDTO, len(rows))
	for i, r := range rows {
		out[i] = TestimonioPublicoDTO{
			ID:            r.ID,
			NombrePublico: r.NombrePublico,
			Calificacion:  r.Calificacion,
			Contenido:     r.Contenido,
			CreatedAt:     r.CreatedAt,
		}
	}
	return out, nil
}

// GetMiTestimonio returns the current user's testimony, or nil if none exists.
func (s *Service) GetMiTestimonio(ctx context.Context, usuarioID int64) (*MiTestimonioDTO, error) {
	row, err := s.repo.GetMiTestimonio(ctx, usuarioID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &MiTestimonioDTO{
		ID:           row.ID,
		Calificacion: row.Calificacion,
		Contenido:    row.Contenido,
		Estado:       row.Estado,
		CreatedAt:    row.CreatedAt,
	}, nil
}

// ListAdmin returns a paginated list for admin moderation.
func (s *Service) ListAdmin(ctx context.Context, estado string, page, perPage int) (AdminListResponse, error) {
	rows, total, err := s.repo.ListAdmin(ctx, estado, page, perPage)
	if err != nil {
		return AdminListResponse{}, err
	}
	out := make([]TestimonioAdminDTO, len(rows))
	for i, r := range rows {
		out[i] = TestimonioAdminDTO{
			ID:             r.ID,
			UsuarioID:      r.UsuarioID,
			Username:       r.Username,
			NombreCompleto: r.NombreCompleto,
			Grado:          r.Grado,
			Calificacion:   r.Calificacion,
			Contenido:      r.Contenido,
			Estado:         r.Estado,
			CreatedAt:      r.CreatedAt,
		}
	}
	return AdminListResponse{Data: out, Total: total, Page: page, PerPage: perPage}, nil
}

// UpdateEstado approves or rejects a testimony.
func (s *Service) UpdateEstado(ctx context.Context, id int64, estado string) error {
	return s.repo.UpdateEstado(ctx, id, estado)
}
