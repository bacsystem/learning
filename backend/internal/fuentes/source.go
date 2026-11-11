// Package fuentes holds the source repository of the SERUMS IA module
// (SIA-E2-02, ADR 0003): the SourceRepository port and its own PostgreSQL
// adapter. HTTP routes, the ingest worker and the admin life cycle come with
// E2-03, E2-05 and E2-06.
package fuentes

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/cbaciliod/learning/internal/audit"
)

// Errors of the port (ADR 0003). Their text is the stable code.
var (
	ErrConsultaInvalida = errors.New("consulta_invalida")
	ErrCitaInvalida     = errors.New("cita_invalida") // missing, or (fuente, versión, pasaje) do not match
	ErrNoEncontrado     = errors.New("no_encontrado")
	// ErrFuenteDuplicada: the content hash already exists in any version of
	// any fuente (E2-03 regla 3); S1 answers it as 409 fuente_duplicada.
	ErrFuenteDuplicada = errors.New("fuente_duplicada")
	// ErrPublicacionInvalida: the passages to publish break an invariant
	// (count, orden, lines, words, text); nothing is published.
	ErrPublicacionInvalida = errors.New("publicacion_invalida")
)

// Limits of the port (SIA-E2-02 reglas 2–3; ADR 0003; migration 062).
const (
	MinConsulta         = 3
	MaxConsulta         = 300
	MaxResultados       = 20
	MaxPasajes          = 5000
	maxSeccion          = 300
	perPageDefecto      = 20
	perPageMax          = 100
	MotivoSinContenido  = "sin_contenido"
	MotivoLimitePasajes = "limite_pasajes"
	MotivoErrorInterno  = "error_interno"
)

// Estados de fuente y de versión (contract E2 § Estados).
const (
	EstadoProcesando = "procesando"
	EstadoActiva     = "activa"
	EstadoFallida    = "fallida"
	EstadoRetirada   = "retirada"
	VersionLista     = "lista"
)

// Cita is immutable: the version of a citation never changes (regla 1).
type Cita struct {
	FuenteID int64
	Version  int // version number (1..n), not the internal id
	PasajeID int64
}

// Busqueda is a passage search. AsignaturaID and TemaID are accepted but the
// MVP adapter does not filter by them (regla 6; ADR 0003 § Área y tema).
type Busqueda struct {
	Consulta       string // 3–300 runes after TrimSpace
	EspecialidadID int64  // required (> 0)
	AsignaturaID   *int64
	TemaID         *int64
	Limite         int // 1..20; 0 (or less) = 20; > 20 is capped at 20
}

// NuevaFuente is a fuente with its first version, created in 'procesando'.
type NuevaFuente struct {
	Titulo          string
	Tipo            string // balotario | guia | norma | otro
	Anio            *int
	ProcesoID       *int64
	EspecialidadIDs []int64 // empty = applies to every especialidad
	NombreArchivo   string
	Contenido       []byte // UTF-8 original; the adapter computes sha256, size and lines
	CreadoPor       *int64
}

// PasajeNuevo is one passage to publish, produced by normalizar.go (E2-05).
type PasajeNuevo struct {
	Orden       int
	LineaInicio int
	LineaFin    int
	Seccion     *string
	Texto       string // stored as is; only tsv is folded
	Palabras    int
}

// Pasaje is a published passage; it carries no user data (regla 4).
type Pasaje struct {
	FuenteID    int64
	Version     int
	PasajeID    int64
	Orden       int
	LineaInicio int
	LineaFin    int
	Seccion     *string
	Texto       string
}

// PasajeEncontrado is a search hit.
type PasajeEncontrado = Pasaje

// PasajeCitado is a resolved citation, with the fuente data of that version.
type PasajeCitado struct {
	Pasaje
	TituloFuente      string
	EstadoFuente      string
	OficialVerificada bool
}

// Pagina is one page of a listing.
type Pagina[T any] struct {
	Data    []T
	Total   int
	Page    int
	PerPage int
}

// SourceRepository is the port (ADR 0003). A v3 adapter (SIA-E2-08) would
// only replace BuscarPasajes; citations always resolve on the own adapter.
type SourceRepository interface {
	// GuardarFuente creates the fuente and its version 1 in 'procesando'.
	GuardarFuente(ctx context.Context, f NuevaFuente) (fuenteID int64, version int, err error)
	ObtenerPasajes(ctx context.Context, fuenteID int64, version int, page, perPage int) (Pagina[Pasaje], error)
	// BuscarPasajes returns ≤ 20 passages of the current version of active
	// fuentes of the especialidad (or of every one), by relevance then id.
	BuscarPasajes(ctx context.Context, b Busqueda) ([]PasajeEncontrado, error)
	// ResolverCita does not filter by the fuente's state: a historic citation
	// returns the exact text of its version (escenario «Resolver cita histórica»).
	ResolverCita(ctx context.Context, c Cita) (PasajeCitado, error)
	// PublicarVersion publishes a 'procesando' version atomically: every
	// passage, the version 'lista', the fuente's current version and state,
	// and auditFn in the same transaction (the in-memory adapter ignores it).
	PublicarVersion(ctx context.Context, fuenteID int64, version int, pasajes []PasajeNuevo, auditFn audit.TxFunc) error
	// MarcarVersionFallida marks a 'procesando' version failed with motivo.
	MarcarVersionFallida(ctx context.Context, fuenteID int64, version int, motivo string, auditFn audit.TxFunc) error
}

// NormalizarBusqueda validates a search and returns the trimmed query and the
// effective limit. It is shared by every adapter so they reject the same input.
func NormalizarBusqueda(b Busqueda) (consulta string, limite int, err error) {
	consulta = strings.TrimSpace(b.Consulta)
	if n := utf8.RuneCountInString(consulta); n < MinConsulta || n > MaxConsulta || b.EspecialidadID <= 0 {
		return "", 0, ErrConsultaInvalida
	}
	limite = b.Limite
	if limite <= 0 || limite > MaxResultados {
		limite = MaxResultados
	}
	return consulta, limite, nil
}

// ValidarPasajes checks the passages of a publication: 1–5000, unique orden in
// 1..5000, lines 1 ≤ inicio ≤ fin, palabras ≥ 1, non-blank text and a section
// of ≤ 300 runes. Shared by every adapter, so an invalid list publishes nothing.
func ValidarPasajes(pasajes []PasajeNuevo) error {
	if len(pasajes) == 0 || len(pasajes) > MaxPasajes {
		return ErrPublicacionInvalida
	}
	ordenes := make(map[int]struct{}, len(pasajes))
	for _, p := range pasajes {
		if _, rep := ordenes[p.Orden]; rep || p.Orden < 1 || p.Orden > MaxPasajes {
			return ErrPublicacionInvalida
		}
		ordenes[p.Orden] = struct{}{}
		if p.LineaInicio < 1 || p.LineaFin < p.LineaInicio || p.Palabras < 1 || strings.TrimSpace(p.Texto) == "" {
			return ErrPublicacionInvalida
		}
		if p.Seccion != nil && utf8.RuneCountInString(*p.Seccion) > maxSeccion {
			return ErrPublicacionInvalida
		}
	}
	return nil
}

// MotivoValido reports whether motivo is a failure reason of migration 062.
func MotivoValido(motivo string) bool {
	return motivo == MotivoSinContenido || motivo == MotivoLimitePasajes || motivo == MotivoErrorInterno
}

// NormalizarPagina applies the module's pagination defaults (README §5).
func NormalizarPagina(page, perPage int) (int, int) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = perPageDefecto
	}
	return page, min(perPage, perPageMax)
}

// ContarLineas counts the lines of the content: 0 when empty, and a final
// newline does not open an extra line.
func ContarLineas(contenido []byte) int {
	if len(contenido) == 0 {
		return 0
	}
	n := strings.Count(string(contenido), "\n")
	if contenido[len(contenido)-1] != '\n' {
		n++
	}
	return n
}
