package balotario_test

// Pruebas de aceptación de QA para SIA-E3-01: un test por escenario de la
// historia, de caja negra (solo la API pública) y con fixtures sintéticos.

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/balotario"
)

func parse(t *testing.T, s string) balotario.Resultado {
	t.Helper()
	r, err := balotario.Parsear([]byte(s))
	if err != nil {
		t.Fatalf("Parsear: %v", err)
	}
	return r
}

func unaValida(t *testing.T, s string) balotario.Pregunta {
	t.Helper()
	r := parse(t, s)
	if len(r.Validas) != 1 || len(r.Problemas) != 0 {
		t.Fatalf("esperaba 1 válida y 0 problemas, got %d/%d: %+v", len(r.Validas), len(r.Problemas), r.Problemas)
	}
	return r.Validas[0]
}

func unProblema(t *testing.T, s string) balotario.Problema {
	t.Helper()
	r := parse(t, s)
	if len(r.Problemas) != 1 || len(r.Validas) != 0 {
		t.Fatalf("esperaba 0 válidas y 1 problema, got %d/%d: %+v", len(r.Validas), len(r.Problemas), r.Validas)
	}
	return r.Problemas[0]
}

func tiene(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func letras(p balotario.Pregunta) string {
	var b strings.Builder
	for _, a := range p.Alternativas {
		b.WriteString(a.Letra)
	}
	return b.String()
}

func TestQA_E301(t *testing.T) {
	t.Run("clave única", func(t *testing.T) {
		p := unaValida(t, "1. ¿Capital?\nA) Lima\n<mark>B) Cusco</mark>\nC) Piura\nD) Ica\n")
		if p.Tipo != "unica" || len(p.Alternativas) != 4 || !p.Alternativas[1].Correcta || p.Alternativas[0].Correcta {
			t.Fatalf("%+v", p)
		}
	})
	t.Run("alternativas pegadas + alerta", func(t *testing.T) {
		p := unaValida(t, "1. Pregunta\nA) Uno B) Dos C) Tres <mark>D) Cuatro</mark>\n")
		if letras(p) != "ABCD" || p.Alternativas[3].Texto != "Cuatro" || !tiene(p.Alertas, "alternativas_separadas") {
			t.Fatalf("%+v", p)
		}
	})
	t.Run("múltiple", func(t *testing.T) {
		p := unaValida(t, "1. P\n<mark>A) a</mark>\n<mark>B) b</mark>\nC) c\n")
		if p.Tipo != "multiple" || !p.Alternativas[0].Correcta || !p.Alternativas[1].Correcta {
			t.Fatalf("%+v", p)
		}
	})
	t.Run("sin clave con línea", func(t *testing.T) {
		pr := unProblema(t, "\n\n1. P\nA) a\nB) b\n")
		if !tiene(pr.Motivos, "sin_clave") || pr.LineaInicio != 3 {
			t.Fatalf("%+v", pr)
		}
	})
	t.Run("exceso: 1001 bloques → limite_preguntas sin resultado", func(t *testing.T) {
		var b strings.Builder
		for i := 1; i <= 1001; i++ {
			if i <= 995 {
				fmt.Fprintf(&b, "%d. P\nA) a\n<mark>B) b</mark>\n\n", i)
			} else {
				fmt.Fprintf(&b, "%d. P\nA) a\nB) b\n\n", i) // con problemas (sin clave)
			}
		}
		r, err := balotario.Parsear([]byte(b.String()))
		if !errors.Is(err, balotario.ErrLimitePreguntas) || len(r.Validas) != 0 || len(r.Problemas) != 0 {
			t.Fatalf("err=%v validas=%d problemas=%d", err, len(r.Validas), len(r.Problemas))
		}
	})
	t.Run("exactamente 1000 sí pasa", func(t *testing.T) {
		var b strings.Builder
		for i := 1; i <= 1000; i++ {
			fmt.Fprintf(&b, "%d. P\nA) a\n<mark>B) b</mark>\n\n", i)
		}
		r, err := balotario.Parsear([]byte(b.String()))
		if err != nil || len(r.Validas) != 1000 {
			t.Fatalf("err=%v n=%d", err, len(r.Validas))
		}
	})
	t.Run("varios motivos", func(t *testing.T) {
		pr := unProblema(t, "1. P\nA) a\n")
		if !tiene(pr.Motivos, "sin_clave") || !tiene(pr.Motivos, "alternativas_insuficientes") {
			t.Fatalf("%+v", pr)
		}
	})
	t.Run("numeración rota 1,2,4,5", func(t *testing.T) {
		var b strings.Builder
		for _, n := range []int{1, 2, 4, 5} {
			fmt.Fprintf(&b, "%d. P\nA) a\n<mark>B) b</mark>\n\n", n)
		}
		r := parse(t, b.String())
		if len(r.Problemas) != 1 || *r.Problemas[0].Numero != 4 || !tiene(r.Problemas[0].Motivos, "numeracion_rota") || r.Problemas[0].LineaInicio != 9 {
			t.Fatalf("%+v", r.Problemas)
		}
		if len(r.Validas) != 3 || r.Validas[2].Numero != 5 {
			t.Fatalf("la 5 debe ser válida: %+v", r.Validas)
		}
	})
	t.Run("numeración por secciones", func(t *testing.T) {
		var b strings.Builder
		for i := 1; i <= 30; i++ {
			fmt.Fprintf(&b, "%d. P\nA) a\n<mark>B) b</mark>\n\n", i)
		}
		b.WriteString("## Sección 2\n\n")
		for i := 1; i <= 20; i++ {
			fmt.Fprintf(&b, "%d. P\nA) a\n<mark>B) b</mark>\n\n", i)
		}
		r := parse(t, b.String())
		if len(r.Validas) != 50 || len(r.Problemas) != 0 {
			t.Fatalf("%d/%d %+v", len(r.Validas), len(r.Problemas), r.Problemas)
		}
	})
	t.Run("textos largos 2001 y 501", func(t *testing.T) {
		pr := unProblema(t, "1. "+strings.Repeat("e", 2001)+"\nA) a\n<mark>B) b</mark>\n")
		if !tiene(pr.Motivos, "enunciado_largo") {
			t.Fatalf("%+v", pr)
		}
		pr = unProblema(t, "1. P\nA) "+strings.Repeat("x", 501)+"\n<mark>B) b</mark>\n")
		if !tiene(pr.Motivos, "alternativa_larga") {
			t.Fatalf("%+v", pr)
		}
		// límites exactos válidos
		unaValida(t, "1. "+strings.Repeat("e", 2000)+"\nA) "+strings.Repeat("x", 500)+"\n<mark>B) b</mark>\n")
	})
	t.Run("marca sobre la etiqueta", func(t *testing.T) {
		p := unaValida(t, "1. P\nA) a\nB) b\n<mark>C)</mark> Tres\n")
		if !p.Alternativas[2].Correcta || p.Alternativas[2].Texto != "Tres" || strings.Contains(p.Alternativas[2].Texto, "mark") {
			t.Fatalf("%+v", p.Alternativas)
		}
	})
	t.Run("HTML residual", func(t *testing.T) {
		p := unaValida(t, "1. Ajuste la <b>dosis</b><script>x</script> diaria\nA) a\n<mark>B) b</mark>\n")
		if p.Enunciado != "Ajuste la dosis diaria" || !tiene(p.Alertas, "html_eliminado") {
			t.Fatalf("%q %v", p.Enunciado, p.Alertas)
		}
	})
	t.Run("menor/mayor no es HTML", func(t *testing.T) {
		p := unaValida(t, "1. Paciente con PA < 90 mmHg y Hb <7 g/dL\nA) a\n<mark>B) b</mark>\n")
		if p.Enunciado != "Paciente con PA < 90 mmHg y Hb <7 g/dL" || tiene(p.Alertas, "html_eliminado") {
			t.Fatalf("%q %v", p.Enunciado, p.Alertas)
		}
	})
	t.Run("líneas del original 120–126", func(t *testing.T) {
		pre := strings.Repeat("texto previo\n", 119)
		r := parse(t, pre+"1. P\nA) a\nB) b\nC) c\nD) d\n<mark>E) e</mark>\nF) f\n")
		if len(r.Validas) != 1 || r.Validas[0].LineaInicio != 120 || r.Validas[0].LineaFin != 126 {
			t.Fatalf("%+v", r)
		}
	})
	t.Run("determinismo", func(t *testing.T) {
		s := "# T\n1. P\nA) a B) b\n<mark>C) c</mark>\n\n2. Q\n- x\n<mark>- y</mark>\n\n3. sin\nA) a\n"
		a, b := parse(t, s), parse(t, s)
		if !reflect.DeepEqual(a, b) {
			t.Fatal("distinto")
		}
	})
	t.Run("conservación del texto", func(t *testing.T) {
		p := unaValida(t, "1.   ¿Qué   *hacer*  con  **Hb**?\n  -   Uno   dos\n  • <mark>Tres  cuatro</mark>\n")
		if p.Enunciado != "¿Qué *hacer* con **Hb**?" || p.Alternativas[0].Texto != "Uno dos" || p.Alternativas[1].Texto != "Tres cuatro" {
			t.Fatalf("%+v", p)
		}
	})
	t.Run("resumen", func(t *testing.T) {
		s := "1. P\nA) a\n<mark>B) b</mark>\n\n2. P\n<mark>A) a</mark>\n<mark>B) b</mark>\nC) c\n\n3. P\nA) a B) b\n\n4. P\nA) a B) b C) c\n<mark>D) d</mark>\n"
		tt := parse(t, s).Totales()
		if tt.Validas != 3 || tt.ConProblemas != 1 || tt.Unica != 2 || tt.Multiple != 1 || tt.ConAlertas != 1 {
			t.Fatalf("%+v", tt)
		}
	})
	t.Run("alternativa vacía", func(t *testing.T) {
		pr := unProblema(t, "1. P\nA) Uno\nB)\n<mark>C) Tres</mark>\n")
		if !tiene(pr.Motivos, "alternativa_vacia") {
			t.Fatalf("%+v", pr)
		}
	})
	t.Run("etiquetas fuera de orden A,B,D", func(t *testing.T) {
		p := unaValida(t, "1. P\nA) Uno\nB) Dos\n<mark>D) Tres</mark>\n")
		if letras(p) != "ABC" || p.Alternativas[2].Texto != "Tres" || !p.Alternativas[2].Correcta || !tiene(p.Alertas, "vinetas_normalizadas") {
			t.Fatalf("%+v", p)
		}
	})
	t.Run("marca sin cerrar", func(t *testing.T) {
		pr := unProblema(t, "1. P\nA) a\nB) b\n<mark>C) Tres\n")
		if !tiene(pr.Motivos, "sin_clave") {
			t.Fatalf("%+v", pr)
		}
	})
	t.Run("E. coli en el enunciado", func(t *testing.T) {
		p := unaValida(t, "1. ¿Cuál es el tratamiento de la infección por\nE. coli enterohemorrágica?\nA) a\nB) b\nC) c\n<mark>D) d</mark>\n")
		if !strings.Contains(p.Enunciado, "E. coli enterohemorrágica?") || letras(p) != "ABCD" {
			t.Fatalf("%q %s", p.Enunciado, letras(p))
		}
	})
	t.Run("C. difficile dentro de alternativas", func(t *testing.T) {
		pr := unProblema(t, "1. P\nA) Antibiótico contra\nC. difficile con vancomicina\n<mark>B) Metronidazol</mark>\nC) Otro\n")
		if !tiene(pr.Motivos, "etiquetas_ambiguas") {
			t.Fatalf("%+v", pr)
		}
	})
	t.Run("letras que retroceden A,C,B,D", func(t *testing.T) {
		pr := unProblema(t, "1. P\nA) a\nC) c\n<mark>B) b</mark>\nD) d\n")
		if !tiene(pr.Motivos, "etiquetas_ambiguas") {
			t.Fatalf("%+v", pr)
		}
	})
	t.Run("etiquetas uniformes a. b. c. d. sin alerta", func(t *testing.T) {
		p := unaValida(t, "1. P\na. uno\nb. dos\n<mark>c. tres</mark>\nd. cuatro\n")
		if letras(p) != "ABCD" || tiene(p.Alertas, "vinetas_normalizadas") || !p.Alternativas[2].Correcta {
			t.Fatalf("%+v", p)
		}
	})
	t.Run("viñetas sin letra o mezcladas", func(t *testing.T) {
		p := unaValida(t, "1. P\n- Uno\nB) Dos\n• <mark>Tres</mark>\nD. Cuatro\n")
		if letras(p) != "ABCD" || !tiene(p.Alertas, "vinetas_normalizadas") || !p.Alternativas[2].Correcta {
			t.Fatalf("%+v", p)
		}
	})
	t.Run("sin intervención → alertas vacía, no nil", func(t *testing.T) {
		p := unaValida(t, "1. P\nA) a\nB) b\nC) c\n<mark>D) d</mark>\n")
		if p.Alertas == nil || len(p.Alertas) != 0 {
			t.Fatalf("%#v", p.Alertas)
		}
	})
	// Reglas adicionales
	t.Run("regla 4: alternativas duplicadas", func(t *testing.T) {
		pr := unProblema(t, "1. P\nA) igual\n<mark>B) igual</mark>\n")
		if !tiene(pr.Motivos, "alternativas_duplicadas") {
			t.Fatalf("%+v", pr)
		}
	})
	t.Run("séptima alternativa con letra → alternativas_excedidas", func(t *testing.T) {
		pr := unProblema(t, "1. P\nA) a\nB) b\nC) c\nD) d\nE) e\nF) f\n<mark>G) g</mark>\n")
		if !tiene(pr.Motivos, "alternativas_excedidas") {
			t.Fatalf("%+v", pr)
		}
	})
	t.Run("letra lejana dentro de las alternativas (S. aureus) → etiquetas_ambiguas", func(t *testing.T) {
		pr := unProblema(t, "1. P\nA) Cubre\nS. aureus resistente\n<mark>B) Vancomicina</mark>\nC) Otro\n")
		if !tiene(pr.Motivos, "etiquetas_ambiguas") {
			t.Fatalf("%+v", pr)
		}
	})
	t.Run("regla 10: más de 6 alternativas", func(t *testing.T) {
		pr := unProblema(t, "1. P\nA) a\nB) b\nC) c\nD) d\nE) e\nF) f\n<mark>G) g</mark>\n")
		if !tiene(pr.Motivos, "alternativas_excedidas") && !tiene(pr.Motivos, "etiquetas_ambiguas") {
			t.Fatalf("%+v", pr)
		}
	})
	t.Run("regla 10: 7 viñetas sin letra → alternativas_excedidas", func(t *testing.T) {
		pr := unProblema(t, "1. P\n- a\n- b\n- c\n- d\n- e\n- f\n<mark>- g</mark>\n")
		if !tiene(pr.Motivos, "alternativas_excedidas") {
			t.Fatalf("%+v", pr)
		}
	})
	t.Run("regla 7: texto fuera de preguntas se excluye", func(t *testing.T) {
		r := parse(t, "BALOTARIO 2025\nPortada\n\n1. P\nA) a\n<mark>B) b</mark>\n\nPágina 1 de 20\n")
		if len(r.Validas) != 1 || r.Validas[0].Alternativas[1].Texto != "b" {
			t.Fatalf("%+v", r)
		}
	})
	t.Run("regla 16: entidad &lt;mark&gt; no cuenta como clave", func(t *testing.T) {
		pr := unProblema(t, "1. P\nA) a\nB) &lt;mark&gt;b&lt;/mark&gt;\n")
		if !tiene(pr.Motivos, "sin_clave") {
			t.Fatalf("%+v", pr)
		}
	})
	t.Run("BOM y CRLF", func(t *testing.T) {
		p := unaValida(t, "\xef\xbb\xbf1. P\r\nA) a\r\n<mark>B) b</mark>\r\n")
		if p.LineaInicio != 1 || len(p.Alternativas) != 2 {
			t.Fatalf("%+v", p)
		}
	})
	t.Run("entrada vacía y sin preguntas", func(t *testing.T) {
		for _, s := range []string{"", "\n\n", "solo texto sin preguntas"} {
			r := parse(t, s)
			if len(r.Validas)+len(r.Problemas) != 0 {
				t.Fatalf("%q → %+v", s, r)
			}
		}
	})
}
