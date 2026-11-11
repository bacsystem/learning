package perfiles

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/cbaciliod/learning/internal/catalogs"
	"github.com/lib/pq"
)

type fakeRepo struct {
	perfil     *perfilRow
	procesos   []ProcesoOpcion
	persona    *int64
	elegirErr  error
	elecciones int
}

func (f *fakeRepo) PerfilActivo(context.Context, int64) (perfilRow, error) {
	if f.perfil == nil {
		return perfilRow{}, ErrProfesionNoDefinida
	}
	return *f.perfil, nil
}
func (f *fakeRepo) ProcesosAprobados(context.Context, int64) ([]ProcesoOpcion, error) {
	return f.procesos, nil
}
func (f *fakeRepo) EspecialidadDePersona(context.Context, int64) (*int64, error) {
	return f.persona, nil
}
func (f *fakeRepo) NombresAsignaturas(_ context.Context, ids []int64) (map[int64]string, error) {
	m := map[int64]string{}
	for _, id := range ids {
		m[id] = fmt.Sprintf("Área %d", id)
	}
	return m, nil
}
func (f *fakeRepo) Elegir(context.Context, int64, Par, bool) error {
	f.elecciones++
	return f.elegirErr
}

type fakePesos struct {
	modo catalogs.ModoPesos
}

func (f *fakePesos) Pesos(_ context.Context, _, _ int64, modo catalogs.ModoPesos) (catalogs.Pesos, error) {
	f.modo = modo
	return catalogs.Pesos{Areas: []catalogs.PesoArea{{AsignaturaID: 7, PesoPorcentual: 100}}, PesosEstimados: true}, nil
}

func id(v int64) *int64 { return &v }

func proceso(pid int64, esps ...int64) ProcesoOpcion {
	p := ProcesoOpcion{ID: pid}
	for _, e := range esps {
		p.Especialidades = append(p.Especialidades, EspecialidadOpcion{ID: e})
	}
	return p
}

// Regla 5 / F1: the preselection.
func TestOpciones_Preseleccion(t *testing.T) {
	cases := []struct {
		name string
		repo fakeRepo
		want *Preseleccion
	}{
		{"perfil activo manda", fakeRepo{perfil: &perfilRow{ProcesoID: 12, EspecialidadID: 5}, persona: id(2),
			procesos: []ProcesoOpcion{proceso(12, 2, 5)}}, &Preseleccion{ProcesoID: id(12), EspecialidadID: id(5)}},
		{"perfil de un proceso ya no aprobado se ignora y vale la persona", fakeRepo{perfil: &perfilRow{ProcesoID: 99, EspecialidadID: 5}, persona: id(2),
			procesos: []ProcesoOpcion{proceso(12, 2, 5)}}, &Preseleccion{ProcesoID: id(12), EspecialidadID: id(2)}},
		{"perfil con especialidad desactivada se ignora", fakeRepo{perfil: &perfilRow{ProcesoID: 12, EspecialidadID: 7}, persona: id(5),
			procesos: []ProcesoOpcion{proceso(12, 2, 5)}}, &Preseleccion{ProcesoID: id(12), EspecialidadID: id(5)}},
		{"perfil ignorado y persona no válida: null", fakeRepo{perfil: &perfilRow{ProcesoID: 99, EspecialidadID: 5}, persona: id(9),
			procesos: []ProcesoOpcion{proceso(12, 2, 5)}}, nil},
		{"perfil de otro proceso aprobado: su especialidad debe estar en ese proceso", fakeRepo{perfil: &perfilRow{ProcesoID: 13, EspecialidadID: 2},
			procesos: []ProcesoOpcion{proceso(12, 2), proceso(13, 5)}}, nil},
		{"persona válida y un solo proceso", fakeRepo{persona: id(5), procesos: []ProcesoOpcion{proceso(12, 2, 5)}},
			&Preseleccion{ProcesoID: id(12), EspecialidadID: id(5)}},
		{"persona válida y varios procesos", fakeRepo{persona: id(5), procesos: []ProcesoOpcion{proceso(12, 5), proceso(13, 5)}},
			&Preseleccion{EspecialidadID: id(5)}},
		{"especialidad de persona no ofrecida (inactiva u otra institución)", fakeRepo{persona: id(9), procesos: []ProcesoOpcion{proceso(12, 2, 5)}}, nil},
		{"persona sin especialidad", fakeRepo{procesos: []ProcesoOpcion{proceso(12, 2)}}, nil},
		{"sin procesos aprobados", fakeRepo{persona: id(5)}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := c.repo
			o, err := NewService(&repo, &fakePesos{}).Opciones(context.Background(), 1)
			if err != nil {
				t.Fatal(err)
			}
			got, _ := json.Marshal(o.Preseleccion)
			want, _ := json.Marshal(c.want)
			if string(got) != string(want) {
				t.Errorf("preseleccion = %s, want %s", got, want)
			}
			if repo.elecciones != 0 {
				t.Error("F1 must not store anything")
			}
		})
	}
}

func TestPerfil(t *testing.T) {
	t.Run("inscripción ya no aprobada", func(t *testing.T) {
		repo := &fakeRepo{perfil: &perfilRow{ProcesoID: 12, EspecialidadID: 5, InscripcionVigente: false}}
		if _, err := NewService(repo, &fakePesos{}).Perfil(context.Background(), 1); !errors.Is(err, ErrInscripcionRequerida) {
			t.Errorf("err = %v, want ErrInscripcionRequerida", err)
		}
	})
	t.Run("pesos estimables con nombres", func(t *testing.T) {
		pesos := &fakePesos{modo: catalogs.ModoOficial}
		repo := &fakeRepo{perfil: &perfilRow{ProcesoID: 12, EspecialidadID: 5, InscripcionVigente: true}}
		p, err := NewService(repo, pesos).Perfil(context.Background(), 1)
		if err != nil || pesos.modo != catalogs.ModoEstimable || !p.Pesos.PesosEstimados || p.Pesos.Areas[0].Nombre != "Área 7" {
			t.Errorf("perfil = %+v, modo %d, %v", p, pesos.modo, err)
		}
	})
}

func TestElegir_Validacion(t *testing.T) {
	cases := []struct {
		req    ElegirRequest
		campos string
	}{
		{ElegirRequest{ProcesoID: 12, EspecialidadID: 5}, ""},
		{ElegirRequest{EspecialidadID: 5}, "map[proceso_id:requerido]"},
		{ElegirRequest{ProcesoID: -1, EspecialidadID: 0}, "map[especialidad_id:requerido proceso_id:requerido]"},
	}
	for _, c := range cases {
		repo := &fakeRepo{perfil: &perfilRow{InscripcionVigente: true}}
		_, err := NewService(repo, &fakePesos{}).Elegir(context.Background(), 1, c.req)
		var verr *ValidacionError
		switch {
		case c.campos == "" && err != nil:
			t.Errorf("%+v: %v", c.req, err)
		case c.campos != "" && (!errors.As(err, &verr) || fmt.Sprint(verr.Campos) != c.campos || repo.elecciones != 0):
			t.Errorf("%+v: err %v, elecciones %d, want campos %s", c.req, err, repo.elecciones, c.campos)
		}
	}
}

func TestResponderError(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{&ValidacionError{Campos: map[string]string{"proceso_id": "requerido"}}, 422, "validacion"},
		{&ConfirmacionError{Actual: Par{ProcesoID: 12, EspecialidadID: 5}}, 409, "confirmacion_requerida"},
		{ErrInscripcionRequerida, 403, "inscripcion_requerida"},
		{ErrEspecialidadInvalida, 422, "especialidad_invalida"},
		{ErrProfesionNoDefinida, 404, "profesion_no_definida"},
		{ErrEstadoInvalido, 409, "estado_invalido"},
		{mapearActivo(&pq.Error{Code: "23505", Constraint: constraintActivo}), 409, "estado_invalido"},
		{errors.New("pq: boom"), 500, "error_interno"},
	}
	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			rr := httptest.NewRecorder()
			responderError(rr, "test", c.err)
			var body map[string]any
			_ = json.Unmarshal(rr.Body.Bytes(), &body)
			if rr.Code != c.status || body["error"] != c.code || body["message"] == "" {
				t.Errorf("got %d %v, want %d %s", rr.Code, body, c.status, c.code)
			}
			if c.code == "confirmacion_requerida" {
				actual := body["detalle"].(map[string]any)["actual"].(map[string]any)
				if actual["proceso_id"] != 12.0 || actual["especialidad_id"] != 5.0 {
					t.Errorf("detalle.actual = %v", actual)
				}
			}
		})
	}
	if mapearActivo(&pq.Error{Code: "23505", Constraint: "otra"}) == ErrEstadoInvalido {
		t.Error("another unique violation must not map to estado_invalido")
	}
}
