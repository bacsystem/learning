package fuentes

import (
	"errors"
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
