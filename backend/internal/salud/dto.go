// Package salud reports the operational status of the platform's services
// to admins: GET /admin/salud (PLT-01, docs/api/plataforma-salud.md, ADR 0005).
// It is a read without side effects, so it writes no audit_log.
package salud

import "time"

// Service states (docs/api/plataforma-salud.md § S1).
const (
	EstadoOperativo     = "operativo"
	EstadoDegradado     = "degradado"
	EstadoCaido         = "caido"
	EstadoNoConfigurado = "no_configurado"
	EstadoDesconocido   = "desconocido"
)

// Closed failure categories (decisión 8 del PO). Never an error message.
const (
	CategoriaSinConexion   = "sin_conexion"
	CategoriaTiempoAgotado = "tiempo_agotado"
	CategoriaErrorInterno  = "error_interno"
	CategoriaAtrasado      = "atrasado"
	CategoriaSinDatos      = "sin_datos"
)

// Respuesta is the body of GET /admin/salud.
type Respuesta struct {
	Estado       string     `json:"estado"`
	ComprobadoEn time.Time  `json:"comprobado_en"` // UTC
	Servicios    []Servicio `json:"servicios"`
}

// Servicio is the status of one service. Datos holds only numbers, versions
// and UTC dates; a value that could not be obtained is null inside it, and
// Datos itself is never null.
type Servicio struct {
	ID        string         `json:"id"`
	Estado    string         `json:"estado"`
	Categoria *string        `json:"categoria"`
	Datos     map[string]any `json:"datos"`
}
