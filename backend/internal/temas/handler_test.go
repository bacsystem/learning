package temas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/auth"
	jwtpkg "github.com/cbaciliod/learning/internal/jwt"
	"github.com/cbaciliod/learning/internal/middleware"
)

// fakeServicio answers every call with err (and records the filter and ids it
// got), so the handler's parsing and error translation run without a DB.
type fakeServicio struct {
	err     error
	filtro  Filtro
	ordenID []int64
}

func (f *fakeServicio) Listar(_ context.Context, fl Filtro) (ListaTemas, error) {
	f.filtro = fl
	return ListaTemas{Data: []Tema{}, Page: fl.Page, PerPage: fl.PerPage}, f.err
}
func (f *fakeServicio) Obtener(context.Context, int64) (Tema, error) { return Tema{}, f.err }
func (f *fakeServicio) ListarActivos(context.Context, int64, int64) ([]TemaActivo, error) {
	return []TemaActivo{}, f.err
}
func (f *fakeServicio) Crear(context.Context, CrearTemaRequest, Auditor) (Tema, error) {
	return Tema{}, f.err
}
func (f *fakeServicio) Editar(context.Context, int64, EditarTemaRequest, Auditor) (Tema, error) {
	return Tema{}, f.err
}
func (f *fakeServicio) CambiarEstado(context.Context, int64, CambiarEstadoRequest, Auditor) (Tema, error) {
	return Tema{}, f.err
}
func (f *fakeServicio) Reordenar(_ context.Context, _ int64, ids []int64, _ Auditor) ([]PosicionTema, error) {
	f.ordenID = ids
	return []PosicionTema{}, f.err
}

// adminMux mounts the handler behind a stub that puts admin claims in the
// context, as RequireAuth would.
func adminMux(svc servicio) *http.ServeMux {
	h := NewHandler(svc, nil)
	conClaims := func(next http.HandlerFunc) http.Handler {
		return auth.RequireAuth("unit-secret-32-bytes-exactly-ok!!", func(context.Context, string) (*auth.Session, error) {
			return &auth.Session{UserID: 1, Rol: "admin", Estado: "activo"}, nil
		})(next)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /admin/temas", conClaims(h.Listar))
	mux.Handle("GET /admin/temas/{id}", conClaims(h.Obtener))
	mux.Handle("POST /admin/temas", conClaims(h.Crear))
	mux.Handle("PUT /admin/temas/{id}", conClaims(h.Editar))
	mux.Handle("PATCH /admin/temas/{id}/estado", conClaims(h.CambiarEstado))
	mux.Handle("PUT /admin/asignaturas/{id}/temas/orden", conClaims(h.Reordenar))
	mux.Handle("GET /asignaturas/{id}/temas", conClaims(h.ListarActivos))
	return mux
}

func call(t *testing.T, mux http.Handler, method, path, body string) (int, map[string]any) {
	t.Helper()
	tok, _, err := jwtpkg.Issue("unit-secret-32-bytes-exactly-ok!!", 1, "admin", "", 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("%s %s: body is not a JSON object: %s", method, path, rr.Body)
	}
	return rr.Code, out
}

func TestHandler_TraduceErroresDeDominio(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{&ValidacionError{Campos: map[string]string{"nombre": "min"}}, http.StatusUnprocessableEntity, "validacion"},
		{ErrNoEncontrado, http.StatusNotFound, "no_encontrado"},
		{ErrAsignaturaNoEncontrada, http.StatusNotFound, "no_encontrado"},
		{ErrReferenciaInvalida, http.StatusUnprocessableEntity, "referencia_invalida"},
		{ErrTemaDuplicado, http.StatusUnprocessableEntity, "tema_duplicado"},
		{ErrLimiteTemas, http.StatusUnprocessableEntity, "limite_temas"},
		{ErrEstadoInvalido, http.StatusConflict, "estado_invalido"},
		{ErrOrdenInvalido, http.StatusUnprocessableEntity, "orden_invalido"},
		{ErrOrdenDesactualizado, http.StatusConflict, "orden_desactualizado"},
		{fmt.Errorf("envuelto: %w", ErrLimiteTemas), http.StatusUnprocessableEntity, "limite_temas"},
		{errors.New("pq: connection refused"), http.StatusInternalServerError, "error_interno"},
	}
	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			mux := adminMux(&fakeServicio{err: c.err})
			status, body := call(t, mux, "POST", "/admin/temas", `{"asignatura_id":7,"nombre":"Tema"}`)
			if status != c.status || body["error"] != c.code {
				t.Fatalf("got %d %v, want %d %s", status, body, c.status, c.code)
			}
			if msg, _ := body["message"].(string); msg == "" {
				t.Errorf("missing message: %v", body)
			}
			if c.code == "error_interno" && strings.Contains(fmt.Sprint(body), "connection") {
				t.Errorf("500 leaks the cause: %v", body)
			}
		})
	}
}

func TestHandler_DetalleDeValidacion(t *testing.T) {
	mux := adminMux(&fakeServicio{err: &ValidacionError{Campos: map[string]string{"nombre": "min"}}})
	_, body := call(t, mux, "POST", "/admin/temas", `{"asignatura_id":7,"nombre":"a"}`)
	detalle, _ := body["detalle"].(map[string]any)
	campos, _ := detalle["campos"].(map[string]any)
	if campos["nombre"] != "min" {
		t.Errorf("detalle = %v", body["detalle"])
	}
}

func TestHandler_ErroresDeSintaxis(t *testing.T) {
	cases := []struct {
		name, method, path, body string
		code                     string
	}{
		{"JSON mal formado", "POST", "/admin/temas", `{"nombre":`, "cuerpo_invalido"},
		{"campo desconocido", "POST", "/admin/temas", `{"asignatura_id":7,"nombre":"Tema","color":"rojo"}`, "cuerpo_invalido"},
		{"asignatura_id en PUT", "PUT", "/admin/temas/3", `{"asignatura_id":8,"nombre":"Tema"}`, "cuerpo_invalido"},
		{"id no numérico", "GET", "/admin/temas/abc", ``, "parametro_invalido"},
		{"id cero", "PUT", "/admin/temas/0", `{"nombre":"Tema"}`, "parametro_invalido"},
		{"id negativo en estado", "PATCH", "/admin/temas/-4/estado", `{"estado":"activo"}`, "parametro_invalido"},
		{"tema_ids ausente", "PUT", "/admin/asignaturas/7/temas/orden", `{}`, "cuerpo_invalido"},
		{"tema_ids null", "PUT", "/admin/asignaturas/7/temas/orden", `{"tema_ids":null}`, "cuerpo_invalido"},
		{"tema_ids no es array", "PUT", "/admin/asignaturas/7/temas/orden", `{"tema_ids":"1,2"}`, "cuerpo_invalido"},
		{"tema_ids con texto", "PUT", "/admin/asignaturas/7/temas/orden", `{"tema_ids":[1,"x"]}`, "cuerpo_invalido"},
		{"asignatura de orden inválida", "PUT", "/admin/asignaturas/x/temas/orden", `{"tema_ids":[]}`, "parametro_invalido"},
		{"T7 asignatura inválida", "GET", "/asignaturas/0/temas", ``, "parametro_invalido"},
		{"T7 especialidad inválida", "GET", "/asignaturas/7/temas?especialidad_id=x", ``, "parametro_invalido"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, body := call(t, adminMux(&fakeServicio{}), c.method, c.path, c.body)
			if status != http.StatusBadRequest || body["error"] != c.code {
				t.Errorf("got %d %v, want 400 %s", status, body, c.code)
			}
		})
	}
}

func TestHandler_ReordenarListaVaciaLlegaAlServicio(t *testing.T) {
	svc := &fakeServicio{}
	status, _ := call(t, adminMux(svc), "PUT", "/admin/asignaturas/7/temas/orden", `{"tema_ids":[]}`)
	if status != http.StatusOK || svc.ordenID == nil || len(svc.ordenID) != 0 {
		t.Errorf("status %d, ids %v: an empty list must reach the service", status, svc.ordenID)
	}
}

func TestHandler_FiltroDeListado(t *testing.T) {
	cases := []struct {
		query string
		ok    bool
		want  Filtro
	}{
		{"asignatura_id=7", true, Filtro{AsignaturaID: 7, Page: 1, PerPage: 20}},
		{"asignatura_id=7&especialidad_id=2&estado=archivado&page=3&per_page=5", true,
			Filtro{AsignaturaID: 7, EspecialidadID: 2, Estado: "archivado", Page: 3, PerPage: 5}},
		{"asignatura_id=7&per_page=500", true, Filtro{AsignaturaID: 7, Page: 1, PerPage: 100}},
		{"", false, Filtro{}},
		{"asignatura_id=0", false, Filtro{}},
		{"asignatura_id=x", false, Filtro{}},
		{"asignatura_id=7&especialidad_id=0", false, Filtro{}},
		{"asignatura_id=7&estado=inactivo", false, Filtro{}},
		{"asignatura_id=7&page=0", false, Filtro{}},
		{"asignatura_id=7&per_page=abc", false, Filtro{}},
	}
	for _, c := range cases {
		t.Run(c.query, func(t *testing.T) {
			svc := &fakeServicio{}
			status, body := call(t, adminMux(svc), "GET", "/admin/temas?"+c.query, ``)
			if !c.ok {
				if status != http.StatusBadRequest || body["error"] != "parametro_invalido" {
					t.Errorf("got %d %v, want 400 parametro_invalido", status, body)
				}
				return
			}
			if status != http.StatusOK || svc.filtro != c.want {
				t.Errorf("got %d filtro %+v, want 200 %+v", status, svc.filtro, c.want)
			}
		})
	}
}

func TestHandler_SinClaimsNoAudita(t *testing.T) {
	h := NewHandler(&fakeServicio{}, nil)
	rr := httptest.NewRecorder()
	h.Crear(rr, httptest.NewRequest("POST", "/admin/temas", strings.NewReader(`{}`)))
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", rr.Code)
	}
}

// R3 (docs/api/README.md §3): a JSON body without Content-Length passes the
// middleware, which wraps it in a MaxBytesReader; going past the limit while
// decoding answers 413 cuerpo_demasiado_grande, not 400.
func TestHandler_CuerpoDemasiadoGrande(t *testing.T) {
	const limite = 64
	h := middleware.MaxBodySize(limite, limite)(adminMux(&fakeServicio{}))
	grande := `{"asignatura_id":7,"nombre":"` + strings.Repeat("a", 2*limite) + `"}`
	cases := []struct {
		method, path string
	}{
		{"POST", "/admin/temas"},
		{"PUT", "/admin/temas/3"},
		{"PATCH", "/admin/temas/3/estado"},
		{"PUT", "/admin/asignaturas/7/temas/orden"},
	}
	for _, c := range cases {
		t.Run(c.method+" "+c.path, func(t *testing.T) {
			req := httptest.NewRequest(c.method, c.path, strings.NewReader(grande))
			req.ContentLength = -1 // chunked: only the reader can enforce the limit
			req.Header.Set("Content-Type", "application/json")
			tok, _, err := jwtpkg.Issue("unit-secret-32-bytes-exactly-ok!!", 1, "admin", "", 0, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+tok)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			var body map[string]any
			_ = json.Unmarshal(rr.Body.Bytes(), &body)
			if rr.Code != http.StatusRequestEntityTooLarge || body["error"] != "cuerpo_demasiado_grande" {
				t.Errorf("got %d %v, want 413 cuerpo_demasiado_grande", rr.Code, body)
			}
		})
	}
}
