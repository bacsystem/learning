package balotario

import (
	"regexp"
	"strings"
	"unicode"
)

// texto is a run of text with a per-rune "inside <mark>" mask. The grammar
// works on the cleaned text; the mask tells which alternative carries the key
// even when the tag wrapped only its label (contract E3-01/E3-03 §2.3).
//
// tent marks the runes of a <mark> that is still open at the end of the line:
// it only counts if a later line of the same alternative closes it (§2.3).
type texto struct {
	r    []rune
	m    []bool
	tent []bool
}

func (t texto) tramo(desde, hasta int) texto {
	return texto{r: t.r[desde:hasta], m: t.m[desde:hasta], tent: t.tent[desde:hasta]}
}

func (t texto) agregar(r rune, marca, tentativa bool) texto {
	t.r, t.m, t.tent = append(t.r, r), append(t.m, marca), append(t.tent, tentativa)
	return t
}

// marcado: some rune is inside a <mark>, closed or not.
func (t texto) marcado() bool { return algun(t.m, nil) }

// confirmada: some rune is inside a <mark> that is closed.
func (t texto) confirmada() bool { return algun(t.m, t.tent) }

// tentativa: some rune is inside a <mark> still open at the end of the line.
func (t texto) tentativa() bool { return algun(t.tent, nil) }

// algun reports whether a[i] is true (and excluir[i] is not) for some i.
func algun(a, excluir []bool) bool {
	for i, v := range a {
		if v && (excluir == nil || !excluir[i]) {
			return true
		}
	}
	return false
}

func (t texto) String() string { return string(t.r) }

// recortar drops the spaces at both ends.
func (t texto) recortar() texto {
	desde, hasta := 0, len(t.r)
	for desde < hasta && unicode.IsSpace(t.r[desde]) {
		desde++
	}
	for hasta > desde && unicode.IsSpace(t.r[hasta-1]) {
		hasta--
	}
	return t.tramo(desde, hasta)
}

var (
	// reScriptStyle matches <script> and <style> with their content (rule 16).
	reScriptStyle = regexp.MustCompile(`(?is)<(script|style)(?:\s[^<>]*)?>.*?</(script|style)\s*>`)
	// reEtiquetaHTML matches a recognisable tag; a "<" that is not one ("PA < 90") is text.
	reEtiquetaHTML = regexp.MustCompile(`</?([A-Za-z][A-Za-z0-9-]*)(?:\s[^<>]*)?/?>`)
)

const (
	etiquetaMarcaAbre   = "<mark>"
	etiquetaMarcaCierra = "</mark>"
	etiquetaSaltoLinea  = "br"
)

// Kind of every tag found in a line.
const (
	tagOtra          = iota // any HTML: removed, alert html_eliminado
	tagAbre                 // <mark> that closes in the same line
	tagCierra               // its </mark>
	tagSinPar               // <mark> or </mark> without its pair: removed as HTML
	tagAbreSinCierre        // <mark> still open at the end of the line (a continuation may close it)
)

// lineaLimpia is the result of limpiar.
type lineaLimpia struct {
	t texto
	// html: some HTML (or an unpaired mark) was removed.
	html bool
	// abierta: a <mark> is still open at the end of the line.
	abierta bool
	// cerroInicial: the <mark> that came open from the previous line was closed here.
	cerroInicial bool
}

// limpiar collapses the whitespace of a raw line (the only rewrite of the
// text, §2.1), removes the HTML and returns the cleaned text with the <mark>
// mask. inicial says that a <mark> of the previous line of the same
// alternative is still open. Entities are decoded later, after the marks are
// known (§2.3).
func limpiar(cruda string, inicial bool) lineaLimpia {
	s := strings.Join(strings.Fields(cruda), " ")
	out := lineaLimpia{}
	if reScriptStyle.MatchString(s) {
		s = reScriptStyle.ReplaceAllString(s, "")
		out.html = true
	}
	tags := reEtiquetaHTML.FindAllStringSubmatchIndex(s, -1)
	tipos, cerroInicial := clasificarTags(s, tags, inicial)
	out.cerroInicial = cerroInicial

	anterior, enMarca := 0, inicial
	enTent := inicial && !cerroInicial
	agregar := func(frag string) {
		for _, r := range frag {
			out.t = out.t.agregar(r, enMarca, enTent)
		}
	}
	for i, loc := range tags {
		agregar(s[anterior:loc[0]])
		anterior = loc[1]
		switch tipos[i] {
		case tagAbre:
			enMarca = true
		case tagAbreSinCierre:
			enMarca, enTent = true, true
		case tagCierra:
			enMarca, enTent = false, false
		default:
			out.html = true
			if strings.EqualFold(s[loc[2]:loc[3]], etiquetaSaltoLinea) {
				agregar(" ")
			}
		}
	}
	agregar(s[anterior:])
	out.abierta = enMarca
	out.t = compactar(out.t)
	return out
}

// clasificarTags pairs each <mark> with the next </mark> of the same line. A
// <mark> that stays open at the end of the line may be closed by a
// continuation of the same alternative; a </mark> without an open one, or a
// <mark> inside another, does not form a mark (§2.3). The bool tells that
// the mark that came open from the previous line was closed here.
func clasificarTags(s string, tags [][]int, inicial bool) ([]int, bool) {
	const deLaLineaAnterior = -2
	tipos := make([]int, len(tags))
	abierta, cerroInicial := -1, false
	if inicial {
		abierta = deLaLineaAnterior
	}
	for i, loc := range tags {
		switch strings.ToLower(s[loc[0]:loc[1]]) {
		case etiquetaMarcaAbre:
			if abierta == -1 {
				abierta = i
			} else {
				tipos[i] = tagSinPar
			}
		case etiquetaMarcaCierra:
			switch abierta {
			case -1:
				tipos[i] = tagSinPar
			case deLaLineaAnterior:
				tipos[i], cerroInicial, abierta = tagCierra, true, -1
			default:
				tipos[abierta], tipos[i], abierta = tagAbre, tagCierra, -1
			}
		}
	}
	if abierta >= 0 {
		tipos[abierta] = tagAbreSinCierre
	}
	return tipos, cerroInicial
}

// compactar collapses the spaces that removing a tag may leave and trims.
func compactar(t texto) texto {
	var out texto
	for i, r := range t.r {
		if unicode.IsSpace(r) {
			if n := len(out.r); n > 0 && out.r[n-1] == ' ' {
				out.m[n-1] = out.m[n-1] || t.m[i]
				out.tent[n-1] = out.tent[n-1] && t.tent[i]
				continue
			}
			r = ' '
		}
		out = out.agregar(r, t.m[i], t.tent[i])
	}
	return out.recortar()
}
