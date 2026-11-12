package balotario

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/questions"
)

// alts is the shorthand of an expected alternative: "Texto" or "*Texto" when it is the key.
func alts(textos ...string) []Alternativa {
	out := make([]Alternativa, len(textos))
	for i, t := range textos {
		out[i] = Alternativa{Letra: string(rune('A' + i)), Texto: strings.TrimPrefix(t, "*"), Correcta: strings.HasPrefix(t, "*")}
	}
	return out
}

func parsear(t *testing.T, md string) Resultado {
	t.Helper()
	r, err := Parsear([]byte(md))
	if err != nil {
		t.Fatalf("Parsear: %v", err)
	}
	return r
}

type esperada struct {
	numero          int
	ini, fin        int
	enunciado, tipo string
	alternativas    []Alternativa
	alertas         []string
}

func sinAlertas() []string { return []string{} }

func comprobarValidas(t *testing.T, got []Pregunta, want []esperada) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("validas = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		g := got[i]
		if g.Numero != w.numero || g.LineaInicio != w.ini || g.LineaFin != w.fin || g.Enunciado != w.enunciado || g.Tipo != w.tipo ||
			!reflect.DeepEqual(g.Alternativas, w.alternativas) || !reflect.DeepEqual(g.Alertas, w.alertas) {
			t.Errorf("valida %d = %+v\nwant %+v", i, g, w)
		}
	}
}

func motivosDe(r Resultado) []string {
	var out []string
	for _, p := range r.Problemas {
		n := "?"
		if p.Numero != nil {
			n = fmt.Sprint(*p.Numero)
		}
		out = append(out, fmt.Sprintf("%s@%d-%d:%s", n, p.LineaInicio, p.LineaFin, strings.Join(p.Motivos, "+")))
	}
	return out
}

func TestParsear_Validas(t *testing.T) {
	tests := []struct {
		name string
		md   string
		want []esperada
	}{
		{"clave unica", "1. ¿Cuál es?\nA) Uno\n<mark>B) Dos</mark>\nC) Tres\nD) Cuatro\n",
			[]esperada{{1, 1, 5, "¿Cuál es?", "unica", alts("Uno", "*Dos", "Tres", "Cuatro"), sinAlertas()}}},
		{"seleccion multiple", "1. Pregunta\nA) Uno\n<mark>B) Dos</mark>\n<mark>C) Tres</mark>\n",
			[]esperada{{1, 1, 4, "Pregunta", "multiple", alts("Uno", "*Dos", "*Tres"), sinAlertas()}}},
		{"alternativas pegadas", "1. Pregunta\nA) Uno B) Dos C) <mark>Tres</mark> D) Cuatro\n",
			[]esperada{{1, 1, 2, "Pregunta", "unica", alts("Uno", "Dos", "*Tres", "Cuatro"), []string{AlertaAlternativasSeparadas}}}},
		{"pegadas con el enunciado en la misma linea", "1. ¿Cuál? A) Uno B) <mark>Dos</mark>\n",
			[]esperada{{1, 1, 1, "¿Cuál?", "unica", alts("Uno", "*Dos"), []string{AlertaAlternativasSeparadas}}}},
		{"marca que envuelve solo la etiqueta", "1. P\nA) Uno\n<mark>C)</mark> Tres\n",
			[]esperada{{1, 1, 3, "P", "unica", alts("Uno", "*Tres"), []string{AlertaVinetasNormalizadas}}}},
		{"marca que cruza el corte de pegadas", "1. P\nA) Uno <mark>B) Dos</mark> C) Tres\n",
			[]esperada{{1, 1, 2, "P", "unica", alts("Uno", "*Dos", "Tres"), []string{AlertaAlternativasSeparadas}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := parsear(t, tt.md)
			if len(r.Problemas) != 0 {
				t.Fatalf("problemas: %v", motivosDe(r))
			}
			comprobarValidas(t, r.Validas, tt.want)
		})
	}
}

// "vitamina b)" is lower case: not the same style as "A)", so it is not cut.
func TestParsear_VitaminaNoSeParte(t *testing.T) {
	r := parsear(t, "1. P\nA) Uno B) <mark>con vitamina b) y mas</mark>\n")
	comprobarValidas(t, r.Validas, []esperada{{1, 1, 2, "P", "unica", alts("Uno", "*con vitamina b) y mas"), []string{AlertaAlternativasSeparadas}}})
}

func TestParsear_Estilos(t *testing.T) {
	tests := []struct {
		name    string
		alt     []string
		want    []Alternativa
		alertas []string
	}{
		{"minuscula con parentesis uniforme", []string{"a) Uno", "<mark>b) Dos</mark>", "c) Tres"}, alts("Uno", "*Dos", "Tres"), sinAlertas()},
		{"mayuscula con punto uniforme", []string{"A. Uno", "<mark>B. Dos</mark>"}, alts("Uno", "*Dos"), sinAlertas()},
		{"minuscula con punto uniforme", []string{"a. Uno", "<mark>b. Dos</mark>"}, alts("Uno", "*Dos"), sinAlertas()},
		{"entre parentesis", []string{"(a) Uno", "<mark>(b) Dos</mark>"}, alts("Uno", "*Dos"), sinAlertas()},
		{"con guion", []string{"A- Uno", "<mark>B- Dos</mark>"}, alts("Uno", "*Dos"), sinAlertas()},
		{"viñetas sin letra", []string{"- Uno", "<mark>- Dos</mark>"}, alts("Uno", "*Dos"), []string{AlertaVinetasNormalizadas}},
		{"viñetas de varios simbolos", []string{"* Uno", "• Dos", "<mark>+ Tres</mark>"}, alts("Uno", "Dos", "*Tres"), []string{AlertaVinetasNormalizadas}},
		{"estilos mezclados", []string{"A) Uno", "<mark>b. Dos</mark>"}, alts("Uno", "*Dos"), []string{AlertaVinetasNormalizadas}},
		{"letras fuera de orden", []string{"A) Uno", "B) Dos", "<mark>D) Tres</mark>"}, alts("Uno", "Dos", "*Tres"), []string{AlertaVinetasNormalizadas}},
		{"viñeta, letra, viñeta y letra con salto", []string{"- Uno", "B) Dos", "• Tres", "<mark>D. Cuatro</mark>"}, alts("Uno", "Dos", "Tres", "*Cuatro"), []string{AlertaVinetasNormalizadas}},
		{"envuelta en negrita", []string{"**A)** Uno", "<mark>**B)** Dos</mark>"}, alts("Uno", "*Dos"), sinAlertas()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := parsear(t, "1. P\n"+strings.Join(tt.alt, "\n")+"\n")
			if len(r.Problemas) != 0 {
				t.Fatalf("problemas: %v", motivosDe(r))
			}
			comprobarValidas(t, r.Validas, []esperada{{1, 1, 1 + len(tt.alt), "P", "unica", tt.want, tt.alertas}})
		})
	}
}

func TestParsear_InicioDePregunta(t *testing.T) {
	// The three ways of opening a question of §2.2: (a) none open, (b) the open
	// one already has an alternative, (c) N is the next expected number.
	md := "" +
		"1) Primera\nA) Uno\n<mark>B) Dos</mark>\n" + // (a)
		"2. Segunda\nA) Uno\n<mark>B) Dos</mark>\n3. Tercera\nA) Uno\n<mark>B) Dos</mark>\n" + // (b)
		"4. Cuarta sin alternativas\n5. Quinta\nA) Uno\n<mark>B) Dos</mark>\n" // (c)
	r := parsear(t, md)
	var numeros []int
	for _, v := range r.Validas {
		numeros = append(numeros, v.Numero)
	}
	if fmt.Sprint(numeros) != "[1 2 3 5]" {
		t.Errorf("validas = %v, want [1 2 3 5]", numeros)
	}
	if got := fmt.Sprint(motivosDe(r)); got != "[4@10-10:alternativas_insuficientes+sin_clave]" {
		t.Errorf("problemas = %s", got)
	}
}

func TestParsear_ListaNumeradaEnElEnunciado(t *testing.T) {
	// A number that is not the expected one, with no alternative yet, is
	// continuation; "2." inside question 1 is the next expected one (rule c), so
	// it opens question 2 and the first one stays without alternatives.
	r := parsear(t, "1. Considere:\n3. algo\nA) Uno\n<mark>B) Dos</mark>\n")
	comprobarValidas(t, r.Validas, []esperada{{1, 1, 4, "Considere: 3. algo", "unica", alts("Uno", "*Dos"), sinAlertas()}})

	r = parsear(t, "1. Considere:\n2. algo\nA) Uno\n<mark>B) Dos</mark>\n")
	if got := fmt.Sprint(motivosDe(r)); got != "[1@1-1:alternativas_insuficientes+sin_clave]" {
		t.Errorf("problemas = %s", got)
	}
	if len(r.Validas) != 1 || r.Validas[0].Numero != 2 || r.Validas[0].Enunciado != "algo" {
		t.Errorf("validas = %+v", r.Validas)
	}
}

func TestParsear_Numeracion(t *testing.T) {
	bloque := func(n int) string { return fmt.Sprintf("%d. P\nA) Uno\n<mark>B) Dos</mark>\n", n) }
	tests := []struct {
		name     string
		numeros  []int
		cabecera int // index before which a heading goes; -1 none
		want     []string
	}{
		{"1 2 4 5", []int{1, 2, 4, 5}, -1, []string{"4@7-9:numeracion_rota"}},
		{"repetido", []int{1, 2, 2}, -1, []string{"2@7-9:numeracion_rota"}},
		{"vuelve a 1 sin encabezado", []int{1, 2, 1}, -1, []string{"1@7-9:numeracion_rota"}},
		{"vuelve a 1 tras encabezado", []int{1, 2, 1, 2}, 2, nil},
		{"sigue corrida tras encabezado", []int{1, 2, 3, 4}, 2, nil},
		{"salta tras encabezado", []int{1, 2, 7}, 2, []string{"7@8-10:numeracion_rota"}},
		{"empieza en 5", []int{5, 6}, -1, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var md strings.Builder
			for i, n := range tt.numeros {
				if i == tt.cabecera {
					md.WriteString("# Sección\n")
				}
				md.WriteString(bloque(n))
			}
			r := parsear(t, md.String())
			if got := fmt.Sprint(motivosDe(r)); got != fmt.Sprint(tt.want) {
				t.Errorf("problemas = %s, want %v", got, tt.want)
			}
		})
	}
	t.Run("la 5 es valida tras la 4 rota", func(t *testing.T) {
		r := parsear(t, bloque(1)+bloque(2)+bloque(4)+bloque(5))
		if len(r.Validas) != 3 || r.Validas[2].Numero != 5 {
			t.Errorf("validas = %+v", r.Validas)
		}
	})
	t.Run("cero no abre pregunta", func(t *testing.T) {
		r := parsear(t, "0. Portada\n"+bloque(1))
		if len(r.Validas) != 1 || len(r.Problemas) != 0 {
			t.Errorf("validas=%d problemas=%v", len(r.Validas), motivosDe(r))
		}
	})
}

func TestParsear_Motivos(t *testing.T) {
	largo := func(n int) string { return strings.Repeat("a", n) }
	tests := []struct {
		name string
		md   string
		want string
	}{
		{"sin clave", "1. P\nA) Uno\nB) Dos\n", "[1@1-3:sin_clave]"},
		{"una alternativa y sin clave", "1. P\nA) Uno\n", "[1@1-2:alternativas_insuficientes+sin_clave]"},
		{"una alternativa con clave", "1. P\n<mark>A) Uno</mark>\n", "[1@1-2:alternativas_insuficientes]"},
		{"enunciado vacio", "1.\nA) Uno\n<mark>B) Dos</mark>\n", "[1@1-3:enunciado_vacio]"},
		{"2 alternativas es el minimo", "1. P\nA) Uno\n<mark>B) Dos</mark>\n", "[]"},
		{"6 alternativas es el maximo", "1. P\nA) 1\nB) 2\nC) 3\nD) 4\nE) 5\n<mark>F) 6</mark>\n", "[]"},
		{"7 alternativas", "1. P\nA) 1\nB) 2\nC) 3\nD) 4\nE) 5\nF) 6\n<mark>- 7</mark>\n", "[1@1-8:alternativas_excedidas]"},
		{"alternativa vacia", "1. P\nA) Uno\nB)\n<mark>C) Tres</mark>\n", "[1@1-4:alternativa_vacia]"},
		{"etiqueta sola con continuacion no es vacia", "1. P\nA) Uno\n<mark>B)</mark>\nDos\n", "[]"},
		{"duplicadas por texto plegado", "1. P\nA) Ácido\n<mark>B) acido</mark>\n", "[1@1-3:alternativas_duplicadas]"},
		{"enunciado de 2000", "1. " + largo(2000) + "\nA) Uno\n<mark>B) Dos</mark>\n", "[]"},
		{"enunciado de 2001", "1. " + largo(2001) + "\nA) Uno\n<mark>B) Dos</mark>\n", "[1@1-3:enunciado_largo]"},
		{"alternativa de 500", "1. P\nA) " + largo(500) + "\n<mark>B) Dos</mark>\n", "[]"},
		{"alternativa de 501", "1. P\nA) " + largo(501) + "\n<mark>B) Dos</mark>\n", "[1@1-3:alternativa_larga]"},
		{"el largo cuenta runes", "1. P\nA) " + strings.Repeat("ñ", 500) + "\n<mark>B) Dos</mark>\n", "[]"},
		{"todos los motivos en su orden", "1. P\nA) \nB) " + largo(501) + "\nC) x\nD) X\n", "[1@1-5:alternativa_vacia+alternativa_larga+alternativas_duplicadas+sin_clave]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fmt.Sprint(motivosDe(parsear(t, tt.md))); got != tt.want {
				t.Errorf("problemas = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestParsear_MotivosEnOrden(t *testing.T) {
	// numeracion_rota, enunciado_vacio, alternativas_insuficientes, sin_clave at once.
	r := parsear(t, "1. P\nA) Uno\n<mark>B) Dos</mark>\n3.\n")
	if got := fmt.Sprint(motivosDe(r)); got != "[3@4-4:numeracion_rota+enunciado_vacio+alternativas_insuficientes+sin_clave]" {
		t.Errorf("problemas = %s", got)
	}
}

func TestParsear_HTMLyEntidades(t *testing.T) {
	tests := []struct {
		name      string
		md        string
		enunciado string
		alt       []string
		alertas   []string
	}{
		{"html residual", "1. Ajuste la <b>dosis</b><script>x</script> diaria\nA) Uno\n<mark>B) Dos</mark>\n", "Ajuste la dosis diaria", []string{"Uno", "*Dos"}, []string{AlertaHTMLEliminado}},
		{"menor que no es html", "1. Paciente con PA < 90 mmHg y Hb <7 g/dL\nA) Uno\n<mark>B) Dos</mark>\n", "Paciente con PA < 90 mmHg y Hb <7 g/dL", []string{"Uno", "*Dos"}, sinAlertas()},
		{"style con su contenido", "1. P<style>.a{color:red}</style> fin\nA) Uno\n<mark>B) Dos</mark>\n", "P fin", []string{"Uno", "*Dos"}, []string{AlertaHTMLEliminado}},
		{"br es un espacio", "1. Uno<br>dos\nA) Uno\n<mark>B) Dos</mark>\n", "Uno dos", []string{"Uno", "*Dos"}, []string{AlertaHTMLEliminado}},
		{"html en una alternativa", "1. P\nA) <b>Uno</b>\n<mark>B) Dos</mark>\n", "P", []string{"Uno", "*Dos"}, []string{AlertaHTMLEliminado}},
		{"marca en el enunciado", "1. <mark>P</mark>\nA) Uno\n<mark>B) Dos</mark>\n", "P", []string{"Uno", "*Dos"}, []string{AlertaHTMLEliminado}},
		{"marca sin cerrar", "1. P\n<mark>A) Uno\n<mark>B) Dos</mark>\n", "P", []string{"Uno", "*Dos"}, []string{AlertaHTMLEliminado}},
		{"cierre sin apertura", "1. P\nA) Uno</mark>\n<mark>B) Dos</mark>\n", "P", []string{"Uno", "*Dos"}, []string{AlertaHTMLEliminado}},
		{"entidad de marca es texto", "1. P\nA) &lt;mark&gt;Uno\n<mark>B) Dos</mark>\n", "P", []string{"<mark>Uno", "*Dos"}, sinAlertas()},
		{"entidades se decodifican", "1. a &amp; b &gt; c\nA) Uno\n<mark>B) Dos</mark>\n", "a & b > c", []string{"Uno", "*Dos"}, sinAlertas()},
		{"markdown interno se conserva", "1. **P** _x_\nA) Uno\n<mark>B) Dos</mark>\n", "**P** _x_", []string{"Uno", "*Dos"}, sinAlertas()},
		{"espacios irregulares se colapsan", "1.   P \t  x\nA)    Uno   \n<mark>B)\tDos</mark>\n", "P x", []string{"Uno", "*Dos"}, sinAlertas()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := parsear(t, tt.md)
			if len(r.Problemas) != 0 {
				t.Fatalf("problemas: %v", motivosDe(r))
			}
			comprobarValidas(t, r.Validas, []esperada{{1, 1, 3, tt.enunciado, "unica", alts(tt.alt...), tt.alertas}})
		})
	}
}

func TestParsear_FueraDeLasPreguntas(t *testing.T) {
	md := "" +
		"# BALOTARIO 2025\nPortada del documento\n\n" + // header: out
		"1. P\nA) Uno\n<mark>B) Dos</mark>\n\nPie de página 1\nsigue el pie\n\n" + // footer after a blank line: out
		"2. Q\nA) Uno\n\nB) Dos con linea en blanco antes\n<mark>C) Tres</mark>\n" // blank line followed by another label: continues
	r := parsear(t, md)
	comprobarValidas(t, r.Validas, []esperada{
		{1, 4, 6, "P", "unica", alts("Uno", "*Dos"), sinAlertas()},
		{2, 11, 15, "Q", "unica", alts("Uno", "Dos con linea en blanco antes", "*Tres"), sinAlertas()},
	})
}

func TestParsear_EnunciadoDeVariasLineas(t *testing.T) {
	md := "1. Caso clínico:\nvarón de 40 años\n\ncon fiebre.\nA) Uno\ncontinúa\n<mark>B) Dos</mark>\n"
	r := parsear(t, md)
	comprobarValidas(t, r.Validas, []esperada{{1, 1, 7, "Caso clínico: varón de 40 años con fiebre.", "unica", alts("Uno continúa", "*Dos"), sinAlertas()}})
}

func TestParsear_PrefijosDeNumero(t *testing.T) {
	for _, inicio := range []string{"12. ", "12) ", "12- ", "12: ", "**12.** ", "**12**. ", "__12.__ ", "Pregunta 12: ", "pregunta 12. ", "Nº 12. "} {
		t.Run(strings.TrimSpace(inicio), func(t *testing.T) {
			r := parsear(t, inicio+"¿Cuál?\nA) Uno\n<mark>B) Dos</mark>\n")
			comprobarValidas(t, r.Validas, []esperada{{12, 1, 3, "¿Cuál?", "unica", alts("Uno", "*Dos"), sinAlertas()}})
		})
	}
}

func TestParsear_Preparacion(t *testing.T) {
	md := "\xef\xbb\xbf1. P\r\nA) Uno\r\n<mark>B) Dos</mark>\r\n"
	comprobarValidas(t, parsear(t, md).Validas, []esperada{{1, 1, 3, "P", "unica", alts("Uno", "*Dos"), sinAlertas()}})

	invalido := "1. P\xff\nA) Uno\n<mark>B) Dos</mark>\n"
	if r := parsear(t, invalido); len(r.Validas) != 1 || r.Validas[0].Enunciado != "P�" {
		t.Errorf("utf-8 invalido: %+v", r.Validas)
	}
	if r := parsear(t, ""); len(r.Validas)+len(r.Problemas) != 0 {
		t.Errorf("vacio: %+v", r)
	}
}

func bloquesSinar(n int) string {
	var md strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&md, "%d. Pregunta\nA) Uno\n<mark>B) Dos</mark>\n", i)
	}
	return md.String()
}

func TestParsear_Limite(t *testing.T) {
	if r := parsear(t, bloquesSinar(MaxPreguntas)); len(r.Validas) != MaxPreguntas {
		t.Errorf("1000 bloques: %d validas", len(r.Validas))
	}
	r, err := Parsear([]byte(bloquesSinar(MaxPreguntas + 5)))
	var limite *LimiteError
	if !errors.Is(err, ErrLimitePreguntas) || !errors.As(err, &limite) || limite.Detectadas != MaxPreguntas+5 || len(r.Validas)+len(r.Problemas) != 0 {
		t.Errorf("1005 bloques: err=%v resultado=%+v", err, r)
	}
	if _, err := Parsear([]byte(bloquesSinar(MaxPreguntas + 1))); !errors.Is(err, ErrLimitePreguntas) {
		t.Errorf("1001 bloques: err=%v", err)
	}
	// 995 valid + 6 with problems = 1001 blocks.
	var md strings.Builder
	md.WriteString(bloquesSinar(995))
	for i := 996; i <= 1001; i++ {
		fmt.Fprintf(&md, "%d. Sin clave\nA) Uno\nB) Dos\n", i)
	}
	if _, err := Parsear([]byte(md.String())); !errors.Is(err, ErrLimitePreguntas) {
		t.Errorf("995+6: err=%v", err)
	}
}

func TestParsear_Totales(t *testing.T) {
	md := "" +
		"1. U\nA) x\n<mark>B) y</mark>\n" +
		"2. M\nA) x\n<mark>B) y</mark>\n<mark>C) z</mark>\n" +
		"3. G\nA) x B) <mark>y</mark>\n" +
		"4. Sin clave\nA) x\nB) y\n"
	got := parsear(t, md).Totales()
	if want := (Totales{Validas: 3, ConProblemas: 1, Unica: 2, Multiple: 1, ConAlertas: 1}); got != want {
		t.Errorf("totales = %+v, want %+v", got, want)
	}
}

func TestParsear_Determinista(t *testing.T) {
	md := "1. A) x B) <mark>y</mark>\n2. P\n- a\n<mark>- b</mark>\n3. <b>x</b>\nA) q\n"
	a, b := parsear(t, md), parsear(t, md)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("distintos:\n%+v\n%+v", a, b)
	}
}

func TestParsear_LasValidasPasanValidateRespuestas(t *testing.T) {
	r := parsear(t, bloquesSinar(20)+"21. M\nA) x\n<mark>B) y</mark>\n<mark>C) z</mark>\n")
	for _, p := range r.Validas {
		items := make([]questions.CreateRespuestaItem, len(p.Alternativas))
		for i, a := range p.Alternativas {
			items[i] = questions.CreateRespuestaItem{Texto: a.Texto, EsCorrecta: a.Correcta, Orden: i + 1}
		}
		if err := questions.ValidateRespuestas(p.Tipo, items); err != nil {
			t.Errorf("pregunta %d: %v", p.Numero, err)
		}
	}
}

func TestParsear_SignosDeLimiteDeLineas(t *testing.T) {
	// 120–126 of the original: blank lines before the question count.
	md := strings.Repeat("\n", 119) + "1. P\nA) Uno\nB) Dos\nC) Tres\nD) Cuatro\nE) Cinco\n<mark>F) Seis</mark>\n"
	comprobarValidas(t, parsear(t, md).Validas, []esperada{{1, 120, 126, "P", "unica", alts("Uno", "Dos", "Tres", "Cuatro", "Cinco", "*Seis"), sinAlertas()}})
}

func TestParsear_BalotarioSintetico(t *testing.T) {
	md, err := os.ReadFile("testdata/balotario_sintetico.md")
	if err != nil {
		t.Fatal(err)
	}
	r, err := Parsear(md)
	if err != nil {
		t.Fatal(err)
	}
	var validas []string
	for _, p := range r.Validas {
		validas = append(validas, fmt.Sprintf("%d@%d-%d:%s:%s", p.Numero, p.LineaInicio, p.LineaFin, p.Tipo, strings.Join(p.Alertas, "+")))
	}
	wantValidas := "[1@6-10:unica: 2@12-15:unica:vinetas_normalizadas 3@17-18:unica:alternativas_separadas " +
		"4@20-23:unica:vinetas_normalizadas+html_eliminado 1@27-30:multiple:]"
	if got := fmt.Sprint(validas); got != wantValidas {
		t.Errorf("validas = %s\nwant      %s", got, wantValidas)
	}
	// The 2 has no key, the 4 after the 2 is out of sequence and the 5 has one alternative.
	wantProblemas := "[2@32-34:sin_clave 4@36-38:numeracion_rota 5@40-41:alternativas_insuficientes]"
	if got := fmt.Sprint(motivosDe(r)); got != wantProblemas {
		t.Errorf("problemas = %s, want %s", got, wantProblemas)
	}
	if got, want := r.Totales(), (Totales{Validas: 5, ConProblemas: 3, Unica: 4, Multiple: 1, ConAlertas: 3}); got != want {
		t.Errorf("totales = %+v, want %+v", got, want)
	}
}

func TestParsear_ReglaDeLetras(t *testing.T) {
	t.Run("E. coli dentro del enunciado es continuacion", func(t *testing.T) {
		r := parsear(t, "1. Un paciente con diarrea.\nE. coli enterohemorrágica?\nA) Uno\nB) Dos\n<mark>C) Tres</mark>\nD) Cuatro\n")
		comprobarValidas(t, r.Validas, []esperada{{1, 1, 6, "Un paciente con diarrea. E. coli enterohemorrágica?", "unica", alts("Uno", "Dos", "*Tres", "Cuatro"), sinAlertas()}})
	})
	t.Run("la primera con letra debe ser A", func(t *testing.T) {
		r := parsear(t, "1. P\nB) Uno\n<mark>C) Dos</mark>\n")
		if got := fmt.Sprint(motivosDe(r)); got != "[1@1-3:alternativas_insuficientes+sin_clave]" {
			t.Errorf("problemas = %s", got)
		}
	})
	t.Run("un salto se reetiqueta", func(t *testing.T) {
		r := parsear(t, "1. P\nA) Uno\n<mark>B) Dos</mark>\nE) Tres\n")
		comprobarValidas(t, r.Validas, []esperada{{1, 1, 4, "P", "unica", alts("Uno", "*Dos", "Tres"), []string{AlertaVinetasNormalizadas}}})
	})
}

func TestParsear_SeparadorConEspacio(t *testing.T) {
	t.Run("2.5 mg no abre pregunta", func(t *testing.T) {
		r := parsear(t, "1. Dosis de\n2.5 mg de X\n0.9% NaCl\nA) Uno\n<mark>B) Dos</mark>\n")
		comprobarValidas(t, r.Validas, []esperada{{1, 1, 5, "Dosis de 2.5 mg de X 0.9% NaCl", "unica", alts("Uno", "*Dos"), sinAlertas()}})
	})
	t.Run("A)Uno sin espacio no es etiqueta", func(t *testing.T) {
		r := parsear(t, "1. P\nA) Uno\n<mark>B) Dos</mark>\nC)Tres\n")
		comprobarValidas(t, r.Validas, []esperada{{1, 1, 4, "P", "unica", alts("Uno", "*Dos C)Tres"), sinAlertas()}})
	})
	t.Run("etiqueta sola es alternativa vacia", func(t *testing.T) {
		r := parsear(t, "1. P\nA) Uno\n<mark>B)</mark>\n\n")
		if got := fmt.Sprint(motivosDe(r)); got != "[1@1-3:alternativa_vacia]" {
			t.Errorf("problemas = %s", got)
		}
	})
	t.Run("pegadas: b) pegado a texto no corta", func(t *testing.T) {
		r := parsear(t, "1. P\nA) <mark>Uno</mark> B)Dos\nB) Tres\n")
		comprobarValidas(t, r.Validas, []esperada{{1, 1, 3, "P", "unica", alts("*Uno B)Dos", "Tres"), sinAlertas()}})
	})
}

func TestParsear_MarcasPorElemento(t *testing.T) {
	t.Run("abre en la alternativa y cierra en su continuacion", func(t *testing.T) {
		r := parsear(t, "1. P\nA) Uno\n<mark>B) Dos\ny tres</mark>\nC) Cuatro\n")
		comprobarValidas(t, r.Validas, []esperada{{1, 1, 5, "P", "unica", alts("Uno", "*Dos y tres", "Cuatro"), sinAlertas()}})
	})
	t.Run("abre en una alternativa y cierra en otra: no es marca", func(t *testing.T) {
		r := parsear(t, "1. P\nA) <mark>Uno</mark>\n<mark>B) Dos\nC) Tres</mark>\n")
		comprobarValidas(t, r.Validas, []esperada{{1, 1, 4, "P", "unica", alts("*Uno", "Dos", "Tres"), []string{AlertaHTMLEliminado}}})
	})
	t.Run("abierta sin cerrar nunca", func(t *testing.T) {
		r := parsear(t, "1. P\nA) <mark>Uno</mark>\nB) <mark>Dos\ny mas\n")
		comprobarValidas(t, r.Validas, []esperada{{1, 1, 4, "P", "unica", alts("*Uno", "Dos y mas"), []string{AlertaHTMLEliminado}}})
	})
	t.Run("la alternativa abierta sin cierre no es clave", func(t *testing.T) {
		r := parsear(t, "1. P\nA) Uno\n<mark>B) Dos\n")
		if got := fmt.Sprint(motivosDe(r)); got != "[1@1-3:sin_clave]" {
			t.Errorf("problemas = %s", got)
		}
	})
	t.Run("marca abierta en la linea de la alternativa pegada", func(t *testing.T) {
		r := parsear(t, "1. P\nA) Uno B) <mark>Dos\ny tres</mark>\n")
		comprobarValidas(t, r.Validas, []esperada{{1, 1, 3, "P", "unica", alts("Uno", "*Dos y tres"), []string{AlertaAlternativasSeparadas}}})
	})
}

func TestParsear_FixtureReglasDeGramatica(t *testing.T) {
	md, err := os.ReadFile("testdata/reglas_gramatica.md")
	if err != nil {
		t.Fatal(err)
	}
	r := parsear(t, string(md))
	comprobarValidas(t, r.Validas, []esperada{
		{1, 3, 8, "Un paciente con diarrea sanguinolenta. E. coli enterohemorrágica?", "unica", alts("Uno", "Dos", "*Tres", "Cuatro"), sinAlertas()},
		{2, 10, 14, "Viñetas y letras mezcladas", "unica", alts("Uno", "Dos", "Tres", "*Cuatro"), []string{AlertaVinetasNormalizadas}},
		{3, 16, 20, "Dosis de 2.5 mg de X 0.9% NaCl", "unica", alts("Uno", "*Dos"), sinAlertas()},
		{4, 22, 26, "Marca que abre en la alternativa y cierra en su continuación", "unica", alts("Uno", "*Dos y tres", "Cuatro"), sinAlertas()},
		{5, 28, 31, "Marca que cruza dos alternativas", "unica", alts("*Uno", "Dos", "Tres"), []string{AlertaHTMLEliminado}},
		{6, 33, 34, "Pegadas con marca en medio", "unica", alts("Uno", "*Dos", "Tres"), []string{AlertaAlternativasSeparadas}},
		{8, 37, 39, "Pregunta siguiente", "unica", alts("Uno", "*Dos"), sinAlertas()},
	})
	// The 7 has no alternatives: the 8 is the next expected number (rule c).
	if got := fmt.Sprint(motivosDe(r)); got != "[7@36-36:alternativas_insuficientes+sin_clave]" {
		t.Errorf("problemas = %s", got)
	}
}

func TestParsear_ScriptDeVariasLineas(t *testing.T) {
	// The block disappears with its content and the lines of the rest do not move.
	md := "1. ¿Qué <script>\nalert(1)\n</script> pasa?\nA) Uno\n<mark>B) Dos</mark>\n" +
		"2. <style>\n.a { color: red }\n</style>Otra\nA) Uno\n<mark>B) Dos</mark>\n"
	comprobarValidas(t, parsear(t, md).Validas, []esperada{
		{1, 1, 5, "¿Qué pasa?", "unica", alts("Uno", "*Dos"), []string{AlertaHTMLEliminado}},
		{2, 6, 10, "Otra", "unica", alts("Uno", "*Dos"), []string{AlertaHTMLEliminado}},
	})
	// CRLF line breaks are kept as they are.
	crlf := "1. P <script>\r\nx\r\n</script>\r\nA) Uno\r\n<mark>B) Dos</mark>\r\n"
	comprobarValidas(t, parsear(t, crlf).Validas, []esperada{{1, 1, 5, "P", "unica", alts("Uno", "*Dos"), []string{AlertaHTMLEliminado}}})
}

func TestParsear_CierreDeMarcaAlPrincipioDeLaContinuacion(t *testing.T) {
	r := parsear(t, "1. P\nA) Uno\n<mark>B) Dos\n</mark> y más\n")
	comprobarValidas(t, r.Validas, []esperada{{1, 1, 4, "P", "unica", alts("Uno", "*Dos y más"), sinAlertas()}})
}

func TestParsear_EtiquetasAmbiguas(t *testing.T) {
	tests := []struct {
		name string
		md   string
		want string
	}{
		{"nombre cientifico dentro de las alternativas", "1. P\nA) Antibiótico contra\nC. difficile con vancomicina\n<mark>B) Metronidazol</mark>\nC) Otro\n",
			"[1@1-5:etiquetas_ambiguas]"}, // «C. difficile» abre una C; la B marcada retrocede: el único motivo es la ambigüedad
		{"letras que retroceden", "1. P\nA) Uno\nC) Tres\n<mark>B) Dos</mark>\n", "[1@1-4:etiquetas_ambiguas]"},
		{"letras repetidas", "1. P\nA) Uno\n<mark>B) Dos</mark>\nB) Dos otra vez\n", "[1@1-4:etiquetas_ambiguas]"},
		{"letra repetida con punto", "1. P\na. Uno\n<mark>b. Dos</mark>\nb. otra\n", "[1@1-4:etiquetas_ambiguas]"},
		{"la etiqueta envuelta en negrita tambien", "1. P\nA) Uno\n<mark>B) Dos</mark>\n**A)** otra\n", "[1@1-4:etiquetas_ambiguas]"},
		{"etiqueta sola al final", "1. P\nA) Uno\n<mark>B) Dos</mark>\nA)\n", "[1@1-4:etiquetas_ambiguas]"},
		{"va tras el limite de alternativas y antes de la vacia", "1. P\nA) 1\nB) 2\nC) 3\nD) 4\nE) 5\nF) 6\n<mark>- 7</mark>\nA) x\n",
			"[1@1-9:alternativas_excedidas+etiquetas_ambiguas]"},
		// What does not look like a label: the shape is a letter A–F plus ")" or "." and a space.
		{"con guion tambien es etiqueta", "1. P\nA) Uno\n<mark>B) Dos</mark>\nA- otra\n", "[1@1-4:etiquetas_ambiguas]"},
		{"entre parentesis tambien es etiqueta", "1. P\nA) Uno\n<mark>B) Dos</mark>\n(a) otra\n", "[1@1-4:etiquetas_ambiguas]"},
		{"entre parentesis: la clave pegada a la B", "1. P\n(a) Uno\n(b) Dos\n<mark>(a) Tres</mark>\n", "[1@1-4:etiquetas_ambiguas]"},
		{"con guion: A, C, B", "1. P\nA- Uno\nC- Dos\n<mark>B- Tres</mark>\n", "[1@1-4:etiquetas_ambiguas]"},
		{"ambigua y sin ninguna marca: los dos motivos", "1. P\nA) Uno\nB) Dos\nA) otra\n", "[1@1-4:etiquetas_ambiguas+sin_clave]"},
		{"ambigua con la marca en una alternativa valida", "1. P\n<mark>A) Uno</mark>\nB) Dos\nA) otra\n", "[1@1-4:etiquetas_ambiguas]"},
		{"una marca solo en el enunciado no es clave", "1. <mark>P</mark>\nA) Uno\nB) Dos\nA) otra\n", "[1@1-4:etiquetas_ambiguas+sin_clave]"},
		{"sin espacio tras el separador no parece etiqueta", "1. P\nA) Uno\n<mark>B) Dos</mark>\nA.otra\n", "[]"},
		{"antes de la primera alternativa es enunciado", "1. P\nE. coli enterohemorrágica\nA) Uno\n<mark>B) Dos</mark>\n", "[]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fmt.Sprint(motivosDe(parsear(t, tt.md))); got != tt.want {
				t.Errorf("problemas = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestParsear_BordeDeLaUltimaLetra(t *testing.T) {
	// D) plus "E. coli" at the end opens a real E: the alert comes from the mixed styles.
	r := parsear(t, "1. P\nA) Uno\n<mark>B) Dos</mark>\nC) Tres\nD) Infección por\nE. coli\n")
	comprobarValidas(t, r.Validas, []esperada{{1, 1, 6, "P", "unica", alts("Uno", "*Dos", "Tres", "Infección por", "coli"), []string{AlertaVinetasNormalizadas}}})
	// With "." everywhere the false E is indistinguishable: 5 alternatives, no alert (accepted risk).
	r = parsear(t, "1. P\nA. Uno\n<mark>B. Dos</mark>\nC. Tres\nD. Infección por\nE. coli\n")
	comprobarValidas(t, r.Validas, []esperada{{1, 1, 6, "P", "unica", alts("Uno", "*Dos", "Tres", "Infección por", "coli"), sinAlertas()}})
}

func TestParsear_SaltoHaciaAdelanteNoEsAmbiguo(t *testing.T) {
	r := parsear(t, "1. P\nA) Uno\n<mark>B) Dos</mark>\nD) Tres\n")
	comprobarValidas(t, r.Validas, []esperada{{1, 1, 4, "P", "unica", alts("Uno", "*Dos", "Tres"), []string{AlertaVinetasNormalizadas}}})
}

// The lines of a multi-line script are not blank lines: the text after its
// closing still continues the alternative (H3 of the review of #193).
func TestParsear_ScriptDeVariasLineasDentroDeUnaAlternativa(t *testing.T) {
	r := parsear(t, "1. P\nA) Uno <script>\nx\n</script> más\n<mark>B) Dos</mark>\n")
	comprobarValidas(t, r.Validas, []esperada{{1, 1, 5, "P", "unica", alts("Uno más", "*Dos"), []string{AlertaHTMLEliminado}}})
	// A script on its own lines does not hide the real blank line that follows.
	r = parsear(t, "1. P\nA) Uno\n<mark>B) Dos</mark>\n<script>\nx\n</script>\n\nPie de página\n")
	comprobarValidas(t, r.Validas, []esperada{{1, 1, 3, "P", "unica", alts("Uno", "*Dos"), []string{AlertaHTMLEliminado}}})
}

// Past the limit of 1 000 the parser only counts: it does not keep the
// questions it is going to discard (memory is bounded by the limit, not by the
// size of the file).
func TestParsear_PasadoElLimiteSoloCuenta(t *testing.T) {
	var p lector
	for i, linea := range strings.Split(bloquesSinar(5000), "\n") {
		p.linea(i+1, linea)
	}
	p.cerrar()
	if p.detectados != 5000 {
		t.Errorf("detectados = %d, want 5000", p.detectados)
	}
	if guardadas := len(p.res.Validas) + len(p.res.Problemas); guardadas > MaxPreguntas {
		t.Errorf("guardadas = %d, want at most %d", guardadas, MaxPreguntas)
	}
	if _, err := Parsear([]byte(bloquesSinar(5000))); err == nil {
		t.Error("5000 bloques deben dar ErrLimitePreguntas")
	}
}

// Rule 17, «Letras G–Z»: after the first alternative a line shaped like a label
// with a letter G–Z is never joined to the previous one.
func TestParsear_LetrasGaZ(t *testing.T) {
	seis := "1. P\nA) 1\nB) 2\nC) 3\nD) 4\nE) 5\n<mark>F) 6</mark>\n"
	tests := []struct {
		name string
		md   string
		want string
	}{
		{"G tras la F abre la septima", seis + "G) 7\n", "[1@1-8:alternativas_excedidas]"},
		{"G tras la F con la clave en la G", "1. P\nA) 1\nB) 2\nC) 3\nD) 4\nE) 5\nF) 6\n<mark>G) Siete</mark>\n", "[1@1-8:alternativas_excedidas]"},
		{"G envuelta en negrita", seis + "**G)** 7\n", "[1@1-8:alternativas_excedidas]"},
		{"H tras la G", seis + "G) 7\nH) 8\n", "[1@1-9:alternativas_excedidas]"},
		{"H tras la F no es la siguiente", seis + "H) 7\n", "[1@1-8:etiquetas_ambiguas]"},
		{"Z tras la B", "1. P\nA) Uno\n<mark>B) Dos</mark>\nZ. algo\n", "[1@1-4:etiquetas_ambiguas]"},
		{"G tras la C no es la siguiente", "1. P\nA) Uno\n<mark>B) Dos</mark>\nC) Tres\nG) siete\n", "[1@1-5:etiquetas_ambiguas]"},
		{"minuscula entre parentesis", "1. P\n(a) Uno\n<mark>(b) Dos</mark>\n(z) algo\n", "[1@1-4:etiquetas_ambiguas]"},
		{"S. aureus dentro de las alternativas", "1. P\nA) Infección por\nS. aureus\n<mark>B) Otra</mark>\n", "[1@1-4:etiquetas_ambiguas]"},
		{"ambigua sin ninguna marca: los dos motivos", "1. P\nA) Uno\nB) Dos\nZ. algo\n", "[1@1-4:etiquetas_ambiguas+sin_clave]"},
		// Before the first alternative nothing changes: continuation of the enunciado.
		{"S. aureus en el enunciado", "1. Infección por\nS. aureus\nA) Uno\n<mark>B) Dos</mark>\n", "[]"},
		{"G. lamblia en el enunciado", "1. Parásito\nG. lamblia en niños\nA) Uno\n<mark>B) Dos</mark>\n", "[]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fmt.Sprint(motivosDe(parsear(t, tt.md))); got != tt.want {
				t.Errorf("problemas = %s, want %s", got, tt.want)
			}
		})
	}
	t.Run("el enunciado conserva S. aureus", func(t *testing.T) {
		r := parsear(t, "1. Infección por\nS. aureus\nA) Uno\n<mark>B) Dos</mark>\n")
		comprobarValidas(t, r.Validas, []esperada{{1, 1, 4, "Infección por S. aureus", "unica", alts("Uno", "*Dos"), sinAlertas()}})
	})
}

// A letter G–Z after a blank line is text that follows the question: the blank
// line closes it, as it always did. Only the next letter (G after F) goes on.
func TestParsear_LetrasGaZTrasLineaEnBlanco(t *testing.T) {
	r := parsear(t, "1. P\nA) Uno\n<mark>B) Dos</mark>\n\nS. Pérez, 2020\n")
	comprobarValidas(t, r.Validas, []esperada{{1, 1, 3, "P", "unica", alts("Uno", "*Dos"), sinAlertas()}})

	seis := "1. P\nA) 1\nB) 2\nC) 3\nD) 4\nE) 5\n<mark>F) 6</mark>\n\n"
	if got := fmt.Sprint(motivosDe(parsear(t, seis+"G) 7\n"))); got != "[1@1-9:alternativas_excedidas]" {
		t.Errorf("G tras F y un blanco: %s", got)
	}
	// A–F that go back after a blank line stay ambiguous (the blank line is not a footer).
	if got := fmt.Sprint(motivosDe(parsear(t, "1. P\nA) Uno\n<mark>B) Dos</mark>\n\nA) otra\n"))); got != "[1@1-5:etiquetas_ambiguas]" {
		t.Errorf("A tras un blanco: %s", got)
	}
}
