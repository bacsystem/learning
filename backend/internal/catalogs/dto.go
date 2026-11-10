package catalogs

// ListQuery holds pagination and filter options for catalog list endpoints.
type ListQuery struct {
	Page          int
	Limit         int // 0 = all
	Estado        string
	InstitucionID int64
	Tipo          string
	Anio          int
	Nivel         string
}

// EstadoRequest is used for PATCH /*/estado endpoints.
type EstadoRequest struct {
	Estado string `json:"estado" validate:"required,oneof=activo inactivo"`
}

// ── Instituciones ──────────────────────────────────────────────────────────────

type CreateInstitucionRequest struct {
	Nombre string `json:"nombre" validate:"required,min=3,max=150"`
	Siglas string `json:"siglas" validate:"required,min=2,max=20"`
}

type UpdateInstitucionRequest = CreateInstitucionRequest

type InstitucionResponse struct {
	ID        int64  `json:"id"`
	Nombre    string `json:"nombre"`
	Siglas    string `json:"siglas"`
	Estado    string `json:"estado"`
	CreatedAt string `json:"created_at"`
}

// ── Grados ────────────────────────────────────────────────────────────────────

type CreateGradoRequest struct {
	InstitucionID int64  `json:"institucion_id" validate:"required"`
	Nombre        string `json:"nombre" validate:"required,min=2,max=100"`
	Abreviatura   string `json:"abreviatura" validate:"required,min=2,max=20"`
	Nivel         string `json:"nivel" validate:"required,oneof=suboficial oficial"`
	Orden         int    `json:"orden" validate:"required,min=1"`
}

type GradoResponse struct {
	ID            int64  `json:"id"`
	InstitucionID int64  `json:"institucion_id"`
	Institucion   string `json:"institucion"`
	Nombre        string `json:"nombre"`
	Abreviatura   string `json:"abreviatura"`
	Nivel         string `json:"nivel"`
	Orden         int    `json:"orden"`
	Estado        string `json:"estado"`
	CreatedAt     string `json:"created_at"`
}

// ── Especialidades ────────────────────────────────────────────────────────────

type CreateEspecialidadRequest struct {
	InstitucionID int64  `json:"institucion_id" validate:"required"`
	Nombre        string `json:"nombre" validate:"required,min=3,max=150"`
	Codigo        string `json:"codigo" validate:"required,min=2,max=20"`
	Nivel         string `json:"nivel" validate:"omitempty,oneof=suboficial oficial"`
}

type EspecialidadResponse struct {
	ID            int64  `json:"id"`
	InstitucionID int64  `json:"institucion_id"`
	Institucion   string `json:"institucion"`
	Nombre        string `json:"nombre"`
	Codigo        string `json:"codigo"`
	Nivel         string `json:"nivel"`
	Estado        string `json:"estado"`
	CreatedAt     string `json:"created_at"`
}

// ── Asignaturas ───────────────────────────────────────────────────────────────

type CreateAsignaturaRequest struct {
	Nombre string `json:"nombre" validate:"required,min=3,max=100"`
	Codigo string `json:"codigo" validate:"required,min=2,max=20"`
}

type AsignaturaResponse struct {
	ID        int64  `json:"id"`
	Nombre    string `json:"nombre"`
	Codigo    string `json:"codigo"`
	Estado    string `json:"estado"`
	CreatedAt string `json:"created_at"`
}

// ── Procesos ──────────────────────────────────────────────────────────────────

type CreateProcesoRequest struct {
	InstitucionID int64  `json:"institucion_id" validate:"required"`
	Nombre        string `json:"nombre" validate:"required,min=3,max=150"`
	Tipo          string `json:"tipo" validate:"required,oneof=ascenso pase_efectividad admision"`
	GradoID       *int64 `json:"grado_id"`
	Descripcion   string `json:"descripcion" validate:"max=500"`
	Anio          int    `json:"anio" validate:"required,min=2024,max=2099"`
}

type ProcesoResponse struct {
	ID            int64   `json:"id"`
	InstitucionID int64   `json:"institucion_id"`
	Institucion   string  `json:"institucion"`
	Nombre        string  `json:"nombre"`
	Tipo          string  `json:"tipo"`
	GradoID       *int64  `json:"grado_id,omitempty"`
	Grado         *string `json:"grado,omitempty"`
	Descripcion   string  `json:"descripcion"`
	Anio          int     `json:"anio"`
	Estado        string  `json:"estado"`
	CreatedAt     string  `json:"created_at"`
}

type ProcesoDetailResponse struct {
	ProcesoResponse
	Asignaturas []ProcesoAsignaturaItem `json:"asignaturas"`
}

type ProcesoAsignaturaItem struct {
	AsignaturaID         int64   `json:"asignatura_id"`
	Nombre               string  `json:"nombre"`
	Codigo               string  `json:"codigo"`
	NumeroPreguntas      int     `json:"numero_preguntas"`
	PesoPorcentual       float64 `json:"peso_porcentual"`
	Coeficiente          float64 `json:"coeficiente"`
	PreguntasDisponibles int     `json:"preguntas_disponibles"`
}

// ── Proceso-Asignaturas ───────────────────────────────────────────────────────

type AsignarAsignaturaRequest struct {
	AsignaturaID    int64   `json:"asignatura_id" validate:"required"`
	NumeroPreguntas int     `json:"numero_preguntas" validate:"required,min=1,max=200"`
	PesoPorcentual  float64 `json:"peso_porcentual" validate:"min=0.01,max=100,dec2"` // NUMERIC(5,2)
	Coeficiente     float64 `json:"coeficiente" validate:"min=0.01,lte=999.99,dec2"`  // DECIMAL(5,2)
}

type UpdateProcesoAsignaturaRequest struct {
	NumeroPreguntas int     `json:"numero_preguntas" validate:"required,min=1,max=200"`
	PesoPorcentual  float64 `json:"peso_porcentual" validate:"min=0.01,max=100,dec2"` // NUMERIC(5,2)
	Coeficiente     float64 `json:"coeficiente" validate:"min=0.01,lte=999.99,dec2"`  // DECIMAL(5,2)
}
