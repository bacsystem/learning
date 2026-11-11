package simulations

import (
	"fmt"
	"net/http"
	"sort"
	"testing"
)

// SIA-E1-03 / ADR 0001 (L6, L7): the simulacro and its preview use the set of
// the user's especialidad when the proceso has one, otherwise the
// compatibility rows exactly as before (escenario «Compatibilidad»). The
// especialidad set differs from compatibility in areas and values, so mixing
// them row by row would show up.
func TestPesosPorConjunto_SimulacroYConfiguracion(t *testing.T) {
	cases := []struct {
		name           string
		conjuntoPropio bool
		// want: "asignatura index|numero_preguntas|peso|coeficiente" of the snapshot
		want []string
	}{
		{"sin conjunto de especialidad: compatibilidad intacta", false, []string{"0|2|60.00|1.00", "1|1|40.00|1.00"}},
		{"con conjunto de especialidad: solo sus filas", true, []string{"0|1|100.00|2.00"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, defaultOpts())
			if c.conjuntoPropio {
				mustExec(t, e, `INSERT INTO proceso_asignaturas
					(proceso_id, especialidad_id, asignatura_id, numero_preguntas, peso_porcentual, coeficiente)
					VALUES ($1, $2, $3, 1, 100, 2)`, e.f.ProcesoID, e.f.EspecialidadID, e.f.Asignaturas[0])
			}
			indice := map[int64]int{e.f.Asignaturas[0]: 0, e.f.Asignaturas[1]: 1}

			// L6: the preview lists the same set.
			code, body := e.do(t, "GET", fmt.Sprintf("/simulacros/configuracion?proceso_id=%d", e.f.ProcesoID), e.tok, "")
			if code != http.StatusOK {
				t.Fatalf("configuracion: %d %v", code, body)
			}
			var preview []string
			cfg, _ := body["configuracion"].(map[string]any)
			if cfg == nil {
				t.Fatalf("configuracion = %v", body)
			}
			for _, item := range cfg["asignaturas"].([]any) {
				m := item.(map[string]any)
				preview = append(preview, fmt.Sprintf("%d|%v|%.2f", indice[int64(m["asignatura_id"].(float64))], m["numero_preguntas"], m["peso_porcentual"]))
			}
			var wantPreview []string
			for _, w := range c.want {
				var i, n int
				var peso float64
				_, _ = fmt.Sscanf(w, "%d|%d|%f", &i, &n, &peso)
				wantPreview = append(wantPreview, fmt.Sprintf("%d|%d|%.2f", i, n, peso))
			}
			sort.Strings(preview)
			if fmt.Sprint(preview) != fmt.Sprint(wantPreview) {
				t.Errorf("preview = %v, want %v", preview, wantPreview)
			}

			// L7: the simulacro snapshot takes exactly that set.
			s := e.create(t)
			rows, err := e.f.DB.Query(`SELECT asignatura_id, numero_preguntas, peso, coeficiente
				FROM simulacro_asignaturas WHERE simulacro_id = $1 ORDER BY asignatura_id`, s.ID)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			var got []string
			for rows.Next() {
				var asig int64
				var n int
				var peso, coef string
				if err := rows.Scan(&asig, &n, &peso, &coef); err != nil {
					t.Fatal(err)
				}
				got = append(got, fmt.Sprintf("%d|%d|%s|%s", indice[asig], n, peso, coef))
			}
			if fmt.Sprint(got) != fmt.Sprint(c.want) {
				t.Errorf("snapshot = %v, want %v", got, c.want)
			}
			total := 0
			for _, w := range c.want {
				var i, n int
				_, _ = fmt.Sscanf(w, "%d|%d|", &i, &n)
				total += n
			}
			if len(s.Preguntas) != total {
				t.Errorf("%d preguntas, want %d", len(s.Preguntas), total)
			}
		})
	}
}
