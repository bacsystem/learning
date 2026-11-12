package balotario

import (
	"regexp"
	"strings"
	"unicode"
)

// texto is a run of text with a per-rune "inside <mark>" mask. The grammar
// works on the cleaned text; the mask tells which alternative carries the key
// even when the tag wrapped only its label (contract E3-01/E3-03 §2.3).
type texto struct {
	r []rune
	m []bool
}

func (t texto) tramo(desde, hasta int) texto {
	return texto{r: t.r[desde:hasta], m: t.m[desde:hasta]}
}

func (t texto) marcado() bool {
	for _, m := range t.m {
		if m {
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
	tagOtra   = iota // any HTML: removed, alert html_eliminado
	tagAbre          // <mark> that closes in the same line
	tagCierra        // its </mark>
	tagSinPar        // <mark> or </mark> without its pair: removed as HTML
)

// limpiar collapses the whitespace of a raw line (the only rewrite of the
// text, §2.1), removes the HTML and returns the cleaned text with the <mark>
// mask. The bool reports that some HTML (or an unpaired mark) was removed.
// Entities are decoded later, after the marks are known (§2.3).
func limpiar(cruda string) (texto, bool) {
	s := strings.Join(strings.Fields(cruda), " ")
	html := false
	if reScriptStyle.MatchString(s) {
		s = reScriptStyle.ReplaceAllString(s, "")
		html = true
	}
	tags := reEtiquetaHTML.FindAllStringSubmatchIndex(s, -1)
	tipos := clasificarTags(s, tags)

	var t texto
	anterior, enMarca := 0, false
	agregar := func(frag string) {
		for _, r := range frag {
			t.r = append(t.r, r)
			t.m = append(t.m, enMarca)
		}
	}
	for i, loc := range tags {
		agregar(s[anterior:loc[0]])
		anterior = loc[1]
		switch tipos[i] {
		case tagAbre:
			enMarca = true
		case tagCierra:
			enMarca = false
		default:
			html = true
			if strings.EqualFold(s[loc[2]:loc[3]], etiquetaSaltoLinea) {
				agregar(" ")
			}
		}
	}
	agregar(s[anterior:])
	return compactar(t), html
}

// clasificarTags pairs each <mark> with the next </mark> of the same line. A
// mark without its pair does not form a mark (§2.3).
func clasificarTags(s string, tags [][]int) []int {
	tipos := make([]int, len(tags))
	abierta := -1
	for i, loc := range tags {
		switch strings.ToLower(s[loc[0]:loc[1]]) {
		case etiquetaMarcaAbre:
			if abierta == -1 {
				abierta = i
			} else {
				tipos[i] = tagSinPar
			}
		case etiquetaMarcaCierra:
			if abierta == -1 {
				tipos[i] = tagSinPar
			} else {
				tipos[abierta], tipos[i] = tagAbre, tagCierra
				abierta = -1
			}
		}
	}
	if abierta != -1 {
		tipos[abierta] = tagSinPar
	}
	return tipos
}

// compactar collapses the spaces that removing a tag may leave and trims.
func compactar(t texto) texto {
	var out texto
	for i, r := range t.r {
		if unicode.IsSpace(r) {
			if n := len(out.r); n > 0 && out.r[n-1] == ' ' {
				out.m[n-1] = out.m[n-1] || t.m[i]
				continue
			}
			r = ' '
		}
		out.r = append(out.r, r)
		out.m = append(out.m, t.m[i])
	}
	return out.recortar()
}
