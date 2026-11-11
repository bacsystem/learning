package temas

import "time"

// Estados de un tema (SIA-E1-01 regla 4).
const (
	EstadoActivo    = "activo"
	EstadoArchivado = "archivado"
)

// ── Requests ──────────────────────────────────────────────────────────────────

// CrearTemaRequest is the body of POST /admin/temas.
type CrearTemaRequest struct {
	AsignaturaID    int64   `json:"asignatura_id"`
	Nombre          string  `json:"nombre"`
	Descripcion     *string `json:"descripcion"`
	EspecialidadIDs []int64 `json:"especialidad_ids"`
}

// EditarTemaRequest is the body of PUT /admin/temas/{id}. The asignatura is
// not editable: an asignatura_id in the body is an unknown field (400).
type EditarTemaRequest struct {
	Nombre          string  `json:"nombre"`
	Descripcion     *string `json:"descripcion"`
	EspecialidadIDs []int64 `json:"especialidad_ids"`
}

// CambiarEstadoRequest is the body of PATCH /admin/temas/{id}/estado.
type CambiarEstadoRequest struct {
	Estado string `json:"estado"`
}

// ReordenarRequest is the body of PUT /admin/asignaturas/{id}/temas/orden.
// A pointer tells a missing or null list (400) from an empty one.
type ReordenarRequest struct {
	TemaIDs *[]int64 `json:"tema_ids"`
}

// ── Responses ─────────────────────────────────────────────────────────────────

// EspecialidadRef is an especialidad of a tema.
type EspecialidadRef struct {
	ID     int64  `json:"id"`
	Nombre string `json:"nombre"`
}

// Tema is the admin view of a tema (T1–T5).
type Tema struct {
	ID             int64             `json:"id"`
	AsignaturaID   int64             `json:"asignatura_id"`
	Nombre         string            `json:"nombre"`
	Descripcion    *string           `json:"descripcion"`
	Estado         string            `json:"estado"`
	Orden          int               `json:"orden"`
	AplicaATodas   bool              `json:"aplica_a_todas"`
	Especialidades []EspecialidadRef `json:"especialidades"`
	PreguntasCount int               `json:"preguntas_count"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// ListaTemas is the paginated envelope of T1.
type ListaTemas struct {
	Data    []Tema `json:"data"`
	Total   int    `json:"total"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
}

// TemaActivo is the catalog view of T7, for any authenticated user.
type TemaActivo struct {
	ID              int64   `json:"id"`
	Nombre          string  `json:"nombre"`
	Descripcion     *string `json:"descripcion"`
	Orden           int     `json:"orden"`
	AplicaATodas    bool    `json:"aplica_a_todas"`
	EspecialidadIDs []int64 `json:"especialidad_ids"`
}

// PosicionTema is one row of the T6 response.
type PosicionTema struct {
	ID    int64 `json:"id"`
	Orden int   `json:"orden"`
}

// ── Domain inputs ─────────────────────────────────────────────────────────────

// Filtro selects the temas of T1. EspecialidadID 0 and Estado "" mean no filter.
type Filtro struct {
	AsignaturaID   int64
	EspecialidadID int64
	Estado         string
	Page           int
	PerPage        int
}

// DatosTema are the validated, normalised editable fields of a tema.
type DatosTema struct {
	Nombre          string
	NombreClave     string
	Descripcion     *string
	EspecialidadIDs []int64
}
