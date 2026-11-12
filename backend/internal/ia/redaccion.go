package ia

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// patronDatoPersonal is one personal data pattern (regla 3, contract §4), RE2
// without lookbehind. soloAislado asks that no digit touches the match: it
// replaces \b, which fails when the number is glued to a "+" or to letters.
type patronDatoPersonal struct {
	tipo        string
	re          *regexp.Regexp
	soloAislado bool
}

// The input that matches is rejected, not masked: it never leaves the server.
var patronesDatosPersonales = []patronDatoPersonal{
	// 8 digits with no digit beside them: "DNI12345678" and "A12345678"
	// match, "123456789" does not. A date like 20261111 also matches; it is
	// an accepted false positive.
	{TipoDNI, regexp.MustCompile(`\d{8}`), true},
	{TipoEmail, regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`), false},
	// Peruvian mobile, with an optional +51 that the leftmost-first match
	// tries before the form without it.
	{TipoTelefono, regexp.MustCompile(`(?:\+?51[\s-]?)?9\d{2}[\s-]?\d{3}[\s-]?\d{3}`), true},
	// Landline with its area code in parentheses.
	{TipoTelefono, regexp.MustCompile(`\(0\d{1,2}\)\s?\d{6,7}`), false},
}

// marcadorEnmascarado is what Enmascarar writes in place of a match. It has no
// digits nor "@", so masking a masked text changes nothing.
func marcadorEnmascarado(tipo string) string { return "[" + tipo + "]" }

type coincidencia struct {
	tipo     string
	ini, fin int // byte offsets in the text
}

// buscar returns every match of the pattern. A candidate that fails the digit
// boundary is skipped by one rune from its start, never past its end: it may
// overlap the real number ("Ref 12345678901 987654321" first yields
// "901 987654", which hides 987654321 from a plain FindAll).
func (p patronDatoPersonal) buscar(texto string) []coincidencia {
	var res []coincidencia
	for pos := 0; pos < len(texto); {
		loc := p.re.FindStringIndex(texto[pos:])
		if loc == nil {
			break
		}
		ini, fin := pos+loc[0], pos+loc[1]
		if p.soloAislado && !sinDigitosAlLado(texto, ini, fin) {
			_, ancho := utf8.DecodeRuneInString(texto[ini:])
			pos = ini + ancho
			continue
		}
		res = append(res, coincidencia{p.tipo, ini, fin})
		pos = fin
	}
	return res
}

// plegarDigitos replaces every Unicode decimal digit (Nd) with its ASCII
// digit, so that full-width, Arabic-Indic or mathematical digits cannot hide
// a number from the ASCII patterns. Standard library only (no NFKC).
func plegarDigitos(texto string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x80 || !unicode.Is(unicode.Nd, r) {
			return r
		}
		return '0' + valorDigito(r)
	}, texto)
}

// valorDigito is the value of an Nd rune: its offset in the range of the
// table, modulo 10 (every Nd range holds whole sets of ten digits).
func valorDigito(r rune) rune {
	for _, rg := range unicode.Nd.R16 {
		if r >= rune(rg.Lo) && r <= rune(rg.Hi) {
			return (r - rune(rg.Lo)) % 10
		}
	}
	for _, rg := range unicode.Nd.R32 {
		if r >= rune(rg.Lo) && r <= rune(rg.Hi) {
			return (r - rune(rg.Lo)) % 10
		}
	}
	return 0
}

// esDigitoASCII is the digit of \d in RE2; the text is folded before the
// boundary is checked.
func esDigitoASCII(r rune) bool { return r >= '0' && r <= '9' }

func sinDigitosAlLado(texto string, ini, fin int) bool {
	if r, _ := utf8.DecodeLastRuneInString(texto[:ini]); ini > 0 && esDigitoASCII(r) {
		return false
	}
	if r, _ := utf8.DecodeRuneInString(texto[fin:]); fin < len(texto) && esDigitoASCII(r) {
		return false
	}
	return true
}

// coincidenciasDatosPersonales returns the matches of every pattern, ordered
// by position, with the overlapping ones merged into the earliest and longest
// (a DNI-looking local part inside an e-mail is one e-mail).
func coincidenciasDatosPersonales(texto string) []coincidencia {
	// texto must already be folded with plegarDigitos.
	var todas []coincidencia
	for _, p := range patronesDatosPersonales {
		todas = append(todas, p.buscar(texto)...)
	}
	sort.Slice(todas, func(i, j int) bool {
		if todas[i].ini != todas[j].ini {
			return todas[i].ini < todas[j].ini
		}
		return todas[i].fin > todas[j].fin
	})
	var res []coincidencia
	for _, c := range todas {
		if n := len(res); n > 0 && c.ini < res[n-1].fin {
			continue
		}
		res = append(res, c)
	}
	return res
}

// Enmascarar replaces e-mails, phones and 8-digit numbers with a marker. It
// uses the same detection as the service, which never calls it: consumers
// mask the official-document passages before building their input.
// The result is the text with its digits folded to ASCII.
func Enmascarar(texto string) string {
	texto = plegarDigitos(texto)
	var sb strings.Builder
	ultimo := 0
	for _, c := range coincidenciasDatosPersonales(texto) {
		sb.WriteString(texto[ultimo:c.ini])
		sb.WriteString(marcadorEnmascarado(c.tipo))
		ultimo = c.fin
	}
	sb.WriteString(texto[ultimo:])
	return sb.String()
}

// buscarDatosPersonales checks the instructions and every data block and
// reports the first type found and its block (BloqueInstrucciones for the
// instructions), never the matched text.
func buscarDatosPersonales(s Solicitud) error {
	if tipo, ok := tipoDatoPersonal(s.Instrucciones); ok {
		return &ErrorDatosPersonales{Tipo: tipo, Bloque: BloqueInstrucciones}
	}
	for i, b := range s.Datos {
		if tipo, ok := tipoDatoPersonal(b.Texto); ok {
			return &ErrorDatosPersonales{Tipo: tipo, Bloque: i}
		}
	}
	return nil
}

func tipoDatoPersonal(texto string) (string, bool) {
	if cs := coincidenciasDatosPersonales(plegarDigitos(texto)); len(cs) > 0 {
		return cs[0].tipo, true
	}
	return "", false
}

// runesDeDatos is the input size of a call: the runes of every data text.
func runesDeDatos(s Solicitud) int {
	n := 0
	for _, b := range s.Datos {
		n += utf8.RuneCountInString(b.Texto)
	}
	return n
}
