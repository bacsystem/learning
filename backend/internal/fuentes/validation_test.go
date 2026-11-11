package fuentes

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func motivoDe(err error) string {
	var e *ArchivoInvalidoError
	if errors.As(err, &e) {
		return e.Motivo
	}
	return ""
}

// S1 steps 3–6, in order: name, type, encoding, lines.
func TestValidarArchivo(t *testing.T) {
	ok := ArchivoSubido{Nombre: "guia.md", ContentType: "text/markdown", Contenido: []byte("# Guía\nTexto.\n")}
	con := func(mod func(a *ArchivoSubido)) ArchivoSubido { a := ok; mod(&a); return a }
	lineas := func(n int, sep string) []byte { return []byte(strings.Repeat("x"+sep, n)) }
	cases := []struct {
		name   string
		a      ArchivoSubido
		nombre string
		motivo string
	}{
		{"válido", ok, "guia.md", ""},
		{"extensión en mayúsculas", con(func(a *ArchivoSubido) { a.Nombre = "GUIA.MD" }), "GUIA.MD", ""},
		{"ruta relativa saneada", con(func(a *ArchivoSubido) { a.Nombre = "../../etc/x.md" }), "x.md", ""},
		{"ruta de Windows saneada", con(func(a *ArchivoSubido) { a.Nombre = `C:\docs\a.md` }), "a.md", ""},
		{"caracteres de control fuera", con(func(a *ArchivoSubido) { a.Nombre = "gu\x00ia\n.md" }), "guia.md", ""},
		{"text/plain con charset", con(func(a *ArchivoSubido) { a.ContentType = "text/plain; charset=utf-8" }), "guia.md", ""},
		{"BOM aceptado", con(func(a *ArchivoSubido) { a.Contenido = append([]byte{0xEF, 0xBB, 0xBF}, ok.Contenido...) }), "guia.md", ""},
		{"vacío se acepta (el worker lo deja sin_contenido)", con(func(a *ArchivoSubido) { a.Contenido = nil }), "guia.md", ""},
		{"20 000 líneas", con(func(a *ArchivoSubido) { a.Contenido = lineas(20000, "\n") }), "guia.md", ""},
		{"20 000 líneas CRLF", con(func(a *ArchivoSubido) { a.Contenido = lineas(20000, "\r\n") }), "guia.md", ""},
		{"pdf", con(func(a *ArchivoSubido) { a.Nombre = "a.pdf" }), "", MotivoExtension},
		{"doble extensión", con(func(a *ArchivoSubido) { a.Nombre = "a.md.pdf" }), "", MotivoExtension},
		{"sin nombre", con(func(a *ArchivoSubido) { a.Nombre = "" }), "", MotivoExtension},
		{"256 caracteres", con(func(a *ArchivoSubido) { a.Nombre = strings.Repeat("ñ", 253) + ".md" }), "", MotivoExtension},
		{"255 caracteres", con(func(a *ArchivoSubido) { a.Nombre = strings.Repeat("ñ", 252) + ".md" }), strings.Repeat("ñ", 252) + ".md", ""},
		{"octet-stream", con(func(a *ArchivoSubido) { a.ContentType = "application/octet-stream" }), "", MotivoTipo},
		{"tipo vacío", con(func(a *ArchivoSubido) { a.ContentType = "" }), "", MotivoTipo},
		{"extensión antes que tipo", con(func(a *ArchivoSubido) { a.Nombre = "a.pdf"; a.ContentType = "application/pdf" }), "", MotivoExtension},
		{"bytes no UTF-8", con(func(a *ArchivoSubido) { a.Contenido = []byte{0xff, 0xfe, 'a'} }), "", MotivoCodificacion},
		{"byte NUL", con(func(a *ArchivoSubido) { a.Contenido = []byte("a\x00b") }), "", MotivoCodificacion},
		{"20 001 líneas", con(func(a *ArchivoSubido) { a.Contenido = lineas(20001, "\n") }), "", MotivoLineas},
		{"20 001 líneas con CR suelto", con(func(a *ArchivoSubido) { a.Contenido = lineas(20001, "\r") }), "", MotivoLineas},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			nombre, err := validarArchivo(c.a)
			if motivoDe(err) != c.motivo || (c.motivo == "" && (err != nil || nombre != c.nombre)) {
				t.Errorf("got %q, %v; want %q motivo %q", nombre, err, c.nombre, c.motivo)
			}
		})
	}
}

// S1 step 7: the shape of the metadata (existence is checked under the lock).
func TestValidarFormulario(t *testing.T) {
	ok := FormularioFuente{Titulo: "  Guía de vigilancia ", Tipo: "guia", Anio: "2025", ProcesoID: "12", EspecialidadIDs: []string{"5", "2"}}
	con := func(mod func(f *FormularioFuente)) FormularioFuente { f := ok; mod(&f); return f }
	ids := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = string(rune('0'+i%10)) + strings.Repeat("1", i/10+1)
		}
		return out
	}
	cases := []struct {
		name   string
		f      FormularioFuente
		campos map[string]string
	}{
		{"válido", ok, nil},
		{"mínimo", FormularioFuente{Titulo: "abc", Tipo: "otro"}, nil},
		{"título corto", con(func(f *FormularioFuente) { f.Titulo = " ab " }), map[string]string{"titulo": "min"}},
		{"título de 151", con(func(f *FormularioFuente) { f.Titulo = strings.Repeat("é", 151) }), map[string]string{"titulo": "max"}},
		{"sin título ni tipo", FormularioFuente{}, map[string]string{"titulo": "requerido", "tipo": "requerido"}},
		{"tipo desconocido", con(func(f *FormularioFuente) { f.Tipo = "libro" }), map[string]string{"tipo": "invalido"}},
		{"año 1899", con(func(f *FormularioFuente) { f.Anio = "1899" }), map[string]string{"anio": "invalido"}},
		{"año texto", con(func(f *FormularioFuente) { f.Anio = "dos mil" }), map[string]string{"anio": "invalido"}},
		{"proceso 0", con(func(f *FormularioFuente) { f.ProcesoID = "0" }), map[string]string{"proceso_id": "invalido"}},
		{"especialidad repetida", con(func(f *FormularioFuente) { f.EspecialidadIDs = []string{"2", "2"} }), map[string]string{"especialidad_ids": "repetido"}},
		{"especialidad no numérica", con(func(f *FormularioFuente) { f.EspecialidadIDs = []string{"x"} }), map[string]string{"especialidad_ids": "invalido"}},
		{"51 especialidades", con(func(f *FormularioFuente) { f.EspecialidadIDs = ids(51) }), map[string]string{"especialidad_ids": "max"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, err := validarFormulario(c.f)
			if c.campos == nil {
				if err != nil {
					t.Fatalf("want valid, got %v", err)
				}
				return
			}
			var verr *ValidacionError
			if !errors.As(err, &verr) || !reflect.DeepEqual(verr.Campos, c.campos) {
				t.Errorf("got %v (%+v), want campos %v", err, d, c.campos)
			}
		})
	}
	d, _ := validarFormulario(ok)
	if d.Titulo != "Guía de vigilancia" || *d.Anio != 2025 || *d.ProcesoID != 12 || !reflect.DeepEqual(d.EspecialidadIDs, []int64{5, 2}) {
		t.Errorf("parsed = %+v", d)
	}
}

func ptr[T any](v T) *T { return &v }

// camposDe returns the broken fields of a ValidacionError ("" when err is nil).
func camposDe(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		return ""
	}
	var verr *ValidacionError
	if !errors.As(err, &verr) {
		t.Fatalf("err = %v, want a ValidacionError", err)
	}
	return fmt.Sprint(verr.Campos)
}

// S5 shape: a known target and, to retire, a motivo of 5–500 runes after
// trimming, stored trimmed; a motivo for another target is ignored.
func TestValidarCambioEstado(t *testing.T) {
	cases := []struct {
		name   string
		req    CambioEstadoRequest
		campos string
		motivo string
	}{
		{"retirar", CambioEstadoRequest{Estado: "retirada", Motivo: ptr("  Norma derogada  ")}, "", "Norma derogada"},
		{"retirar sin motivo", CambioEstadoRequest{Estado: "retirada"}, "map[motivo:requerido]", ""},
		{"motivo solo espacios", CambioEstadoRequest{Estado: "retirada", Motivo: ptr("     ")}, "map[motivo:requerido]", ""},
		{"motivo de 4 tras recortar", CambioEstadoRequest{Estado: "retirada", Motivo: ptr(" abcd ")}, "map[motivo:min]", ""},
		{"motivo de 5 con tildes", CambioEstadoRequest{Estado: "retirada", Motivo: ptr("ñáéíó")}, "", "ñáéíó"},
		{"motivo de 500", CambioEstadoRequest{Estado: "retirada", Motivo: ptr(strings.Repeat("ñ", 500))}, "", strings.Repeat("ñ", 500)},
		{"motivo de 501", CambioEstadoRequest{Estado: "retirada", Motivo: ptr(strings.Repeat("ñ", 501))}, "map[motivo:max]", ""},
		{"restaurar ignora el motivo", CambioEstadoRequest{Estado: "activa", Motivo: ptr("x")}, "", ""},
		{"reintentar", CambioEstadoRequest{Estado: "procesando"}, "", ""},
		{"sin estado", CambioEstadoRequest{}, "map[estado:requerido]", ""},
		{"estado fallida", CambioEstadoRequest{Estado: "fallida"}, "map[estado:invalido]", ""},
		{"estado desconocido", CambioEstadoRequest{Estado: "borrada"}, "map[estado:invalido]", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tr, err := validarCambioEstado(c.req)
			if got := camposDe(t, err); got != c.campos {
				t.Fatalf("campos = %q, want %q", got, c.campos)
			}
			if err == nil && (tr.Destino != c.req.Estado || tr.Motivo != c.motivo) {
				t.Errorf("transicion = %+v", tr)
			}
		})
	}
}

// S6 shape: oficial_verificada is required; marking needs entidad 3–100 and
// nota 10–500, unmarking a motivo 5–500, all trimmed.
func TestValidarOficial(t *testing.T) {
	cases := []struct {
		name   string
		req    OficialRequest
		campos string
		want   marca
	}{
		{"marcar", OficialRequest{OficialVerificada: ptr(true), EntidadEmisora: ptr(" MINSA "), Nota: ptr(" Web oficial del MINSA ")}, "",
			marca{Marcar: true, EntidadEmisora: "MINSA", Nota: "Web oficial del MINSA"}},
		{"marcar sin campos", OficialRequest{OficialVerificada: ptr(true)}, "map[entidad_emisora:requerido nota:requerido]", marca{}},
		{"entidad de 2", OficialRequest{OficialVerificada: ptr(true), EntidadEmisora: ptr("MI"), Nota: ptr("Nota suficiente")}, "map[entidad_emisora:min]", marca{}},
		{"entidad de 101", OficialRequest{OficialVerificada: ptr(true), EntidadEmisora: ptr(strings.Repeat("e", 101)), Nota: ptr("Nota suficiente")}, "map[entidad_emisora:max]", marca{}},
		{"nota de 9", OficialRequest{OficialVerificada: ptr(true), EntidadEmisora: ptr("MINSA"), Nota: ptr("123456789")}, "map[nota:min]", marca{}},
		{"nota solo espacios", OficialRequest{OficialVerificada: ptr(true), EntidadEmisora: ptr("MINSA"), Nota: ptr("            ")}, "map[nota:requerido]", marca{}},
		{"nota de 501", OficialRequest{OficialVerificada: ptr(true), EntidadEmisora: ptr("MINSA"), Nota: ptr(strings.Repeat("n", 501))}, "map[nota:max]", marca{}},
		{"marcar ignora el motivo", OficialRequest{OficialVerificada: ptr(true), EntidadEmisora: ptr("MINSA"), Nota: ptr("Nota suficiente"), Motivo: ptr("x")}, "",
			marca{Marcar: true, EntidadEmisora: "MINSA", Nota: "Nota suficiente"}},
		{"desmarcar", OficialRequest{OficialVerificada: ptr(false), Motivo: ptr(" Era otra edición ")}, "", marca{Motivo: "Era otra edición"}},
		{"desmarcar sin motivo", OficialRequest{OficialVerificada: ptr(false)}, "map[motivo:requerido]", marca{}},
		{"desmarcar motivo de 4", OficialRequest{OficialVerificada: ptr(false), Motivo: ptr("abcd")}, "map[motivo:min]", marca{}},
		{"desmarcar ignora entidad y nota", OficialRequest{OficialVerificada: ptr(false), Motivo: ptr("Motivo válido"), EntidadEmisora: ptr("x")}, "",
			marca{Motivo: "Motivo válido"}},
		{"sin oficial_verificada", OficialRequest{EntidadEmisora: ptr("MINSA"), Nota: ptr("Nota suficiente")}, "map[oficial_verificada:requerido]", marca{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, err := validarOficial(c.req)
			if got := camposDe(t, err); got != c.campos {
				t.Fatalf("campos = %q, want %q", got, c.campos)
			}
			if err == nil && m != c.want {
				t.Errorf("marca = %+v, want %+v", m, c.want)
			}
		})
	}
}

// S7 shape: titulo 3–150 after trimming and especialidad_ids present (empty
// means all), ≤ 50, > 0, without repeats.
func TestValidarEdicion(t *testing.T) {
	cases := []struct {
		name   string
		req    EditarRequest
		campos string
	}{
		{"válida", EditarRequest{Titulo: "  Balotario  ", EspecialidadIDs: ptr([]int64{5, 2})}, ""},
		{"para todas", EditarRequest{Titulo: "Balotario", EspecialidadIDs: ptr([]int64{})}, ""},
		{"sin especialidad_ids", EditarRequest{Titulo: "Balotario"}, "map[especialidad_ids:requerido]"},
		{"título de 2", EditarRequest{Titulo: " ab ", EspecialidadIDs: ptr([]int64{})}, "map[titulo:min]"},
		{"título vacío", EditarRequest{EspecialidadIDs: ptr([]int64{})}, "map[titulo:requerido]"},
		{"título de 151", EditarRequest{Titulo: strings.Repeat("t", 151), EspecialidadIDs: ptr([]int64{})}, "map[titulo:max]"},
		{"id cero", EditarRequest{Titulo: "Balotario", EspecialidadIDs: ptr([]int64{0})}, "map[especialidad_ids:invalido]"},
		{"repetidos", EditarRequest{Titulo: "Balotario", EspecialidadIDs: ptr([]int64{2, 2})}, "map[especialidad_ids:repetido]"},
		{"51 ids", EditarRequest{Titulo: "Balotario", EspecialidadIDs: ptr(make([]int64, 51))}, "map[especialidad_ids:max]"},
		{"todo mal", EditarRequest{Titulo: "a"}, "map[especialidad_ids:requerido titulo:min]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, err := validarEdicion(c.req)
			if got := camposDe(t, err); got != c.campos {
				t.Fatalf("campos = %q, want %q", got, c.campos)
			}
			if err == nil && (d.Titulo != strings.TrimSpace(c.req.Titulo) || fmt.Sprint(d.EspecialidadIDs) != fmt.Sprint(*c.req.EspecialidadIDs)) {
				t.Errorf("edicion = %+v", d)
			}
		})
	}
}
