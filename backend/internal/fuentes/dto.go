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

// ── FuenteDetalle (S1, S2, and S4–S6 later; contract E2 § Objetos) ────────────

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

// FuenteDetalle is the admin view of a fuente (contract E2 § FuenteDetalle).
type FuenteDetalle struct {
	ID                 int64                  `json:"id"`
	Titulo             string                 `json:"titulo"`
	Tipo               string                 `json:"tipo"`
	Estado             string                 `json:"estado"`
	AplicaATodas       bool                   `json:"aplica_a_todas"`
	Especialidades     []EspecialidadRef      `json:"especialidades"`
	VersionVigente     *VersionVigenteResumen `json:"version_vigente"`
	VersionEnProceso   *VersionEnProceso      `json:"version_en_proceso"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
	Anio               *int                   `json:"anio"`
	Proceso            *ProcesoRef            `json:"proceso"`
	MotivoRetiro       *string                `json:"motivo_retiro"`
	Oficial            *Oficial               `json:"oficial"`
	ContenidosCitantes int                    `json:"contenidos_citantes"`
	Versiones          []VersionDetalle       `json:"versiones"`
}
