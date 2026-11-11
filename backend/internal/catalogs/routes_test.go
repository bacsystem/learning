package catalogs

import (
	"net/http"
	"testing"

	"github.com/cbaciliod/learning/internal/db/dbtest"
)

// H88: the real mux + real session checker reject non-admins on every
// mutating catalog route and every /admin/procesos route.
func TestRoutes_AdminOnly(t *testing.T) {
	db := dbtest.Open(t)
	mux := http.NewServeMux()
	Mount(mux, NewService(NewRepository(db)), dbtest.JWTSecret, dbtest.SessionChecker(t, db))
	routes := []dbtest.Route{
		"GET /admin/procesos",
		"GET /admin/procesos/999999999",
		"POST /admin/procesos",
		"PUT /admin/procesos/999999999",
		"PATCH /admin/procesos/999999999/estado",
		"GET /admin/procesos/999999999/asignaturas",
		"POST /admin/procesos/999999999/asignaturas",
		"PUT /admin/procesos/999999999/asignaturas/999999999",
		"DELETE /admin/procesos/999999999/asignaturas/999999999",
		"GET /admin/procesos/999999999/pesos",
		"GET /admin/procesos/999999999/pesos/999999999",
		"PUT /admin/procesos/999999999/pesos/999999999",
	}
	for _, e := range []string{"instituciones", "grados", "especialidades", "asignaturas"} {
		routes = append(routes,
			dbtest.Route("POST /"+e),
			dbtest.Route("PUT /"+e+"/999999999"),
			dbtest.Route("PATCH /"+e+"/999999999/estado"),
		)
	}
	dbtest.AssertAdminOnly(t, db, mux, routes)
}
