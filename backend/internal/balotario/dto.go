package balotario

import "time"

// Defaults and limits of the pagination of the problems (README §5, D11).
const (
	PerPageDefecto = 20
	PerPageMax     = 100
	MaxPagina      = 1_000_000
	// PreguntasVistaPrevia is how many valid questions the preview shows.
	PreguntasVistaPrevia = 20
)

// Review routes (informative: E3-05 recomputes it on the server, rule 7).
const (
	RutaEstructural       = "estructural"
	RutaVerificacionClave = "verificacion_clave"
)

// MotivoSinPasaje is added by the import, not by the parser (§3).
const MotivoSinPasaje = "sin_pasaje"

// Reasons why the preview cannot be confirmed.
const (
	NoConfirmableSinValidas  = CodigoSinValidas
	NoConfirmableYaImportado = CodigoYaImportado
)

// VistaPreviaResponse is the body of GET /admin/fuentes/{id}/balotario (§3).
type VistaPreviaResponse struct {
	Fuente              FuenteVista       `json:"fuente"`
	RutaRevision        string            `json:"ruta_revision"`
	Totales             TotalesVista      `json:"totales"`
	Preguntas           []PreguntaVista   `json:"preguntas"`
	Problemas           PaginaProblemas   `json:"problemas"`
	Importacion         *ImportacionVista `json:"importacion"`
	OtrasImportaciones  []OtraImportacion `json:"otras_importaciones"`
	PuedeConfirmar      bool              `json:"puede_confirmar"`
	MotivoNoConfirmable *string           `json:"motivo_no_confirmable"`
}

// FuenteVista never carries the file name, the hash or the content.
type FuenteVista struct {
	ID                int64   `json:"id"`
	Titulo            string  `json:"titulo"`
	Version           int     `json:"version"`
	OficialVerificada bool    `json:"oficial_verificada"`
	OficialEntidad    *string `json:"oficial_entidad"`
}

type TotalesVista struct {
	Validas      int `json:"validas"`
	ConProblemas int `json:"con_problemas"`
	Unica        int `json:"unica"`
	Multiple     int `json:"multiple"`
	ConAlertas   int `json:"con_alertas"`
}

type PreguntaVista struct {
	Numero            int                `json:"numero"`
	LineaInicio       int                `json:"linea_inicio"`
	LineaFin          int                `json:"linea_fin"`
	Tipo              string             `json:"tipo"`
	Enunciado         string             `json:"enunciado"`
	Alternativas      []AlternativaVista `json:"alternativas"`
	AlertasEstructura []string           `json:"alertas_estructura"`
}

type AlternativaVista struct {
	Letra    string `json:"letra"`
	Texto    string `json:"texto"`
	Correcta bool   `json:"correcta"`
}

// ProblemaVista carries the lines and the reasons, never the text: the admin
// finds it by the number of line.
type ProblemaVista struct {
	Numero      *int     `json:"numero"`
	LineaInicio int      `json:"linea_inicio"`
	LineaFin    int      `json:"linea_fin"`
	Motivos     []string `json:"motivos"`
}

type PaginaProblemas struct {
	Data    []ProblemaVista `json:"data"`
	Total   int             `json:"total"`
	Page    int             `json:"page"`
	PerPage int             `json:"per_page"`
}

// ImportacionVista is the import of the current version, if any.
type ImportacionVista struct {
	ID        int64     `json:"id"`
	Creadas   int       `json:"creadas"`
	CreatedAt time.Time `json:"created_at"`
}

// OtraImportacion is an import of another version of the same fuente.
type OtraImportacion struct {
	Version   int       `json:"version"`
	Creadas   int       `json:"creadas"`
	CreatedAt time.Time `json:"created_at"`
}

// ConfirmarRequest is the body of POST /admin/fuentes/{id}/balotario/importaciones
// (§4). AsignaturaID is the initial and provisional area of every question
// (option A of the PO): E3-05 asks for the confirmed one on approval.
type ConfirmarRequest struct {
	Version        int   `json:"version"`
	ProcesoID      int64 `json:"proceso_id"`
	EspecialidadID int64 `json:"especialidad_id"`
	AsignaturaID   int64 `json:"asignatura_id"`
}

// ConfirmarResponse is the 201 of the import.
type ConfirmarResponse struct {
	ImportacionID int64 `json:"importacion_id"`
	FuenteID      int64 `json:"fuente_id"`
	Version       int   `json:"version"`
	Creadas       int   `json:"creadas"`
	ConProblemas  int   `json:"con_problemas"`
	Unica         int   `json:"unica"`
	Multiple      int   `json:"multiple"`
	ConAlertas    int   `json:"con_alertas"`
}
