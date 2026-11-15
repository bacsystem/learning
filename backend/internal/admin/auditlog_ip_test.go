package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/cbaciliod/learning/internal/db/dbtest"
)

// The audit log returns the client IP as a bare address: the inet column
// must not leak its netmask ("172.18.0.1/32").
func TestAuditLog_IPHasNoMask(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")

	tests := []struct {
		name   string
		stored string
		want   string
	}{
		{"ipv4", "172.18.0.1", "172.18.0.1"},
		{"ipv6", "2001:db8::1", "2001:db8::1"},
		{"ipv6 loopback", "::1", "::1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actor := dbtest.SeedUser(t, db, cat, "usuario", "activo")
			if _, err := db.Exec(
				`INSERT INTO audit_log (usuario_id, accion, entidad, ip) VALUES ($1, 'ip_mask_test', 'x', $2::inet)`,
				actor.ID, tt.stored); err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodGet, "/admin/audit-log?q="+url.QueryEscape(actor.Username), nil)
			rr := dbtest.Serve(t, h.handleGetAuditLog, req, admin.ID, "admin")
			var out AuditLogResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
			if len(out.Data) != 1 {
				t.Fatalf("got %d rows, want 1", len(out.Data))
			}
			if got := out.Data[0].IP; got != tt.want {
				t.Errorf("ip = %q, want %q", got, tt.want)
			}
		})
	}
}
