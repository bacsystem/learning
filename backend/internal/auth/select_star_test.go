package auth

import (
	"os"
	"regexp"
	"testing"
)

// Migrations are applied before the new binary starts and a rollback runs the
// old one against the migrated schema, so reads of usuarios must name their
// columns: sqlx fails on a column the struct does not map (migration 069 added
// idioma, which the previous release's SELECT * could not scan).
func TestRepository_NoSelectStarFromUsuarios(t *testing.T) {
	src, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`(?is)select\s+(?:\w+\.)?\*\s+from\s+usuarios\b`)
	if m := re.Find(src); m != nil {
		t.Fatalf("repository.go reads usuarios with %q; list the columns explicitly", m)
	}
}
