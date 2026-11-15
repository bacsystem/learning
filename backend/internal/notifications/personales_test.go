package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/jmoiron/sqlx"
)

// PLT-12: personal notifications (destinatario 'usuario') are visible only to
// their owner, and carry the clave and parametros the bell translates.

func seedPersonal(t *testing.T, db *sqlx.DB, usuarioID int64) int64 {
	t.Helper()
	var id int64
	err := db.QueryRow(`
		INSERT INTO notificaciones (titulo, cuerpo, tipo, destinatario, usuario_id, clave, parametros)
		VALUES ('Inscripción rechazada', 'cuerpo', 'sistema', 'usuario', $1, 'inscripcion_rechazada',
		        '{"proceso":"P","institucion":"I","motivo":"Documento ilegible"}')
		RETURNING id`, usuarioID).Scan(&id)
	if err != nil {
		t.Fatalf("seed personal: %v", err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM notificaciones WHERE id=$1`, id) })
	return id
}

func status(t *testing.T, fn http.HandlerFunc, id, userID int64, rol, method string) int {
	t.Helper()
	req := httptest.NewRequest(method, "/x", nil)
	req.SetPathValue("id", fmt.Sprint(id))
	return dbtest.Serve(t, fn, req, userID, rol).Code
}

func TestPersonales_SoloLasVeSuDueno(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	owner := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	other := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	id := seedPersonal(t, db, owner.ID)

	if !has(listFor(t, h, owner.ID, "usuario"), id) {
		t.Fatal("the owner does not see the personal notification")
	}
	if has(listFor(t, h, other.ID, "usuario"), id) {
		t.Fatal("another user sees a personal notification")
	}
	if has(listFor(t, h, admin.ID, "admin"), id) {
		t.Fatal("an admin sees a personal notification of someone else")
	}

	var unread = func(userID int64, rol string) int {
		rr := dbtest.Serve(t, h.UnreadCount, httptest.NewRequest(http.MethodGet, "/x", nil), userID, rol)
		var r UnreadCountResponse
		_ = json.Unmarshal(rr.Body.Bytes(), &r)
		return r.Count
	}
	before := unread(owner.ID, "usuario")
	if before < 1 {
		t.Fatalf("owner unread = %d, want at least 1", before)
	}
	if got := status(t, h.MarkRead, id, other.ID, "usuario", http.MethodPost); got != http.StatusNotFound {
		t.Fatalf("another user marks it read: want 404, got %d", got)
	}
	if got := status(t, h.MarkRead, id, owner.ID, "usuario", http.MethodPost); got != http.StatusNoContent && got != http.StatusOK {
		t.Fatalf("owner marks it read: got %d", got)
	}
	if after := unread(owner.ID, "usuario"); after != before-1 {
		t.Fatalf("owner unread = %d after reading, want %d", after, before-1)
	}
}

func TestPersonales_MarkAllReadNoToca(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	owner := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	other := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	id := seedPersonal(t, db, owner.ID)

	dbtest.Serve(t, h.MarkAllRead, httptest.NewRequest(http.MethodPost, "/x", nil), other.ID, "usuario")
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM notificacion_leidas WHERE notificacion_id=$1`, id).Scan(&n); err != nil || n != 0 {
		t.Fatalf("a stranger's mark-all-read touched the personal notification (n=%d, err=%v)", n, err)
	}
}

func TestPersonales_ClaveYParametrosEnElListado(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	owner := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	id := seedPersonal(t, db, owner.ID)
	broadcast := create(t, db, h, admin.ID, "rol:usuario")
	var b Notificacion
	_ = json.Unmarshal(broadcast.Body.Bytes(), &b)

	rr := dbtest.Serve(t, h.List, httptest.NewRequest(http.MethodGet, "/notificaciones", nil), owner.ID, "usuario")
	var raw []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, n := range raw {
		switch int64(n["id"].(float64)) {
		case id:
			seen++
			p, _ := n["parametros"].(map[string]any)
			if n["clave"] != "inscripcion_rechazada" || p["motivo"] != "Documento ilegible" || p["proceso"] != "P" {
				t.Fatalf("personal item = %v", n)
			}
		case b.ID:
			seen++
			if v, ok := n["clave"]; !ok || v != nil {
				t.Fatalf("broadcast clave must be present and null, got %v (present=%v)", v, ok)
			}
			if v, ok := n["parametros"]; !ok || v != nil {
				t.Fatalf("broadcast parametros must be present and null, got %v (present=%v)", v, ok)
			}
		}
	}
	if seen != 2 {
		t.Fatalf("saw %d of the 2 expected notifications", seen)
	}
}

func TestPersonales_AdminNoLasListaNiBorra(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	owner := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	id := seedPersonal(t, db, owner.ID)

	rr := dbtest.Serve(t, h.AdminList, httptest.NewRequest(http.MethodGet, "/admin/notificaciones", nil), admin.ID, "admin")
	var list []Notificacion
	_ = json.Unmarshal(rr.Body.Bytes(), &list)
	if has(list, id) {
		t.Fatal("the admin panel lists a personal notification")
	}
	if got := status(t, h.AdminDelete, id, admin.ID, "admin", http.MethodDelete); got != http.StatusNotFound {
		t.Fatalf("admin deletes a personal notification: want 404, got %d", got)
	}
	var n int
	_ = db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM notificaciones WHERE id=$1`, id).Scan(&n)
	if n != 1 {
		t.Fatal("the personal notification was deleted")
	}
}

func TestPersonales_ChecksDeLaBD(t *testing.T) {
	db, _ := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	u := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	for name, q := range map[string]string{
		"usuario sin usuario_id":      `INSERT INTO notificaciones (titulo, cuerpo, destinatario) VALUES ('t','c','usuario')`,
		"usuario_id sin destinatario": fmt.Sprintf(`INSERT INTO notificaciones (titulo, cuerpo, destinatario, usuario_id) VALUES ('t','c','todos',%d)`, u.ID),
		"parametros sin clave":        `INSERT INTO notificaciones (titulo, cuerpo, parametros) VALUES ('t','c','{}')`,
	} {
		if _, err := db.Exec(q); err == nil {
			t.Errorf("%s: the CHECK did not reject the row", name)
			_, _ = db.Exec(`DELETE FROM notificaciones WHERE titulo='t' AND cuerpo='c'`)
		}
	}
}
