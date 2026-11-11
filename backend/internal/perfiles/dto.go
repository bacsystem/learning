// Package perfiles holds the profession (proceso, especialidad) that a
// postulante picks for the SERUMS IA module (SIA-E1-04). Every route resolves
// the user from the session; none takes a usuario_id.
package perfiles

import "time"

// Pesos availability of an especialidad in F1 (E1-04 regla 2).
const (
	PesosCompleta = "completa"
	PesosEstimada = "estimada"

	EstadoActivo    = "activo"
	EstadoArchivado = "archivado"
)

// ── F1 ────────────────────────────────────────────────────────────────────────

// EspecialidadOpcion is one selectable profession of a proceso.
type EspecialidadOpcion struct {
	ID     int64  `json:"id"`
	Nombre string `json:"nombre"`
	Codigo string `json:"codigo"`
	Pesos  string `json:"pesos"`
}

// ProcesoOpcion is a proceso where the user has an approved inscripción.
type ProcesoOpcion struct {
	ID             int64                `json:"id"`
	Nombre         string               `json:"nombre"`
	Anio           int                  `json:"anio"`
	Especialidades []EspecialidadOpcion `json:"especialidades"`
}

// Preseleccion is the suggestion of F1; nothing is stored (regla 5). The
// especialidad is always set: without a valid one there is no preselection
// (contract F1, PR #85); the proceso is null when there are several.
type Preseleccion struct {
	ProcesoID      *int64 `json:"proceso_id"`
	EspecialidadID int64  `json:"especialidad_id"`
}

// Opciones is the F1 response.
type Opciones struct {
	Procesos     []ProcesoOpcion `json:"procesos"`
	Preseleccion *Preseleccion   `json:"preseleccion"`
}

// ── F2, F3 ────────────────────────────────────────────────────────────────────

// Ref is an id with its name.
type Ref struct {
	ID     int64  `json:"id"`
	Nombre string `json:"nombre"`
}

// AreaPeso is one area of the profile's weights.
type AreaPeso struct {
	AsignaturaID   int64   `json:"asignatura_id"`
	Nombre         string  `json:"nombre"`
	PesoPorcentual float64 `json:"peso_porcentual"`
}

// PesosPerfil are the weights of the profile, from catalogs PE6 (ModoEstimable).
type PesosPerfil struct {
	PesosEstimados bool       `json:"pesos_estimados"`
	Areas          []AreaPeso `json:"areas"`
}

// Perfil is the active profile (F2, and the F3 response).
type Perfil struct {
	ID           int64       `json:"id"`
	Proceso      Ref         `json:"proceso"`
	Especialidad Ref         `json:"especialidad"`
	Estado       string      `json:"estado"`
	CreatedAt    time.Time   `json:"created_at"`
	Pesos        PesosPerfil `json:"pesos"`
}

// ElegirRequest is the F3 body.
type ElegirRequest struct {
	ProcesoID         int64 `json:"proceso_id"`
	EspecialidadID    int64 `json:"especialidad_id"`
	ConfirmarReinicio bool  `json:"confirmar_reinicio"`
}

// Par is a (proceso, especialidad) pair.
type Par struct {
	ProcesoID      int64 `json:"proceso_id" db:"proceso_id"`
	EspecialidadID int64 `json:"especialidad_id" db:"especialidad_id"`
}
