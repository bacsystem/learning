package admin

import "encoding/json"

// ── Dashboard ─────────────────────────────────────────────────────────────────

type DashboardResponse struct {
	TotalUsuariosRegistrados int64                   `json:"total_usuarios_registrados"`
	TotalUsuariosActivos     int64                   `json:"total_usuarios_activos"`
	TotalUsuariosPendientes  int64                   `json:"total_usuarios_pendientes"`
	TotalPracticasHoy        int64                   `json:"total_practicas_hoy"`
	TotalSimulacrosHoy       int64                   `json:"total_simulacros_hoy"`
	UsuariosOnline           int64                   `json:"usuarios_online"`
	PracticasUltimaSemana    []DiaMetricaItem        `json:"practicas_ultima_semana"`
	SimulacrosUltimaSemana   []DiaMetricaItem        `json:"simulacros_ultima_semana"`
	TopAsignaturas           []AsignaturaMetricaItem `json:"top_asignaturas"`
}

type DiaMetricaItem struct {
	Fecha string `json:"fecha"`
	Total int64  `json:"total"`
}

type AsignaturaMetricaItem struct {
	Nombre string `json:"nombre"`
	Total  int64  `json:"total"`
}

// ── Usuario Detalle ───────────────────────────────────────────────────────────

type UsuarioDetalleResponse struct {
	ID              int64   `json:"id"`
	PersonaID       int64   `json:"persona_id"`
	DNI             string  `json:"dni"`
	Nombres         string  `json:"nombres"`
	ApellidoPaterno string  `json:"apellido_paterno"`
	ApellidoMaterno string  `json:"apellido_materno"`
	Grado           string  `json:"grado"`
	Especialidad    string  `json:"especialidad"`
	Institucion     string  `json:"institucion"`
	Telefono        string  `json:"telefono"`
	Email           string  `json:"email"`
	Username        string  `json:"username"`
	Rol             string  `json:"rol"`
	Estado          string  `json:"estado"`
	ActivatedAt     *string `json:"activated_at,omitempty"`
	ActivatedBy     *string `json:"activated_by,omitempty"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
	TotalPracticas  int64   `json:"total_practicas"`
	TotalSimulacros int64   `json:"total_simulacros"`
	NotaPromedio    float64 `json:"nota_promedio"`
}

type UpdateRolRequest struct {
	Rol string `json:"rol" validate:"required,oneof=admin usuario"`
}

// ── Audit Log ─────────────────────────────────────────────────────────────────

type AuditLogItem struct {
	ID        int64                  `json:"id"`
	UsuarioID *int64                 `json:"usuario_id,omitempty"`
	Username  string                 `json:"username,omitempty"`
	Accion    string                 `json:"accion"`
	Entidad   string                 `json:"entidad"`
	EntidadID *int64                 `json:"entidad_id,omitempty"`
	Detalle   map[string]interface{} `json:"detalle,omitempty"`
	IP        string                 `json:"ip,omitempty"`
	UserAgent string                 `json:"user_agent,omitempty"`
	CreatedAt string                 `json:"created_at"`
}

type AuditLogResponse struct {
	Data    []AuditLogItem `json:"data"`
	Total   int64          `json:"total"`
	Page    int            `json:"page"`
	PerPage int            `json:"per_page"`
}

// ── Ingresos ──────────────────────────────────────────────────────────────────

// IngresosResponse carries money as exact decimal strings with 2 places
// ("1234.50"), straight from the DECIMAL sum's text form — never float64
// (H66). Clients format them as strings and parse only for charting.
type IngresosResponse struct {
	TotalRecaudado     string           `json:"total_recaudado"`
	TotalEsteAnio      string           `json:"total_este_anio"`
	TotalEsteMes       string           `json:"total_este_mes"`
	TotalInscripciones int64            `json:"total_inscripciones"`
	SerieMensual       []MesIngresoItem `json:"serie_mensual"`
}

type MesIngresoItem struct {
	Mes   string `json:"mes"`
	Total string `json:"total"`
}

// ConfigPago is serialised with Monto as an exact JSON number taken from the
// DECIMAL(10,2) column's text form (e.g. 150.10) — no float64 round-trip.
type ConfigPago struct {
	Anio      int         `json:"anio"`
	Monto     json.Number `json:"monto"`
	UpdatedAt string      `json:"updated_at"`
}

// UpdateConfigPagoRequest: monto is required (a missing field is not 0) and is
// kept as the literal the client sent; parseMonto validates it.
type UpdateConfigPagoRequest struct {
	Monto *json.Number `json:"monto"`
}
