package simulations

// ── Request structs ────────────────────────────────────────────────────────────

// CreateSimulacroRequest is the payload for POST /simulacros.
type CreateSimulacroRequest struct {
	ProcesoID int64 `json:"proceso_id" validate:"required"`
}

// ResponderSimulacroRequest is the payload for POST /simulacros/:id/responder.
type ResponderSimulacroRequest struct {
	PreguntaID  int64 `json:"pregunta_id" validate:"required"`
	RespuestaID int64 `json:"respuesta_id" validate:"required"`
}

// SetRespuestasRequest is the payload for PUT /simulacros/:id/preguntas/:preguntaId/respuestas.
type SetRespuestasRequest struct {
	RespuestaIDs []int64 `json:"respuesta_ids"`
}

// SeleccionResponse returns the current selection of a question.
type SeleccionResponse struct {
	RespuestaIDs []int64 `json:"respuesta_ids"`
}

// ── Response structs ───────────────────────────────────────────────────────────

// RespuestaSimulacroItem is an answer option shown during an active simulacro.
// MUST NOT include es_correcta — Constitution Principle III.
type RespuestaSimulacroItem struct {
	ID    int64  `json:"id"`
	Texto string `json:"texto"`
	Letra string `json:"letra"`
	Orden int    `json:"orden"`
}

// PreguntaSimulacroItem is one question in the active simulacro view.
type PreguntaSimulacroItem struct {
	ID         int64                    `json:"id"`
	Orden      int                      `json:"orden"`
	Enunciado  string                   `json:"enunciado"`
	Tipo       string                   `json:"tipo"`
	Asignatura string                   `json:"asignatura"`
	Respuestas []RespuestaSimulacroItem `json:"respuestas"`
	Estado     string                   `json:"estado"` // sin_responder | respondida
	// RespuestaID is the first selected alternative (kept for 'unica' clients).
	RespuestaID  *int64  `json:"respuesta_id"`
	RespuestaIDs []int64 `json:"respuesta_ids"`
	Marcada      bool    `json:"marcada"`
}

// SimulacroEnCursoResponse is returned by GET /simulacros/:id.
type SimulacroEnCursoResponse struct {
	ID              int64                   `json:"id"`
	Proceso         string                  `json:"proceso"`
	TiempoLimiteSeg int                     `json:"tiempo_limite_seg"`
	FechaInicio     string                  `json:"fecha_inicio"`
	TiempoRestante  int                     `json:"tiempo_restante_seg"`
	TotalPreguntas  int                     `json:"total_preguntas"`
	Preguntas       []PreguntaSimulacroItem `json:"preguntas"`
}

// ResultadoAsignaturaSimulacro is the per-subject breakdown in SimulacroResultadoResponse.
type ResultadoAsignaturaSimulacro struct {
	Asignatura     string  `json:"asignatura"`
	TotalPreguntas int     `json:"total_preguntas"`
	Correctas      int     `json:"correctas"`
	Incorrectas    int     `json:"incorrectas"`
	SinResponder   int     `json:"sin_responder"`
	Puntaje        float64 `json:"puntaje"`
	Porcentaje     float64 `json:"porcentaje"`
	Peso           float64 `json:"peso"`
}

// PosicionEstimada is the estimated ranking for the user.
type PosicionEstimada struct {
	Posicion      int `json:"posicion"`
	TotalUsuarios int `json:"total_usuarios"`
}

// SimulacroResultadoResponse is returned by GET /simulacros/:id/resultado.
type SimulacroResultadoResponse struct {
	ID              int64                          `json:"id"`
	Proceso         string                         `json:"proceso"`
	TotalPreguntas  int                            `json:"total_preguntas"`
	Correctas       int                            `json:"correctas"`
	Incorrectas     int                            `json:"incorrectas"`
	SinResponder    int                            `json:"sin_responder"`
	PuntajeTotal    float64                        `json:"puntaje_total"`
	PuntajeMaximo   float64                        `json:"puntaje_maximo"`
	Porcentaje      float64                        `json:"porcentaje"` // puntaje_total / puntaje_maximo * 100
	NotaCentesimal  float64                        `json:"nota_centesimal"`
	Aprobado        bool                           `json:"aprobado"`
	TiempoUsadoSeg  int                            `json:"tiempo_usado_seg"`
	TiempoLimiteSeg int                            `json:"tiempo_limite_seg"`
	FechaInicio     string                         `json:"fecha_inicio"`
	FechaFin        string                         `json:"fecha_fin"`
	PorAsignatura   []ResultadoAsignaturaSimulacro `json:"por_asignatura"`
	Posicion        *PosicionEstimada              `json:"posicion,omitempty"`
}

// RespuestaRevisionSimItem is an answer in revision view (includes es_correcta).
type RespuestaRevisionSimItem struct {
	ID         int64  `json:"id"`
	Texto      string `json:"texto"`
	Letra      string `json:"letra"`
	EsCorrecta bool   `json:"es_correcta"`
}

// RevisionPreguntaSimulacro is one question in the revision view.
type RevisionPreguntaSimulacro struct {
	PreguntaID           int64   `json:"pregunta_id"`
	Tipo                 string  `json:"tipo"`
	RespuestaUsuarioIDs  []int64 `json:"respuesta_usuario_ids"`
	RespuestaCorrectaIDs []int64 `json:"respuesta_correcta_ids"`
	Orden                int     `json:"orden"`
	Enunciado            string  `json:"enunciado"`
	Asignatura           string  `json:"asignatura"`
	// RespuestaUsuarioID / RespuestaCorrectaID: first id of each set (legacy 'unica' clients).
	RespuestaUsuarioID  *int64                     `json:"respuesta_usuario_id"`
	RespuestaCorrectaID int64                      `json:"respuesta_correcta_id"`
	EsCorrecta          *bool                      `json:"es_correcta"`
	Respuestas          []RespuestaRevisionSimItem `json:"respuestas"`
	Explicacion         string                     `json:"explicacion,omitempty"`
	Referencia          string                     `json:"referencia,omitempty"`
}

// RevisionSimulacroResponse is returned by GET /simulacros/:id/revision.
type RevisionSimulacroResponse struct {
	ID        int64                       `json:"id"`
	Preguntas []RevisionPreguntaSimulacro `json:"preguntas"`
}

// HistorialSimulacroItem is one row in the paginated simulacro history.
type HistorialSimulacroItem struct {
	ID             int64   `json:"id"`
	Numero         int     `json:"numero"`
	Proceso        string  `json:"proceso"`
	Fecha          string  `json:"fecha"`
	PuntajeTotal   float64 `json:"puntaje_total"`
	PuntajeMaximo  float64 `json:"puntaje_maximo"`
	Porcentaje     float64 `json:"porcentaje"` // puntaje_total / puntaje_maximo * 100
	NotaCentesimal float64 `json:"nota_centesimal"`
	Aprobado       bool    `json:"aprobado"`
	TiempoUsadoSeg int     `json:"tiempo_usado_seg"`
	Estado         string  `json:"estado"`
}

// HistorialSimulacroResponse is returned by GET /simulacros/historial.
type HistorialSimulacroResponse struct {
	Data    []HistorialSimulacroItem `json:"data"`
	Total   int                      `json:"total"`
	Page    int                      `json:"page"`
	PerPage int                      `json:"per_page"`
}

// HistorialFiltros are optional filters for the historial query.
type HistorialFiltros struct {
	ProcesoID *int64
	Aprobado  *bool
}

// ── Configuración Examen DTOs ─────────────────────────────────────────────────

// CreateConfiguracionRequest is the payload for POST /simulacros/configuracion.
// Absent nota_minima/nota_maxima default to DefaultNotaMinima/DefaultNotaMaxima
// (an explicit 0 is kept).
type CreateConfiguracionRequest struct {
	ProcesoID      int64    `json:"proceso_id" validate:"required"`
	TiempoTotalSeg int      `json:"tiempo_total_seg" validate:"required,min=60"`
	TotalPreguntas int      `json:"total_preguntas" validate:"required,min=1"`
	NotaMinima     *float64 `json:"nota_minima"`
	NotaMaxima     *float64 `json:"nota_maxima"`
	Descripcion    string   `json:"descripcion"`
}

// AsignaturaConfigItem is one subject in the simulacro with its available question count for the user.
type AsignaturaConfigItem struct {
	AsignaturaID         int64   `json:"asignatura_id"`
	Nombre               string  `json:"nombre"`
	NumeroPreguntas      int     `json:"numero_preguntas"`
	PesoPorcentual       float64 `json:"peso_porcentual"`
	PreguntasDisponibles int     `json:"preguntas_disponibles"`
}

// ConfiguracionResponse is returned by GET /simulacros/configuracion and POST /simulacros/configuracion.
type ConfiguracionResponse struct {
	ID             int64                  `json:"id"`
	ProcesoID      int64                  `json:"proceso_id"`
	Proceso        string                 `json:"proceso"`
	TiempoTotalSeg int                    `json:"tiempo_total_seg"`
	TotalPreguntas int                    `json:"total_preguntas"`
	NotaMinima     float64                `json:"nota_minima"`
	NotaMaxima     float64                `json:"nota_maxima"`
	Descripcion    string                 `json:"descripcion,omitempty"`
	Estado         string                 `json:"estado"`
	Asignaturas    []AsignaturaConfigItem `json:"asignaturas"`
}

// ConfiguracionListResponse is returned by GET /simulacros/configuraciones.
type ConfiguracionListResponse struct {
	Data []ConfiguracionResponse `json:"data"`
}

// UpdateConfiguracionRequest is the payload for PATCH /simulacros/configuracion/{id}.
// Absent fields keep their current value.
type UpdateConfiguracionRequest struct {
	TiempoTotalSeg *int     `json:"tiempo_total_seg"`
	TotalPreguntas *int     `json:"total_preguntas"`
	NotaMinima     *float64 `json:"nota_minima"`
	NotaMaxima     *float64 `json:"nota_maxima"`
	Descripcion    *string  `json:"descripcion"`
	Estado         *string  `json:"estado"`
}
