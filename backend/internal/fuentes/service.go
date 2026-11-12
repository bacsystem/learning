package fuentes

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/cbaciliod/learning/internal/texto"
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
	Retirar(ctx context.Context, id int64, motivo string, actor int64, suscriptores []SuscriptorRetiro, aud Auditor) (int, error)
	Restaurar(ctx context.Context, id int64, aud Auditor) error
	Reintentar(ctx context.Context, id int64, aud Auditor) error
	MarcarOficial(ctx context.Context, id int64, entidad, nota string, actor int64, aud Auditor) error
	DesmarcarOficial(ctx context.Context, id int64, motivo string, aud Auditor) error
	Editar(ctx context.Context, id int64, titulo string, especialidades []int64, aud Auditor) error
	BuscarPasajes(ctx context.Context, consulta string) ([]PasajeConFuente, error)
}

// Service holds the upload rules of SIA-E2-03 (S1), the new version, state
// and mark of SIA-E2-06 (S2, S5, S6) and the admin listing, detail and edit
// of SIA-E2-07 (S3, S4, S7).
type Service struct {
	repo         almacen
	citantes     Citantes
	suscriptores []SuscriptorRetiro
}

// NewService creates a Service. A nil citantes means nothing cites a fuente
// yet; without suscriptores a retirement notifies nobody (MVP: SIA-E3-02,
// E6-06 and E6-08 register theirs in cmd/server/main.go).
func NewService(repo almacen, citantes Citantes, suscriptores ...SuscriptorRetiro) *Service {
	if citantes == nil {
		citantes = sinCitantes{}
	}
	return &Service{repo: repo, citantes: citantes, suscriptores: suscriptores}
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

// CambiarEstado applies S5: the body's shape first (422), then the change in
// the repository (404, 409, 422 limite_fuentes). A retirement answers how many
// contents its subscribers marked, 0 included.
func (s *Service) CambiarEstado(ctx context.Context, id int64, req CambioEstadoRequest, actor int64, aud Auditor) (FuenteDetalle, error) {
	t, err := validarCambioEstado(req)
	if err != nil {
		return FuenteDetalle{}, err
	}
	var marcados *int
	switch t.Destino {
	case EstadoRetirada:
		n, rerr := s.repo.Retirar(ctx, id, t.Motivo, actor, s.suscriptores, aud)
		marcados, err = &n, rerr
	case EstadoActiva:
		err = s.repo.Restaurar(ctx, id, aud)
	default: // EstadoProcesando, the only other destino validarCambioEstado lets through
		err = s.repo.Reintentar(ctx, id, aud)
	}
	if err != nil {
		return FuenteDetalle{}, err
	}
	d, err := s.repo.Detalle(ctx, id, s.citantes)
	d.ContenidosMarcados = marcados
	return d, err
}

// Oficial applies S6 to the current version: mark or unmark.
func (s *Service) Oficial(ctx context.Context, id int64, req OficialRequest, actor int64, aud Auditor) (FuenteDetalle, error) {
	m, err := validarOficial(req)
	if err != nil {
		return FuenteDetalle{}, err
	}
	if m.Marcar {
		err = s.repo.MarcarOficial(ctx, id, m.EntidadEmisora, m.Nota, actor, aud)
	} else {
		err = s.repo.DesmarcarOficial(ctx, id, m.Motivo, aud)
	}
	if err != nil {
		return FuenteDetalle{}, err
	}
	return s.repo.Detalle(ctx, id, s.citantes)
}

// Editar applies S7: title and especialidades, without a new version.
func (s *Service) Editar(ctx context.Context, id int64, req EditarRequest, aud Auditor) (FuenteDetalle, error) {
	d, err := validarEdicion(req)
	if err != nil {
		return FuenteDetalle{}, err
	}
	if err := s.repo.Editar(ctx, id, d.Titulo, d.EspecialidadIDs, aud); err != nil {
		return FuenteDetalle{}, err
	}
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

// BuscarPasajes is the admin search for citing a passage (E3-07 §2): ErrConsultaInvalida
// outside 3–300 runes, otherwise at most MaxResultados hits with their extract.
func (s *Service) BuscarPasajes(ctx context.Context, q string) (ListaPasajes, error) {
	consulta, err := NormalizarConsulta(q)
	if err != nil {
		return ListaPasajes{}, err
	}
	hallados, err := s.repo.BuscarPasajes(ctx, consulta)
	if err != nil {
		return ListaPasajes{}, err
	}
	lista := ListaPasajes{Data: make([]PasajeBuscado, 0, len(hallados))}
	for _, h := range hallados {
		lista.Data = append(lista.Data, PasajeBuscado{PasajeID: h.PasajeID, FuenteID: h.FuenteID,
			FuenteTitulo: h.TituloFuente, Version: h.Version, Seccion: h.Seccion,
			LineaInicio: h.LineaInicio, LineaFin: h.LineaFin, Extracto: texto.Extracto(h.Texto)})
	}
	return lista, nil
}
