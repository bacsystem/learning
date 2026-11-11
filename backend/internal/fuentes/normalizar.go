package fuentes

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// NormalizadorVersion identifies the algorithm below; it is stored in
// fuente_versiones.normalizador_version so a future algorithm never rewrites
// passages that are already cited (ADR 0003 § Normalización, paso 9).
const NormalizadorVersion = 1

// Errors of Normalizar. The worker records them with MarcarVersionFallida as
// MotivoSinContenido and MotivoLimitePasajes.
var (
	ErrSinContenido  = errors.New(MotivoSinContenido)
	ErrLimitePasajes = errors.New(MotivoLimitePasajes)
)

// Passage size in words (SIA-E2-05 regla 2).
const (
	MinPalabrasPasaje = 150
	MaxPalabrasPasaje = 800
)

const (
	bom         = "\uFEFF"
	marcaFence  = "```"
	vinetaNueva = "- "
)

var (
	// encabezado: "#" to "######" followed by a space, at the start of the line.
	encabezado = regexp.MustCompile(`^(#{1,6}) (.*)$`)
	// vineta: optional indentation, a bullet and at least one space.
	vineta = regexp.MustCompile(`^( ?)[*+•▪◦] +`)
	// marcas are kept literally in the text but are not words.
	marcas = strings.NewReplacer("<mark>", "", "</mark>", "")
)

// separador is what precedes a token inside the normalised document.
type separador uint8

const (
	sepEspacio separador = iota // same line
	sepLinea                    // next line of the same paragraph
	sepParrafo                  // new paragraph (blank line or heading)
)

// token is one strings.Fields token of a normalised line.
type token struct {
	texto   string
	linea   int // 1-based line of the original content
	bloque  int // index into documento.secciones
	sep     separador
	palabra bool
}

// documento is the normalised content as a flat token list, plus the section
// of each heading block (nil before the first heading).
type documento struct {
	tokens    []token
	secciones []*string
	// palabrasAntes[i] is the number of words in tokens[:i].
	palabrasAntes []int
}

// tramo is the half-open token range [ini, fin) of a passage or unit.
type tramo struct{ ini, fin int }

// Normalizar normalises the content and splits it into citable passages
// (SIA-E2-05, ADR 0003 § Normalización y segmentación). It is pure and
// deterministic: the same content always gives the same passages in the same
// order. It returns ErrSinContenido when the content has no word and
// ErrLimitePasajes when it would give more than MaxPasajes passages.
func Normalizar(contenido []byte) ([]PasajeNuevo, error) {
	doc := analizar(contenido)
	total := doc.palabras(tramo{0, len(doc.tokens)})
	if total == 0 {
		return nil, ErrSinContenido
	}
	tramos := []tramo{{0, len(doc.tokens)}}
	if total >= MinPalabrasPasaje {
		tramos = doc.segmentar()
	}
	if len(tramos) > MaxPasajes {
		return nil, ErrLimitePasajes
	}
	pasajes := make([]PasajeNuevo, len(tramos))
	for i, t := range tramos {
		pasajes[i] = doc.pasaje(i+1, t)
	}
	return pasajes, nil
}

// ── Limpieza y estructura ─────────────────────────────────────────────────────

// analizar cleans every line (pasos 1–4) and tokenises the document,
// recording paragraphs and heading blocks (paso 5).
func analizar(contenido []byte) documento {
	contenido = bytes.TrimPrefix(contenido, []byte(bom))
	lineas := DividirLineas([]byte(strings.ToValidUTF8(string(contenido), string(utf8.RuneError))))

	doc := documento{secciones: []*string{nil}}
	enFence, pendiente := false, sepParrafo
	for i, cruda := range lineas {
		linea := colapsarEspacios(cruda)
		// Fences and headings start at column 1: an indented "```" or "#" is text.
		esFence := strings.HasPrefix(linea, marcaFence)
		titulo := encabezado.FindStringSubmatch(linea)
		if enFence || esFence {
			titulo = nil
		} else {
			linea = vineta.ReplaceAllString(linea, "${1}"+vinetaNueva)
		}
		if esFence {
			enFence = !enFence
		}
		linea = strings.TrimSpace(linea)
		if linea == "" {
			pendiente = sepParrafo
			continue
		}
		if titulo != nil {
			doc.secciones = append(doc.secciones, seccionDeEncabezado(titulo[2]))
			pendiente = sepParrafo
		}
		doc.agregarLinea(linea, i+1, pendiente)
		pendiente = sepLinea
	}
	doc.palabrasAntes = make([]int, len(doc.tokens)+1)
	for i, t := range doc.tokens {
		doc.palabrasAntes[i+1] = doc.palabrasAntes[i]
		if t.palabra {
			doc.palabrasAntes[i+1]++
		}
	}
	return doc
}

// colapsarEspacios applies paso 1 to one line: control characters are
// dropped, and every run of white space (tabs, NBSP, and the white-space
// controls \f, \v and U+0085, so no two words get glued) becomes one space.
// Leading and trailing space is kept for the column-1 checks of the caller.
func colapsarEspacios(linea string) string {
	var b strings.Builder
	espacio := false
	for _, r := range linea {
		switch {
		case unicode.IsSpace(r):
			espacio = true
		case unicode.IsControl(r):
			// dropped (paso 1)
		default:
			if espacio {
				b.WriteByte(' ')
				espacio = false
			}
			b.WriteRune(r)
		}
	}
	if espacio {
		b.WriteByte(' ')
	}
	return b.String()
}

// seccionDeEncabezado is the heading text, trimmed and cut to the column
// limit; a heading without text has no section.
func seccionDeEncabezado(texto string) *string {
	s := strings.TrimSpace(texto)
	if s == "" {
		return nil
	}
	if r := []rune(s); len(r) > maxSeccion {
		s = string(r[:maxSeccion])
	}
	return &s
}

func (d *documento) agregarLinea(linea string, numero int, sep separador) {
	if len(d.tokens) == 0 {
		sep = sepParrafo
	}
	bloque := len(d.secciones) - 1
	for _, campo := range strings.Fields(linea) {
		d.tokens = append(d.tokens, token{texto: campo, linea: numero, bloque: bloque, sep: sep, palabra: esPalabra(campo)})
		sep = sepEspacio
	}
}

// esPalabra: a token is a word if, without <mark> tags, it has a letter or a
// digit, so bullets, "#", "|" or "—" alone do not count.
func esPalabra(campo string) bool {
	return strings.IndexFunc(marcas.Replace(campo), func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r)
	}) >= 0
}

// ── Segmentación (paso 6) ─────────────────────────────────────────────────────

func (d *documento) palabras(t tramo) int { return d.palabrasAntes[t.fin] - d.palabrasAntes[t.ini] }

// segmentar cuts each heading block greedily into chunks of ≤ 800 words and
// then joins every chunk under 150 words to its neighbour.
func (d *documento) segmentar() []tramo {
	var trozos []tramo
	for _, bloque := range d.bloques() {
		trozos = append(trozos, d.acumular(d.unidades(bloque))...)
	}
	return d.unirCortos(trozos)
}

// bloques cuts the token list at each heading.
func (d *documento) bloques() []tramo {
	var out []tramo
	ini := 0
	for i := 1; i < len(d.tokens); i++ {
		if d.tokens[i].bloque != d.tokens[i-1].bloque {
			out = append(out, tramo{ini, i})
			ini = i
		}
	}
	return append(out, tramo{ini, len(d.tokens)})
}

// unidades are the cut units of a block: paragraphs; a paragraph over 800
// words, by sentences; a sentence over 800 words, by balanced word parts.
func (d *documento) unidades(bloque tramo) []tramo {
	var out []tramo
	for _, parrafo := range d.cortar(bloque, d.inicioParrafo) {
		if d.palabras(parrafo) <= MaxPalabrasPasaje {
			out = append(out, parrafo)
			continue
		}
		for _, oracion := range d.cortar(parrafo, d.inicioOracion) {
			if n := d.palabras(oracion); n > MaxPalabrasPasaje {
				out = append(out, d.cortarPorPalabras(oracion, partesEquilibradas(n))...)
			} else {
				out = append(out, oracion)
			}
		}
	}
	return out
}

// cortar splits t before every token i where esInicio(i) holds.
func (d *documento) cortar(t tramo, esInicio func(i int) bool) []tramo {
	var out []tramo
	ini := t.ini
	for i := t.ini + 1; i < t.fin; i++ {
		if esInicio(i) {
			out = append(out, tramo{ini, i})
			ini = i
		}
	}
	return append(out, tramo{ini, t.fin})
}

func (d *documento) inicioParrafo(i int) bool { return d.tokens[i].sep == sepParrafo }

// inicioOracion: the previous token ends with ".", "?" or "!" (it is always
// followed by a space, a line break or the end).
func (d *documento) inicioOracion(i int) bool {
	previo := d.tokens[i-1].texto
	return strings.LastIndexAny(previo, ".?!") == len(previo)-1
}

// partesEquilibradas: k = ⌈n/800⌉ parts of ⌊n/k⌋ or ⌈n/k⌉ words, the larger
// first; with n > 800 each part has at least 400 words.
func partesEquilibradas(n int) []int {
	k := (n + MaxPalabrasPasaje - 1) / MaxPalabrasPasaje
	partes := make([]int, k)
	for i := range partes {
		partes[i] = n / k
		if i < n%k {
			partes[i]++
		}
	}
	return partes
}

// cortarPorPalabras splits t into consecutive ranges with the given word
// counts. A cut falls just before the first word of the next part, so the
// non-word tokens after a part's last word stay with it.
func (d *documento) cortarPorPalabras(t tramo, partes []int) []tramo {
	out := make([]tramo, 0, len(partes))
	ini, objetivo := t.ini, d.palabrasAntes[t.ini]
	for _, n := range partes[:len(partes)-1] {
		objetivo += n
		i := ini
		for i < t.fin && (d.palabrasAntes[i] < objetivo || !d.tokens[i].palabra) {
			i++
		}
		out = append(out, tramo{ini, i})
		ini = i
	}
	return append(out, tramo{ini, t.fin})
}

// acumular joins consecutive units while the chunk stays ≤ 800 words.
func (d *documento) acumular(unidades []tramo) []tramo {
	out := []tramo{unidades[0]}
	for _, u := range unidades[1:] {
		ultimo := &out[len(out)-1]
		if d.palabras(tramo{ultimo.ini, u.fin}) <= MaxPalabrasPasaje {
			ultimo.fin = u.fin
		} else {
			out = append(out, u)
		}
	}
	return out
}

// unirCortos joins every chunk under 150 words to the previous one, or to the
// next one when it is the first; a join over 800 words is split in two.
func (d *documento) unirCortos(trozos []tramo) []tramo {
	var out []tramo
	for _, t := range trozos {
		switch {
		case len(out) == 0:
			out = append(out, t)
		case d.palabras(t) < MinPalabrasPasaje,
			len(out) == 1 && d.palabras(out[0]) < MinPalabrasPasaje:
			unido := tramo{out[len(out)-1].ini, t.fin}
			out = append(out[:len(out)-1], d.dividirSiExcede(unido)...)
		default:
			out = append(out, t)
		}
	}
	return out
}

// dividirSiExcede splits a join of 801–949 words in two: at the paragraph
// boundary nearest to the middle that leaves ≥ 150 words on each side, else
// at such a sentence boundary, else by words at the middle.
func (d *documento) dividirSiExcede(t tramo) []tramo {
	n := d.palabras(t)
	if n <= MaxPalabrasPasaje {
		return []tramo{t}
	}
	for _, esInicio := range []func(int) bool{d.inicioParrafo, d.inicioOracion} {
		if corte, ok := d.corteCentral(t, esInicio); ok {
			return []tramo{{t.ini, corte}, {corte, t.fin}}
		}
	}
	return d.cortarPorPalabras(t, []int{n - n/2, n / 2})
}

// corteCentral finds the boundary nearest to the middle of t (the first one
// on a tie) that leaves at least 150 words on each side.
func (d *documento) corteCentral(t tramo, esInicio func(int) bool) (int, bool) {
	n := d.palabras(t)
	mejor, mejorDist := 0, -1
	for i := t.ini + 1; i < t.fin; i++ {
		izq := d.palabras(tramo{t.ini, i})
		if !esInicio(i) || izq < MinPalabrasPasaje || n-izq < MinPalabrasPasaje {
			continue
		}
		if dist := abs(2*izq - n); mejorDist < 0 || dist < mejorDist {
			mejor, mejorDist = i, dist
		}
	}
	return mejor, mejorDist >= 0
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ── Pasajes ───────────────────────────────────────────────────────────────────

// pasaje renders t: tokens joined by a space, lines by "\n" and paragraphs by
// "\n\n"; the section is the heading of the passage's first block.
func (d *documento) pasaje(orden int, t tramo) PasajeNuevo {
	var b strings.Builder
	for i := t.ini; i < t.fin; i++ {
		tk := d.tokens[i]
		if i > t.ini {
			switch tk.sep {
			case sepEspacio:
				b.WriteByte(' ')
			case sepLinea:
				b.WriteByte('\n')
			case sepParrafo:
				b.WriteString("\n\n")
			}
		}
		b.WriteString(tk.texto)
	}
	primero, ultimo := d.tokens[t.ini], d.tokens[t.fin-1]
	return PasajeNuevo{
		Orden:       orden,
		LineaInicio: primero.linea,
		LineaFin:    ultimo.linea,
		Seccion:     d.secciones[primero.bloque],
		Texto:       b.String(),
		Palabras:    d.palabras(t),
	}
}
