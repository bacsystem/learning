package fuentes

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

// maxConsultaTitulo bounds the S3 title search (contract S3, q).
const maxConsultaTitulo = 100

// almacen is what the Service needs from the admin repository.
type almacen interface {
	CrearFuente(ctx context.Context, f NuevaFuente, aud Auditor) (int64, error)
	NuevaVersion(ctx context.Context, fuenteID int64, nombre string, contenido []byte, creadoPor *int64, citantes Citantes, aud Auditor) (int, error)
	Detalle(ctx context.Context, id int64, citantes Citantes) (FuenteDetalle, error)
	DuplicadoDe(ctx context.Context, sha string) error
	Listar(ctx context.Context, f FiltroListado) (ListaFuentes, error)
}

// Service holds the upload rules of SIA-E2-03 (S1), the new version of
// SIA-E2-06 (S2) and the admin listing and detail of SIA-E2-07 (S3, S4).
type Service struct {
	repo     almacen
	citantes Citantes
}

// NewService creates a Service. A nil citantes means nothing cites a fuente
// yet (MVP; SIA-E3-02 registers one).
func NewService(repo almacen, citantes Citantes) *Service {
	if citantes == nil {
		citantes = sinCitantes{}
	}
	return &Service{repo: repo, citantes: citantes}
}

// Crear validates the file (S1 steps 3–6) and the metadata (step 7), then
// creates the fuente with its version 1 in 'procesando' (steps 7–9 under the
// lock) and returns its detail.
func (s *Service) Crear(ctx context.Context, archivo ArchivoSubido, formulario FormularioFuente, actor *int64, aud Auditor) (FuenteDetalle, error) {
	nombre, err := validarArchivo(archivo)
	if err != nil {
		return FuenteDetalle{}, err
	}
	d, err := validarFormulario(formulario)
	if err != nil {
		return FuenteDetalle{}, err
	}
	id, err := s.repo.CrearFuente(ctx, NuevaFuente{Titulo: d.Titulo, Tipo: d.Tipo, Anio: d.Anio, ProcesoID: d.ProcesoID,
		EspecialidadIDs: d.EspecialidadIDs, NombreArchivo: nombre, Contenido: archivo.Contenido, CreadoPor: actor}, aud)
	if err != nil {
		return FuenteDetalle{}, s.conDetalleDuplicado(ctx, err, archivo.Contenido)
	}
	return s.repo.Detalle(ctx, id, s.citantes)
}

// NuevaVersion validates the file (S1 steps 3–6) and adds the next version
// in 'procesando' (S2), returning the fuente's detail.
func (s *Service) NuevaVersion(ctx context.Context, fuenteID string, archivo ArchivoSubido, actor *int64, aud Auditor) (FuenteDetalle, error) {
	nombre, err := validarArchivo(archivo)
	if err != nil {
		return FuenteDetalle{}, err
	}
	id, err := parsearFuenteID(fuenteID)
	if err != nil {
		return FuenteDetalle{}, err
	}
	if _, err := s.repo.NuevaVersion(ctx, id, nombre, archivo.Contenido, actor, s.citantes, aud); err != nil {
		return FuenteDetalle{}, s.conDetalleDuplicado(ctx, err, archivo.Contenido)
	}
	return s.repo.Detalle(ctx, id, s.citantes)
}

// Listar returns one page of S3. The title search is trimmed and, past 100
// characters, is 422 validacion (campos.q).
func (s *Service) Listar(ctx context.Context, f FiltroListado) (ListaFuentes, error) {
	f.Q = strings.TrimSpace(f.Q)
	if utf8.RuneCountInString(f.Q) > maxConsultaTitulo {
		return ListaFuentes{}, &ValidacionError{Campos: map[string]string{"q": reglaMax}}
	}
	return s.repo.Listar(ctx, f)
}

// Obtener returns the FuenteDetalle of S4, or ErrNoEncontrado.
func (s *Service) Obtener(ctx context.Context, id int64) (FuenteDetalle, error) {
	return s.repo.Detalle(ctx, id, s.citantes)
}

// conDetalleDuplicado: when the unique index fired instead of the check under
// the lock (a race), the detail is read after the rollback (ADR 0003).
func (s *Service) conDetalleDuplicado(ctx context.Context, err error, contenido []byte) error {
	var dup *DuplicadaError
	if !errors.Is(err, ErrFuenteDuplicada) || errors.As(err, &dup) {
		return err
	}
	sha, _, _ := Huella(contenido)
	if derr := s.repo.DuplicadoDe(ctx, sha); errors.As(derr, &dup) {
		return dup
	}
	return err
}
