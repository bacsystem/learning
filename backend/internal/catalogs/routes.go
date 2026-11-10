package catalogs

import (
	"net/http"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/auth"
)

// Mount registers all catalog routes on the given mux.
// All routes require authentication; mutating routes also require admin role.
func Mount(mux *http.ServeMux, svc *Service, jwtSecret string, checkSession auth.SessionChecker) {
	// Mount's signature has no audit logger and cmd/server/main.go is outside
	// this package's ownership, so the logger is still built here on the same
	// DB pool (H86, pending: add an *audit.Logger parameter like the other
	// Mount functions and pass main's audLog).
	h := NewHandler(svc, audit.New(svc.repo.db))

	authn := auth.RequireAuth(jwtSecret, checkSession)
	admin := auth.RequireAdmin

	// ── Instituciones ─────────────────────────────────────────────────────────
	mux.Handle("GET /instituciones", authn(http.HandlerFunc(h.ListInstituciones)))
	mux.Handle("GET /instituciones/{id}", authn(http.HandlerFunc(h.GetInstitucion)))
	mux.Handle("POST /instituciones", authn(admin(http.HandlerFunc(h.CreateInstitucion))))
	mux.Handle("PUT /instituciones/{id}", authn(admin(http.HandlerFunc(h.UpdateInstitucion))))
	mux.Handle("PATCH /instituciones/{id}/estado", authn(admin(http.HandlerFunc(h.UpdateInstitucionEstado))))

	// ── Grados ────────────────────────────────────────────────────────────────
	mux.Handle("GET /grados", authn(http.HandlerFunc(h.ListGrados)))
	mux.Handle("GET /grados/{id}", authn(http.HandlerFunc(h.GetGrado)))
	mux.Handle("POST /grados", authn(admin(http.HandlerFunc(h.CreateGrado))))
	mux.Handle("PUT /grados/{id}", authn(admin(http.HandlerFunc(h.UpdateGrado))))
	mux.Handle("PATCH /grados/{id}/estado", authn(admin(http.HandlerFunc(h.UpdateGradoEstado))))

	// ── Especialidades ────────────────────────────────────────────────────────
	mux.Handle("GET /especialidades", authn(http.HandlerFunc(h.ListEspecialidades)))
	mux.Handle("GET /especialidades/{id}", authn(http.HandlerFunc(h.GetEspecialidad)))
	mux.Handle("POST /especialidades", authn(admin(http.HandlerFunc(h.CreateEspecialidad))))
	mux.Handle("PUT /especialidades/{id}", authn(admin(http.HandlerFunc(h.UpdateEspecialidad))))
	mux.Handle("PATCH /especialidades/{id}/estado", authn(admin(http.HandlerFunc(h.UpdateEspecialidadEstado))))

	// ── Asignaturas ───────────────────────────────────────────────────────────
	mux.Handle("GET /asignaturas", authn(http.HandlerFunc(h.ListAsignaturas)))
	mux.Handle("GET /asignaturas/{id}", authn(http.HandlerFunc(h.GetAsignatura)))
	mux.Handle("POST /asignaturas", authn(admin(http.HandlerFunc(h.CreateAsignatura))))
	mux.Handle("PUT /asignaturas/{id}", authn(admin(http.HandlerFunc(h.UpdateAsignatura))))
	mux.Handle("PATCH /asignaturas/{id}/estado", authn(admin(http.HandlerFunc(h.UpdateAsignaturaEstado))))

	// ── Procesos ──────────────────────────────────────────────────────────────
	mux.Handle("GET /admin/procesos", authn(admin(http.HandlerFunc(h.ListProcesos))))
	mux.Handle("GET /admin/procesos/{id}", authn(admin(http.HandlerFunc(h.GetProceso))))
	mux.Handle("POST /admin/procesos", authn(admin(http.HandlerFunc(h.CreateProceso))))
	mux.Handle("PUT /admin/procesos/{id}", authn(admin(http.HandlerFunc(h.UpdateProceso))))
	mux.Handle("PATCH /admin/procesos/{id}/estado", authn(admin(http.HandlerFunc(h.UpdateProcesoEstado))))

	// ── Proceso-Asignaturas ───────────────────────────────────────────────────
	mux.Handle("GET /procesos/{id}/asignaturas", authn(http.HandlerFunc(h.ListProcesoAsignaturas)))
	mux.Handle("GET /admin/procesos/{id}/asignaturas", authn(admin(http.HandlerFunc(h.ListProcesoAsignaturas))))
	mux.Handle("POST /admin/procesos/{id}/asignaturas", authn(admin(http.HandlerFunc(h.AsignarAsignatura))))
	mux.Handle("PUT /admin/procesos/{id}/asignaturas/{asignaturaId}", authn(admin(http.HandlerFunc(h.UpdateProcesoAsignatura))))
	mux.Handle("DELETE /admin/procesos/{id}/asignaturas/{asignaturaId}", authn(admin(http.HandlerFunc(h.DeleteProcesoAsignatura))))
}
