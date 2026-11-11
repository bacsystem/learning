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

// Mount registers the fuentes admin routes (contract E2). For now S1 and S2;
// S3–S7 come with E2-06 and E2-07.
func Mount(mux *http.ServeMux, svc *Service, audLog *audit.Logger, jwtSecret string, checkSession auth.SessionChecker) {
	h := NewHandler(svc, audLog)
	authn := auth.RequireAuth(jwtSecret, checkSession)
	admin := auth.RequireAdmin

	mux.Handle(RutaCrear, authn(admin(http.HandlerFunc(h.Crear))))
	mux.Handle(RutaNuevaVersion, authn(admin(http.HandlerFunc(h.NuevaVersion))))
}
