package catalogs

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/lib/pq"
)

func area(id int64, peso float64, preguntas int) AreaPesoRequest {
	return AreaPesoRequest{AsignaturaID: id, PesoPorcentual: peso, NumeroPreguntas: preguntas, Coeficiente: 1}
}

func areasN(n int) []AreaPesoRequest {
	out := make([]AreaPesoRequest, n)
	for i := range out {
		out[i] = area(int64(i+1), 1, 1)
	}
	return out
}

func camposDe(t *testing.T, err error) map[string]string {
	t.Helper()
	var ep *errorPesos
	if !errors.As(err, &ep) || !errors.Is(err, ErrPesosValidacion) {
		t.Fatalf("want validation error, got %v", err)
	}
	return ep.detalle["campos"].(map[string]string)
}

func TestValidarAreasPesos(t *testing.T) {
	cases := []struct {
		name   string
		areas  []AreaPesoRequest
		campos map[string]string // nil = valid
	}{
		{"una área", []AreaPesoRequest{area(7, 100, 20)}, nil},
		{"50 áreas", areasN(50), nil},
		{"sin áreas", nil, map[string]string{"areas": "min"}},
		{"51 áreas", areasN(51), map[string]string{"areas": "max"}},
		{"asignatura cero", []AreaPesoRequest{area(0, 100, 20)}, map[string]string{"areas[0].asignatura_id": "requerido"}},
		{"0 preguntas", []AreaPesoRequest{area(7, 100, 0)}, map[string]string{"areas[0].numero_preguntas": "min"}},
		{"200 preguntas", []AreaPesoRequest{area(7, 100, 200)}, nil},
		{"201 preguntas", []AreaPesoRequest{area(7, 100, 201)}, map[string]string{"areas[0].numero_preguntas": "max"}},
		{"peso 0", []AreaPesoRequest{area(7, 0, 1)}, map[string]string{"areas[0].peso_porcentual": "min"}},
		{"peso 0,01", []AreaPesoRequest{area(7, 0.01, 1)}, nil},
		{"peso 100,01", []AreaPesoRequest{area(7, 100.01, 1)}, map[string]string{"areas[0].peso_porcentual": "max"}},
		{"peso con 3 decimales", []AreaPesoRequest{area(7, 33.333, 1)}, map[string]string{"areas[0].peso_porcentual": "decimales"}},
		{"coeficiente 0", []AreaPesoRequest{{AsignaturaID: 7, PesoPorcentual: 100, NumeroPreguntas: 1}},
			map[string]string{"areas[0].coeficiente": "min"}},
		{"coeficiente 999,99", []AreaPesoRequest{{AsignaturaID: 7, PesoPorcentual: 100, NumeroPreguntas: 1, Coeficiente: 999.99}}, nil},
		{"coeficiente 1000", []AreaPesoRequest{{AsignaturaID: 7, PesoPorcentual: 100, NumeroPreguntas: 1, Coeficiente: 1000}},
			map[string]string{"areas[0].coeficiente": "max"}},
		{"varias filas y campos", []AreaPesoRequest{area(7, 100, 1), {AsignaturaID: -1, PesoPorcentual: 0.001, NumeroPreguntas: 0, Coeficiente: 1}},
			map[string]string{"areas[1].asignatura_id": "requerido", "areas[1].peso_porcentual": "decimales", "areas[1].numero_preguntas": "min"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validarAreasPesos(c.areas)
			if c.campos == nil {
				if err != nil {
					t.Fatalf("want valid, got %v", err)
				}
				return
			}
			if got := camposDe(t, err); !reflect.DeepEqual(got, c.campos) {
				t.Errorf("campos = %v, want %v", got, c.campos)
			}
		})
	}
}

// Rules 3–5 of PE3 in the contract's order: repeated area, then Σ preguntas,
// then Σ pesos, with sums in exact hundredths.
func TestReglasDeConjunto(t *testing.T) {
	cases := []struct {
		name    string
		areas   []AreaPesoRequest
		want    error
		detalle map[string]any
	}{
		{"suma 100", []AreaPesoRequest{area(1, 60, 50), area(2, 40, 50)}, nil, nil},
		{"tres tercios 33,33 = 99,99 dentro de la tolerancia", []AreaPesoRequest{area(1, 33.33, 1), area(2, 33.33, 1), area(3, 33.33, 1)}, nil, nil},
		{"100,01 dentro de la tolerancia", []AreaPesoRequest{area(1, 50.01, 1), area(2, 50, 1)}, nil, nil},
		{"99,98 fuera", []AreaPesoRequest{area(1, 49.98, 1), area(2, 50, 1)}, ErrPesosNoSuman100, map[string]any{"suma_pesos": 99.98}},
		{"100,02 fuera", []AreaPesoRequest{area(1, 50.02, 1), area(2, 50, 1)}, ErrPesosNoSuman100, map[string]any{"suma_pesos": 100.02}},
		{"suma 95", []AreaPesoRequest{area(1, 45, 1), area(2, 50, 1)}, ErrPesosNoSuman100, map[string]any{"suma_pesos": 95.0}},
		{"200 preguntas", []AreaPesoRequest{area(1, 50, 100), area(2, 50, 100)}, nil, nil},
		{"210 preguntas", []AreaPesoRequest{area(1, 50, 110), area(2, 50, 100)}, ErrLimitePreguntas, map[string]any{"suma_preguntas": 210}},
		{"área repetida", []AreaPesoRequest{area(1, 50, 1), area(1, 50, 1)}, ErrPesoDuplicado, map[string]any{"asignatura_id": int64(1)}},
		{"repetida gana a 210 preguntas y suma mala", []AreaPesoRequest{area(1, 10, 110), area(1, 10, 100)}, ErrPesoDuplicado, map[string]any{"asignatura_id": int64(1)}},
		{"210 preguntas gana a suma mala", []AreaPesoRequest{area(1, 10, 110), area(2, 10, 100)}, ErrLimitePreguntas, map[string]any{"suma_preguntas": 210}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := reglasDeConjunto(c.areas)
			if c.want == nil {
				if err != nil {
					t.Fatalf("want nil, got %v", err)
				}
				return
			}
			var ep *errorPesos
			if !errors.Is(err, c.want) || !errors.As(err, &ep) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			if !reflect.DeepEqual(ep.detalle, c.detalle) {
				t.Errorf("detalle = %#v, want %#v", ep.detalle, c.detalle)
			}
		})
	}
}

func TestPesosIguales(t *testing.T) {
	cases := []struct {
		ids  []int64
		want []float64
	}{
		{nil, []float64{}},
		{[]int64{7}, []float64{100}},
		{[]int64{3, 5, 9}, []float64{33.34, 33.33, 33.33}},
		{[]int64{1, 2, 3, 4, 5, 6, 7}, []float64{14.29, 14.29, 14.29, 14.29, 14.28, 14.28, 14.28}},
	}
	for _, c := range cases {
		t.Run(fmt.Sprint(len(c.ids)), func(t *testing.T) {
			areas := pesosIguales(c.ids)
			got := make([]float64, len(areas))
			var total int64
			for i, a := range areas {
				got[i] = a.PesoPorcentual
				total += centesimas(a.PesoPorcentual)
				if a.AsignaturaID != c.ids[i] || a.Coeficiente != 1 || a.NumeroPreguntas != nil {
					t.Errorf("area %d = %+v", i, a)
				}
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("pesos = %v, want %v", got, c.want)
			}
			if len(areas) > 0 && total != totalPesoCentesimas {
				t.Errorf("total = %d hundredths, want 10000", total)
			}
		})
	}
}

func TestMapearPesoDuplicado(t *testing.T) {
	otra := &pq.Error{Code: "23505", Constraint: "otra_restriccion"}
	cases := []struct {
		name string
		err  error
		want error
	}{
		{"restricción de pesos", &pq.Error{Code: "23505", Constraint: constraintPesos}, ErrPesoDuplicado},
		{"envuelta", fmt.Errorf("guardar: %w", &pq.Error{Code: "23505", Constraint: constraintPesos}), ErrPesoDuplicado},
		{"otra restricción única", otra, otra},
		{"FK de la misma tabla", &pq.Error{Code: "23503", Constraint: constraintPesos}, nil},
		{"nil", nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := mapearPesoDuplicado(c.err)
			switch {
			case c.want == ErrPesoDuplicado && !errors.Is(got, ErrPesoDuplicado):
				t.Errorf("got %v, want ErrPesoDuplicado", got)
			case c.want != ErrPesoDuplicado && errors.Is(got, ErrPesoDuplicado):
				t.Errorf("got ErrPesoDuplicado for %v", c.err)
			}
		})
	}
}

func TestResponderErrorPesos(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{ErrNotFound, http.StatusNotFound, "no_encontrado"},
		{validarAreasPesos(nil), http.StatusUnprocessableEntity, "validacion"},
		{invalidRef("x"), http.StatusUnprocessableEntity, "referencia_invalida"},
		{reglasDeConjunto([]AreaPesoRequest{area(1, 50, 1), area(1, 50, 1)}), http.StatusConflict, "peso_duplicado"},
		{ErrPesoDuplicado, http.StatusConflict, "peso_duplicado"},
		{reglasDeConjunto([]AreaPesoRequest{area(1, 50, 150), area(2, 50, 60)}), http.StatusUnprocessableEntity, "limite_preguntas"},
		{reglasDeConjunto([]AreaPesoRequest{area(1, 50, 1), area(2, 45, 1)}), http.StatusUnprocessableEntity, "pesos_no_suman_100"},
		{errors.New("pq: connection refused"), http.StatusInternalServerError, "error_interno"},
	}
	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			rr := httptest.NewRecorder()
			responderErrorPesos(rr, "test", c.err)
			var body map[string]any
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if rr.Code != c.status || body["error"] != c.code || body["message"] == "" {
				t.Errorf("got %d %v, want %d %s", rr.Code, body, c.status, c.code)
			}
		})
	}
}
