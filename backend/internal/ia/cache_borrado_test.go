package ia

import (
	"context"
	"testing"
	"time"

	"github.com/cbaciliod/learning/internal/db/dbtest"
)

// A stale entry is deleted by what was read: if a fresh one was written under
// the same key in between, the delete leaves it alone.
func TestPgCache_BorrarEntradaLeidaNoBorraUnaNueva(t *testing.T) {
	db := dbtest.Open(t)
	clave := "borrado" + dbtest.Uniq()
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM ia_cache WHERE clave = $1`, clave) })
	c := NewPgCache(db)
	viejaCreada := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := db.Exec(`INSERT INTO ia_cache (clave, operacion, salida, expira_en, created_at) VALUES ($1, 'clasificar', '{"v":"vieja"}', $2, $3)`,
		clave, viejaCreada.Add(time.Hour), viejaCreada); err != nil {
		t.Fatal(err)
	}
	// Another request replaces the stale entry before this one deletes it.
	if _, err := db.Exec(`DELETE FROM ia_cache WHERE clave = $1`, clave); err != nil {
		t.Fatal(err)
	}
	nueva := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	if _, err := db.Exec(`INSERT INTO ia_cache (clave, operacion, salida, expira_en, created_at) VALUES ($1, 'clasificar', '{"v":"nueva"}', $2, $3)`,
		clave, nueva.Add(time.Hour), nueva); err != nil {
		t.Fatal(err)
	}
	if err := c.borrarEntradaLeida(context.Background(), clave, viejaCreada); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.Get(&n, `SELECT COUNT(*) FROM ia_cache WHERE clave = $1`, clave); err != nil || n != 1 {
		t.Fatalf("the fresh entry was deleted along with the stale one (rows = %d, %v)", n, err)
	}
	if err := c.borrarEntradaLeida(context.Background(), clave, nueva); err != nil {
		t.Fatal(err)
	}
	if err := db.Get(&n, `SELECT COUNT(*) FROM ia_cache WHERE clave = $1`, clave); err != nil || n != 0 {
		t.Errorf("the entry that was read is still there (rows = %d, %v)", n, err)
	}
}
