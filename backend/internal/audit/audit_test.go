package audit

import (
	"context"
	"database/sql"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/cbaciliod/learning/internal/middleware"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func testDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Fatalf("db connect: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// H3: an invalid IP must not make the audit insert fail silently; the row is
// stored with ip = NULL.
func TestLog_InvalidIP_StoresRowWithNullIP(t *testing.T) {
	db := testDB(t)
	entidad := fmt.Sprintf("test_h3_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM audit_log WHERE entidad=$1`, entidad); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})

	New(db).Log(context.Background(), nil, "test", entidad, nil, nil, "x", "ua")

	var ip sql.NullString
	err := db.QueryRow(`SELECT ip::text FROM audit_log WHERE entidad=$1`, entidad).Scan(&ip)
	if err != nil {
		t.Fatalf("audit row not inserted: %v", err)
	}
	if ip.Valid {
		t.Errorf("want NULL ip for invalid input, got %q", ip.String)
	}
}

func TestLog_ValidIP_IsStored(t *testing.T) {
	db := testDB(t)
	entidad := fmt.Sprintf("test_h3v_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM audit_log WHERE entidad=$1`, entidad); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})

	New(db).Log(context.Background(), nil, "test", entidad, nil, map[string]any{"k": 1}, "203.0.113.7", "ua")

	var ip sql.NullString
	if err := db.QueryRow(`SELECT host(ip) FROM audit_log WHERE entidad=$1`, entidad).Scan(&ip); err != nil {
		t.Fatalf("audit row not inserted: %v", err)
	}
	if ip.String != "203.0.113.7" {
		t.Errorf("want 203.0.113.7, got %q", ip.String)
	}
}

// H3: X-Forwarded-For must be ignored unless the direct peer is a trusted proxy.
func TestClientIP_IgnoresXFFFromUntrustedPeer(t *testing.T) {
	middleware.SetTrustedProxies("")
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "198.51.100.10:5555"
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := ClientIP(r); got != "198.51.100.10" {
		t.Errorf("want peer IP 198.51.100.10, got %q", got)
	}
}

func TestClientIP_UsesXFFFromTrustedProxy(t *testing.T) {
	middleware.SetTrustedProxies("10.0.0.0/8")
	t.Cleanup(func() { middleware.SetTrustedProxies("") })
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.5:5555"
	r.Header.Set("X-Forwarded-For", "203.0.113.9")
	if got := ClientIP(r); got != "203.0.113.9" {
		t.Errorf("want 203.0.113.9, got %q", got)
	}
}

func TestClientIP_InvalidXFFFromTrustedProxy_FallsBackToPeer(t *testing.T) {
	middleware.SetTrustedProxies("10.0.0.0/8")
	t.Cleanup(func() { middleware.SetTrustedProxies("") })
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.5:5555"
	r.Header.Set("X-Forwarded-For", "x")
	if got := ClientIP(r); got != "10.0.0.5" {
		t.Errorf("want 10.0.0.5, got %q", got)
	}
}

// H3: a cancelled request context (client disconnect after the mutation) must
// not drop the audit row.
func TestLog_CancelledContext_StillWritesRow(t *testing.T) {
	db := testDB(t)
	entidad := fmt.Sprintf("test_h3c_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM audit_log WHERE entidad=$1`, entidad); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	New(db).Log(ctx, nil, "test", entidad, nil, nil, "203.0.113.7", "ua")

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE entidad=$1`, entidad).Scan(&n); err != nil || n != 1 {
		t.Fatalf("want 1 audit row with a cancelled ctx, got %d (%v)", n, err)
	}
}

// LogTx writes in the caller's transaction: the row exists only if the
// transaction commits, and an invalid IP is stored as NULL like Log does.
func TestLogTx_FollowsCallerTx(t *testing.T) {
	db := testDB(t)
	entidad := fmt.Sprintf("test_logtx_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM audit_log WHERE entidad=$1`, entidad); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})

	tx, err := db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	if err := New(db).LogTx(context.Background(), tx, nil, "test", entidad, nil, nil, "x", "ua"); err != nil {
		t.Fatalf("LogTx: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE entidad=$1`, entidad).Scan(&n); err != nil || n != 0 {
		t.Fatalf("want 0 rows after rollback, got %d (%v)", n, err)
	}

	tx, err = db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	if err := New(db).LogTx(context.Background(), tx, nil, "test", entidad, nil, map[string]any{"k": 1}, "x", "ua"); err != nil {
		t.Fatalf("LogTx: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var ip sql.NullString
	if err := db.QueryRow(`SELECT ip::text FROM audit_log WHERE entidad=$1`, entidad).Scan(&ip); err != nil {
		t.Fatalf("want 1 row after commit: %v", err)
	}
	if ip.Valid {
		t.Errorf("invalid IP must be stored as NULL, got %q", ip.String)
	}
}

// LogTx returns the insert error instead of swallowing it; a nil Logger
// audits nothing.
func TestLogTx_ReturnsInsertError(t *testing.T) {
	db := testDB(t)
	tx, err := db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	_ = tx.Rollback()
	if err := New(db).LogTx(context.Background(), tx, nil, "test", "x", nil, nil, "", ""); err == nil {
		t.Fatal("want error from a finished transaction, got nil")
	}
	var nilLogger *Logger
	if err := nilLogger.LogTx(context.Background(), tx, nil, "test", "x", nil, nil, "", ""); err != nil {
		t.Fatalf("nil Logger must audit nothing, got %v", err)
	}
}
