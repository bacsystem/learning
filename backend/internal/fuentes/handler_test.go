package fuentes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/auth"
	jwtpkg "github.com/cbaciliod/learning/internal/jwt"
	"github.com/cbaciliod/learning/internal/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const secretoUnitario = "unit-secret-32-bytes-exactly-ok!!"

// fakeServicio records the file it got and answers with err.
type fakeServicio struct {
	err     error
	archivo ArchivoSubido
	form    FormularioFuente
	filtro  FiltroListado
	id      int64
	cambios int // calls of S5–S7
}

func (f *fakeServicio) Crear(_ context.Context, a ArchivoSubido, fm FormularioFuente, _ *int64, _ Auditor) (FuenteDetalle, error) {
	f.archivo, f.form = a, fm
	return FuenteDetalle{ResumenFuente: ResumenFuente{ID: 7, Estado: EstadoProcesando}}, f.err
}

func (f *fakeServicio) NuevaVersion(_ context.Context, _ string, a ArchivoSubido, _ *int64, _ Auditor) (FuenteDetalle, error) {
	f.archivo = a
	return FuenteDetalle{ResumenFuente: ResumenFuente{ID: 7}}, f.err
}

func (f *fakeServicio) Listar(_ context.Context, fl FiltroListado) (ListaFuentes, error) {
	f.filtro = fl
	return ListaFuentes{Data: []ResumenFuente{}}, f.err
}

func (f *fakeServicio) Obtener(_ context.Context, id int64) (FuenteDetalle, error) {
	f.id = id
	return FuenteDetalle{ResumenFuente: ResumenFuente{ID: id}}, f.err
}

func (f *fakeServicio) CambiarEstado(_ context.Context, id int64, _ CambioEstadoRequest, _ int64, _ Auditor) (FuenteDetalle, error) {
	f.id, f.cambios = id, f.cambios+1
	return FuenteDetalle{}, f.err
}

func (f *fakeServicio) Oficial(_ context.Context, id int64, _ OficialRequest, _ int64, _ Auditor) (FuenteDetalle, error) {
	f.id, f.cambios = id, f.cambios+1
	return FuenteDetalle{}, f.err
}

func (f *fakeServicio) Editar(_ context.Context, id int64, _ EditarRequest, _ Auditor) (FuenteDetalle, error) {
	f.id, f.cambios = id, f.cambios+1
	return FuenteDetalle{}, f.err
}

// subidaMux is the handler behind the real MaxBodySize with the routes of
// RutasSubida (the list cmd/server/main.go passes) and admin claims.
func subidaMux(svc servicio) http.Handler {
	h := NewHandler(svc, nil)
	conClaims := func(next http.HandlerFunc) http.Handler {
		return auth.RequireAuth(secretoUnitario, func(context.Context, string) (*auth.Session, error) {
			return &auth.Session{UserID: 1, Rol: "admin", Estado: "activo"}, nil
		})(next)
	}
	mux := http.NewServeMux()
	mux.Handle(RutaCrear, conClaims(h.Crear))
	mux.Handle(RutaNuevaVersion, conClaims(h.NuevaVersion))
	mux.Handle(RutaListar, conClaims(h.Listar))
	mux.Handle(RutaObtener, conClaims(h.Obtener))
	mux.Handle(RutaCambiarEstado, conClaims(h.CambiarEstado))
	mux.Handle(RutaOficial, conClaims(h.Oficial))
	mux.Handle(RutaEditar, conClaims(h.Editar))
	return middleware.MaxBodySize(middleware.DefaultJSONBodyLimit, middleware.DefaultMultipartBodyLimit, RutasSubida...)(mux)
}

// multipartBody builds a form with the file part (type tipo) and fields.
func multipartBody(t *testing.T, nombre, tipo string, contenido []byte, campos map[string][]string) (*bytes.Buffer, string) {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for k, vs := range campos {
		for _, v := range vs {
			_ = w.WriteField(k, v)
		}
	}
	if contenido != nil {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, nombre))
		h.Set("Content-Type", tipo)
		part, err := w.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write(contenido)
	}
	_ = w.Close()
	return &b, w.FormDataContentType()
}

func enviar(t *testing.T, h http.Handler, ruta string, body *bytes.Buffer, contentType string) (int, map[string]any) {
	t.Helper()
	method, path, _ := strings.Cut(ruta, " ")
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", contentType)
	tok, _, err := jwtpkg.Issue(secretoUnitario, 1, "admin", "", 0, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	var out map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	return rr.Code, out
}

// S1 step 1 through the real body-limit middleware on both upload routes:
// exactly 2 MiB passes; one byte more is 413 archivo_demasiado_grande.
func TestSubida_Tamano(t *testing.T) {
	for _, ruta := range RutasSubida {
		for _, c := range []struct {
			bytes  int
			status int
		}{{MaxArchivoBytes, http.StatusAccepted}, {MaxArchivoBytes + 1, http.StatusRequestEntityTooLarge}, {3 * 1024 * 1024, http.StatusRequestEntityTooLarge}} {
			t.Run(fmt.Sprintf("%s %d", ruta, c.bytes), func(t *testing.T) {
				svc := &fakeServicio{}
				body, ct := multipartBody(t, "a.md", "text/markdown", bytes.Repeat([]byte("a"), c.bytes),
					map[string][]string{"titulo": {"Guía"}, "tipo": {"guia"}, "fuente_id": {"1"}})
				status, resp := enviar(t, subidaMux(svc), ruta, body, ct)
				if status != c.status {
					t.Fatalf("got %d %v, want %d", status, resp, c.status)
				}
				if c.status == http.StatusRequestEntityTooLarge && resp["error"] != "archivo_demasiado_grande" {
					t.Errorf("code = %v", resp["error"])
				}
				if c.status == http.StatusAccepted && len(svc.archivo.Contenido) != c.bytes {
					t.Errorf("service got %d bytes", len(svc.archivo.Contenido))
				}
			})
		}
	}
}

// S1 step 2 and the form fields reaching the service.
func TestSubida_FormularioYArchivo(t *testing.T) {
	svc := &fakeServicio{}
	body, ct := multipartBody(t, "../g.md", "text/plain; charset=utf-8", []byte("hola"),
		map[string][]string{"titulo": {"Guía"}, "tipo": {"guia"}, "anio": {"2025"}, "proceso_id": {"3"}, "especialidad_ids": {"2", "5"}})
	if status, resp := enviar(t, subidaMux(svc), RutaCrear, body, ct); status != http.StatusAccepted || resp["id"] != 7.0 {
		t.Fatalf("got %d %v", status, resp)
	}
	// mime/multipart already drops the client path; validarArchivo sanitises again.
	if svc.archivo.Nombre != "g.md" || svc.archivo.ContentType != "text/plain; charset=utf-8" || string(svc.archivo.Contenido) != "hola" {
		t.Errorf("archivo = %+v", svc.archivo)
	}
	if svc.form.Titulo != "Guía" || svc.form.Anio != "2025" || svc.form.ProcesoID != "3" || fmt.Sprint(svc.form.EspecialidadIDs) != "[2 5]" {
		t.Errorf("form = %+v", svc.form)
	}

	sinArchivo, ct := multipartBody(t, "", "", nil, map[string][]string{"titulo": {"Guía"}})
	if status, resp := enviar(t, subidaMux(&fakeServicio{}), RutaCrear, sinArchivo, ct); status != 400 || resp["error"] != "cuerpo_invalido" {
		t.Errorf("without file: %d %v", status, resp)
	}
	if status, resp := enviar(t, subidaMux(&fakeServicio{}), RutaCrear, bytes.NewBufferString(`{"titulo":"x"}`), "application/json"); status != 400 || resp["error"] != "cuerpo_invalido" {
		t.Errorf("not multipart: %d %v", status, resp)
	}
}

func TestResponderError(t *testing.T) {
	esp := int64(2)
	cases := []struct {
		err     error
		status  int
		code    string
		detalle string
	}{
		{&ArchivoInvalidoError{Motivo: MotivoTipo}, 422, "archivo_invalido", "map[motivo:tipo]"},
		{&ValidacionError{Campos: map[string]string{"titulo": "min"}}, 422, "validacion", "map[campos:map[titulo:min]]"},
		{ErrReferenciaInvalida, 422, "referencia_invalida", "<nil>"},
		{ErrCuerpoInvalido, 400, "cuerpo_invalido", "<nil>"},
		{&DuplicadaError{FuenteID: 9, Titulo: "A", Estado: "retirada", Version: 2}, 409, "fuente_duplicada", "map[estado:retirada fuente_id:9 titulo:A version:2]"},
		{ErrFuenteDuplicada, 409, "fuente_duplicada", "<nil>"},
		{&LimiteError{Limite: 100, EspecialidadID: &esp}, 422, "limite_fuentes", "map[especialidad_id:2 limite:100]"},
		{&LimiteError{Limite: 500}, 422, "limite_fuentes", "map[limite:500]"},
		{ErrNoEncontrado, 404, "no_encontrado", "<nil>"},
		{ErrEstadoInvalido, 409, "estado_invalido", "<nil>"},
		{ErrLimiteVersiones, 422, "limite_versiones", "<nil>"},
		{errors.New("pq: boom"), 500, "error_interno", "<nil>"},
	}
	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			rr := httptest.NewRecorder()
			responderError(rr, "test", c.err)
			var body map[string]any
			_ = json.Unmarshal(rr.Body.Bytes(), &body)
			if rr.Code != c.status || body["error"] != c.code || body["message"] == "" || fmt.Sprint(body["detalle"]) != c.detalle {
				t.Errorf("got %d %v, want %d %s %s", rr.Code, body, c.status, c.code, c.detalle)
			}
		})
	}
}

// S3 parses its query strictly: every malformed value is 400
// parametro_invalido before the service runs; valid ones reach it parsed.
func TestListado_Parametros(t *testing.T) {
	verdadero, falso := true, false
	cases := []struct {
		name   string
		query  string
		status int
		filtro FiltroListado
	}{
		{"sin filtros", "", 200, FiltroListado{Page: 1, PerPage: 20}},
		{"todos los filtros", "?estado=activa&tipo=norma&especialidad_id=4&oficial=true&q=Guía&page=3&per_page=50", 200,
			FiltroListado{Estado: "activa", Tipo: "norma", EspecialidadID: 4, Oficial: &verdadero, Q: "Guía", Page: 3, PerPage: 50}},
		{"oficial false", "?oficial=false", 200, FiltroListado{Oficial: &falso, Page: 1, PerPage: 20}},
		{"per_page sobre el máximo", "?per_page=500", 200, FiltroListado{Page: 1, PerPage: 100}},
		{"estado desconocido", "?estado=borrada", 400, FiltroListado{}},
		{"estado de versión", "?estado=lista", 400, FiltroListado{}},
		{"tipo desconocido", "?tipo=libro", 400, FiltroListado{}},
		{"especialidad no numérica", "?especialidad_id=abc", 400, FiltroListado{}},
		{"especialidad cero", "?especialidad_id=0", 400, FiltroListado{}},
		{"oficial no booleano", "?oficial=si", 400, FiltroListado{}},
		{"página cero", "?page=0", 400, FiltroListado{}},
		{"per_page negativo", "?per_page=-1", 400, FiltroListado{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := &fakeServicio{}
			status, body := enviar(t, subidaMux(svc), "GET /admin/fuentes"+c.query, &bytes.Buffer{}, "")
			if status != c.status {
				t.Fatalf("got %d %v, want %d", status, body, c.status)
			}
			if c.status == 400 && body["error"] != "parametro_invalido" {
				t.Errorf("code = %v", body["error"])
			}
			if fmt.Sprint(svc.filtro.Oficial == nil, svc.filtro.Estado, svc.filtro.Tipo, svc.filtro.EspecialidadID, svc.filtro.Q, svc.filtro.Page, svc.filtro.PerPage) !=
				fmt.Sprint(c.filtro.Oficial == nil, c.filtro.Estado, c.filtro.Tipo, c.filtro.EspecialidadID, c.filtro.Q, c.filtro.Page, c.filtro.PerPage) ||
				(c.filtro.Oficial != nil && *svc.filtro.Oficial != *c.filtro.Oficial) {
				t.Errorf("filtro = %+v, want %+v", svc.filtro, c.filtro)
			}
		})
	}
}

// S4 answers 400 parametro_invalido for an id that is not a positive number.
func TestObtener_ID(t *testing.T) {
	for _, c := range []struct {
		id     string
		status int
	}{{"9", 200}, {"abc", 400}, {"0", 400}, {"-3", 400}, {"99999999999999999999", 400}} {
		t.Run(c.id, func(t *testing.T) {
			svc := &fakeServicio{}
			status, body := enviar(t, subidaMux(svc), "GET /admin/fuentes/"+c.id, &bytes.Buffer{}, "")
			if status != c.status || (c.status == 400 && body["error"] != "parametro_invalido") || (c.status == 200 && svc.id != 9) {
				t.Errorf("got %d %v (id %d), want %d", status, body, svc.id, c.status)
			}
		})
	}
}

// S5–S7 read the path id and a strict JSON body before the service runs:
// a bad id is 400 parametro_invalido, an unknown field (tipo in S7, say) 400
// cuerpo_invalido and a body over 1 MB 413. That 413 comes from the shared
// MaxBodySize middleware, whose body has no code yet (same on every JSON
// route), so only the status is checked.
func TestCambios_IDYCuerpo(t *testing.T) {
	rutas := map[string]string{
		"S5": "PATCH /admin/fuentes/%s/estado",
		"S6": "PATCH /admin/fuentes/%s/oficial",
		"S7": "PUT /admin/fuentes/%s",
	}
	cases := []struct {
		name   string
		id     string
		body   string
		status int
		code   string
	}{
		{"válido llega al servicio", "9", `{}`, 200, ""},
		{"id no numérico", "abc", `{}`, 400, "parametro_invalido"},
		{"id cero", "0", `{}`, 400, "parametro_invalido"},
		{"campo desconocido", "9", `{"tipo":"guia"}`, 400, "cuerpo_invalido"},
		{"JSON roto", "9", `{`, 400, "cuerpo_invalido"},
		{"cuerpo de más de 1 MB", "9", `{"titulo":"` + strings.Repeat("a", 1<<20) + `"}`, 413, ""},
	}
	for nombre, ruta := range rutas {
		for _, c := range cases {
			t.Run(nombre+" "+c.name, func(t *testing.T) {
				svc := &fakeServicio{}
				status, body := enviar(t, subidaMux(svc), fmt.Sprintf(ruta, c.id), bytes.NewBufferString(c.body), "application/json")
				if status != c.status || (c.code != "" && body["error"] != c.code) {
					t.Fatalf("got %d %v, want %d %s", status, body, c.status, c.code)
				}
				if want := map[bool]int{true: 1, false: 0}[c.status == 200]; svc.cambios != want || (want == 1 && svc.id != 9) {
					t.Errorf("service calls = %d (id %d), want %d", svc.cambios, svc.id, want)
				}
			})
		}
	}
}

// A failing retirement subscriber is a 500 whose log carries op and the
// subscriber's name, never the text of its error (contract S5).
func TestResponderError_SuscriptorSinDatosEnElLog(t *testing.T) {
	var buf bytes.Buffer
	anterior := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = anterior })

	causa := errors.New("pregunta 4 de la persona 12345678")
	rr := httptest.NewRecorder()
	responderError(rr, "cambiar_estado", &SuscriptorError{Nombre: "preguntas", FuenteID: 9, Causa: causa})
	if rr.Code != http.StatusInternalServerError || !strings.Contains(rr.Body.String(), "error_interno") {
		t.Fatalf("got %d %s", rr.Code, rr.Body)
	}
	salida := buf.String()
	if !strings.Contains(salida, `"op":"fuentes.cambiar_estado"`) || !strings.Contains(salida, "preguntas") || !strings.Contains(salida, "fuente 9") ||
		strings.Contains(salida, "12345678") {
		t.Errorf("log = %s", salida)
	}
	if !errors.Is(&SuscriptorError{Causa: causa}, causa) {
		t.Error("the cause is not reachable with errors.Is")
	}
}
