package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/cbaciliod/learning/internal/db/dbtest"
)

// The audit log search is a literal substring: `%`, `_` and `\` in q match
// themselves, not "anything" or "any one character".
func TestAuditLog_SearchEscapesLikeMetacharacters(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	suffix := dbtest.Uniq()
	usernames := []string{"lk" + suffix + "a_b", "lk" + suffix + "axb", "lk" + suffix + "50%", "lk" + suffix + "500", `lk` + suffix + `c\d`}
	for _, name := range usernames {
		u := dbtest.SeedUser(t, db, cat, "admin", "activo")
		if _, err := db.Exec(`UPDATE usuarios SET username = $1 WHERE id = $2`, name, u.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO audit_log (usuario_id, accion, entidad) VALUES ($1, 'like_test', 'x')`, u.ID); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name string
		q    string
		want []string
	}{
		{"underscore is literal", "lk" + suffix + "a_b", []string{usernames[0]}},
		{"percent is literal", "lk" + suffix + "50%", []string{usernames[2]}},
		{"backslash is literal", `lk` + suffix + `c\d`, []string{usernames[4]}},
		{"plain prefix still matches by substring", "lk" + suffix + "a", []string{usernames[0], usernames[1]}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin/audit-log?q="+url.QueryEscape(tt.q), nil)
			rr := dbtest.Serve(t, h.handleGetAuditLog, req, admin.ID, "admin")
			var out AuditLogResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
			got := map[string]bool{}
			for _, row := range out.Data {
				got[row.Username] = true
			}
			if len(got) != len(tt.want) {
				t.Fatalf("usernames %v, want %v", got, tt.want)
			}
			for _, w := range tt.want {
				if !got[w] {
					t.Errorf("missing %s in %v", w, got)
				}
			}
		})
	}
}
