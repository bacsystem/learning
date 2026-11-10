package auth_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/cbaciliod/learning/internal/auth"
	"github.com/cbaciliod/learning/internal/db/dbtest"
)

// H103: the real mux + real session checker reject anonymous and non-admin
// callers on every admin route of auth.Mount, and GET /admin/config/recursos
// (the header shortcuts every user reads) stays open to any session.
// External package: dbtest imports auth.
func TestRoutes_AdminOnly(t *testing.T) {
	db := dbtest.Open(t)
	repo := auth.NewRepository(db, dbtest.GCM(t), nil)
	// PUT /admin/config/recursos with {} as admin stores an empty list:
	// restore the configured value afterwards.
	recursos, err := repo.GetRecursos(context.Background())
	if err != nil {
		t.Fatalf("get recursos: %v", err)
	}
	t.Cleanup(func() {
		if err := repo.SetRecursos(context.Background(), recursos, nil); err != nil {
			t.Errorf("restore recursos: %v", err)
		}
	})
	mux := http.NewServeMux()
	auth.Mount(mux, auth.NewService(repo, dbtest.JWTSecret, "http://localhost:3000"), dbtest.JWTSecret, dbtest.SessionChecker(t, db))

	dbtest.AssertAdminOnly(t, db, mux, []dbtest.Route{
		"GET /admin/personas",
		"GET /admin/personas/999999999",
		"PUT /admin/personas/999999999",
		"GET /admin/usuarios",
		"PATCH /admin/usuarios/999999999/estado",
		"PATCH /admin/usuarios/999999999/email",
		"DELETE /admin/usuarios/999999999/tour",
		"DELETE /admin/registro-limites",
		"POST /admin/personas/batch",
		"POST /admin/personas/preview",
		"POST /admin/personas/import",
		"GET /admin/config/sesiones",
		"PUT /admin/config/sesiones",
		"PUT /admin/config/recursos",
		"GET /admin/usuarios/online",
		"GET /admin/usuarios/999999999/sesiones",
	})

	const recursosRoute = dbtest.Route("GET /admin/config/recursos")
	if rr := dbtest.Do(t, mux, recursosRoute, ""); rr.Code != http.StatusUnauthorized {
		t.Errorf("%s anonymous: want 401, got %d", recursosRoute, rr.Code)
	}
	cat := dbtest.SeedCatalog(t, db)
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	if rr := dbtest.Do(t, mux, recursosRoute, dbtest.RealBearer(t, db, user.ID, "usuario")); rr.Code != http.StatusOK {
		t.Errorf("%s as usuario: want 200, got %d %s", recursosRoute, rr.Code, rr.Body)
	}
}
