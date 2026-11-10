package inscripciones

import (
	"net/http"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/auth"
)

func Mount(mux *http.ServeMux, svc *Service, audLog *audit.Logger, jwtSecret string, checkSession auth.SessionChecker) {
	h := NewHandler(svc, audLog)

	requireAuth := auth.RequireAuth(jwtSecret, checkSession)
	requireAdmin := auth.RequireAdmin

	// Público: sin auth
	mux.HandleFunc("GET /stats/publico", h.GetStatsPublicos)
	mux.HandleFunc("GET /procesos/publico", h.ListProcesosPublico)

	// Autenticado: procesos filtrados por nivel del usuario
	mux.Handle("GET /procesos", requireAuth(http.HandlerFunc(h.ListProcesos)))

	// Usuario: sus inscripciones
	mux.Handle("POST /inscripciones", requireAuth(http.HandlerFunc(h.Inscribir)))
	mux.Handle("GET /inscripciones/me", requireAuth(http.HandlerFunc(h.MisInscripciones)))

	// Admin: gestión de inscripciones
	mux.Handle("GET /admin/inscripciones", requireAuth(requireAdmin(http.HandlerFunc(h.ListInscripciones))))
	mux.Handle("PATCH /admin/inscripciones/{id}/estado", requireAuth(requireAdmin(http.HandlerFunc(h.UpdateInscripcionEstado))))
}
