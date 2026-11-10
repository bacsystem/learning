package admin

import (
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/cbaciliod/learning/internal/db/dbtest"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// rat parses an exact decimal string.
func rat(t *testing.T, s string) *big.Rat {
	t.Helper()
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		t.Fatalf("not a decimal: %q", s)
	}
	return r
}

// delta returns after-before as an exact decimal with 2 places.
func delta(t *testing.T, before, after string) string {
	t.Helper()
	return new(big.Rat).Sub(rat(t, after), rat(t, before)).FloatString(2)
}

func getIngresos(t *testing.T, h *Handler, adminID int64) (IngresosResponse, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/admin/ingresos", nil)
	rr := dbtest.Serve(t, h.handleGetIngresos, req, adminID, "admin")
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", rr.Code, rr.Body)
	}
	var out IngresosResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &raw)
	return out, raw
}

func mes(r IngresosResponse, m string) string {
	for _, it := range r.SerieMensual {
		if it.Mes == m {
			return it.Total
		}
	}
	return "0.00"
}

var moneyRe = regexp.MustCompile(`^[0-9]+\.[0-9]{2}$`)

// H66: revenue totals travel as exact decimal strings (no float64).
func TestIngresos_MoneyAsDecimalStrings(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	proc := dbtest.SeedProceso(t, db, cat, 2097, "activo")
	before, _ := getIngresos(t, h, admin.ID)
	// 0.1 + 0.2 is the classic float trap.
	for _, m := range []string{"0.10", "0.20", "12345678.91"} {
		u := dbtest.SeedUser(t, db, cat, "usuario", "activo")
		if _, err := db.Exec(`INSERT INTO inscripciones (usuario_id, proceso_id, estado, monto, revisado_at, revisado_por)
			VALUES ($1, $2, 'aprobado', $3::numeric, NOW(), $4)`, u.ID, proc, m, admin.ID); err != nil {
			t.Fatal(err)
		}
	}
	_ = user
	after, raw := getIngresos(t, h, admin.ID)
	for _, k := range []string{"total_recaudado", "total_este_anio", "total_este_mes"} {
		s, ok := raw[k].(string)
		if !ok || !moneyRe.MatchString(s) {
			t.Fatalf("%s = %#v, want a decimal string with 2 places", k, raw[k])
		}
	}
	if d := delta(t, before.TotalRecaudado, after.TotalRecaudado); d != "12345679.21" {
		t.Fatalf("total delta = %s, want 12345679.21", d)
	}
	serie := raw["serie_mensual"].([]any)
	if len(serie) == 0 {
		t.Fatal("empty serie_mensual")
	}
	for _, it := range serie {
		if s, ok := it.(map[string]any)["total"].(string); !ok || !moneyRe.MatchString(s) {
			t.Fatalf("serie total = %#v, want decimal string", it)
		}
	}
}

// H79: months and years follow the business calendar (America/Lima), not UTC.
// An approval on 31/10 20:00 Lima (= 01/11 01:00 UTC) belongs to October.
func TestIngresos_BusinessTimezone(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	proc := dbtest.SeedProceso(t, db, cat, 2097, "activo")
	approve := func(monto, at string) {
		t.Helper()
		u := dbtest.SeedUser(t, db, cat, "usuario", "activo")
		if _, err := db.Exec(`INSERT INTO inscripciones (usuario_id, proceso_id, estado, monto, revisado_at, revisado_por)
			VALUES ($1, $2, 'aprobado', $3::numeric, $4::timestamptz, $5)`, u.ID, proc, monto, at, admin.ID); err != nil {
			t.Fatal(err)
		}
	}

	// now = 31/10/2030 22:00 Lima
	h.repo.now = func() time.Time { return mustTime(t, "2030-11-01T03:00:00Z") }
	before, _ := getIngresos(t, h, admin.ID)
	approve("100.10", "2030-11-01T01:00:00Z") // 31/10 20:00 Lima → October
	approve("200.20", "2030-10-01T04:00:00Z") // 30/09 23:00 Lima → September
	after, _ := getIngresos(t, h, admin.ID)
	if d := delta(t, before.TotalEsteMes, after.TotalEsteMes); d != "100.10" {
		t.Fatalf("este mes delta = %s, want 100.10", d)
	}
	if d := delta(t, before.TotalEsteAnio, after.TotalEsteAnio); d != "300.30" {
		t.Fatalf("este año delta = %s, want 300.30", d)
	}
	if d := delta(t, mes(before, "2030-10"), mes(after, "2030-10")); d != "100.10" {
		t.Fatalf("serie 2030-10 delta = %s, want 100.10", d)
	}
	if d := delta(t, mes(before, "2030-09"), mes(after, "2030-09")); d != "200.20" {
		t.Fatalf("serie 2030-09 delta = %s, want 200.20", d)
	}
	if d := delta(t, mes(before, "2030-11"), mes(after, "2030-11")); d != "0.00" {
		t.Fatalf("serie 2030-11 delta = %s, want 0.00 (no November yet in Lima)", d)
	}

	// Year boundary: now = 31/12/2030 23:00 Lima, approval 31/12 21:00 Lima.
	h.repo.now = func() time.Time { return mustTime(t, "2031-01-01T04:00:00Z") }
	before, _ = getIngresos(t, h, admin.ID)
	approve("5.05", "2031-01-01T02:00:00Z")
	after, _ = getIngresos(t, h, admin.ID)
	if d := delta(t, before.TotalEsteAnio, after.TotalEsteAnio); d != "5.05" {
		t.Fatalf("este año delta at year end = %s, want 5.05", d)
	}
}

// H79: "today" on the dashboard is the Lima day, and the weekly series groups
// by Lima day.
func TestDashboard_TodayIsBusinessDay(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	proc := dbtest.SeedProceso(t, db, cat, 2097, "activo")
	h.repo.now = func() time.Time { return mustTime(t, "2030-11-01T03:00:00Z") } // 31/10 22:00 Lima

	get := func() DashboardResponse {
		req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
		rr := dbtest.Serve(t, h.handleDashboard, req, admin.ID, "admin")
		if rr.Code != http.StatusOK {
			t.Fatalf("want 200, got %d %s", rr.Code, rr.Body)
		}
		var out DashboardResponse
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
		return out
	}
	before := get()
	for _, at := range []string{
		"2030-10-31T04:00:00Z", // 30/10 23:00 Lima — yesterday
		"2030-10-31T06:00:00Z", // 31/10 01:00 Lima — today
		"2030-10-31T20:00:00Z", // 31/10 15:00 Lima — today
		"2030-11-01T05:30:00Z", // 01/11 00:30 Lima — tomorrow
	} {
		if _, err := db.Exec(`INSERT INTO practicas (usuario_id, institucion_id, proceso_id, modo_calificacion, estado, created_at)
			VALUES ($1,$2,$3,'inmediato','completada',$4::timestamptz)`, user.ID, cat.InstitucionID, proc, at); err != nil {
			t.Fatal(err)
		}
	}
	if d := get().TotalPracticasHoy - before.TotalPracticasHoy; d != 2 {
		t.Fatalf("practicas hoy delta = %d, want 2 (Lima day)", d)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/actividad", nil)
	rr := dbtest.Serve(t, h.handleDashboardActividad, req, admin.ID, "admin")
	var act struct {
		Practicas []DiaMetricaItem `json:"practicas_ultima_semana"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &act); err != nil {
		t.Fatal(err)
	}
	got := map[string]int64{}
	for _, it := range act.Practicas {
		got[it.Fecha] = it.Total
	}
	if got["2030-10-31"] != 2 || got["2030-10-30"] != 1 || got["2030-11-01"] != 0 {
		t.Fatalf("weekly series by Lima day = %v, want 10-30:1 10-31:2 and no 11-01", got)
	}
}

// H79: the audit log date filter uses Lima days.
func TestAuditLog_DateFilterBusinessDay(t *testing.T) {
	db, h := setup(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	for _, at := range []string{
		"2030-10-31T02:00:00Z", // 30/10 21:00 Lima
		"2030-10-31T04:00:00Z", // 30/10 23:00 Lima
		"2030-10-31T06:00:00Z", // 31/10 Lima
		"2030-11-01T04:59:00Z", // 31/10 23:59 Lima
		"2030-11-01T05:30:00Z", // 01/11 Lima
	} {
		if _, err := db.Exec(`INSERT INTO audit_log (usuario_id, accion, entidad, created_at)
			VALUES ($1, 'tz_test', 'x', $2::timestamptz)`, admin.ID, at); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/admin/audit-log?from=2030-10-31&to=2030-10-31&q="+admin.Username, nil)
	rr := dbtest.Serve(t, h.handleGetAuditLog, req, admin.ID, "admin")
	var out AuditLogResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Total != 2 {
		t.Fatalf("audit rows on 31/10 Lima = %d, want 2 (%s)", out.Total, rr.Body)
	}
}
