package fuentes

import (
	"net/http"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/auth"
)

// Route patterns of the uploads. They have no {id}: MaxBodySize raises the
// body limit only for exact paths listed in cmd/server/main.go.
const (
	RutaCrear        = "POST /admin/fuentes"
	RutaNuevaVersion = "POST /admin/fuentes/versiones"
)

// RutasSubida are the routes cmd/server/main.go must list in MaxBodySize so
// the 2 MiB file is not cut by the 1 MB JSON limit.
var RutasSubida = []string{RutaCrear, RutaNuevaVersion}

// Route patterns of the admin listing, detail and changes (S3–S7).
const (
	RutaListar        = "GET /admin/fuentes"
	RutaObtener       = "GET /admin/fuentes/{id}"
	RutaCambiarEstado = "PATCH /admin/fuentes/{id}/estado"
	RutaOficial       = "PATCH /admin/fuentes/{id}/oficial"
	RutaEditar        = "PUT /admin/fuentes/{id}"
)

// RutaBuscarPasajes is the admin search of passages to cite (E3-07, ADR 0007).
const RutaBuscarPasajes = "GET /admin/pasajes"

// Mount registers the fuentes admin routes S1–S7 (contract E2).
func Mount(mux *http.ServeMux, svc *Service, audLog *audit.Logger, jwtSecret string, checkSession auth.SessionChecker) {
	h := NewHandler(svc, audLog)
	authn := auth.RequireAuth(jwtSecret, checkSession)
	admin := auth.RequireAdmin

	mux.Handle(RutaCrear, authn(admin(http.HandlerFunc(h.Crear))))
	mux.Handle(RutaNuevaVersion, authn(admin(http.HandlerFunc(h.NuevaVersion))))
	mux.Handle(RutaListar, authn(admin(http.HandlerFunc(h.Listar))))
	mux.Handle(RutaObtener, authn(admin(http.HandlerFunc(h.Obtener))))
	mux.Handle(RutaCambiarEstado, authn(admin(http.HandlerFunc(h.CambiarEstado))))
	mux.Handle(RutaOficial, authn(admin(http.HandlerFunc(h.Oficial))))
	mux.Handle(RutaEditar, authn(admin(http.HandlerFunc(h.Editar))))
	mux.Handle(RutaBuscarPasajes, authn(admin(http.HandlerFunc(h.BuscarPasajes))))
}
