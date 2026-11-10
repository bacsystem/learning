// Package grading holds the evaluation rules shared by simulacros and prácticas:
// how answer rows are grouped per question, when a question counts as correct,
// and how results are counted overall and per asignatura.
//
// A question is correct only when the set of selected alternatives is exactly
// the set of correct alternatives (for 'unica' that is the single correct one;
// for 'multiple' every correct option and no incorrect one).
package grading

import (
	"math"
	"sort"
)

// Fila is one stored answer row: a question of the attempt and, optionally,
// one selected alternative. A question with no selection appears with a nil
// RespuestaID (placeholder row). Multiple rows per question are allowed (one
// per selected alternative).
type Fila struct {
	PreguntaID   int64
	AsignaturaID int64
	RespuestaID  *int64
	// EsCorrecta is the stored question-level correctness (nil = not graded).
	EsCorrecta *bool
}

// Pregunta is the per-question view after grouping rows.
type Pregunta struct {
	PreguntaID   int64
	AsignaturaID int64
	Seleccion    []int64 // distinct selected alternatives, ascending
	Respondida   bool
	Correcta     bool
	Calificada   bool // Correcta comes from a grading (stored flag or key)
}

// Conteo aggregates question results.
type Conteo struct {
	Total        int
	Correctas    int
	Incorrectas  int
	SinResponder int
}

// Agrupar groups rows by question, preserving the order in which each question
// first appears, and de-duplicates selections. Stored flags (Fila.EsCorrecta)
// are combined with AND, so a question is correct only if every graded row is.
func Agrupar(filas []Fila) []Pregunta {
	idx := map[int64]int{}
	var out []Pregunta
	seen := map[[2]int64]bool{}
	for _, f := range filas {
		i, ok := idx[f.PreguntaID]
		if !ok {
			i = len(out)
			idx[f.PreguntaID] = i
			out = append(out, Pregunta{PreguntaID: f.PreguntaID, AsignaturaID: f.AsignaturaID, Correcta: true})
		}
		p := &out[i]
		if f.EsCorrecta != nil {
			p.Calificada = true
			p.Correcta = p.Correcta && *f.EsCorrecta
		}
		if f.RespuestaID != nil {
			k := [2]int64{f.PreguntaID, *f.RespuestaID}
			if !seen[k] {
				seen[k] = true
				p.Seleccion = append(p.Seleccion, *f.RespuestaID)
			}
		}
	}
	for i := range out {
		p := &out[i]
		sort.Slice(p.Seleccion, func(a, b int) bool { return p.Seleccion[a] < p.Seleccion[b] })
		p.Respondida = len(p.Seleccion) > 0
		if !p.Calificada || !p.Respondida {
			p.Correcta = false
		}
	}
	return out
}

// Evaluar reports whether seleccion is exactly the set correctas.
// An empty selection is never correct.
func Evaluar(seleccion, correctas []int64) bool {
	if len(seleccion) == 0 || len(correctas) == 0 {
		return false
	}
	want := map[int64]bool{}
	for _, c := range correctas {
		want[c] = true
	}
	got := map[int64]bool{}
	for _, s := range seleccion {
		if !want[s] {
			return false
		}
		got[s] = true
	}
	return len(got) == len(want)
}

// Calificar grades every question against clave (pregunta_id → correct ids),
// overriding any stored flag.
func Calificar(preguntas []Pregunta, clave map[int64][]int64) {
	for i := range preguntas {
		p := &preguntas[i]
		p.Calificada = true
		p.Correcta = Evaluar(p.Seleccion, clave[p.PreguntaID])
	}
}

// Contar counts results over all questions.
func Contar(preguntas []Pregunta) Conteo {
	var c Conteo
	for _, p := range preguntas {
		c.add(p)
	}
	return c
}

// ContarPorAsignatura counts results per asignatura.
func ContarPorAsignatura(preguntas []Pregunta) map[int64]Conteo {
	out := map[int64]Conteo{}
	for _, p := range preguntas {
		c := out[p.AsignaturaID]
		c.add(p)
		out[p.AsignaturaID] = c
	}
	return out
}

func (c *Conteo) add(p Pregunta) {
	c.Total++
	switch {
	case !p.Respondida:
		c.SinResponder++
	case p.Correcta:
		c.Correctas++
	default:
		c.Incorrectas++
	}
}

// Intento is one graded question attempt, used for question statistics.
type Intento struct {
	PreguntaID int64
	Correcta   bool
}

// Intentos returns one attempt per answered question (unanswered ones are not
// counted as attempts).
func Intentos(preguntas []Pregunta) []Intento {
	var out []Intento
	for _, p := range preguntas {
		if p.Respondida {
			out = append(out, Intento{PreguntaID: p.PreguntaID, Correcta: p.Correcta})
		}
	}
	return out
}

// Porcentaje returns part/total*100 rounded to 2 decimals (0 when total is 0).
func Porcentaje(part, total int) float64 {
	if total <= 0 {
		return 0
	}
	return Round2(float64(part) / float64(total) * 100)
}

// Round2 rounds to two decimals.
func Round2(v float64) float64 { return math.Round(v*100) / 100 }
