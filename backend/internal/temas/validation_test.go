package temas

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func ptr(s string) *string { return &s }

func idsHasta(n int) []int64 {
	ids := make([]int64, n)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	return ids
}

func TestNormalizarNombre(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Vigilancia", "Vigilancia"},
		{"  Vigilancia   epidemiológica  ", "Vigilancia epidemiológica"},
		{"a\t\nb", "a b"},
		{"   ", ""},
	}
	for _, c := range cases {
		if got := normalizarNombre(c.in); got != c.want {
			t.Errorf("normalizarNombre(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestValidarCrear(t *testing.T) {
	cases := []struct {
		name   string
		req    CrearTemaRequest
		campos map[string]string // nil = valid
	}{
		{"válido mínimo", CrearTemaRequest{AsignaturaID: 7, Nombre: "abc"}, nil},
		{"nombre de 3 runes con tilde", CrearTemaRequest{AsignaturaID: 7, Nombre: "áéí"}, nil},
		{"nombre de 120 runes", CrearTemaRequest{AsignaturaID: 7, Nombre: strings.Repeat("ñ", 120)}, nil},
		{"nombre de 2 runes", CrearTemaRequest{AsignaturaID: 7, Nombre: "ab"}, map[string]string{"nombre": "min"}},
		{"nombre de 121 runes", CrearTemaRequest{AsignaturaID: 7, Nombre: strings.Repeat("ñ", 121)}, map[string]string{"nombre": "max"}},
		{"espacios no cuentan", CrearTemaRequest{AsignaturaID: 7, Nombre: "  a   b  "}, nil},
		{"espacios recortados quedan cortos", CrearTemaRequest{AsignaturaID: 7, Nombre: "  ab  "}, map[string]string{"nombre": "min"}},
		{"nombre vacío", CrearTemaRequest{AsignaturaID: 7, Nombre: "   "}, map[string]string{"nombre": "requerido"}},
		{"sin asignatura", CrearTemaRequest{Nombre: "abc"}, map[string]string{"asignatura_id": "requerido"}},
		{"asignatura negativa", CrearTemaRequest{AsignaturaID: -1, Nombre: "abc"}, map[string]string{"asignatura_id": "requerido"}},
		{"descripción de 500", CrearTemaRequest{AsignaturaID: 7, Nombre: "abc", Descripcion: ptr(strings.Repeat("é", 500))}, nil},
		{"descripción de 501", CrearTemaRequest{AsignaturaID: 7, Nombre: "abc", Descripcion: ptr(strings.Repeat("é", 501))}, map[string]string{"descripcion": "max"}},
		{"50 especialidades", CrearTemaRequest{AsignaturaID: 7, Nombre: "abc", EspecialidadIDs: idsHasta(50)}, nil},
		{"51 especialidades", CrearTemaRequest{AsignaturaID: 7, Nombre: "abc", EspecialidadIDs: idsHasta(51)}, map[string]string{"especialidad_ids": "max"}},
		{"especialidad repetida", CrearTemaRequest{AsignaturaID: 7, Nombre: "abc", EspecialidadIDs: []int64{2, 5, 2}}, map[string]string{"especialidad_ids": "repetido"}},
		{"especialidad cero", CrearTemaRequest{AsignaturaID: 7, Nombre: "abc", EspecialidadIDs: []int64{0}}, map[string]string{"especialidad_ids": "invalido"}},
		{"varios campos a la vez", CrearTemaRequest{Nombre: "a", EspecialidadIDs: []int64{-3}},
			map[string]string{"asignatura_id": "requerido", "nombre": "min", "especialidad_ids": "invalido"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := validarCrear(c.req)
			if c.campos == nil {
				if err != nil {
					t.Fatalf("want valid, got %v", err)
				}
				return
			}
			var verr *ValidacionError
			if !errors.As(err, &verr) || !errors.Is(err, ErrValidacion) {
				t.Fatalf("want ValidacionError, got %v", err)
			}
			if !reflect.DeepEqual(verr.Campos, c.campos) {
				t.Errorf("campos = %v, want %v", verr.Campos, c.campos)
			}
		})
	}
}

func TestValidarDatos_Normaliza(t *testing.T) {
	cases := []struct {
		name            string
		req             EditarTemaRequest
		nombre, clave   string
		descripcion     *string
		especialidadIDs []int64
	}{
		{"clave plegada y espacios colapsados", EditarTemaRequest{Nombre: "  Vigilancia   Epidemiológica "},
			"Vigilancia Epidemiológica", "vigilancia epidemiologica", nil, []int64{}},
		{"descripción vacía es null", EditarTemaRequest{Nombre: "abc", Descripcion: ptr("")}, "abc", "abc", nil, []int64{}},
		{"descripción solo espacios es null", EditarTemaRequest{Nombre: "abc", Descripcion: ptr("   ")}, "abc", "abc", nil, []int64{}},
		{"descripción recortada", EditarTemaRequest{Nombre: "abc", Descripcion: ptr("  Brotes  ")}, "abc", "abc", ptr("Brotes"), []int64{}},
		{"especialidades se conservan", EditarTemaRequest{Nombre: "abc", EspecialidadIDs: []int64{5, 2}}, "abc", "abc", nil, []int64{5, 2}},
		{"la ñ no se pliega", EditarTemaRequest{Nombre: "Año"}, "Año", "año", nil, []int64{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, err := validarEditar(c.req)
			if err != nil {
				t.Fatal(err)
			}
			if d.Nombre != c.nombre || d.NombreClave != c.clave {
				t.Errorf("nombre/clave = %q/%q, want %q/%q", d.Nombre, d.NombreClave, c.nombre, c.clave)
			}
			if !reflect.DeepEqual(d.Descripcion, c.descripcion) {
				t.Errorf("descripcion = %v, want %v", d.Descripcion, c.descripcion)
			}
			if !reflect.DeepEqual(d.EspecialidadIDs, c.especialidadIDs) {
				t.Errorf("especialidad_ids = %v, want %v", d.EspecialidadIDs, c.especialidadIDs)
			}
		})
	}
}

func TestValidarEstado(t *testing.T) {
	cases := []struct {
		estado string
		ok     bool
	}{
		{"activo", true}, {"archivado", true}, {"", false}, {"inactivo", false}, {"ACTIVO", false},
	}
	for _, c := range cases {
		if err := validarEstado(c.estado); (err == nil) != c.ok || (err != nil && !errors.Is(err, ErrValidacion)) {
			t.Errorf("validarEstado(%q) = %v, want ok=%v", c.estado, err, c.ok)
		}
	}
}

func TestValidarOrden(t *testing.T) {
	cases := []struct {
		name string
		ids  []int64
		ok   bool
	}{
		{"vacía", []int64{}, true},
		{"válida", []int64{41, 38, 40}, true},
		{"60 elementos", idsHasta(60), true},
		{"61 elementos", idsHasta(61), false},
		{"repetidos", []int64{41, 38, 41}, false},
		{"cero", []int64{0, 1}, false},
		{"negativo", []int64{-1}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validarOrden(c.ids)
			if (err == nil) != c.ok || (err != nil && !errors.Is(err, ErrOrdenInvalido)) {
				t.Errorf("validarOrden = %v, want ok=%v", err, c.ok)
			}
		})
	}
}

func TestMismoConjunto(t *testing.T) {
	cases := []struct {
		name string
		a, b []int64
		want bool
	}{
		{"vacíos", []int64{}, []int64{}, true},
		{"mismo orden", []int64{1, 2, 3}, []int64{1, 2, 3}, true},
		{"otro orden", []int64{1, 2, 3}, []int64{3, 1, 2}, true},
		{"falta uno", []int64{1, 2, 3}, []int64{1, 2}, false},
		{"sobra uno", []int64{1, 2}, []int64{1, 2, 3}, false},
		{"uno distinto", []int64{1, 2, 3}, []int64{1, 2, 4}, false},
	}
	for _, c := range cases {
		if got := mismoConjunto(c.a, c.b); got != c.want {
			t.Errorf("%s: mismoConjunto = %v, want %v", c.name, got, c.want)
		}
	}
}
