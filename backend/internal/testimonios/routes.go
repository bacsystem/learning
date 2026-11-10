package testimonios

import (
	"net/http"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/auth"
)

func Mount(mux *http.ServeMux, svc *Service, audLog *audit.Logger, jwtSecret string, checkSession auth.SessionChecker) {
	h := NewHandler(svc, audLog)
	requireAuth := auth.RequireAuth(jwtSecret, checkSession)
	requireAdmin := auth.RequireAdmin

	mux.HandleFunc("GET /testimonios/publico", h.GetPublicos)
	mux.Handle("GET /testimonios/me", requireAuth(http.HandlerFunc(h.GetMio)))
	mux.Handle("POST /testimonios", requireAuth(http.HandlerFunc(h.Submit)))
	mux.Handle("GET /admin/testimonios", requireAuth(requireAdmin(http.HandlerFunc(h.ListAdmin))))
	mux.Handle("PATCH /admin/testimonios/{id}/estado", requireAuth(requireAdmin(http.HandlerFunc(h.UpdateEstado))))
}
