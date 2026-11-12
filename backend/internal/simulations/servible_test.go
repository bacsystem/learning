package simulations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"testing"

	"github.com/cbaciliod/learning/internal/catalogs"
	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

// servibleEnv has one area of two questions per simulacro and a bank of five
// in different states (SIA-E3-02): only aprobada and the inherited activo are
// servable.
func servibleEnv(t *testing.T) (*env, []int64) {
	t.Helper()
	e := newEnv(t, qt.Options{Inscripcion: "aprobado",
		Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 2, Peso: 100, Unicas: 5}}})
	ids := make([]int64, len(e.f.Preguntas))
	for i, p := range e.f.Preguntas {
		ids[i] = p.ID
	}
	estados := []string{"borrador", "aprobada", "activo", "en_revision", "rechazada"}
	for i, estado := range estados {
		e.estadoPregunta(t, ids[i], estado)
	}
	return e, ids
}

func (e *env) estadoPregunta(t *testing.T, id int64, estado string) {
	t.Helper()
	var motivo *string
	if estado == "rechazada" {
		m := "Clave incorrecta"
		motivo = &m
	}
	if _, err := e.f.DB.Exec(`UPDATE preguntas SET estado = $2, motivo_rechazo = $3 WHERE id = $1`, id, estado, motivo); err != nil {
		t.Fatal(err)
	}
}

func idsDelSimulacro(s SimulacroEnCursoResponse) []int64 {
	var ids []int64
	for _, p := range s.Preguntas {
		ids = append(ids, p.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// Escenarios «Borrador no se sirve» and «Compatibilidad con el banco
// existente»: a new simulacro takes only the aprobada and the inherited activo
// question; borrador, en_revision and rechazada are left out.
func TestServible_SeleccionDelSimulacro(t *testing.T) {
	e, ids := servibleEnv(t)
	s := e.create(t)
	if got, want := idsDelSimulacro(s), []int64{ids[1], ids[2]}; len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("simulacro questions = %v, want %v (aprobada, activo)", got, want)
	}
}

// Escenario «Intento en curso»: a question of a simulacro in progress that
// goes to en_revision can still be answered there, and a new simulacro of
// another user no longer takes it.
func TestServible_IntentoEnCurso(t *testing.T) {
	e, ids := servibleEnv(t)
	s := e.create(t)
	e.estadoPregunta(t, ids[1], "en_revision") // was aprobada, now under review
	e.estadoPregunta(t, ids[4], "aprobada")    // a rejected one, approved again
	p := e.f.Pregunta(ids[1])
	if code := e.responder(t, s.ID, p.ID, p.Correctas[0]); code != http.StatusOK {
		t.Errorf("answering the in-progress question: %d", code)
	}
	otro := e.f.NewUser(t, "aprobado")
	w := qt.Do(e.mux, "POST", "/simulacros", e.f.Token(t, otro, "usuario"), fmt.Sprintf(`{"proceso_id":%d}`, e.f.ProcesoID))
	if w.Code != http.StatusCreated {
		t.Fatalf("second simulacro: %d %s", w.Code, w.Body)
	}
	var nuevo SimulacroEnCursoResponse
	if err := json.Unmarshal(w.Body.Bytes(), &nuevo); err != nil {
		t.Fatal(err)
	}
	if got := idsDelSimulacro(nuevo); len(got) != 2 || got[0] != ids[2] || got[1] != ids[4] {
		t.Errorf("new simulacro = %v, want [%d %d] (without the one under review)", got, ids[2], ids[4])
	}
}

// preguntas_disponibles counts only servable questions, in the simulacro's
// configuration screen and in the catalog's area listing (contract E3-02 I4, I5).
func TestServible_PreguntasDisponibles(t *testing.T) {
	e, _ := servibleEnv(t)
	ctx := context.Background()
	cfg, err := NewRepository(e.f.DB).GetConfiguracionDetalle(ctx, e.f.ProcesoID, 0, 0)
	if err != nil || cfg == nil || len(cfg.Asignaturas) != 1 || cfg.Asignaturas[0].PreguntasDisponibles != 2 {
		t.Errorf("simulacro configuration = %+v, %v; want 2 available", cfg, err)
	}
	areas, err := catalogs.NewRepository(e.f.DB).ListProcesoAsignaturas(ctx, e.f.ProcesoID, 0, 0)
	if err != nil || len(areas) != 1 || areas[0].PreguntasDisponibles != 2 {
		t.Errorf("catalog areas = %+v, %v; want 2 available", areas, err)
	}
}
