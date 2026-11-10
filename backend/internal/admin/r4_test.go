package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cbaciliod/learning/internal/db/dbtest"
)

type actividadResp struct {
	Practicas  []DiaMetricaItem        `json:"practicas_ultima_semana"`
	Simulacros []DiaMetricaItem        `json:"simulacros_ultima_semana"`
	Top        []AsignaturaMetricaItem `json:"top_asignaturas"`
}

func getActividad(t *testing.T, h *Handler, adminID int64) actividadResp {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/actividad", nil)
	rr := dbtest.Serve(t, h.handleDashboardActividad, req, adminID, "admin")
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", rr.Code, rr.Body)
	}
	var out actividadResp
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// H141: top_asignaturas counts only the questions of completed practices,
// the same rule as every other activity metric of the dashboard.
func TestDashboard_TopAsignaturasOnlyCompleted(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	proc := dbtest.SeedProceso(t, db, cat, 2097, "activo")
	done := dbtest.SeedAsignatura(t, db)
	discarded := dbtest.SeedAsignatura(t, db)
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM practica_asignaturas WHERE asignatura_id IN ($1,$2)`, done, discarded)
	})
	// Huge counts so the shared test DB cannot push them out of the top 5.
	for _, p := range []struct {
		estado string
		asig   int64
		n      int
	}{
		{"completada", done, 50_000_000},
		{"en_curso", discarded, 80_000_000},
		{"eliminada", discarded, 80_000_000},
		{"abandonada", discarded, 80_000_000},
	} {
		var pid int64
		if err := db.QueryRow(`INSERT INTO practicas (usuario_id, institucion_id, proceso_id, modo_calificacion, estado)
			VALUES ($1,$2,$3,'inmediato',$4) RETURNING id`, user.ID, cat.InstitucionID, proc, p.estado).Scan(&pid); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO practica_asignaturas (practica_id, asignatura_id, numero_preguntas)
			VALUES ($1,$2,$3)`, pid, p.asig, p.n); err != nil {
			t.Fatal(err)
		}
	}
	var doneName, discardedName string
	_ = db.QueryRow(`SELECT nombre FROM asignaturas WHERE id=$1`, done).Scan(&doneName)
	_ = db.QueryRow(`SELECT nombre FROM asignaturas WHERE id=$1`, discarded).Scan(&discardedName)

	top := getActividad(t, h, admin.ID).Top
	got := map[string]int64{}
	for _, it := range top {
		got[it.Nombre] = it.Total
	}
	if got[doneName] != 50_000_000 {
		t.Errorf("completed asignatura total = %d, want 50000000 (top %v)", got[doneName], top)
	}
	if n, ok := got[discardedName]; ok {
		t.Errorf("asignatura of non-completed practices in the top with %d questions (top %v)", n, top)
	}
}

// H142: the weekly series always carries the 7 Lima days, with 0 for the
// days without activity, in ascending order.
func TestDashboard_WeeklySeriesZeroFilled(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	proc := dbtest.SeedProceso(t, db, cat, 2097, "activo")
	// now = 15/03/2043 22:00 Lima (16/03 03:00 UTC): a week nobody else uses.
	h.repo.now = func() time.Time { return mustTime(t, "2043-03-16T03:00:00Z") }
	if _, err := db.Exec(`INSERT INTO practicas (usuario_id, institucion_id, proceso_id, modo_calificacion, estado, created_at)
		VALUES ($1,$2,$3,'inmediato','completada','2043-03-12T15:00:00Z')`, user.ID, cat.InstitucionID, proc); err != nil {
		t.Fatal(err)
	}
	act := getActividad(t, h, admin.ID)
	want := []string{"2043-03-09", "2043-03-10", "2043-03-11", "2043-03-12", "2043-03-13", "2043-03-14", "2043-03-15"}
	for name, serie := range map[string][]DiaMetricaItem{"practicas": act.Practicas, "simulacros": act.Simulacros} {
		if len(serie) != len(want) {
			t.Fatalf("%s: %d points %v, want the 7 days %v", name, len(serie), serie, want)
		}
		for i, it := range serie {
			if it.Fecha != want[i] {
				t.Fatalf("%s[%d] = %s, want %s (%v)", name, i, it.Fecha, want[i], serie)
			}
		}
	}
	for i, it := range act.Practicas {
		wantTotal := int64(0)
		if it.Fecha == "2043-03-12" {
			wantTotal = 1
		}
		if it.Total != wantTotal {
			t.Errorf("practicas[%d] %s = %d, want %d", i, it.Fecha, it.Total, wantTotal)
		}
	}
}

// H142: the monthly revenue series always carries the 12 Lima months, with
// "0.00" for the months without approvals.
func TestIngresos_SerieMensualZeroFilled(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	proc := dbtest.SeedProceso(t, db, cat, 2097, "activo")
	// now = 31/05/2043 23:00 Lima (01/06 04:00 UTC): still May in Lima.
	h.repo.now = func() time.Time { return mustTime(t, "2043-06-01T04:00:00Z") }
	if _, err := db.Exec(`INSERT INTO inscripciones (usuario_id, proceso_id, estado, monto, revisado_at, revisado_por)
		VALUES ($1, $2, 'aprobado', 12.30, '2043-01-10T15:00:00Z', $3)`, user.ID, proc, admin.ID); err != nil {
		t.Fatal(err)
	}
	out, _ := getIngresos(t, h, admin.ID)
	if len(out.SerieMensual) != 12 {
		t.Fatalf("%d months %v, want 12", len(out.SerieMensual), out.SerieMensual)
	}
	for i, it := range out.SerieMensual {
		wantMes := fmt.Sprintf("%04d-%02d", 2042+(5+i)/12, (5+i)%12+1) // 2042-06 … 2043-05
		if it.Mes != wantMes {
			t.Fatalf("serie[%d] = %s, want %s (%v)", i, it.Mes, wantMes, out.SerieMensual)
		}
		if !moneyRe.MatchString(it.Total) {
			t.Fatalf("serie[%d] total %q, want a decimal string", i, it.Total)
		}
		if it.Mes != "2043-01" && it.Total != "0.00" {
			t.Errorf("serie %s = %s, want 0.00", it.Mes, it.Total)
		}
	}
	if got := mes(out, "2043-01"); got != "12.30" {
		t.Errorf("serie 2043-01 = %s, want 12.30", got)
	}
}

// H138: rows with the same created_at are paginated in a total order
// (created_at DESC, id DESC): per_page=1 gives 3 different rows.
func TestAuditLog_PaginationTieBreakByID(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	var ids []int64
	for i := 0; i < 3; i++ {
		var id int64
		if err := db.QueryRow(`INSERT INTO audit_log (usuario_id, accion, entidad, created_at)
			VALUES ($1, 'tie_test', 'x', '2031-02-03T10:00:00Z') RETURNING id`, admin.ID).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	seen := map[int64]bool{}
	for page := 1; page <= 3; page++ {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/audit-log?per_page=1&page=%d&q=%s", page, admin.Username), nil)
		rr := dbtest.Serve(t, h.handleGetAuditLog, req, admin.ID, "admin")
		var out AuditLogResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if len(out.Data) != 1 {
			t.Fatalf("page %d: %d rows (%s)", page, len(out.Data), rr.Body)
		}
		id := out.Data[0].ID
		if want := ids[3-page]; id != want {
			t.Errorf("page %d: id %d, want %d (id DESC among equal created_at)", page, id, want)
		}
		seen[id] = true
	}
	if len(seen) != 3 {
		t.Fatalf("3 pages returned %d distinct rows", len(seen))
	}
}
