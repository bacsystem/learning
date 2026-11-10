package testimonios

// SubmitRequest is the body for POST /testimonios.
type SubmitRequest struct {
	Contenido    string `json:"contenido"    validate:"required,min=10,max=300"`
	Calificacion int    `json:"calificacion" validate:"required,min=1,max=5"`
}

// UpdateEstadoRequest is the body for PATCH /admin/testimonios/{id}/estado.
type UpdateEstadoRequest struct {
	Estado string `json:"estado" validate:"required,oneof=aprobado rechazado"`
}

// TestimonioPublicoDTO is returned by GET /testimonios/publico. It is shown to
// anonymous visitors, so it carries only "Nombre I." (first name + initial of
// the apellido paterno) and no grado or other identifying data (H80).
type TestimonioPublicoDTO struct {
	ID            int64  `json:"id"`
	NombrePublico string `json:"nombre_publico"`
	Calificacion  int    `json:"calificacion"`
	Contenido     string `json:"contenido"`
	CreatedAt     string `json:"created_at"`
}

// MiTestimonioDTO is returned by GET /testimonios/me.
type MiTestimonioDTO struct {
	ID           int64  `json:"id"`
	Calificacion int    `json:"calificacion"`
	Contenido    string `json:"contenido"`
	Estado       string `json:"estado"`
	CreatedAt    string `json:"created_at"`
}

// TestimonioAdminDTO is an item in the admin list.
type TestimonioAdminDTO struct {
	ID             int64  `json:"id"`
	UsuarioID      int64  `json:"usuario_id"`
	Username       string `json:"username"`
	NombreCompleto string `json:"nombre_completo"`
	Grado          string `json:"grado"`
	Calificacion   int    `json:"calificacion"`
	Contenido      string `json:"contenido"`
	Estado         string `json:"estado"`
	CreatedAt      string `json:"created_at"`
}

// AdminListResponse wraps the admin list with pagination.
type AdminListResponse struct {
	Data    []TestimonioAdminDTO `json:"data"`
	Total   int64                `json:"total"`
	Page    int                  `json:"page"`
	PerPage int                  `json:"per_page"`
}
