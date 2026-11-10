// Package audittest holds test helpers for code that audits inside a
// transaction. It is imported only from _test.go files.
package audittest

import (
	"fmt"
	"testing"

	"github.com/jmoiron/sqlx"
)

// FailInsertsBy makes every audit_log INSERT attributed to usuarioID fail
// until the test ends, so a test can prove the audited mutation rolls back.
// The trigger only matches that user, so tests running in parallel (other
// packages share the database) are not affected. The returned func removes the
// trigger early (it is also removed at test end).
func FailInsertsBy(t *testing.T, db *sqlx.DB, usuarioID int64) (stop func()) {
	t.Helper()
	name := fmt.Sprintf("audittest_fail_%d", usuarioID)
	stmts := []string{
		fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$
			BEGIN RAISE EXCEPTION 'audittest: forced audit_log failure'; END $$`, name),
		fmt.Sprintf(`CREATE TRIGGER %s BEFORE INSERT ON audit_log
			FOR EACH ROW WHEN (NEW.usuario_id = %d) EXECUTE FUNCTION %s()`, name, usuarioID, name),
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("audittest: %v", err)
		}
	}
	stop = func() {
		_, _ = db.Exec(fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON audit_log`, name))
		_, _ = db.Exec(fmt.Sprintf(`DROP FUNCTION IF EXISTS %s()`, name))
	}
	t.Cleanup(stop)
	return stop
}

// Count returns the audit rows for accion on entidadID.
func Count(t *testing.T, db *sqlx.DB, accion string, entidadID int64) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE accion=$1 AND entidad_id=$2`,
		accion, entidadID).Scan(&n); err != nil {
		t.Fatalf("audittest count: %v", err)
	}
	return n
}
