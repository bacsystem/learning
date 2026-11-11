// Package fuentes holds the source repository of the SERUMS IA module
// (SIA-E2-02, ADR 0003): the SourceRepository port and its own PostgreSQL
// adapter. HTTP routes, the ingest worker and the admin life cycle come with
// E2-03, E2-05 and E2-06.
package fuentes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jmoiron/sqlx"
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

// Audit actions written by the adapter (contract E2 § Auditoría).
const (
	AccionCrear    = "fuente.crear"
	AccionProcesar = "fuente.procesar"
	AccionFallar   = "fuente.fallar"
	EntidadFuente  = "fuente"
)

// Evento is an audit row that the adapter builds inside its transaction, once
// it knows the fuente id (same pattern as temas.Evento). Detalle never holds
// content or personal data (README §7).
type Evento struct {
	Accion    string
	Entidad   string
	EntidadID int64
	Detalle   map[string]any
}

// Auditor writes ev in the mutation's transaction; its error rolls the
// mutation back. The caller binds the actor, IP and user agent (actor nil =
// system). A nil Auditor audits nothing (tests); the in-memory adapter never
// calls it.
type Auditor func(ctx context.Context, tx sqlx.ExecerContext, ev Evento) error

// NuevaFuente is a fuente with its first version, created in 'procesando'.
// The caller has already validated the metadata and sanitised NombreArchivo
// (S1 of contract E2).
type NuevaFuente struct {
	Titulo          string
	Tipo            string // balotario | guia | norma | otro
	Anio            *int
	ProcesoID       *int64
	EspecialidadIDs []int64 // empty = applies to every especialidad
	NombreArchivo   string
	Contenido       []byte // UTF-8 original; the adapter computes sha256, size and lines with Huella
	CreadoPor       *int64 // nil = system or test
}

// FiltroFuentes selects ListarFuentes. In EspecialidadID, Estado and Tipo the
// zero value means no filter. Page and PerPage are normalised without error
// (NormalizarPagina); the HTTP handler validates the query strictly.
type FiltroFuentes struct {
	EspecialidadID int64  // > 0: fuentes of that especialidad plus those for all
	Estado         string // procesando | activa | fallida | retirada
	Tipo           string // balotario | guia | norma | otro
	Page, PerPage  int
}

// FuenteResumen is the minimum any adapter can give: ids, not catalog names.
// The admin listing S3 (names, version_en_proceso…) is an admin query
// outside the port (E2-07).
type FuenteResumen struct {
	ID              int64
	Titulo, Tipo    string
	Estado          string
	Anio            *int
	ProcesoID       *int64
	EspecialidadIDs []int64 // empty = applies to every one; ascending
	VersionVigente  *int    // version number; nil if there is none
	CreatedAt       time.Time
	UpdatedAt       time.Time
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

// PasajeEncontrado is a published passage; it carries no user data (regla 4).
type PasajeEncontrado struct {
	FuenteID    int64
	Version     int
	PasajeID    int64
	Orden       int
	LineaInicio int
	LineaFin    int
	Seccion     *string
	Texto       string
}

// Pasaje has the same fields as PasajeEncontrado (ADR 0003).
type Pasaje = PasajeEncontrado

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
	// GuardarFuente creates the fuente and its version 1 in 'procesando' and
	// calls aud with fuente.crear and the new id in the same transaction.
	GuardarFuente(ctx context.Context, f NuevaFuente, aud Auditor) (fuenteID int64, version int, err error)
	// ListarFuentes: UpdatedAt DESC, ID DESC.
	ListarFuentes(ctx context.Context, f FiltroFuentes) (Pagina[FuenteResumen], error)
	// ObtenerPasajes: by orden, whatever the fuente's state; ErrNoEncontrado
	// if the fuente or version does not exist; empty Data for a version
	// without passages.
	ObtenerPasajes(ctx context.Context, fuenteID int64, version int, page, perPage int) (Pagina[Pasaje], error)
	// BuscarPasajes returns ≤ 20 passages of the current version of active
	// fuentes of the especialidad (or of every one), by relevance then id.
	BuscarPasajes(ctx context.Context, b Busqueda) ([]PasajeEncontrado, error)
	// ResolverCita does not filter by the fuente's state: a historic citation
	// returns the exact text of its version (escenario «Resolver cita histórica»).
	ResolverCita(ctx context.Context, c Cita) (PasajeCitado, error)
	// PublicarVersion publishes a 'procesando' version atomically: every
	// passage, the version 'lista', the fuente's current version and state,
	// and aud (fuente.procesar) in the same transaction.
	PublicarVersion(ctx context.Context, fuenteID int64, version int, pasajes []PasajeNuevo, aud Auditor) error
	// MarcarVersionFallida marks a 'procesando' version failed with motivo
	// and calls aud (fuente.fallar) in the same transaction.
	MarcarVersionFallida(ctx context.Context, fuenteID int64, version int, motivo string, aud Auditor) error
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

// NormalizarPagina applies the port's pagination without error: page ≤ 0 → 1,
// perPage ≤ 0 → 20, perPage > 100 → 100 (ADR 0003 § El puerto).
func NormalizarPagina(page, perPage int) (int, int) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = perPageDefecto
	}
	return page, min(perPage, perPageMax)
}

// Huella returns the hex sha256, the size in bytes and the lines of the
// content. It is the single implementation: the adapters and E2-03 (duplicate
// by hash) use it.
func Huella(contenido []byte) (sha string, tamanoBytes, lineas int) {
	suma := sha256.Sum256(contenido)
	return hex.EncodeToString(suma[:]), len(contenido), ContarLineas(contenido)
}

// Lineas splits the content into lines with the single line rule of ADR 0003
// § Normalización, paso 1, shared by Huella, the 20 000-line check of S1 and
// the normaliser (E2-05), so linea_inicio and linea_fin always fall in
// 1..len(Lineas): "\r\n", a lone "\r" and "\n" are one break each, a final
// break does not add an empty line, and empty content has no lines. The
// returned lines do not include their break.
func Lineas(contenido []byte) []string {
	var lineas []string
	inicio := 0
	for i := 0; i < len(contenido); i++ {
		switch contenido[i] {
		case '\n':
			lineas = append(lineas, string(contenido[inicio:i]))
			inicio = i + 1
		case '\r':
			lineas = append(lineas, string(contenido[inicio:i]))
			if i+1 < len(contenido) && contenido[i+1] == '\n' {
				i++
			}
			inicio = i + 1
		}
	}
	if inicio < len(contenido) {
		lineas = append(lineas, string(contenido[inicio:]))
	}
	return lineas
}

// ContarLineas is len(Lineas(contenido)).
func ContarLineas(contenido []byte) int {
	return len(Lineas(contenido))
}
