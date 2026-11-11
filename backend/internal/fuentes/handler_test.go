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
)

const secretoUnitario = "unit-secret-32-bytes-exactly-ok!!"

// fakeServicio records the file it got and answers with err.
type fakeServicio struct {
	err     error
	archivo ArchivoSubido
	form    FormularioFuente
}

func (f *fakeServicio) Crear(_ context.Context, a ArchivoSubido, fm FormularioFuente, _ *int64, _ Auditor) (FuenteDetalle, error) {
	f.archivo, f.form = a, fm
	return FuenteDetalle{ID: 7, Estado: EstadoProcesando}, f.err
}

func (f *fakeServicio) NuevaVersion(_ context.Context, _ string, a ArchivoSubido, _ *int64, _ Auditor) (FuenteDetalle, error) {
	f.archivo = a
	return FuenteDetalle{ID: 7}, f.err
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
