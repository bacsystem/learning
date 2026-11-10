package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cbaciliod/learning/internal/crypto"
)

// All repository tests require a live database (via testDB) and skip otherwise.

func testRepo(t *testing.T) *Repository {
	t.Helper()
	db := testDB(t)
	gcm, err := crypto.NewCipher(testHexKey)
	if err != nil {
		t.Fatalf("gcm: %v", err)
	}
	return NewRepository(db, gcm, testRawKey(t))
}

func TestRepository_FindPersonaByDNI_NotFound(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()
	_, err := repo.FindPersonaByDNI(ctx, "00000000")
	if err == nil {
		t.Error("expected error for non-existent DNI")
	}
}

func TestRepository_FindByID_NotFound(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()
	u, err := repo.FindByID(ctx, 99999999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u != nil {
		t.Error("expected nil for non-existent user")
	}
}

func TestRepository_FindByUsername_NotFound(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()
	u, err := repo.FindByUsername(ctx, "nonexistent_user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u != nil {
		t.Error("expected nil for non-existent username")
	}
}

func TestRepository_ResetPasswordWithToken_NotFound(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()
	_, err := repo.ResetPasswordWithToken(ctx, "0000000000000000000000000000000000000000000000000000000000000000", "x")
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("want ErrInvalidToken, got %v", err)
	}
}

func TestRepository_ExistsByUsername_False(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()
	exists, err := repo.ExistsByUsername(ctx, "definitely_not_existing_user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected false for non-existent username")
	}
}

func TestRepository_BatchInsert_EmptySlice(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()
	inserted, duplicates, errs, err := repo.BatchInsert(ctx, []BatchPersonaItem{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if inserted != 0 || duplicates != 0 || len(errs) != 0 {
		t.Errorf("expected all zeros for empty batch, got ins=%d dup=%d errs=%v", inserted, duplicates, errs)
	}
}

func TestRepository_AuditLog_WriteDoesNotPanic(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()
	entidad := fmt.Sprintf("test_wa_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		if _, err := repo.db.Exec(`DELETE FROM audit_log WHERE entidad=$1`, entidad); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	// WriteAudit logs errors instead of returning them; an invalid IP is stored as NULL.
	repo.WriteAudit(ctx, AuditEntry{Accion: "test", Entidad: entidad, IP: "127.0.0.1"})
	repo.WriteAudit(ctx, AuditEntry{Accion: "test", Entidad: entidad, IP: "not-an-ip"})
	var n int
	if err := repo.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE entidad=$1`, entidad).Scan(&n); err != nil || n != 2 {
		t.Fatalf("want 2 audit rows, got %d (%v)", n, err)
	}
}

func TestRepository_CreateSesion_InvalidatesOnDelete(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()
	// InvalidateSesionByHash on a non-existent hash should not error
	err := repo.InvalidateSesionByHash(ctx, "nonexistenthash")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRepository_InvalidatePrevTokens_NoRows(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()
	// Should succeed even when no rows match
	err := repo.InvalidatePreviosTokens(ctx, 99999999)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRepository_InvalidateAllSesiones_NoRows(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()
	err := repo.InvalidateAllSesiones(ctx, 99999999)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRepository_RegisterFailedLogin_UnknownUser(t *testing.T) {
	repo := testRepo(t)
	ctx := context.Background()
	_, locked, err := repo.RegisterFailedLogin(ctx, 99999999, 5, 15*time.Minute)
	if !errors.Is(err, sql.ErrNoRows) || locked {
		t.Errorf("want sql.ErrNoRows and not locked, got locked=%v err=%v", locked, err)
	}
}

// H3: WriteAudit must survive a cancelled request context.
func TestRepository_WriteAudit_CancelledContext(t *testing.T) {
	repo := testRepo(t)
	entidad := fmt.Sprintf("test_wac_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		if _, err := repo.db.Exec(`DELETE FROM audit_log WHERE entidad=$1`, entidad); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo.WriteAudit(ctx, AuditEntry{Accion: "test", Entidad: entidad, IP: "127.0.0.1"})
	var n int
	if err := repo.db.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE entidad=$1`, entidad).Scan(&n); err != nil || n != 1 {
		t.Fatalf("want 1 audit row, got %d (%v)", n, err)
	}
}
