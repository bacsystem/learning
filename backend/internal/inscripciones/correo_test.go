package inscripciones

import (
	"strings"
	"testing"
	"unicode/utf8"
)

const testBase = "https://app.example.test"

func datos(idioma, estado, motivo string) datosAviso {
	return datosAviso{
		Idioma: idioma, Estado: estado, Motivo: motivo, Nombres: "Ana",
		Proceso: "Proceso 2026", Institucion: "Institución A",
	}
}

func TestRenderAviso_Idioma(t *testing.T) {
	cases := []struct {
		name, idioma, estado, asunto, clave, titulo string
	}{
		{"es aprobada", "es", estadoAprobado, "Tu inscripción fue aprobada: Proceso 2026", claveAprobada, "Inscripción aprobada"},
		{"es rechazada", "es", estadoRechazado, "Tu inscripción fue rechazada: Proceso 2026", claveRechazada, "Inscripción rechazada"},
		{"en aprobada", "en", estadoAprobado, "Your enrollment was approved: Proceso 2026", claveAprobada, "Enrollment approved"},
		{"en rechazada", "en", estadoRechazado, "Your enrollment was rejected: Proceso 2026", claveRechazada, "Enrollment rejected"},
		{"idioma vacio cae a es", "", estadoAprobado, "Tu inscripción fue aprobada: Proceso 2026", claveAprobada, "Inscripción aprobada"},
		{"idioma raro cae a es", "fr", estadoRechazado, "Tu inscripción fue rechazada: Proceso 2026", claveRechazada, "Inscripción rechazada"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := renderAviso(datos(c.idioma, c.estado, "Documento ilegible"), testBase)
			if a.Asunto != c.asunto || a.Clave != c.clave || a.Titulo != c.titulo {
				t.Fatalf("aviso = %q / %q / %q", a.Asunto, a.Clave, a.Titulo)
			}
			if !strings.Contains(a.CorreoHTML, testBase+"/procesos") {
				t.Fatal("the e-mail must link to /procesos")
			}
			if !strings.Contains(a.CorreoHTML, "Proceso 2026") || !strings.Contains(a.CorreoHTML, "Institución A") || !strings.Contains(a.CorreoHTML, "Ana") {
				t.Fatal("the e-mail must name the person, the proceso and the institución")
			}
			if !strings.Contains(a.Cuerpo, "Proceso 2026") {
				t.Fatalf("cuerpo = %q", a.Cuerpo)
			}
		})
	}
}

func TestRenderAviso_Motivo(t *testing.T) {
	rech := renderAviso(datos("en", estadoRechazado, "Documento ilegible"), testBase)
	if !strings.Contains(rech.CorreoHTML, "Documento ilegible") || !strings.Contains(rech.Cuerpo, "Documento ilegible") {
		t.Fatal("a rejection must carry the motivo as the admin wrote it")
	}
	if rech.Parametros["motivo"] != "Documento ilegible" || rech.Parametros["proceso"] != "Proceso 2026" || rech.Parametros["institucion"] != "Institución A" {
		t.Fatalf("parametros = %v", rech.Parametros)
	}
	apro := renderAviso(datos("es", estadoAprobado, "no debe salir"), testBase)
	if _, ok := apro.Parametros["motivo"]; ok || strings.Contains(apro.CorreoHTML, "no debe salir") {
		t.Fatal("an approval must not carry a motivo")
	}
}

// The values from the database reach the HTML escaped (H130); the motivo is
// data, never markup.
func TestRenderAviso_EscapaElHTML(t *testing.T) {
	d := datos("es", estadoRechazado, `<script>alert(1)</script>`)
	d.Proceso = `P<b>"x"`
	d.Nombres = `<i>Ana</i>`
	a := renderAviso(d, testBase)
	for _, raw := range []string{"<script>", "<b>", "<i>Ana"} {
		if strings.Contains(a.CorreoHTML, raw) {
			t.Fatalf("unescaped %q in the e-mail", raw)
		}
	}
	if !strings.Contains(a.CorreoHTML, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatal("the escaped motivo is missing")
	}
}

func TestRenderAviso_TextoNeutro(t *testing.T) {
	for _, idioma := range []string{"es", "en"} {
		for _, estado := range []string{estadoAprobado, estadoRechazado} {
			a := renderAviso(datos(idioma, estado, "x"), testBase)
			texto := strings.ToLower(a.Asunto + a.Titulo + a.Cuerpo + a.CorreoHTML)
			for _, prohibida := range []string{"oficial", "official", "certificado", "certified", "acreditado", "accredited"} {
				if strings.Contains(texto, prohibida) {
					t.Errorf("%s/%s: contains %q", idioma, estado, prohibida)
				}
			}
		}
	}
}

func TestRenderAviso_Limites(t *testing.T) {
	d := datos("es", estadoRechazado, "m")
	d.Proceso = strings.Repeat("ñ", 400)
	a := renderAviso(d, testBase)
	if n := utf8.RuneCountInString(a.Asunto); n > maxAsunto {
		t.Fatalf("asunto has %d characters, want at most %d", n, maxAsunto)
	}
	if n := utf8.RuneCountInString(a.Titulo); n > maxTitulo {
		t.Fatalf("titulo has %d characters, want at most %d", n, maxTitulo)
	}
	if !utf8.ValidString(a.Asunto) {
		t.Fatal("the asunto was cut in the middle of a character")
	}
}
