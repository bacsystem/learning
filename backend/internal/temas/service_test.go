package temas

import (
	"context"
	"errors"
	"testing"
)

// fakeRepo counts mutations and answers Obtener with obtenerErr.
type fakeRepo struct {
	Repositorio
	mutaciones int
	obtenerErr error
}

func (f *fakeRepo) Obtener(context.Context, int64) (Tema, error) { return Tema{ID: 1}, f.obtenerErr }
func (f *fakeRepo) Crear(context.Context, int64, DatosTema, Auditor) (int64, error) {
	f.mutaciones++
	return 1, nil
}
func (f *fakeRepo) Editar(context.Context, int64, DatosTema, Auditor) error {
	f.mutaciones++
	return nil
}
func (f *fakeRepo) CambiarEstado(context.Context, int64, string, Auditor) error {
	f.mutaciones++
	return nil
}
func (f *fakeRepo) Reordenar(context.Context, int64, []int64, Auditor) ([]PosicionTema, error) {
	f.mutaciones++
	return nil, nil
}

// Invalid input never reaches the repository, so nothing is written or audited.
func TestService_ValidaAntesDeEscribir(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		call func(s *Service) error
		want error
	}{
		{"crear nombre corto", func(s *Service) error {
			_, err := s.Crear(ctx, CrearTemaRequest{AsignaturaID: 7, Nombre: "ab"}, nil)
			return err
		}, ErrValidacion},
		{"editar especialidad repetida", func(s *Service) error {
			_, err := s.Editar(ctx, 1, EditarTemaRequest{Nombre: "abc", EspecialidadIDs: []int64{2, 2}}, nil)
			return err
		}, ErrValidacion},
		{"estado inválido", func(s *Service) error {
			_, err := s.CambiarEstado(ctx, 1, CambiarEstadoRequest{Estado: "borrado"}, nil)
			return err
		}, ErrValidacion},
		{"orden repetido", func(s *Service) error {
			_, err := s.Reordenar(ctx, 7, []int64{1, 1}, nil)
			return err
		}, ErrOrdenInvalido},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &fakeRepo{}
			if err := c.call(NewService(repo)); !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			if repo.mutaciones != 0 {
				t.Errorf("repository called %d times", repo.mutaciones)
			}
		})
	}
}

// A tema that vanishes right after its own commit is an internal error, never
// a 404 for the request that just created it.
func TestService_ReleerNoDevuelve404(t *testing.T) {
	repo := &fakeRepo{obtenerErr: ErrNoEncontrado}
	_, err := NewService(repo).Crear(context.Background(), CrearTemaRequest{AsignaturaID: 7, Nombre: "abc"}, nil)
	if err == nil || errors.Is(err, ErrNoEncontrado) {
		t.Fatalf("err = %v, want an internal (non-404) error", err)
	}
}
