package questions

import (
	"net/http"

	"github.com/cbaciliod/learning/internal/auth"
)

// Mount registers all questions routes on the given mux.
//
// Route matrix:
//   - GET /preguntas          (admin)
//   - POST /preguntas         (admin)
//   - GET /preguntas/stats    (admin)
//   - POST /preguntas/import  (admin)
//   - POST /preguntas/generate-set (admin)
//   - GET /preguntas/{id}     (admin)
//   - PUT /preguntas/{id}     (admin)
//   - DELETE /preguntas/{id}  (admin)
//   - GET /etiquetas          (admin)
//   - POST /etiquetas         (admin)
//   - PUT /etiquetas/{id}     (admin)
//   - DELETE /etiquetas/{id}  (admin)
func Mount(mux *http.ServeMux, svc *Service, jwtSecret string, checkSession auth.SessionChecker) {
	h := NewHandler(svc)

	authn := auth.RequireAuth(jwtSecret, checkSession)
	admin := auth.RequireAdmin

	// ── Preguntas (specific routes before parameterized) ──────────────────────
	mux.Handle("GET /preguntas/stats", authn(admin(http.HandlerFunc(h.GetStats))))
	mux.Handle("POST /preguntas/import", authn(admin(http.HandlerFunc(h.ImportPreguntas))))
	mux.Handle("POST /preguntas/preview", authn(admin(http.HandlerFunc(h.PreviewPreguntas))))
	mux.Handle("POST /preguntas/generate-set", authn(admin(http.HandlerFunc(h.GenerateSet))))

	mux.Handle("GET /preguntas", authn(admin(http.HandlerFunc(h.ListPreguntas))))
	mux.Handle("POST /preguntas", authn(admin(http.HandlerFunc(h.CreatePregunta))))
	mux.Handle("GET /preguntas/{id}", authn(admin(http.HandlerFunc(h.GetPregunta))))
	mux.Handle("PUT /preguntas/{id}", authn(admin(http.HandlerFunc(h.UpdatePregunta))))
	mux.Handle("DELETE /preguntas/{id}", authn(admin(http.HandlerFunc(h.DeletePregunta))))

	// ── Etiquetas ─────────────────────────────────────────────────────────────
	mux.Handle("GET /etiquetas", authn(admin(http.HandlerFunc(h.ListEtiquetas))))
	mux.Handle("POST /etiquetas", authn(admin(http.HandlerFunc(h.CreateEtiqueta))))
	mux.Handle("PUT /etiquetas/{id}", authn(admin(http.HandlerFunc(h.UpdateEtiqueta))))
	mux.Handle("DELETE /etiquetas/{id}", authn(admin(http.HandlerFunc(h.DeleteEtiqueta))))
}
