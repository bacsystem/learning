package admin

import (
	"net/http"
	"testing"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/db/dbtest"
)

// H88: the real mux + real session checker reject non-admins on every
// /admin route of this package.
func TestRoutes_AdminOnly(t *testing.T) {
	db := dbtest.Open(t)
	mux := http.NewServeMux()
	RegisterRoutes(mux, db, dbtest.GCM(t), nil, dbtest.JWTSecret, dbtest.SessionChecker(t, db), audit.New(db))
	dbtest.AssertAdminOnly(t, db, mux, []dbtest.Route{
		"GET /admin/dashboard",
		"GET /admin/dashboard/actividad",
		"GET /admin/usuarios/999999999",
		"PATCH /admin/usuarios/999999999/rol",
		"GET /admin/audit-log",
		"GET /admin/ingresos",
		"GET /admin/ingresos/config",
		"PUT /admin/ingresos/config/2099",
	})
}
