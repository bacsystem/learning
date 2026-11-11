package fuentes

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
)

// palabras returns n distinct words "pN" starting at desde, joined by spaces.
func palabras(desde, n int) string {
	ws := make([]string, n)
	for i := range ws {
		ws[i] = fmt.Sprintf("p%d", desde+i)
	}
	return strings.Join(ws, " ")
}

// oraciones returns n words grouped in sentences of tam words ("… pN.").
func oraciones(desde, n, tam int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "p%d", desde+i)
		if (i+1)%tam == 0 || i == n-1 {
			b.WriteByte('.')
		}
	}
	return b.String()
}

func normalizarOK(t *testing.T, contenido string) []PasajeNuevo {
	t.Helper()
	ps, err := Normalizar([]byte(contenido))
	if err != nil {
		t.Fatalf("Normalizar: %v", err)
	}
	if err := ValidarPasajes(ps); err != nil {
		t.Fatalf("ValidarPasajes: %v (%d pasajes)", err, len(ps))
	}
	return ps
}

func conteos(ps []PasajeNuevo) []int {
	out := make([]int, len(ps))
	for i, p := range ps {
		out[i] = p.Palabras
	}
	return out
}

func ptrTexto(s string) *string { return &s }

// Pasos 1–4 on a short document (a single passage): the text shows the clean-up.
func TestNormalizar_Limpieza(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"BOM", "\uFEFFhola mundo", "hola mundo"},
		{"CRLF and lone CR", "uno\r\ndos\rtres\n", "uno\ndos\ntres"},
		{"control characters", "a\x00b\x07c \x1bd", "abc d"},
		{"tabs and repeated spaces", "a\t\tb    c  d", "a b c d"},
		{"trimmed lines", "   hola   \n  mundo  ", "hola\nmundo"},
		{"invalid UTF-8", "caf\xe9 ok", "caf� ok"},
		{"blank lines collapse to one paragraph break", "a\n\n\n   \n\nb", "a\n\nb"},
		{"mixed bullets", "* uno\n+ dos\n• tres\n▪ cuatro\n◦ cinco\n  * seis\n- siete", "- uno\n- dos\n- tres\n- cuatro\n- cinco\n- seis\n- siete"},
		{"bold is not a bullet", "**negrita** y *cursiva*\n*sin espacio", "**negrita** y *cursiva*\n*sin espacio"},
		{"marks are kept", "la <mark>respuesta</mark> es esta", "la <mark>respuesta</mark> es esta"},
		{"other HTML kept as text", "<b>x</b> <script>y</script>", "<b>x</b> <script>y</script>"},
		{"pasted alternatives are kept", "¿Cuál? a) uno b) dos c) tres d) vitamina b) doce", "¿Cuál? a) uno b) dos c) tres d) vitamina b) doce"},
		{"code block keeps headings and bullets", "texto\n```\n# no es encabezado\n* no es viñeta\n```\n* sí", "texto\n```\n# no es encabezado\n* no es viñeta\n```\n- sí"},
		{"unclosed code block runs to the end", "```\n* a\n# b", "```\n* a\n# b"},
		{"heading opens a paragraph", "intro\n## Título\ncuerpo", "intro\n\n## Título\ncuerpo"},
		{"indented heading and fence are text", "intro\n  ## no\n ```\n* sí", "intro\n## no\n```\n- sí"},
		{"seven hashes or no space is not a heading", "intro\n####### no\n#no", "intro\n####### no\n#no"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ps := normalizarOK(t, c.in)
			if len(ps) != 1 {
				t.Fatalf("got %d pasajes, want 1", len(ps))
			}
			if ps[0].Texto != c.want {
				t.Errorf("Texto %q, want %q", ps[0].Texto, c.want)
			}
		})
	}
}

func TestNormalizar_CuentaPalabras(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"uno dos tres", 3},
		{"- uno | dos — tres", 3},
		{"## Título de sección", 3},
		{"<mark>respuesta</mark> <mark> </mark>", 1},
		{"3,5 mg 10%", 3},
		{"### --- ***", 0},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			ps, err := Normalizar([]byte("x " + c.in))
			if err != nil {
				t.Fatal(err)
			}
			if got := ps[0].Palabras - 1; got != c.want {
				t.Errorf("palabras %d, want %d", got, c.want)
			}
		})
	}
}

// Escenario «Contenido sin texto».
func TestNormalizar_SinContenido(t *testing.T) {
	for _, in := range []string{"", "   \n\t\n\r\n", "\uFEFF", "- * • --- ### |", "\x00\x01"} {
		if _, err := Normalizar([]byte(in)); !errors.Is(err, ErrSinContenido) {
			t.Errorf("%q: got %v, want ErrSinContenido", in, err)
		}
	}
}

// Escenario «Documento corto»: 1–149 words are a single passage.
func TestNormalizar_DocumentoCorto(t *testing.T) {
	for _, n := range []int{1, 120, 149} {
		in := "solo"
		if n > 1 {
			in = "# Título\n\n" + palabras(0, n-2) + "\n\n## Otra\n\n" // both headings count
		}
		ps := normalizarOK(t, in)
		if len(ps) != 1 || ps[0].Palabras != n {
			t.Errorf("n=%d: got %v, want one passage of %d words", n, conteos(ps), n)
		}
	}
}

// Escenario «Sección corta»: a 60-word section between two long ones joins the previous passage.
func TestNormalizar_SeccionCorta(t *testing.T) {
	in := "# A\n\n" + palabras(0, 400) + "\n\n# B\n\n" + palabras(400, 59) + "\n\n# C\n\n" + palabras(500, 400)
	ps := normalizarOK(t, in)
	if got, want := conteos(ps), []int{401 + 60, 401}; !reflect.DeepEqual(got, want) {
		t.Fatalf("palabras %v, want %v", got, want)
	}
	if !strings.Contains(ps[0].Texto, "# B\n\n"+palabras(400, 59)) || *ps[0].Seccion != "A" || *ps[1].Seccion != "C" {
		t.Errorf("the short section must close passage 1: %+v", ps)
	}
}

func TestNormalizar_Union(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []int
	}{
		{
			"short first section joins the next one",
			"# A\n\n" + palabras(0, 59) + "\n\n# B\n\n" + palabras(100, 299),
			[]int{60 + 300},
		},
		{
			"short final remainder joins the previous passage",
			palabras(0, 700) + "\n\n" + palabras(700, 100) + "\n\n" + palabras(800, 100),
			[]int{700, 200},
		},
		{
			"join over 800 splits at the paragraph nearest the middle",
			"# A\n\n" + palabras(0, 300) + "\n\n" + palabras(300, 450) + "\n\n# B\n\n" + palabras(800, 99),
			[]int{1 + 300, 450 + 100},
		},
		{
			"two paragraph cuts equally near the middle: the first one wins",
			"# A\n\n" + palabras(0, 399) + "\n\n" + palabras(400, 100) + "\n\n" + palabras(500, 300) + "\n\n# B\n\n" + palabras(800, 99),
			[]int{1 + 399, 100 + 300 + 100},
		},
		{
			"join over 800 without a paragraph cut splits at a sentence",
			"# A\n\n" + oraciones(0, 750, 50) + "\n\n# B\n\n" + palabras(800, 99),
			[]int{1 + 400, 350 + 100},
		},
		{
			"join over 800 without boundaries splits by words at the middle",
			"# A\n\n" + palabras(0, 750) + "\n\n# B\n\n" + palabras(800, 100),
			[]int{426, 426},
		},
		{
			"several short sections accumulate into the first",
			"# A\n\n" + palabras(0, 49) + "\n\n# B\n\n" + palabras(100, 49) + "\n\n# C\n\n" + palabras(200, 49) + "\n\n# D\n\n" + palabras(300, 49),
			[]int{200},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := conteos(normalizarOK(t, c.in)); !reflect.DeepEqual(got, c.want) {
				t.Errorf("palabras %v, want %v", got, c.want)
			}
		})
	}
}

func TestNormalizar_Division(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []int
	}{
		{"paragraphs accumulate up to 800", palabras(0, 500) + "\n\n" + palabras(500, 300) + "\n\n" + palabras(800, 200), []int{800, 200}},
		{"long paragraph splits by sentences", oraciones(0, 1000, 100), []int{800, 200}},
		{"long sentence splits in balanced parts (801)", palabras(0, 801), []int{401, 400}},
		{"long sentence splits in balanced parts (2401)", palabras(0, 2401), []int{601, 600, 600, 600}},
		{"balanced parts, then the next sentence accumulates", palabras(0, 900) + ". " + palabras(900, 200) + ".", []int{450, 450 + 200}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := conteos(normalizarOK(t, c.in)); !reflect.DeepEqual(got, c.want) {
				t.Errorf("palabras %v, want %v", got, c.want)
			}
		})
	}
}

func TestPartesEquilibradas(t *testing.T) {
	cases := []struct {
		n    int
		want []int
	}{
		{800, []int{800}},
		{801, []int{401, 400}},
		{1600, []int{800, 800}},
		{1601, []int{534, 534, 533}},
	}
	for _, c := range cases {
		if got := partesEquilibradas(c.n); !reflect.DeepEqual(got, c.want) {
			t.Errorf("partesEquilibradas(%d) = %v, want %v", c.n, got, c.want)
		}
	}
}

// Rango de líneas y sección (escenario «Segmentación»).
func TestNormalizar_LineasYSeccion(t *testing.T) {
	larga := strings.Repeat("x", 400)
	in := "\uFEFFintro " + palabras(0, 199) + "\r\n\r\n# " + larga + "\r\n" + palabras(200, 100) + "\r\n\r\n" +
		palabras(300, 100) + "\r\r##   Dos  \n\n" + palabras(400, 200) + "\n"
	ps := normalizarOK(t, in)
	want := []struct {
		inicio, fin int
		seccion     *string
	}{
		{1, 1, nil},
		{3, 6, ptrTexto(larga[:maxSeccion])},
		{8, 10, ptrTexto("Dos")},
	}
	if len(ps) != len(want) {
		t.Fatalf("got %d pasajes (%v), want %d", len(ps), conteos(ps), len(want))
	}
	for i, w := range want {
		p := ps[i]
		if p.Orden != i+1 || p.LineaInicio != w.inicio || p.LineaFin != w.fin || !reflect.DeepEqual(p.Seccion, w.seccion) {
			t.Errorf("pasaje %d: orden %d líneas %d–%d sección %v; want %d–%d %v",
				i, p.Orden, p.LineaInicio, p.LineaFin, p.Seccion, w.inicio, w.fin, w.seccion)
		}
	}
	if n := ContarLineas([]byte(in)); ps[len(ps)-1].LineaFin > n {
		t.Errorf("linea_fin %d beyond the %d lines of the content", ps[len(ps)-1].LineaFin, n)
	}
}

// Escenario «Exceso de pasajes»: 5 000 passages publish, 5 001 fail.
func TestNormalizar_LimitePasajes(t *testing.T) {
	doc := func(n int) []byte {
		var b strings.Builder
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, "# S%d\n\n%s\n\n", i, palabras(0, MinPalabrasPasaje-1))
		}
		return []byte(b.String())
	}
	ps, err := Normalizar(doc(MaxPasajes))
	if err != nil || len(ps) != MaxPasajes {
		t.Fatalf("5000 sections: %d pasajes, %v", len(ps), err)
	}
	if _, err := Normalizar(doc(MaxPasajes + 1)); !errors.Is(err, ErrLimitePasajes) {
		t.Fatalf("5001 sections: got %v, want ErrLimitePasajes", err)
	}
}

// generarDocumento builds a random markdown document of roughly n words with
// headings, paragraphs, sentences, bullets, marks, code blocks, blank lines
// and CRLF, from a fixed seed.
func generarDocumento(r *rand.Rand, n int) string {
	var b strings.Builder
	saltos := []string{"\n", "\r\n", "\r"}
	salto := func() string { return saltos[r.IntN(len(saltos))] }
	w := 0
	for w < n {
		switch r.IntN(12) {
		case 10:
			largo := []int{801, 900, 1700}[r.IntN(3)]
			b.WriteString(palabras(w, largo) + salto() + salto()) // one sentence > 800
			w += largo
		case 11:
			b.WriteString("## ---" + salto() + salto()) // heading without words
		case 0:
			fmt.Fprintf(&b, "%s Sección %d%s%s", strings.Repeat("#", 1+r.IntN(6)), w, salto(), salto())
			w += 2
		case 1:
			b.WriteString("```" + salto() + "# código " + palabras(w, 5) + salto() + "```" + salto())
			w += 7
		case 2:
			for range 1 + r.IntN(5) {
				fmt.Fprintf(&b, "%s %s%s", []string{"*", "+", "•", "-"}[r.IntN(4)], palabras(w, 8), salto())
				w += 8
			}
			b.WriteString(salto())
		default:
			largo := []int{5, 40, 120, 300, 900, 1700}[r.IntN(6)]
			texto := oraciones(w, largo, 1+r.IntN(60))
			if r.IntN(4) == 0 {
				texto = "<mark>" + texto + "</mark>"
			}
			b.WriteString(texto + salto() + strings.Repeat(salto(), r.IntN(3)))
			w += largo
		}
	}
	return b.String()
}

// Test de propiedad (ADR 0003 § Normalización, paso 6): in every document of
// ≥ 150 words each passage has 150–800 words, orden is 1..n, lines are
// increasing and within the content, and no word is lost or repeated.
func TestNormalizar_Propiedad150a800(t *testing.T) {
	r := rand.New(rand.NewPCG(2026, 11))
	for caso := range 300 {
		in := generarDocumento(r, 150+r.IntN(6000))
		ps := normalizarOK(t, in)
		total, lineas, finPrevio := 0, ContarLineas([]byte(in)), 0
		for i, p := range ps {
			if p.Palabras < MinPalabrasPasaje || p.Palabras > MaxPalabrasPasaje {
				t.Fatalf("caso %d, pasaje %d: %d palabras", caso, i+1, p.Palabras)
			}
			if p.Orden != i+1 || p.LineaInicio < finPrevio || p.LineaFin > lineas {
				t.Fatalf("caso %d, pasaje %d: orden %d, líneas %d–%d (previa %d, total %d)", caso, i+1, p.Orden, p.LineaInicio, p.LineaFin, finPrevio, lineas)
			}
			finPrevio = p.LineaFin
			total += p.Palabras
		}
		var todos strings.Builder
		for _, p := range ps {
			todos.WriteString(p.Texto + " ")
		}
		if got, want := strings.Fields(todos.String()), tokensDe(in); !reflect.DeepEqual(got, want) {
			t.Fatalf("caso %d: the passages do not cover the document exactly once", caso)
		}
		if total != contarPalabras(in) {
			t.Fatalf("caso %d: %d words in passages, %d in the document", caso, total, contarPalabras(in))
		}
	}
}

func tokensDe(in string) []string {
	doc := analizar([]byte(in))
	out := make([]string, len(doc.tokens))
	for i, tk := range doc.tokens {
		out[i] = tk.texto
	}
	return out
}

func contarPalabras(in string) int {
	doc := analizar([]byte(in))
	return doc.palabrasAntes[len(doc.tokens)]
}

// Escenario «Segmentación»: 40 000 words with headings.
func TestNormalizar_Segmentacion40000(t *testing.T) {
	var b strings.Builder
	w := 0
	for s := 0; w < 40000; s++ {
		fmt.Fprintf(&b, "## Tema %d\n\n", s)
		for range 1 + s%4 {
			largo := 20 + (s*37)%500
			b.WriteString(oraciones(w, largo, 25) + "\n\n")
			w += largo
		}
	}
	ps := normalizarOK(t, b.String())
	for _, p := range ps {
		if p.Palabras < MinPalabrasPasaje || p.Palabras > MaxPalabrasPasaje || p.Seccion == nil || p.LineaInicio < 1 {
			t.Fatalf("pasaje %d: %d palabras, sección %v, línea %d", p.Orden, p.Palabras, p.Seccion, p.LineaInicio)
		}
	}
}

// Escenario «Idempotencia»: the same content gives identical passages.
func TestNormalizar_Idempotencia(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 7))
	in := []byte(generarDocumento(r, 5000))
	a, errA := Normalizar(in)
	b, errB := Normalizar(append([]byte(nil), in...))
	if errA != nil || errB != nil || !reflect.DeepEqual(a, b) {
		t.Fatalf("two runs differ (%v, %v)", errA, errB)
	}
}

// A heading without text still cuts a block, but its passage has no section.
func TestNormalizar_EncabezadoVacio(t *testing.T) {
	ps := normalizarOK(t, "# A\n\n"+palabras(0, 300)+"\n\n## \n\n"+palabras(300, 300))
	if len(ps) != 2 || ps[0].Seccion == nil || *ps[0].Seccion != "A" || ps[1].Seccion != nil {
		t.Fatalf("got %d pasajes, secciones %v / %v", len(ps), ps[0].Seccion, ps[len(ps)-1].Seccion)
	}
}
