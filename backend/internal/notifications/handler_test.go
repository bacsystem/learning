package notifications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func setup(t *testing.T) (*sqlx.DB, *Handler) {
	t.Helper()
	db := dbtest.Open(t)
	return db, NewHandler(NewRepository(db), audit.New(db))
}

func create(t *testing.T, db *sqlx.DB, h *Handler, adminID int64, dest string) *httptest.ResponseRecorder {
	t.Helper()
	body := fmt.Sprintf(`{"titulo":"Aviso %s","cuerpo":"cuerpo de prueba","tipo":"sistema","destinatario":%q}`, dbtest.Uniq(), dest)
	req := httptest.NewRequest(http.MethodPost, "/admin/notificaciones", bytes.NewBufferString(body))
	rr := dbtest.Serve(t, h.AdminCreate, req, adminID, "admin")
	if rr.Code == http.StatusCreated {
		var n Notificacion
		_ = json.Unmarshal(rr.Body.Bytes(), &n)
		t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM notificaciones WHERE id=$1`, n.ID) })
	}
	return rr
}

func listFor(t *testing.T, h *Handler, userID int64, rol string) []Notificacion {
	t.Helper()
	rr := dbtest.Serve(t, h.List, httptest.NewRequest(http.MethodGet, "/notificaciones", nil), userID, rol)
	if rr.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rr.Code, rr.Body)
	}
	var out []Notificacion
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	return out
}

func has(list []Notificacion, id int64) bool {
	for _, n := range list {
		if n.ID == id {
			return true
		}
	}
	return false
}

// H21: notifications for regular users must reach role "usuario"; the legacy
// "rol:user" value is accepted and normalised; unknown targets are rejected.
func TestNotificacionesParaUsuarios(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")

	for _, dest := range []string{"rol:usuario", "rol:user"} {
		rr := create(t, db, h, admin.ID, dest)
		if rr.Code != http.StatusCreated {
			t.Fatalf("%s: want 201, got %d %s", dest, rr.Code, rr.Body)
		}
		var n Notificacion
		_ = json.Unmarshal(rr.Body.Bytes(), &n)
		if n.Destinatario != "rol:usuario" {
			t.Fatalf("%s stored as %q, want rol:usuario", dest, n.Destinatario)
		}
		if !has(listFor(t, h, user.ID, "usuario"), n.ID) {
			t.Fatalf("%s: notification not visible to a usuario", dest)
		}
		if has(listFor(t, h, admin.ID, "admin"), n.ID) {
			t.Fatalf("%s: notification for usuarios visible to admin", dest)
		}
	}
	rr := dbtest.Serve(t, h.UnreadCount, httptest.NewRequest(http.MethodGet, "/x", nil), user.ID, "usuario")
	var c UnreadCountResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &c)
	if c.Count < 2 {
		t.Fatalf("unread count = %d, want ≥ 2", c.Count)
	}
	for _, dest := range []string{"rol:superadmin", "user:1", ""} {
		if rr := create(t, db, h, admin.ID, dest); rr.Code != http.StatusBadRequest {
			t.Fatalf("%q: want 400, got %d", dest, rr.Code)
		}
	}
}

// H55: marking an unknown (or not addressed) notification → 404, not FK 500.
func TestMarkRead_NotFound(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")

	mark := func(id int64) int {
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		req.SetPathValue("id", fmt.Sprint(id))
		return dbtest.Serve(t, h.MarkRead, req, user.ID, "usuario").Code
	}
	if code := mark(9_000_000_003); code != http.StatusNotFound {
		t.Fatalf("unknown id: want 404, got %d", code)
	}
	rr := create(t, db, h, admin.ID, "rol:admin")
	var forAdmins Notificacion
	_ = json.Unmarshal(rr.Body.Bytes(), &forAdmins)
	if code := mark(forAdmins.ID); code != http.StatusNotFound {
		t.Fatalf("not addressed to user: want 404, got %d", code)
	}
	rr = create(t, db, h, admin.ID, "todos")
	var all Notificacion
	_ = json.Unmarshal(rr.Body.Bytes(), &all)
	for i := 0; i < 2; i++ { // idempotent
		if code := mark(all.ID); code != http.StatusNoContent {
			t.Fatalf("mark #%d: want 204, got %d", i, code)
		}
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM notificacion_leidas WHERE notificacion_id=$1 AND usuario_id=$2`, all.ID, user.ID).Scan(&n)
	if n != 1 {
		t.Fatalf("read marks = %d, want 1", n)
	}
}

// H55: deleting an unknown id → 404 and no audit row.
func TestAdminDelete_NotFound(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	const ghost = 9_000_000_004
	req := httptest.NewRequest(http.MethodDelete, "/x", nil)
	req.SetPathValue("id", fmt.Sprint(ghost))
	if rr := dbtest.Serve(t, h.AdminDelete, req, admin.ID, "admin"); rr.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d %s", rr.Code, rr.Body)
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE accion='eliminar_notificacion' AND entidad_id=$1`, ghost).Scan(&n)
	if n != 0 {
		t.Fatalf("audit written for a failed delete")
	}
}

// H29: 500s are logged with the operation name.
func TestInternalErrorsAreLogged(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = prev })

	h := NewHandler(NewRepository(dbtest.Closed(t)), nil)
	withID := func(method string) *http.Request {
		r := httptest.NewRequest(method, "/x", nil)
		r.SetPathValue("id", "1")
		return r
	}
	cases := []struct {
		name string
		fn   http.HandlerFunc
		req  *http.Request
	}{
		{"list", h.List, httptest.NewRequest(http.MethodGet, "/x", nil)},
		{"unread", h.UnreadCount, httptest.NewRequest(http.MethodGet, "/x", nil)},
		{"mark", h.MarkRead, withID(http.MethodPost)},
		{"mark_all", h.MarkAllRead, httptest.NewRequest(http.MethodPost, "/x", nil)},
		{"admin_list", h.AdminList, httptest.NewRequest(http.MethodGet, "/x", nil)},
		{"create", h.AdminCreate, httptest.NewRequest(http.MethodPost, "/x",
			bytes.NewBufferString(`{"titulo":"abc","cuerpo":"abc","tipo":"sistema","destinatario":"todos"}`))},
		{"delete", h.AdminDelete, withID(http.MethodDelete)},
	}
	for _, c := range cases {
		buf.Reset()
		rr := dbtest.Serve(t, c.fn, c.req, 1, "admin")
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("%s: want 500, got %d", c.name, rr.Code)
		}
		if out := buf.String(); !strings.Contains(out, `"level":"error"`) || !strings.Contains(out, `"op":"notifications.`) {
			t.Fatalf("%s: 500 not logged with op: %q", c.name, out)
		}
	}
}
