package notifications

import "time"

// Notificacion is the full notification record.
type Notificacion struct {
	ID           int64     `db:"id"           json:"id"`
	Titulo       string    `db:"titulo"       json:"titulo"`
	Cuerpo       string    `db:"cuerpo"       json:"cuerpo"`
	Tipo         string    `db:"tipo"         json:"tipo"`
	Destinatario string    `db:"destinatario" json:"destinatario"`
	CreadoPor    *int64    `db:"creado_por"   json:"creado_por,omitempty"`
	CreatedAt    time.Time `db:"created_at"   json:"created_at"`
	Leida        bool      `db:"leida"        json:"leida"`
}

// CreateRequest is the payload for POST /admin/notificaciones.
//
// Destinatario is "todos" or "rol:<usuarios.rol>", i.e. "rol:admin" or
// "rol:usuario" — the readers match it against "rol:"+claims.Rol. The legacy
// value "rol:user" (sent by older frontends) is normalised to "rol:usuario"
// before validation, see NormalizeDestinatario.
type CreateRequest struct {
	Titulo       string `json:"titulo"       validate:"required,min=3,max=200"`
	Cuerpo       string `json:"cuerpo"       validate:"required,min=3"`
	Tipo         string `json:"tipo"         validate:"required,oneof=sistema mantenimiento funcionalidad alerta"`
	Destinatario string `json:"destinatario" validate:"required,oneof=todos rol:admin rol:usuario"`
}

// legacyDestinatarioUsuarios is the value older clients sent for "all users".
const legacyDestinatarioUsuarios = "rol:user"

// NormalizeDestinatario maps legacy aliases to the canonical destinatario.
func NormalizeDestinatario(d string) string {
	if d == legacyDestinatarioUsuarios {
		return "rol:usuario"
	}
	return d
}

// UnreadCountResponse is returned by GET /notificaciones/no-leidas.
type UnreadCountResponse struct {
	Count int `json:"count"`
}
