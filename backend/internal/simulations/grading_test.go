package simulations

import (
	"testing"

	"github.com/cbaciliod/learning/internal/grading"
)

func preg(id, asig int64, respondida, correcta bool) grading.Pregunta {
	p := grading.Pregunta{PreguntaID: id, AsignaturaID: asig, Respondida: respondida, Correcta: correcta, Calificada: true}
	if respondida {
		p.Seleccion = []int64{id * 10}
	}
	return p
}

func TestCalcularNota_SumaPorAsignaturaIgualTotal(t *testing.T) {
	// H34: coeficiente must be applied to the per-asignatura puntaje too.
	asig := []simulacroAsignaturaRow{
		{AsignaturaID: 1, NumeroPreguntas: 3, Peso: 50, Coeficiente: 2},
		{AsignaturaID: 2, NumeroPreguntas: 3, Peso: 50, Coeficiente: 1},
	}
	ps := []grading.Pregunta{
		preg(1, 1, true, true), preg(2, 1, true, false), preg(3, 1, false, false),
		preg(4, 2, true, true), preg(5, 2, true, true), preg(6, 2, false, false),
	}
	n := CalcularNota(ps, asig, 55, 100)
	var sum float64
	for _, a := range n.PorAsignatura {
		sum += a.Puntaje
	}
	if grading.Round2(sum) != n.PuntajeTotal {
		t.Fatalf("Σ por asignatura %.2f != total %.2f", sum, n.PuntajeTotal)
	}
	// 1/3*100 + 2/3*50 = 33.33 + 33.33
	if n.PuntajeTotal != 66.66 || n.PuntajeMaximo != 150 || n.NotaCentesimal != 44.44 || n.Aprobado {
		t.Fatalf("nota: %+v", n)
	}
	if n.Conteo != (grading.Conteo{Total: 6, Correctas: 3, Incorrectas: 1, SinResponder: 2}) {
		t.Fatalf("conteo: %+v", n.Conteo)
	}
}

func TestCalcularNota_DenominadorEsPreguntasAsignadas(t *testing.T) {
	// H33: 2 assigned questions, both correct → 100 regardless of today's config.
	asig := []simulacroAsignaturaRow{{AsignaturaID: 1, NumeroPreguntas: 2, Peso: 100, Coeficiente: 1}}
	n := CalcularNota([]grading.Pregunta{preg(1, 1, true, true), preg(2, 1, true, true)}, asig, 55, 100)
	if n.NotaCentesimal != 100 || !n.Aprobado {
		t.Fatalf("nota: %+v", n)
	}
	// A missing row (hard-deleted question) counts as unanswered, not as a smaller denominator.
	n = CalcularNota([]grading.Pregunta{preg(1, 1, true, true)}, asig, 55, 100)
	if n.NotaCentesimal != 50 || n.Conteo.SinResponder != 1 || n.Conteo.Total != 2 {
		t.Fatalf("nota con fila faltante: %+v", n)
	}
}

func TestCalcularNota_EscalaYAprobado(t *testing.T) {
	asig := []simulacroAsignaturaRow{{AsignaturaID: 1, NumeroPreguntas: 4, Peso: 100, Coeficiente: 0}}
	ps := []grading.Pregunta{preg(1, 1, true, true), preg(2, 1, true, true), preg(3, 1, true, true), preg(4, 1, true, false)}
	n := CalcularNota(ps, asig, 15, 20)
	if n.NotaCentesimal != 15 || !n.Aprobado || n.PuntajeMaximo != 100 {
		t.Fatalf("nota escala 20: %+v", n)
	}
	if n := CalcularNota(nil, nil, 55, 100); n.NotaCentesimal != 0 || n.Aprobado {
		t.Fatalf("vacío: %+v", n)
	}
}

func TestH132_TodoCorrectoEsNotaMaximaConPesosConDecimales(t *testing.T) {
	// 12.35*1.01 = 12.4735: rounding it per asignatura in puntaje_total but not
	// in puntaje_maximo gave 100.24/100.25 → 99.99 with every answer correct.
	asig := []simulacroAsignaturaRow{
		{AsignaturaID: 1, NumeroPreguntas: 1, Peso: 12.35, Coeficiente: 1.01},
		{AsignaturaID: 2, NumeroPreguntas: 1, Peso: 12.35, Coeficiente: 1.01},
		{AsignaturaID: 3, NumeroPreguntas: 1, Peso: 75.30, Coeficiente: 1},
	}
	ps := []grading.Pregunta{preg(1, 1, true, true), preg(2, 2, true, true), preg(3, 3, true, true)}
	n := CalcularNota(ps, asig, 55, 100)
	if n.NotaCentesimal != 100 || n.PuntajeTotal != n.PuntajeMaximo {
		t.Fatalf("todo correcto: nota %.2f, puntaje %.2f/%.2f", n.NotaCentesimal, n.PuntajeTotal, n.PuntajeMaximo)
	}
}
