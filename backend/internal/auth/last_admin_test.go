package auth

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

// ── H15 / H88: never zero active admins, deterministic and isolated ─────────

// isolatedUsuariosRepo returns a Repository whose connections use a fresh
// schema holding only a copy of the usuarios table (LIKE … INCLUDING ALL, no
// foreign keys), so UpdateEstado's "all active admins" lock sees only the
// admins this test creates, whatever the shared database contains. The
// schema is dropped on cleanup.
func isolatedUsuariosRepo(t *testing.T) *Repository {
	t.Helper()
	base := testDB(t)
	schema := fmt.Sprintf("test_lastadmin_%d_%d", time.Now().UnixNano()%1e9, rand.Intn(1e6))
	if _, err := base.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := base.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); err != nil {
			t.Errorf("drop schema: %v", err)
		}
	})
	if _, err := base.Exec(`CREATE TABLE ` + schema + `.usuarios (LIKE public.usuarios INCLUDING ALL)`); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema) // lib/pq sends it as a run-time parameter
	u.RawQuery = q.Encode()
	db, err := sqlx.Connect("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(4)
	t.Cleanup(func() { _ = db.Close() })
	var sp string
	if err := db.QueryRow(`SELECT current_schema()`).Scan(&sp); err != nil || sp != schema {
		t.Fatalf("search_path not applied: %q %v", sp, err)
	}
	return NewRepository(db, nil, testRawKey(t))
}

// Two active admins deactivate each other at the same time. The first
// transaction to lock the admin rows is held (test seam afterAdminLock)
// until the second one also reaches that point or 500 ms pass: with the
// FOR UPDATE lock the second one is blocked in its SELECT and, once the
// first commits, sees a single active admin (ErrLastAdmin). Without the
// lock both would see two admins and both deactivations would succeed.
func TestUpdateEstado_ConcurrentMutualDeactivation_LeavesOneAdmin(t *testing.T) {
	repo := isolatedUsuariosRepo(t)
	var a, b int64
	for i, p := range []*int64{&a, &b} {
		if err := repo.db.QueryRow(`INSERT INTO usuarios (persona_id, username, password_hash, rol, estado)
			VALUES ($1, $2, 'x', 'admin', 'activo') RETURNING id`, 900000000+i, fmt.Sprintf("lastadm%d", i)).Scan(p); err != nil {
			t.Fatal(err)
		}
	}

	for round := 0; round < 3; round++ {
		if _, err := repo.db.Exec(`UPDATE usuarios SET estado='activo'`); err != nil {
			t.Fatal(err)
		}
		var mu sync.Mutex
		arrived := 0
		both := make(chan struct{})
		repo.afterAdminLock = func() {
			mu.Lock()
			arrived++
			if arrived == 2 {
				close(both)
			}
			mu.Unlock()
			select {
			case <-both:
			case <-time.After(500 * time.Millisecond):
			}
		}

		var wg sync.WaitGroup
		start := make(chan struct{})
		errs := make([]error, 2)
		for i, p := range [][2]int64{{a, b}, {b, a}} {
			wg.Add(1)
			go func(i int, actor, target int64) {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				<-start
				errs[i] = repo.UpdateEstado(ctx, target, "inactivo", actor, nil)
			}(i, p[0], p[1])
		}
		close(start)
		wg.Wait()

		var active int
		if err := repo.db.QueryRow(`SELECT COUNT(*) FROM usuarios WHERE rol='admin' AND estado='activo'`).Scan(&active); err != nil {
			t.Fatal(err)
		}
		lastAdmin := 0
		for _, err := range errs {
			if errors.Is(err, ErrLastAdmin) {
				lastAdmin++
			} else if err != nil {
				t.Fatalf("round %d: unexpected error: %v", round, err)
			}
		}
		if active != 1 || lastAdmin != 1 {
			t.Fatalf("round %d: want 1 active admin and one ErrLastAdmin, got active=%d ErrLastAdmin=%d (%v)",
				round, active, lastAdmin, errs)
		}
	}
}

// The last active admin can never be moved out of "activo", sequentially.
func TestUpdateEstado_LastActiveAdmin_Refused(t *testing.T) {
	repo := isolatedUsuariosRepo(t)
	var a, other int64
	if err := repo.db.QueryRow(`INSERT INTO usuarios (persona_id, username, password_hash, rol, estado)
		VALUES (900000010, 'solo', 'x', 'admin', 'activo') RETURNING id`).Scan(&a); err != nil {
		t.Fatal(err)
	}
	if err := repo.db.QueryRow(`INSERT INTO usuarios (persona_id, username, password_hash, rol, estado)
		VALUES (900000011, 'actor', 'x', 'admin', 'inactivo') RETURNING id`).Scan(&other); err != nil {
		t.Fatal(err)
	}
	for _, estado := range []string{"inactivo", "rechazado"} {
		if err := repo.UpdateEstado(context.Background(), a, estado, other, nil); !errors.Is(err, ErrLastAdmin) {
			t.Fatalf("%s the last active admin: want ErrLastAdmin, got %v", estado, err)
		}
	}
}
