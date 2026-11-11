package fuentes

import "time"

// ── Upload inputs ─────────────────────────────────────────────────────────────

// ArchivoSubido is the uploaded file as the handler received it.
type ArchivoSubido struct {
	Nombre      string
	ContentType string
	Contenido   []byte
}

// FormularioFuente holds the raw metadata fields of S1, validated by the Service.
type FormularioFuente struct {
	Titulo          string
	Tipo            string
	Anio            string
	ProcesoID       string
	EspecialidadIDs []string
}

// ── FuenteResumen and FuenteDetalle (S1–S6; contract E2 § Objetos) ───────────

// EspecialidadRef is an especialidad of a fuente.
type EspecialidadRef struct {
	ID     int64  `json:"id" db:"id"`
	Nombre string `json:"nombre" db:"nombre"`
}

// ProcesoRef is the proceso of a fuente.
type ProcesoRef struct {
	ID     int64  `json:"id"`
	Nombre string `json:"nombre"`
}

// VersionVigenteResumen summarises the current version.
type VersionVigenteResumen struct {
	Numero            int  `json:"numero"`
	OficialVerificada bool `json:"oficial_verificada"`
	PasajesCount      int  `json:"pasajes_count"`
}

// VersionEnProceso is the last version after the current one that is not 'lista'.
type VersionEnProceso struct {
	Numero      int     `json:"numero"`
	Estado      string  `json:"estado"`
	MotivoFallo *string `json:"motivo_fallo"`
}

// Oficial is the «oficial verificada» mark of the current version.
type Oficial struct {
	EntidadEmisora string    `json:"entidad_emisora"`
	Nota           string    `json:"nota"`
	MarcadoAt      time.Time `json:"marcado_at"`
}

// VersionDetalle is one version of a fuente.
type VersionDetalle struct {
	Numero            int        `json:"numero" db:"numero"`
	Estado            string     `json:"estado" db:"estado"`
	MotivoFallo       *string    `json:"motivo_fallo" db:"motivo_fallo"`
	NombreArchivo     string     `json:"nombre_archivo" db:"nombre_archivo"`
	Sha256            string     `json:"sha256" db:"sha256"`
	TamanoBytes       int        `json:"tamano_bytes" db:"tamano_bytes"`
	Lineas            int        `json:"lineas" db:"lineas"`
	PasajesCount      int        `json:"pasajes_count" db:"pasajes_count"`
	OficialVerificada bool       `json:"oficial_verificada" db:"oficial_verificada"`
	OficialEntidad    *string    `json:"oficial_entidad" db:"oficial_entidad"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	ProcesadaAt       *time.Time `json:"procesada_at" db:"procesada_at"`
}

// ResumenFuente is one row of the admin listing (contract E2 § FuenteResumen,
// S3). It is not the port's FuenteResumen, which carries ids only.
type ResumenFuente struct {
	ID               int64                  `json:"id"`
	Titulo           string                 `json:"titulo"`
	Tipo             string                 `json:"tipo"`
	Estado           string                 `json:"estado"`
	AplicaATodas     bool                   `json:"aplica_a_todas"`
	Especialidades   []EspecialidadRef      `json:"especialidades"`
	VersionVigente   *VersionVigenteResumen `json:"version_vigente"`
	VersionEnProceso *VersionEnProceso      `json:"version_en_proceso"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

// FuenteDetalle is the admin view of a fuente: its ResumenFuente plus the
// fields of contract E2 § FuenteDetalle.
type FuenteDetalle struct {
	ResumenFuente
	Anio               *int             `json:"anio"`
	Proceso            *ProcesoRef      `json:"proceso"`
	MotivoRetiro       *string          `json:"motivo_retiro"`
	Oficial            *Oficial         `json:"oficial"`
	ContenidosCitantes int              `json:"contenidos_citantes"`
	Versiones          []VersionDetalle `json:"versiones"`
	// ContenidosMarcados is only in the answer of a retirement (S5), where 0
	// is still sent; nil leaves it out of every other answer.
	ContenidosMarcados *int `json:"contenidos_marcados,omitempty"`
}

// ── Admin changes (S5–S7) ─────────────────────────────────────────────────────

// CambioEstadoRequest is the body of S5: retirar (with motivo), restaurar
// (activa) or reintentar (procesando).
type CambioEstadoRequest struct {
	Estado string  `json:"estado"`
	Motivo *string `json:"motivo"`
}

// OficialRequest is the body of S6: mark with entidad and nota, or unmark
// with motivo.
type OficialRequest struct {
	OficialVerificada *bool   `json:"oficial_verificada"`
	EntidadEmisora    *string `json:"entidad_emisora"`
	Nota              *string `json:"nota"`
	Motivo            *string `json:"motivo"`
}

// EditarRequest is the body of S7: it replaces both fields; an empty
// especialidad_ids means every especialidad, a missing one is invalid.
type EditarRequest struct {
	Titulo          string   `json:"titulo"`
	EspecialidadIDs *[]int64 `json:"especialidad_ids"`
}

// ── Listing (S3) ──────────────────────────────────────────────────────────────

// FiltroListado is the S3 query once parsed; zero values mean no filter.
type FiltroListado struct {
	Estado         string
	Tipo           string
	EspecialidadID int64
	Oficial        *bool // mark of the current version; a fuente without one is not marked
	Q              string
	Page, PerPage  int
}

// ListaFuentes is the S3 page (docs/api/README.md §5).
type ListaFuentes struct {
	Data    []ResumenFuente `json:"data"`
	Total   int             `json:"total"`
	Page    int             `json:"page"`
	PerPage int             `json:"per_page"`
}
