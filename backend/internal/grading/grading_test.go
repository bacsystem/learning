package grading

import (
	"reflect"
	"testing"
)

func id(v int64) *int64 { return &v }
func b(v bool) *bool    { return &v }

func TestEvaluar(t *testing.T) {
	cases := []struct {
		name      string
		sel, corr []int64
		want      bool
	}{
		{"unica correcta", []int64{1}, []int64{1}, true},
		{"unica incorrecta", []int64{2}, []int64{1}, false},
		{"vacía", nil, []int64{1}, false},
		{"multiple exacta", []int64{2, 1}, []int64{1, 2}, true},
		{"multiple subconjunto (H31)", []int64{1}, []int64{1, 2}, false},
		{"multiple con incorrecta", []int64{1, 2, 3}, []int64{1, 2}, false},
		{"duplicados no inflan", []int64{1, 1}, []int64{1, 2}, false},
		{"sin clave", []int64{1}, nil, false},
	}
	for _, c := range cases {
		if got := Evaluar(c.sel, c.corr); got != c.want {
			t.Errorf("%s: Evaluar(%v,%v)=%v want %v", c.name, c.sel, c.corr, got, c.want)
		}
	}
}

func TestAgruparYContarCuentaPreguntasNoFilas(t *testing.T) {
	// H35: a multiple with two selected rows plus a placeholder must count once.
	filas := []Fila{
		{PreguntaID: 10, AsignaturaID: 1, RespuestaID: nil},
		{PreguntaID: 10, AsignaturaID: 1, RespuestaID: id(101)},
		{PreguntaID: 10, AsignaturaID: 1, RespuestaID: id(102)},
		{PreguntaID: 10, AsignaturaID: 1, RespuestaID: id(102)},
		{PreguntaID: 20, AsignaturaID: 2, RespuestaID: nil},
		{PreguntaID: 30, AsignaturaID: 2, RespuestaID: id(301)},
	}
	ps := Agrupar(filas)
	if len(ps) != 3 || ps[0].PreguntaID != 10 || !reflect.DeepEqual(ps[0].Seleccion, []int64{101, 102}) {
		t.Fatalf("agrupar: %+v", ps)
	}
	Calificar(ps, map[int64][]int64{10: {101, 102}, 20: {201}, 30: {302}})
	c := Contar(ps)
	if c != (Conteo{Total: 3, Correctas: 1, Incorrectas: 1, SinResponder: 1}) {
		t.Fatalf("contar: %+v", c)
	}
	pa := ContarPorAsignatura(ps)
	if pa[1] != (Conteo{Total: 1, Correctas: 1}) || pa[2] != (Conteo{Total: 2, Incorrectas: 1, SinResponder: 1}) {
		t.Fatalf("por asignatura: %+v", pa)
	}
	if got := Intentos(ps); !reflect.DeepEqual(got, []Intento{{10, true}, {30, false}}) {
		t.Fatalf("intentos: %+v", got)
	}
}

func TestAgruparUsaFlagGuardadoConAND(t *testing.T) {
	ps := Agrupar([]Fila{
		{PreguntaID: 1, RespuestaID: id(1), EsCorrecta: b(true)},
		{PreguntaID: 1, RespuestaID: id(2), EsCorrecta: b(false)},
		{PreguntaID: 2, RespuestaID: id(3), EsCorrecta: b(true)},
		{PreguntaID: 3, RespuestaID: id(4)},  // not graded
		{PreguntaID: 4, EsCorrecta: b(true)}, // flag without selection
	})
	want := []bool{false, true, false, false}
	for i, p := range ps {
		if p.Correcta != want[i] {
			t.Errorf("pregunta %d correcta=%v want %v", p.PreguntaID, p.Correcta, want[i])
		}
	}
	if ps[2].Calificada {
		t.Errorf("ungraded question marked calificada")
	}
}

func TestPorcentajeYRound2(t *testing.T) {
	if Porcentaje(1, 3) != 33.33 || Porcentaje(2, 3) != 66.67 || Porcentaje(1, 0) != 0 {
		t.Fatalf("porcentaje: %v %v", Porcentaje(1, 3), Porcentaje(2, 3))
	}
	if Round2(83.3333) != 83.33 || Round2(12.345678) != 12.35 {
		t.Fatalf("round2: %v %v", Round2(83.3333), Round2(12.345678))
	}
}
