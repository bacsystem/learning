package notifications

import (
	"context"
	"sync"
	"testing"

	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/jmoiron/sqlx"
)

// seedNotif inserts a notification for everybody with a fixed created_at.
func seedNotif(t *testing.T, db *sqlx.DB, adminID int64, createdAt string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`INSERT INTO notificaciones (titulo, cuerpo, tipo, destinatario, creado_por, created_at)
		VALUES ($1, 'cuerpo', 'sistema', 'todos', $2, $3::timestamptz) RETURNING id`,
		"Aviso "+dbtest.Uniq(), adminID, createdAt).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM notificaciones WHERE id=$1`, id) })
	return id
}

// H138: notifications with the same created_at come in a total order
// (created_at DESC, id DESC), in the admin list and in the user list.
func TestList_TieBreakByID(t *testing.T) {
	db := dbtest.Open(t)
	repo := NewRepository(db)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	// Far future so nothing else in the shared DB sorts before them.
	var ids []int64
	for i := 0; i < 3; i++ {
		ids = append(ids, seedNotif(t, db, admin.ID, "2099-06-01T12:00:00Z"))
	}
	want := []int64{ids[2], ids[1], ids[0]}
	check := func(name string, got []Notificacion) {
		t.Helper()
		if len(got) < 3 {
			t.Fatalf("%s: %d rows", name, len(got))
		}
		for i, w := range want {
			if got[i].ID != w {
				t.Errorf("%s[%d] = %d, want %d (id DESC among equal created_at)", name, i, got[i].ID, w)
			}
		}
	}
	all, err := repo.ListAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	check("ListAll", all)
	mine, err := repo.ListForUser(context.Background(), user.ID, "usuario")
	if err != nil {
		t.Fatal(err)
	}
	check("ListForUser", mine)
}

// H139: concurrent "mark all as read" requests of the same user must not
// collide on the (notificacion_id, usuario_id) key.
func TestMarkAllRead_Concurrent(t *testing.T) {
	db := dbtest.Open(t)
	repo := NewRepository(db)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	for i := 0; i < 30; i++ {
		seedNotif(t, db, admin.ID, "2099-06-02T12:00:00Z")
	}
	for round := 0; round < 5; round++ {
		user := dbtest.SeedUser(t, db, cat, "usuario", "activo") // nothing read yet
		var wg sync.WaitGroup
		errs := make(chan error, 8)
		start := make(chan struct{})
		for g := 0; g < 8; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				errs <- repo.MarkAllRead(context.Background(), user.ID, "usuario")
			}()
		}
		close(start)
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("round %d: concurrent MarkAllRead: %v", round, err)
			}
		}
		if n, err := repo.UnreadCount(context.Background(), user.ID, "usuario"); err != nil || n != 0 {
			t.Fatalf("round %d: unread after MarkAllRead = %d (%v)", round, n, err)
		}
	}
}
