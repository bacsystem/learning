package salud

import (
	"context"
	"encoding/json"
	"net/http"
)

// estado is what the handler needs from the service.
type estado interface {
	Estado(ctx context.Context) Respuesta
}

// Handler serves GET /admin/salud.
type Handler struct {
	svc estado
}

// NewHandler creates the handler.
func NewHandler(svc estado) *Handler {
	return &Handler{svc: svc}
}

// Obtener always answers 200 once authentication passed: service failures go
// in the body as categories, never as an HTTP status.
func (h *Handler) Obtener(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(h.svc.Estado(r.Context()))
}
