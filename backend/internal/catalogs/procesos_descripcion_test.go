package catalogs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cbaciliod/learning/internal/db/dbtest"
)

// Regression (qa): a proceso whose descripcion is NULL (rows seeded or
// imported by SQL; the API always writes a string) made GET /admin/procesos
// and GET /admin/procesos/{id} answer 500, because descripcion was scanned
// into a string. NULL now reads as "".
func TestProcesos_DescripcionNull(t *testing.T) {
	db := dbtest.Open(t)
	mux := http.NewServeMux()
	Mount(mux, NewService(NewRepository(db)), dbtest.JWTSecret, dbtest.SessionChecker(t, db))
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	bearer := dbtest.RealBearer(t, db, admin.ID, "admin")
	proceso := dbtest.SeedProceso(t, db, cat, 2030, "inactivo")
	var esNull bool
	if err := db.Get(&esNull, `SELECT descripcion IS NULL FROM procesos WHERE id = $1`, proceso); err != nil || !esNull {
		t.Fatalf("the seed must leave descripcion NULL (null=%v, err=%v)", esNull, err)
	}

	get := func(t *testing.T, path string) (int, []byte) {
		t.Helper()
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", bearer)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		return rr.Code, rr.Body.Bytes()
	}
	cases := []struct {
		name, path  string
		descripcion func(t *testing.T, body []byte) (any, bool)
	}{
		{"listado", fmt.Sprintf("/admin/procesos?institucion_id=%d", cat.InstitucionID), func(t *testing.T, body []byte) (any, bool) {
			var list []map[string]any
			if err := json.Unmarshal(body, &list); err != nil {
				t.Fatal(err)
			}
			for _, p := range list {
				if int64(p["id"].(float64)) == proceso {
					return p["descripcion"], true
				}
			}
			return nil, false
		}},
		{"detalle", fmt.Sprintf("/admin/procesos/%d", proceso), func(t *testing.T, body []byte) (any, bool) {
			var p map[string]any
			if err := json.Unmarshal(body, &p); err != nil {
				t.Fatal(err)
			}
			d, ok := p["descripcion"]
			return d, ok
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, body := get(t, c.path)
			if status != http.StatusOK {
				t.Fatalf("got %d %s, want 200", status, body)
			}
			if d, found := c.descripcion(t, body); !found || d != "" {
				t.Errorf("descripcion = %v (found %v), want \"\"", d, found)
			}
		})
	}

}
