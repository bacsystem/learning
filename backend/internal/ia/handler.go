package ia

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/auth"
	"github.com/cbaciliod/learning/internal/httpx"
)

// Messages and codes of the endpoints (docs/api/serums-ia-e6-02-control-costos.md §3, §5).
const (
	codigoValidacion = "validacion"
	msgNoAutenticado = "No autenticado."
	msgValidacion    = "Hay campos inválidos."
)

// administracion is what the handlers need from the cost control.
type administracion interface {
	Consumo(ctx context.Context, a Actor) (Consumo, error)
	Configuracion(ctx context.Context) (Configuracion, error)
	ActualizarConfiguracion(ctx context.Context, adminID int64, req ActualizarConfiguracion, aud Auditor) (Configuracion, error)
	Gasto(ctx context.Context) (Gasto, error)
}

// Handler exposes the endpoints of the cost control.
type Handler struct {
	svc    administracion
	audLog *audit.Logger
}

// NewHandler creates the handler; audLog writes the audit row of the PUT.
func NewHandler(svc administracion, audLog *audit.Logger) *Handler {
	return &Handler{svc: svc, audLog: audLog}
}

// Consumo handles GET /serums-ia/ia/consumo: the actions the authenticated
// user has left. The user comes from the claims, never from the request.
func (h *Handler) Consumo(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		httpx.JSONError(w, http.StatusUnauthorized, msgNoAutenticado)
		return
	}
	rol := RolUsuario
	if claims.Rol == "admin" {
		rol = RolAdmin
	}
	id := claims.UserID
	c, err := h.svc.Consumo(r.Context(), Actor{Rol: rol, ID: &id})
	if err != nil {
		httpx.InternalErrorCode(w, "ia.consumo", err)
		return
	}
	responderJSON(w, c)
}

// Configuracion handles GET /admin/ia/configuracion.
func (h *Handler) Configuracion(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.Configuracion(r.Context())
	if err != nil {
		httpx.InternalErrorCode(w, "ia.configuracion", err)
		return
	}
	responderJSON(w, c)
}

// Configurar handles PUT /admin/ia/configuracion: a full replacement.
func (h *Handler) Configurar(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		httpx.JSONError(w, http.StatusUnauthorized, msgNoAutenticado)
		return
	}
	var req ActualizarConfiguracion
	if !httpx.DecodeJSONStrict(w, r, &req) {
		return
	}
	actor, ip, ua := claims.UserID, audit.ClientIP(r), r.UserAgent()
	aud := func(ctx context.Context, tx sqlx.ExecerContext, ev Evento) error {
		return h.audLog.LogTx(ctx, tx, &actor, ev.Accion, ev.Entidad, &ev.EntidadID, ev.Detalle, ip, ua)
	}
	c, err := h.svc.ActualizarConfiguracion(r.Context(), actor, req, aud)
	if err != nil {
		var verr *ValidacionError
		if errors.As(err, &verr) {
			httpx.JSONErrorCode(w, http.StatusUnprocessableEntity, codigoValidacion, msgValidacion, map[string]any{"campos": verr.Campos})
			return
		}
		httpx.InternalErrorCode(w, "ia.configurar", err)
		return
	}
	responderJSON(w, c)
}

// Gasto handles GET /admin/ia/gasto.
func (h *Handler) Gasto(w http.ResponseWriter, r *http.Request) {
	g, err := h.svc.Gasto(r.Context())
	if err != nil {
		httpx.InternalErrorCode(w, "ia.gasto", err)
		return
	}
	responderJSON(w, g)
}

func responderJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Error().Str("op", "ia.responder").Msg("ia: response not written")
	}
}
