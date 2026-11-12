package balotario

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	// reInicio: number of 1–4 digits plus ".", ")", "-" or ":"; optional
	// "Pregunta"/"Nº" prefix and optional **/__ around the number (§2.2).
	reInicio = regexp.MustCompile(`^(?:\*\*|__)?(?:(?i:pregunta)\s*|N[º°]\s*)?(\d{1,4})(?:\*\*|__)?[.):\-](?:\*\*|__)?(?:\s|$)`)
	// reLetra: a letter with ")", "." or "-", or in parentheses, optionally in **.
	// A–F can be alternatives; G–Z never are, but a line shaped like that is not
	// plain continuation after the first alternative (contract §2.2, rule 17).
	reLetra = regexp.MustCompile(`^(\*\*)?(?:\(([A-Za-z])\)|([A-Za-z])([).\-]))(\*\*)?(?:\s|$)`)
	// reVineta: a bullet without letter must be followed by a space, so that
	// "**bold**" is not one.
	reVineta = regexp.MustCompile(`^[-*•+▪◦](?:\s+|$)`)
)

const negrita = "**"

// estilo is the way an alternative is labelled; it decides whether two labels
// are "of the same style" and whether a balotario mixes styles (§2.2, §2.3).
type estilo struct {
	vineta bool
	mayus  bool
	forma  byte // ')', '.', '-' or '(' for "(a)"
}

// etiqueta is a parsed alternative label. letra is 0 for a bullet.
type etiqueta struct {
	letra rune // 'A'..'Z', always upper case
	// confirmada / tentativa: the label itself is inside a closed / still open <mark>.
	confirmada, tentativa bool
	estilo
}

// inicioPregunta reads "12. text": the number (> 0, because migration 067 only
// accepts 1..9999) and the rest of the line.
func inicioPregunta(t texto) (int, texto, bool) {
	loc := reInicio.FindStringSubmatchIndex(string(t.r))
	if loc == nil {
		return 0, texto{}, false
	}
	numero, err := strconv.Atoi(string(t.r)[loc[2]:loc[3]])
	if err != nil || numero < 1 {
		return 0, texto{}, false
	}
	return numero, t.tramo(runas(string(t.r)[:loc[1]]), len(t.r)).recortar(), true
}

// etiquetaAlternativa reads the label at the start of the line and returns
// the rest of the text.
func etiquetaAlternativa(t texto) (etiqueta, texto, bool) {
	s := string(t.r)
	if loc := reLetra.FindStringSubmatch(s); loc != nil {
		et := etiqueta{}
		switch {
		case loc[2] != "":
			et.letra, et.forma = []rune(strings.ToUpper(loc[2]))[0], '('
			et.mayus = loc[2] == strings.ToUpper(loc[2])
		default:
			et.letra, et.forma = []rune(strings.ToUpper(loc[3]))[0], loc[4][0]
			et.mayus = loc[3] == strings.ToUpper(loc[3])
		}
		et.confirmada, et.tentativa = t.tramo(0, runas(loc[0])).confirmada(), t.tramo(0, runas(loc[0])).tentativa()
		resto := t.tramo(runas(loc[0]), len(t.r)).recortar()
		// "**A) Uno**" wraps the whole line: drop the closing ** as well.
		if loc[1] != "" && loc[5] == "" && strings.HasSuffix(resto.String(), negrita) {
			resto = resto.tramo(0, len(resto.r)-len(negrita)).recortar()
		}
		return et, resto, true
	}
	if loc := reVineta.FindStringIndex(s); loc != nil {
		marca := t.tramo(0, runas(s[:loc[1]]))
		et := etiqueta{estilo: estilo{vineta: true}, confirmada: marca.confirmada(), tentativa: marca.tentativa()}
		return et, t.tramo(runas(s[:loc[1]]), len(t.r)).recortar(), true
	}
	return etiqueta{}, texto{}, false
}

func runas(s string) int { return utf8.RuneCountInString(s) }

// escrita renders the label as it is written, for example "B)" or "(b)".
func (e etiqueta) escrita(letra rune) string {
	l := string(letra)
	if !e.mayus {
		l = strings.ToLower(l)
	}
	if e.forma == '(' {
		return "(" + l + ")"
	}
	return l + string(e.forma)
}

// indiceEtiqueta returns the byte index of the space before "etiqueta" in s
// when the label is followed by a space or the end (so "2.5" or "b)c" are not
// labels), or -1.
func indiceEtiqueta(s, etiqueta string) int {
	desde := 0
	for {
		i := strings.Index(s[desde:], " "+etiqueta)
		if i < 0 {
			return -1
		}
		i += desde
		fin := i + 1 + len(etiqueta)
		if fin == len(s) || s[fin] == ' ' {
			return i
		}
		desde = i + 1
	}
}

// siguiente returns the rune index of " B)" (the next letter with the same
// style, after a space) in t, or -1 (§2.2, "Alternativas pegadas").
func (e etiqueta) siguiente(t texto) int {
	if e.vineta || e.letra >= 'F' {
		return -1
	}
	s := " " + t.String()
	i := indiceEtiqueta(s, e.escrita(e.letra+1))
	if i < 0 {
		return -1
	}
	return runas(s[:i])
}

var todosLosEstilos = func() []etiqueta {
	var out []etiqueta
	for _, mayus := range []bool{true, false} {
		for _, forma := range []byte{')', '.', '-', '('} {
			out = append(out, etiqueta{letra: 'A', estilo: estilo{mayus: mayus, forma: forma}})
		}
	}
	return out
}()

// corteEnunciado returns the rune index where the enunciado ends because the
// alternatives start in the same line: "A)" followed, later in that line, by
// "B)" of the same style. -1 when the line is only enunciado. A match at
// index 0 means the whole text is alternatives.
func corteEnunciado(t texto) int {
	s := " " + t.String()
	mejor := -1
	for _, e := range todosLosEstilos {
		i := indiceEtiqueta(s, e.escrita('A'))
		if i < 0 || indiceEtiqueta(s[i+1:], e.escrita('B')) < 0 {
			continue
		}
		if pos := runas(s[:i]); mejor == -1 || pos < mejor {
			mejor = pos
		}
	}
	return mejor
}
