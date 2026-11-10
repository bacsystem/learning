package auth

// ── Register ────────────────────────────────────────────────────────────────

// RegisterRequest only identifies the persona: credentials are chosen when the
// e-mailed link is opened (VerifyEmailRequest).
type RegisterRequest struct {
	DNI   string `json:"dni"   validate:"required,len=8,numeric"`
	Email string `json:"email" validate:"required,email,max=254"`
}

type VerifyEmailRequest struct {
	Token           string `json:"token"            validate:"required,max=128"`
	Username        string `json:"username"         validate:"required,min=4,max=30,alphanum"`
	Password        string `json:"password"         validate:"required,min=8,maxbytes=72"` // bcrypt limit is 72 BYTES
	ConfirmPassword string `json:"confirm_password" validate:"required,eqfield=Password"`
}

// ── Login ───────────────────────────────────────────────────────────────────

type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type UserProfile struct {
	ID              int64  `json:"id"`
	Username        string `json:"username"`
	Nombres         string `json:"nombres"`
	ApellidoPaterno string `json:"apellido_paterno"`
	ApellidoMaterno string `json:"apellido_materno"`
	Rol             string `json:"rol"`
	Estado          string `json:"estado"`
	GradoID         int64  `json:"grado_id"`
	Grado           string `json:"grado"`
	Nivel           string `json:"nivel"`
	Especialidad    string `json:"especialidad"`
	Institucion     string `json:"institucion"`
	TourCompletado  bool   `json:"tour_completado"`
}

type LoginResponse struct {
	Token     string      `json:"token"`
	ExpiresAt int64       `json:"expires_at"`
	User      UserProfile `json:"user"`
}

// ── Password Recovery ───────────────────────────────────────────────────────

type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type ResetPasswordRequest struct {
	Token           string `json:"token"            validate:"required"`
	Password        string `json:"password"         validate:"required,min=8,maxbytes=72"` // bcrypt limit is 72 BYTES
	ConfirmPassword string `json:"confirm_password" validate:"required,eqfield=Password"`
}

// ── Admin: Estado ───────────────────────────────────────────────────────────

type UpdateUsuarioEstadoRequest struct {
	Estado      string `json:"estado"      validate:"required,oneof=activo rechazado inactivo"`
	Observacion string `json:"observacion" validate:"max=500"`
}

type UpdateUsuarioEstadoResponse struct {
	ID        int64  `json:"id"`
	Estado    string `json:"estado"`
	UpdatedAt string `json:"updated_at"`
}

// ── Admin: List Usuarios ────────────────────────────────────────────────────

type UsuarioPendiente struct {
	ID             int64  `json:"id"`
	Username       string `json:"username"`
	NombreCompleto string `json:"nombre_completo"`
	DNI            string `json:"dni"`
	Email          string `json:"email"`
	Grado          string `json:"grado"`
	Especialidad   string `json:"especialidad"`
	Estado         string `json:"estado"`
	FechaSolicitud string `json:"fecha_solicitud"`
}

// ── Admin: Batch Import ─────────────────────────────────────────────────────

type BatchPersonaItem struct {
	DNI             string `json:"dni"              validate:"required,len=8,numeric"`
	Nombres         string `json:"nombres"          validate:"required,min=2,max=100"`
	ApellidoPaterno string `json:"apellido_paterno" validate:"required,min=2,max=100"`
	ApellidoMaterno string `json:"apellido_materno" validate:"required,min=2,max=100"`
	GradoID         int64  `json:"grado_id"         validate:"required"`
	EspecialidadID  int64  `json:"especialidad_id"  validate:"required"`
	InstitucionID   int64  `json:"institucion_id"   validate:"required"`
	Telefono        string `json:"telefono"         validate:"required,min=9,max=15"`
	Email           string `json:"email"            validate:"required,email"`
}

type BatchPersonasRequest struct {
	Personas []BatchPersonaItem `json:"personas" validate:"required,dive"`
}

// ── Admin: Personas list/edit ────────────────────────────────────────────────

type PersonaListItem struct {
	ID             int64  `json:"id"`
	DNI            string `json:"dni"`
	NombreCompleto string `json:"nombre_completo"`
	Nombres        string `json:"nombres"`
	ApellidoP      string `json:"apellido_paterno"`
	ApellidoM      string `json:"apellido_materno"`
	Email          string `json:"email"`
	Telefono       string `json:"telefono"`
	GradoID        int64  `json:"grado_id"`
	Grado          string `json:"grado"`
	EspecialidadID int64  `json:"especialidad_id"`
	Especialidad   string `json:"especialidad"`
	InstitucionID  int64  `json:"institucion_id"`
	Institucion    string `json:"institucion"`
	Estado         string `json:"estado"`
	TieneUsuario   bool   `json:"tiene_usuario"`
}

type UpdatePersonaRequest struct {
	Nombres         string `json:"nombres"          validate:"required,min=2,max=100"`
	ApellidoPaterno string `json:"apellido_paterno" validate:"required,min=2,max=100"`
	ApellidoMaterno string `json:"apellido_materno" validate:"max=100"`
	Email           string `json:"email"            validate:"required,email"`
	Telefono        string `json:"telefono"         validate:"max=20"`
	GradoID         int64  `json:"grado_id"         validate:"required"`
	EspecialidadID  int64  `json:"especialidad_id"  validate:"required"`
	InstitucionID   int64  `json:"institucion_id"   validate:"required"`
	Estado          string `json:"estado"           validate:"required,oneof=activo inactivo"`
}

type BatchPersonasResponse struct {
	Inserted   int      `json:"inserted"`
	Duplicates int      `json:"duplicates"`
	Errors     []string `json:"errors,omitempty"`
}

// Recurso es un acceso directo configurable que se muestra en el header.
type Recurso struct {
	ID          string `json:"id" validate:"required"`
	Titulo      string `json:"titulo" validate:"required,max=120"`
	Descripcion string `json:"descripcion" validate:"max=240"`
	URL         string `json:"url" validate:"required,http_url"`
	Icono       string `json:"icono" validate:"max=60"`
	Orden       int    `json:"orden"`
	Activo      bool   `json:"activo"`
}

// ── Admin: registration limits reset ────────────────────────────────────────

type ResetRegistroLimitesRequest struct {
	DNI string `json:"dni" validate:"required,len=8,numeric"`
}
