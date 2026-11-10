package practices

// ── Request structs ────────────────────────────────────────────────────────────

// PracticaAsignaturaItem is one subject entry in a create-practice request.
type PracticaAsignaturaItem struct {
	AsignaturaID    int64 `json:"asignatura_id"`
	NumeroPreguntas int   `json:"numero_preguntas"`
}

// CreatePracticaRequest is the payload for POST /practicas.
type CreatePracticaRequest struct {
	ProcesoID        int64                    `json:"proceso_id"`
	Asignaturas      []PracticaAsignaturaItem `json:"asignaturas"`
	TiempoLimiteSeg  *int                     `json:"tiempo_limite_seg"`
	ModoCalificacion string                   `json:"modo_calificacion"`
}

// ResponderPreguntaRequest is the payload for POST /practicas/:id/responder.
// For tipo=unica: replaces any prior selection. For tipo=multiple: toggles.
type ResponderPreguntaRequest struct {
	RespuestaID int64 `json:"respuesta_id"`
}

// FinalizarPracticaRequest is the payload for POST /practicas/:id/finalizar.
type FinalizarPracticaRequest struct {
	Abandonar bool `json:"abandonar"`
}

// BatchDeleteRequest is the payload for DELETE /practicas/batch.
type BatchDeleteRequest struct {
	IDs []int64 `json:"ids"`
}

// ── Response structs ───────────────────────────────────────────────────────────

// RespuestaPracticaItem is an answer option shown during active practice.
// MUST NOT include es_correcta — Constitution Principle III.
type RespuestaPracticaItem struct {
	ID    int64  `json:"id"`
	Texto string `json:"texto"`
	Letra string `json:"letra"`
	Orden int    `json:"orden"`
}

// PreguntaPracticaItem is one question in the active-practice view.
//
// Validada is true once the question was checked in inmediato mode: its
// selection is then locked (H74) and, since the key was already revealed by
// POST /validar, RespuestaCorrectaIDs and Explicacion are included so the
// locked state can be shown after a reload. They are omitted otherwise.
type PreguntaPracticaItem struct {
	ID                   int64                   `json:"id"`
	Orden                int                     `json:"orden"`
	Enunciado            string                  `json:"enunciado"`
	Tipo                 string                  `json:"tipo"`
	Asignatura           string                  `json:"asignatura"`
	Respuestas           []RespuestaPracticaItem `json:"respuestas"`
	Estado               string                  `json:"estado"`
	RespuestaIDs         []int64                 `json:"respuesta_ids"`
	Validada             bool                    `json:"validada"`
	RespuestaCorrectaIDs []int64                 `json:"respuesta_correcta_ids,omitempty"`
	Explicacion          string                  `json:"explicacion,omitempty"`
}

// PracticaEnCursoResponse is returned by POST /practicas and GET /practicas/:id.
type PracticaEnCursoResponse struct {
	ID               int64  `json:"id"`
	Proceso          string `json:"proceso"`
	ModoCalificacion string `json:"modo_calificacion"`
	TiempoLimiteSeg  *int   `json:"tiempo_limite_seg"`
	FechaInicio      string `json:"fecha_inicio"`
	// TiempoRestanteSeg is the time left computed with the DB clock (null when
	// untimed); the client derives its deadline from it, not from its own
	// mount time (H82).
	TiempoRestanteSeg *int                   `json:"tiempo_restante_seg"`
	TotalPreguntas    int                    `json:"total_preguntas"`
	Preguntas         []PreguntaPracticaItem `json:"preguntas"`
}

// ResponderResponse is returned by POST /practicas/:id/responder.
type ResponderResponse struct {
	RespuestaIDs []int64 `json:"respuesta_ids"`
}

// ValidarPreguntaResponse is returned by POST /practicas/:id/validar/:preguntaId.
type ValidarPreguntaResponse struct {
	EsCorrecta           bool    `json:"es_correcta"`
	RespuestaCorrectaID  int64   `json:"respuesta_correcta_id,omitempty"` // 'unica' only (legacy)
	RespuestaCorrectaIDs []int64 `json:"respuesta_correcta_ids"`
	Explicacion          string  `json:"explicacion"`
}

// ResultadoAsignaturaItem is the per-subject breakdown in PracticaResultadoResponse.
type ResultadoAsignaturaItem struct {
	Asignatura     string  `json:"asignatura"`
	TotalPreguntas int     `json:"total_preguntas"`
	Correctas      int     `json:"correctas"`
	Incorrectas    int     `json:"incorrectas"`
	SinResponder   int     `json:"sin_responder"`
	Porcentaje     float64 `json:"porcentaje"`
}

// PracticaResultadoResponse is returned by POST /practicas/:id/finalizar and GET /practicas/:id/resultado.
type PracticaResultadoResponse struct {
	ID              int64                     `json:"id"`
	Estado          string                    `json:"estado"`
	Proceso         string                    `json:"proceso"`
	TotalPreguntas  int                       `json:"total_preguntas"`
	Correctas       int                       `json:"correctas"`
	Incorrectas     int                       `json:"incorrectas"`
	SinResponder    int                       `json:"sin_responder"`
	Porcentaje      float64                   `json:"porcentaje"`
	TiempoUsadoSeg  int                       `json:"tiempo_usado_seg"`
	TiempoLimiteSeg *int                      `json:"tiempo_limite_seg"`
	FechaInicio     string                    `json:"fecha_inicio"`
	FechaFin        string                    `json:"fecha_fin"`
	PorAsignatura   []ResultadoAsignaturaItem `json:"por_asignatura"`
}

// RespuestaRevisionItem is an answer option in the revision view (includes es_correcta).
type RespuestaRevisionItem struct {
	ID         int64  `json:"id"`
	Texto      string `json:"texto"`
	Letra      string `json:"letra"`
	EsCorrecta bool   `json:"es_correcta"`
}

// RevisionPreguntaItem is one question in the revision view.
type RevisionPreguntaItem struct {
	Orden                int                     `json:"orden"`
	Enunciado            string                  `json:"enunciado"`
	Asignatura           string                  `json:"asignatura"`
	RespuestaUsuarioIDs  []int64                 `json:"respuesta_usuario_ids"`
	RespuestaCorrectaIDs []int64                 `json:"respuesta_correcta_ids"`
	EsCorrecta           *bool                   `json:"es_correcta"`
	Respuestas           []RespuestaRevisionItem `json:"respuestas"`
	Explicacion          string                  `json:"explicacion"`
	Referencia           string                  `json:"referencia"`
}

// RevisionPracticaResponse is returned by GET /practicas/:id/revision.
type RevisionPracticaResponse struct {
	ID        int64                  `json:"id"`
	Preguntas []RevisionPreguntaItem `json:"preguntas"`
}

// HistorialPracticaItem is one row in the paginated history.
type HistorialPracticaItem struct {
	ID             int64   `json:"id"`
	Numero         int     `json:"numero"`
	Asignaturas    string  `json:"asignaturas"`
	TotalPreguntas int     `json:"total_preguntas"`
	Correctas      int     `json:"correctas"`
	Porcentaje     float64 `json:"porcentaje"`
	TiempoUsadoSeg int     `json:"tiempo_usado_seg"`
	Fecha          string  `json:"fecha"`
	Estado         string  `json:"estado"`
}

// HistorialResponse is returned by GET /practicas.
type HistorialResponse struct {
	Data    []HistorialPracticaItem `json:"data"`
	Total   int                     `json:"total"`
	Page    int                     `json:"page"`
	PerPage int                     `json:"per_page"`
}
