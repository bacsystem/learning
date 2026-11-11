package testimonios

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func seedTestimonio(t *testing.T, db *sqlx.DB, userID int64, estado string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`INSERT INTO testimonios (usuario_id, contenido, calificacion, estado)
		VALUES ($1, 'Muy buena plataforma de estudio', 5, $2) RETURNING id`, userID, estado).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// H80 (supersedes H54): the PUBLIC endpoint shows only the first name and the
// initial of the apellido paterno ("Juan P.") and no grado; an undecryptable
// value is withheld instead of leaking ciphertext.
func TestGetPublicos_NombrePublicoAndDecrypt(t *testing.T) {
	db := dbtest.Open(t)
	h := NewHandler(NewService(NewRepository(db, dbtest.GCM(t))), nil)
	cat := dbtest.SeedCatalog(t, db)
	ok := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	bad := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	idOK := seedTestimonio(t, db, ok.ID, "aprobado")
	idBad := seedTestimonio(t, db, bad.ID, "aprobado")
	if _, err := db.Exec(`UPDATE personas SET nombres='leaky-ciphertext', apellido_paterno='leaky-ciphertext' WHERE id=$1`, bad.PersonaID); err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	h.GetPublicos(rr, httptest.NewRequest(http.MethodGet, "/testimonios/publico", nil))
	var raw []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	got := map[int64]map[string]any{}
	for _, it := range raw {
		got[int64(it["id"].(float64))] = it
	}
	if n := got[idOK]["nombre_publico"]; n != ok.Nombres+" P." {
		t.Fatalf("nombre_publico = %v, want %q", n, ok.Nombres+" P.")
	}
	for _, it := range raw {
		for _, k := range []string{"nombre_completo", "grado", "apellido_paterno", "username", "usuario_id"} {
			if _, present := it[k]; present {
				t.Fatalf("public testimonio exposes %q: %v", k, it)
			}
		}
		if strings.Contains(fmt.Sprint(it), "Paterno") {
			t.Fatalf("public testimonio leaks the full surname: %v", it)
		}
	}
	if n, present := got[idBad]["nombre_publico"]; !present || n != "" {
		t.Fatalf("undecryptable nombre = %v (present %v), want empty", n, present)
	}
}

// H80: the admin list keeps the full name and grado.
func TestListAdmin_KeepsFullData(t *testing.T) {
	db := dbtest.Open(t)
	h := NewHandler(NewService(NewRepository(db, dbtest.GCM(t))), nil)
	cat := dbtest.SeedCatalog(t, db)
	u := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	id := seedTestimonio(t, db, u.ID, "pendiente")
	rr := httptest.NewRecorder()
	h.ListAdmin(rr, httptest.NewRequest(http.MethodGet, "/admin/testimonios?estado=pendiente&per_page=100", nil))
	var out AdminListResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	for _, it := range out.Data {
		if it.ID == id {
			if it.NombreCompleto != u.Nombres+" Paterno" || it.Grado == "" {
				t.Fatalf("admin item lost data: %+v", it)
			}
			return
		}
	}
	t.Fatal("seeded testimonio not listed")
}

func TestNombrePublico(t *testing.T) {
	cases := []struct{ nombres, paterno, want string }{
		{"Juan Carlos", "Pérez", "Juan P."},
		{"  maría  ", "ñahui", "maría Ñ."},
		{"Ana", "", "Ana"},
		{"", "Quispe", ""},
		{"Luis", "  de la Cruz", "Luis D."},
	}
	for _, c := range cases {
		if got := nombrePublico(c.nombres, c.paterno); got != c.want {
			t.Errorf("nombrePublico(%q, %q) = %q, want %q", c.nombres, c.paterno, got, c.want)
		}
	}
}

// H53: a huge page never overflows the OFFSET into a 500.
func TestListAdmin_HugePage(t *testing.T) {
	db := dbtest.Open(t)
	h := NewHandler(NewService(NewRepository(db, dbtest.GCM(t))), nil)
	for _, q := range []string{"page=4611686018427387904", "page=99999999999999999999", "estado=aprobado&page=4611686018427387904"} {
		req := httptest.NewRequest(http.MethodGet, "/admin/testimonios?"+q, nil)
		rr := httptest.NewRecorder()
		h.ListAdmin(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: want 200, got %d %s", q, rr.Code, rr.Body)
		}
	}
}

// H29: 500s are logged with the operation name.
func TestInternalErrorsAreLogged(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = prev })

	h := NewHandler(NewService(NewRepository(dbtest.Closed(t), nil)), nil)
	patch := httptest.NewRequest(http.MethodPatch, "/x", bytes.NewBufferString(`{"estado":"aprobado"}`))
	patch.SetPathValue("id", "1")
	cases := []struct {
		name string
		fn   http.HandlerFunc
		req  *http.Request
	}{
		{"publicos", h.GetPublicos, httptest.NewRequest(http.MethodGet, "/x", nil)},
		{"mio", h.GetMio, httptest.NewRequest(http.MethodGet, "/x", nil)},
		{"submit", h.Submit, httptest.NewRequest(http.MethodPost, "/x",
			bytes.NewBufferString(fmt.Sprintf(`{"contenido":%q,"calificacion":4}`, "contenido valido aqui")))},
		{"list_admin", h.ListAdmin, httptest.NewRequest(http.MethodGet, "/x", nil)},
		{"update_estado", h.UpdateEstado, patch},
	}
	for _, c := range cases {
		buf.Reset()
		rr := dbtest.Serve(t, c.fn, c.req, 1, "admin")
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("%s: want 500, got %d", c.name, rr.Code)
		}
		if out := buf.String(); !strings.Contains(out, `"level":"error"`) || !strings.Contains(out, `"op":"testimonios.`) {
			t.Fatalf("%s: 500 not logged with op: %q", c.name, out)
		}
	}
}

// H138: testimonies with the same created_at are paginated in a total order
// (created_at DESC, id DESC): per_page=1 gives 3 different rows, with and
// without the estado filter.
func TestListAdmin_PaginationTieBreakByID(t *testing.T) {
	db := dbtest.Open(t)
	repo := NewRepository(db, dbtest.GCM(t))
	cat := dbtest.SeedCatalog(t, db)
	var ids []int64
	for i := 0; i < 3; i++ {
		u := dbtest.SeedUser(t, db, cat, "usuario", "activo")
		id := seedTestimonio(t, db, u.ID, "pendiente")
		// Far future so nothing else in the shared DB sorts before them.
		if _, err := db.Exec(`UPDATE testimonios SET created_at='2099-03-01T10:00:00Z' WHERE id=$1`, id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	for _, estado := range []string{"", "pendiente"} {
		for page := 1; page <= 3; page++ {
			rows, _, err := repo.ListAdmin(context.Background(), estado, page, 1)
			if err != nil {
				t.Fatal(err)
			}
			got := make([]int64, len(rows))
			for i, r := range rows {
				got[i] = r.ID
			}
			if len(got) != 1 || got[0] != ids[3-page] {
				t.Errorf("estado=%q page %d: ids %v, want [%d] (id DESC among equal created_at)", estado, page, got, ids[3-page])
			}
		}
	}
}
