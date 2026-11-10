package inscripciones

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
	Mount(mux, NewService(NewRepository(db, dbtest.GCM(t), nil)), audit.New(db), dbtest.JWTSecret, dbtest.SessionChecker(t, db))
	dbtest.AssertAdminOnly(t, db, mux, []dbtest.Route{
		"GET /admin/inscripciones",
		"PATCH /admin/inscripciones/999999999/estado",
	})
}
