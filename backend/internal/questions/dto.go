package questions

import "encoding/json"

// ── Request structs ────────────────────────────────────────────────────────────

// CreateRespuestaItem is one answer option in a create/update request.
type CreateRespuestaItem struct {
	// ID identifies an existing alternative on update (kept in place so attempts
	// that reference it are preserved). Omit it for new alternatives.
	ID         *int64 `json:"id,omitempty"`
	Texto      string `json:"texto"      validate:"required,min=1,max=500"`
	EsCorrecta bool   `json:"es_correcta"`
	Orden      int    `json:"orden"      validate:"required,min=1"`
}

// CreateRetroRequest is the optional feedback block in a create/update request.
type CreateRetroRequest struct {
	Explicacion string  `json:"explicacion" validate:"required,min=10,max=2000"`
	Referencia  *string `json:"referencia"`
}

// CreatePreguntaRequest is the payload for POST /preguntas.
type CreatePreguntaRequest struct {
	ProcesoID         int64                 `json:"proceso_id"      validate:"required"`
	AsignaturaID      int64                 `json:"asignatura_id"   validate:"required"`
	GradoID           *int64                `json:"grado_id"`
	EspecialidadID    *int64                `json:"especialidad_id"`
	TemaID            *int64                `json:"tema_id"` // optional main tema (SIA-E1-01, T8)
	Enunciado         string                `json:"enunciado"       validate:"required,min=10,max=2000"`
	Tipo              string                `json:"tipo"            validate:"required,oneof=unica multiple"`
	Dificultad        string                `json:"dificultad"      validate:"required,oneof=facil medio dificil"`
	EtiquetaIDs       []int64               `json:"etiqueta_ids"`
	Respuestas        []CreateRespuestaItem `json:"respuestas"     validate:"required,min=2,max=6,dive"`
	Retroalimentacion *CreateRetroRequest   `json:"retroalimentacion"`
}

// UpdatePreguntaRequest is the payload for PUT /preguntas/{id}.
// proceso_id is intentionally absent — it's immutable after creation.
// etiqueta_ids replaces the question's tags; when absent (null) they are kept
// and an explicit [] clears them (H100). tema_id follows the same idea (see
// OptionalID and temaTrasEditar).
type UpdatePreguntaRequest struct {
	ProcesoID         *int64                `json:"proceso_id"` // checked for immutability
	AsignaturaID      int64                 `json:"asignatura_id"   validate:"required"`
	GradoID           *int64                `json:"grado_id"`
	EspecialidadID    *int64                `json:"especialidad_id"`
	TemaID            OptionalID            `json:"tema_id"`
	Enunciado         string                `json:"enunciado"       validate:"required,min=10,max=2000"`
	Tipo              string                `json:"tipo"            validate:"required,oneof=unica multiple"`
	Dificultad        string                `json:"dificultad"      validate:"required,oneof=facil medio dificil"`
	EtiquetaIDs       []int64               `json:"etiqueta_ids"`
	Respuestas        []CreateRespuestaItem `json:"respuestas"     validate:"required,min=2,max=6,dive"`
	Retroalimentacion *CreateRetroRequest   `json:"retroalimentacion"`
}

// OptionalID is a nullable id that also records whether the JSON field was
// sent: absent (Set false), null (Set true, Value nil) or a number.
type OptionalID struct {
	Set   bool
	Value *int64
}

// UnmarshalJSON runs only when the field is present, null included.
func (o *OptionalID) UnmarshalJSON(b []byte) error {
	o.Set = true
	o.Value = nil
	if string(b) == "null" {
		return nil
	}
	var v int64
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	o.Value = &v
	return nil
}

// CreateEtiquetaRequest is the payload for POST /etiquetas.
type CreateEtiquetaRequest struct {
	AsignaturaID int64  `json:"asignatura_id" validate:"required"`
	Nombre       string `json:"nombre"        validate:"required,min=1,max=100"`
}

// UpdateEtiquetaRequest is the payload for PUT /etiquetas/{id}.
type UpdateEtiquetaRequest struct {
	Nombre string `json:"nombre" validate:"required,min=1,max=100"`
	Estado string `json:"estado" validate:"omitempty,oneof=activo inactivo"`
}

// GenerateSetRequest is the payload for POST /preguntas/generate-set.
type GenerateSetRequest struct {
	ProcesoID      int64               `json:"proceso_id"      validate:"required"`
	GradoID        *int64              `json:"grado_id"`
	EspecialidadID *int64              `json:"especialidad_id"`
	Asignaturas    []AsignaturaSetItem `json:"asignaturas"    validate:"required,min=1,dive"`
	ExcluirIDs     []int64             `json:"excluir_ids"`
}

// AsignaturaSetItem is one subject entry in a generate-set request.
type AsignaturaSetItem struct {
	AsignaturaID    int64  `json:"asignatura_id"    validate:"required"`
	NumeroPreguntas int    `json:"numero_preguntas" validate:"required,min=1"`
	Dificultad      string `json:"dificultad"       validate:"omitempty,oneof=facil medio dificil"`
}

// ── Response structs ───────────────────────────────────────────────────────────

// CitaResponse is one citation of a question: the passage and the fuente
// version it belongs to (contract E3-02 §4.4); the passage text is resolved
// through the fuentes port.
type CitaResponse struct {
	PasajeID     int64  `json:"pasaje_id"`
	FuenteID     int64  `json:"fuente_id"`
	Version      int    `json:"version"`
	FuenteTitulo string `json:"fuente_titulo"`
	FuenteEstado string `json:"fuente_estado"`
}

// RespuestaResponse is an answer in the admin detail view (includes es_correcta).
type RespuestaResponse struct {
	ID         int64  `json:"id"`
	Texto      string `json:"texto"`
	EsCorrecta bool   `json:"es_correcta"`
	Orden      int    `json:"orden"`
}

// RetroalimentacionResponse is the optional feedback in a PreguntaResponse.
type RetroalimentacionResponse struct {
	Explicacion string  `json:"explicacion"`
	Referencia  *string `json:"referencia"`
}

// EstadisticaResponse holds aggregated stats for a single question.
type EstadisticaResponse struct {
	TotalIntentos        int     `json:"total_intentos"`
	TotalAciertos        int     `json:"total_aciertos"`
	IndiceDificultad     float64 `json:"indice_dificultad"`
	IndiceDiscriminacion float64 `json:"indice_discriminacion"`
}

// PreguntaResponse is the full admin view of a question.
type PreguntaResponse struct {
	ID                int64                      `json:"id"`
	ProcesoID         int64                      `json:"proceso_id"`
	Proceso           string                     `json:"proceso"`
	AsignaturaID      int64                      `json:"asignatura_id"`
	Asignatura        string                     `json:"asignatura"`
	GradoID           *int64                     `json:"grado_id"` // H20: the edit form preselects by id, names are not unique
	Grado             *string                    `json:"grado"`
	EspecialidadID    *int64                     `json:"especialidad_id"`
	Especialidad      *string                    `json:"especialidad"`
	TemaID            *int64                     `json:"tema_id"`
	Tema              *string                    `json:"tema"`
	Enunciado         string                     `json:"enunciado"`
	Tipo              string                     `json:"tipo"`
	Dificultad        string                     `json:"dificultad"`
	Estado            string                     `json:"estado"`
	Origen            string                     `json:"origen"`                // manual | balotario | ia (SIA-E3-02)
	MotivoRechazo     *string                    `json:"motivo_rechazo"`        // set only while rechazada
	Citas             *[]CitaResponse            `json:"citas,omitempty"`       // detail views only
	CitasCount        *int                       `json:"citas_count,omitempty"` // list view only
	Etiquetas         []string                   `json:"etiquetas"`
	EtiquetaIDs       []int64                    `json:"etiqueta_ids"` // ids of Etiquetas (H100); empty in list views
	Respuestas        []RespuestaResponse        `json:"respuestas"`
	Retroalimentacion *RetroalimentacionResponse `json:"retroalimentacion"`
	Estadisticas      *EstadisticaResponse       `json:"estadisticas"`
	CreatedAt         string                     `json:"created_at"`
}

// PreguntaListResponse wraps a paginated list of questions.
type PreguntaListResponse struct {
	Data    []PreguntaResponse `json:"data"`
	Total   int                `json:"total"`
	Page    int                `json:"page"`
	PerPage int                `json:"per_page"`
}

// EtiquetaResponse is a tag in the admin view.
type EtiquetaResponse struct {
	ID           int64  `json:"id"`
	AsignaturaID int64  `json:"asignatura_id"`
	Asignatura   string `json:"asignatura"`
	Nombre       string `json:"nombre"`
	Estado       string `json:"estado"`
}

// RespuestaExamen is an answer in the exam view — NEVER includes EsCorrecta.
type RespuestaExamen struct {
	ID    int64  `json:"id"`
	Texto string `json:"texto"`
	Orden int    `json:"orden"`
}

// PreguntaExamenItem is one question in the exam set — no answer keys.
type PreguntaExamenItem struct {
	ID         int64  `json:"id"`
	Enunciado  string `json:"enunciado"`
	Tipo       string `json:"tipo"`
	Asignatura string `json:"asignatura"`
	// AsignaturaID is the asignatura the question was selected for; exam
	// modules store it as the attempt's snapshot instead of re-reading the
	// question, which may have moved since the selection (H110).
	AsignaturaID int64             `json:"asignatura_id"`
	Respuestas   []RespuestaExamen `json:"respuestas"`
	Orden        int               `json:"orden"`
}

// GenerateSetResponse is the response for POST /preguntas/generate-set.
type GenerateSetResponse struct {
	Preguntas []PreguntaExamenItem `json:"preguntas"`
	Total     int                  `json:"total"`
}

// ImportErrorRow describes one row error in a bulk import.
type ImportErrorRow struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// ImportResultResponse is the response for POST /preguntas/import.
type ImportResultResponse struct {
	Imported int              `json:"imported"`
	Errors   []ImportErrorRow `json:"errors"`
}

// PreviewRowResponse represents one parsed row returned by POST /preguntas/preview.
type PreviewRowResponse struct {
	Row            int    `json:"row"`
	Enunciado      string `json:"enunciado"`
	Tipo           string `json:"tipo"`
	Dificultad     string `json:"dificultad"`
	GradoID        *int64 `json:"grado_id"`
	EspecialidadID *int64 `json:"especialidad_id"`
	NumRespuestas  int    `json:"num_respuestas"`
	HasRetro       bool   `json:"has_retro"`
	Estado         string `json:"estado"` // "valido" | "error"
	Error          string `json:"error,omitempty"`
}

// PreviewResponse is the response for POST /preguntas/preview.
type PreviewResponse struct {
	Rows        []PreviewRowResponse `json:"rows"`
	TotalValido int                  `json:"total_valido"`
	TotalError  int                  `json:"total_error"`
}

// PorProcesoItem is one row in the por_proceso breakdown of stats.
type PorProcesoItem struct {
	Proceso string `json:"proceso"`
	Total   int    `json:"total"`
}

// PorDificultad breakdown in stats.
type PorDificultad struct {
	Facil   int `json:"facil"`
	Medio   int `json:"medio"`
	Dificil int `json:"dificil"`
}

// PorTipo breakdown in stats.
type PorTipo struct {
	Unica    int `json:"unica"`
	Multiple int `json:"multiple"`
}

// StatsGlobalesResponse is the response for GET /preguntas/stats.
type StatsGlobalesResponse struct {
	TotalPreguntas int              `json:"total_preguntas"`
	PorDificultad  PorDificultad    `json:"por_dificultad"`
	PorTipo        PorTipo          `json:"por_tipo"`
	PorProceso     []PorProcesoItem `json:"por_proceso"`
}

// ListQuery holds parsed filter and pagination parameters for listing questions.
type ListQuery struct {
	ProcesoID      int64
	AsignaturaID   int64
	GradoID        int64
	EspecialidadID int64
	Dificultad     string
	EtiquetaID     int64
	TemaID         int64
	Estado         string // exact; one of EstadosListables
	Origen         string // exact; one of Origenes
	Search         string
	Page           int
	PerPage        int
}
