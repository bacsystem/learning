package catalogs

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/cbaciliod/learning/internal/db/dbtest"
)

// H87: coeficiente and peso_porcentual are NUMERIC(5,2): values below 0.01 or
// with more than 2 decimals would be silently rounded (0.004 → 0.00), so they
// are rejected with 422; valid 2-decimal values are stored exactly. H109: 0 and
// an omitted field (which decodes to 0) are rejected by the min=0.01 bound.
func TestProcesoAsignatura_TwoDecimals(t *testing.T) {
	e := setup(t)
	a1, a2 := dbtest.SeedAsignatura(t, e.db), dbtest.SeedAsignatura(t, e.db)
	proc := dbtest.SeedProceso(t, e.db, e.cat, 2097, "inactivo")
	pid := fmt.Sprint(proc)
	bad := []struct{ peso, coef string }{
		{"40", "0.004"},
		{"40", "0.009"},
		{"40", "1.255"},
		{"0.004", "1"},
		{"33.333", "1"},
		{"0.009", "1"},
		{"40", "0"}, // H109: lower bound min=0.01
		{"0", "1"},
	}
	// H109: an omitted field decodes to 0 and must hit the same lower bound.
	omitted := map[string]string{
		"sin coeficiente": `"peso_porcentual":40`,
		"sin peso":        `"coeficiente":1`,
	}
	for _, c := range bad {
		body := fmt.Sprintf(`{"asignatura_id":%d,"numero_preguntas":5,"peso_porcentual":%s,"coeficiente":%s}`, a1, c.peso, c.coef)
		want(t, e.call(t, e.h.AsignarAsignatura, http.MethodPost, body, "id", pid), http.StatusUnprocessableEntity,
			"assign peso="+c.peso+" coef="+c.coef)
	}
	for name, fields := range omitted {
		body := fmt.Sprintf(`{"asignatura_id":%d,"numero_preguntas":5,%s}`, a1, fields)
		want(t, e.call(t, e.h.AsignarAsignatura, http.MethodPost, body, "id", pid), http.StatusUnprocessableEntity,
			"assign "+name)
	}
	if n := count(t, e, proc); n != 0 {
		t.Fatalf("%d rows stored from invalid requests", n)
	}

	body := fmt.Sprintf(`{"asignatura_id":%d,"numero_preguntas":5,"peso_porcentual":33.33,"coeficiente":0.01}`, a1)
	want(t, e.call(t, e.h.AsignarAsignatura, http.MethodPost, body, "id", pid), http.StatusCreated, "assign 33.33/0.01")
	body = fmt.Sprintf(`{"asignatura_id":%d,"numero_preguntas":5,"peso_porcentual":0.29,"coeficiente":1.15}`, a2)
	want(t, e.call(t, e.h.AsignarAsignatura, http.MethodPost, body, "id", pid), http.StatusCreated, "assign 0.29/1.15")
	var peso, coef string
	if err := e.db.QueryRow(`SELECT peso_porcentual::text, coeficiente::text FROM proceso_asignaturas
		WHERE proceso_id=$1 AND asignatura_id=$2`, proc, a1).Scan(&peso, &coef); err != nil {
		t.Fatal(err)
	}
	if peso != "33.33" || coef != "0.01" {
		t.Fatalf("stored peso=%s coef=%s, want 33.33/0.01", peso, coef)
	}

	for _, c := range bad {
		body := fmt.Sprintf(`{"numero_preguntas":10,"peso_porcentual":%s,"coeficiente":%s}`, c.peso, c.coef)
		want(t, e.call(t, e.h.UpdateProcesoAsignatura, http.MethodPut, body, "id", pid, "asignaturaId", fmt.Sprint(a1)),
			http.StatusUnprocessableEntity, "update peso="+c.peso+" coef="+c.coef)
	}
	for name, fields := range omitted {
		body := fmt.Sprintf(`{"numero_preguntas":10,%s}`, fields)
		want(t, e.call(t, e.h.UpdateProcesoAsignatura, http.MethodPut, body, "id", pid, "asignaturaId", fmt.Sprint(a1)),
			http.StatusUnprocessableEntity, "update "+name)
	}
}

func count(t *testing.T, e env, proc int64) int {
	t.Helper()
	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM proceso_asignaturas WHERE proceso_id=$1`, proc).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
