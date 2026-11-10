package simulations

import (
	"math"

	"github.com/cbaciliod/learning/internal/grading"
)

// simulacroAsignaturaRow is one asignatura of a simulacro as snapshotted at
// creation time (simulacro_asignaturas): the number of questions actually
// assigned, its peso and coeficiente.
type simulacroAsignaturaRow struct {
	AsignaturaID    int64
	Nombre          string
	NumeroPreguntas int
	Peso            float64
	Coeficiente     float64
}

// resultadoAsignatura is the graded result of one asignatura.
type resultadoAsignatura struct {
	simulacroAsignaturaRow
	Conteo  grading.Conteo
	Puntaje float64
}

// notaSimulacro is the full grade of a simulacro.
type notaSimulacro struct {
	Conteo         grading.Conteo
	PuntajeTotal   float64
	PuntajeMaximo  float64
	NotaCentesimal float64
	Aprobado       bool
	PorAsignatura  []resultadoAsignatura
}

// effectiveCoef treats a non-positive coeficiente as 1.
func effectiveCoef(c float64) float64 {
	if c <= 0 {
		return 1
	}
	return c
}

// CalcularNota grades graded questions against the snapshot of asignaturas.
//
//   - puntaje(asignatura) = correctas / preguntas_asignadas * peso * coeficiente
//     (rounded to 2 decimals); puntaje_total = Σ puntaje(asignatura), so the
//     per-asignatura breakdown always adds up to the total (H34);
//   - puntaje_maximo = Σ peso * coeficiente (rounded to 2 decimals per
//     asignatura, like puntaje) over asignaturas with questions, so a perfect
//     attempt scores exactly puntaje_maximo (H132);
//   - nota = puntaje_total / puntaje_maximo * notaMaxima (capped at notaMaxima);
//   - aprobado = nota >= notaMinima.
func CalcularNota(preguntas []grading.Pregunta, asignaturas []simulacroAsignaturaRow, notaMinima, notaMaxima float64) notaSimulacro {
	porAsig := grading.ContarPorAsignatura(preguntas)
	var n notaSimulacro
	for _, a := range asignaturas {
		c := porAsig[a.AsignaturaID]
		if missing := a.NumeroPreguntas - c.Total; missing > 0 {
			// Assigned questions without a row (e.g. hard-deleted) count as unanswered.
			c.SinResponder += missing
			c.Total = a.NumeroPreguntas
		}
		res := resultadoAsignatura{simulacroAsignaturaRow: a, Conteo: c}
		if a.NumeroPreguntas > 0 {
			w := a.Peso * effectiveCoef(a.Coeficiente)
			// Both sums round per asignatura (H132): all correct then gives
			// puntaje_total == puntaje_maximo and the top grade.
			n.PuntajeMaximo += grading.Round2(w)
			res.Puntaje = grading.Round2(float64(c.Correctas) / float64(a.NumeroPreguntas) * w)
		}
		n.PuntajeTotal += res.Puntaje
		n.Conteo.Total += c.Total
		n.Conteo.Correctas += c.Correctas
		n.Conteo.Incorrectas += c.Incorrectas
		n.Conteo.SinResponder += c.SinResponder
		n.PorAsignatura = append(n.PorAsignatura, res)
	}
	n.PuntajeTotal = grading.Round2(n.PuntajeTotal)
	n.PuntajeMaximo = grading.Round2(n.PuntajeMaximo)
	if n.PuntajeMaximo > 0 && notaMaxima > 0 {
		n.NotaCentesimal = grading.Round2(math.Min(n.PuntajeTotal/n.PuntajeMaximo, 1) * notaMaxima)
	}
	n.Aprobado = n.NotaCentesimal >= notaMinima
	return n
}
