package practices

import (
	"net/http"

	"github.com/cbaciliod/learning/internal/auth"
)

// Mount registers all practice routes on the given mux.
//
// Route matrix (all require authn):
//   - POST   /practicas
//   - GET    /practicas
//   - DELETE /practicas/batch
//   - GET    /practicas/{id}
//   - POST   /practicas/{id}/responder
//   - POST   /practicas/{id}/validar/{preguntaId}
//   - POST   /practicas/{id}/finalizar
//   - GET    /practicas/{id}/resultado
//   - GET    /practicas/{id}/revision
//   - DELETE /practicas/{id}
func Mount(mux *http.ServeMux, svc *Service, jwtSecret string, checkSession auth.SessionChecker) {
	h := NewHandler(svc)
	authn := auth.RequireAuth(jwtSecret, checkSession)

	// Specific routes before parameterized to avoid ambiguity
	mux.Handle("DELETE /practicas/batch", authn(http.HandlerFunc(h.BatchDeletePracticas)))

	mux.Handle("POST /practicas", authn(http.HandlerFunc(h.CreatePractica)))
	mux.Handle("GET /practicas", authn(http.HandlerFunc(h.ListHistorial)))

	mux.Handle("GET /practicas/{id}", authn(http.HandlerFunc(h.GetPractica)))
	mux.Handle("DELETE /practicas/{id}", authn(http.HandlerFunc(h.DeletePractica)))

	mux.Handle("POST /practicas/{id}/responder", authn(http.HandlerFunc(h.ToggleRespuesta)))
	mux.Handle("POST /practicas/{id}/validar/{preguntaId}", authn(http.HandlerFunc(h.ValidarPregunta)))
	mux.Handle("POST /practicas/{id}/finalizar", authn(http.HandlerFunc(h.FinalizarPractica)))

	mux.Handle("GET /practicas/{id}/resultado", authn(http.HandlerFunc(h.GetResultado)))
	mux.Handle("GET /practicas/{id}/revision", authn(http.HandlerFunc(h.GetRevision)))
}
