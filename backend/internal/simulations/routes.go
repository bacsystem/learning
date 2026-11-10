package simulations

import (
	"net/http"

	"github.com/cbaciliod/learning/internal/auth"
)

// Mount registers all simulacro routes on the given mux.
//
// Route matrix (all require authn):
//   - POST   /simulacros
//   - GET    /simulacros/historial
//   - GET    /simulacros/en-curso
//   - GET    /simulacros/{id}
//   - POST   /simulacros/{id}/responder         ('unica': replace; 'multiple': toggle)
//   - PUT    /simulacros/{id}/preguntas/{preguntaId}/respuestas  (full selection set)
//   - POST   /simulacros/{id}/finalizar
//   - GET    /simulacros/{id}/resultado
//   - GET    /simulacros/{id}/revision
func Mount(mux *http.ServeMux, svc *Service, jwtSecret string, checkSession auth.SessionChecker) {
	h := NewHandler(svc)
	authn := auth.RequireAuth(jwtSecret, checkSession)
	admin := auth.RequireAdmin

	// Specific routes before parameterized to avoid ambiguity
	mux.Handle("GET /simulacros/historial", authn(http.HandlerFunc(h.GetHistorial)))
	mux.Handle("GET /simulacros/en-curso", authn(http.HandlerFunc(h.GetEnCurso)))
	mux.Handle("GET /simulacros/configuracion", authn(http.HandlerFunc(h.GetConfiguracion)))
	mux.Handle("GET /simulacros/configuraciones", authn(admin(http.HandlerFunc(h.ListConfiguraciones))))
	mux.Handle("POST /simulacros/configuracion", authn(admin(http.HandlerFunc(h.CreateConfiguracion))))
	mux.Handle("PATCH /simulacros/configuracion/{id}", authn(admin(http.HandlerFunc(h.UpdateConfiguracion))))

	mux.Handle("POST /simulacros", authn(http.HandlerFunc(h.CreateSimulacro)))

	mux.Handle("GET /simulacros/{id}", authn(http.HandlerFunc(h.GetSimulacro)))
	mux.Handle("POST /simulacros/{id}/responder", authn(http.HandlerFunc(h.ResponderPregunta)))
	mux.Handle("PUT /simulacros/{id}/preguntas/{preguntaId}/respuestas", authn(http.HandlerFunc(h.SetRespuestas)))
	mux.Handle("POST /simulacros/{id}/finalizar", authn(http.HandlerFunc(h.FinalizarSimulacro)))
	mux.Handle("GET /simulacros/{id}/resultado", authn(http.HandlerFunc(h.GetResultado)))
	mux.Handle("GET /simulacros/{id}/revision", authn(http.HandlerFunc(h.GetRevision)))
}
