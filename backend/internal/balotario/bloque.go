package balotario

import (
	"html"
	"strings"
	"unicode/utf8"

	plegado "github.com/cbaciliod/learning/internal/texto"
)

// alternativa is an alternative while its block is being read.
type alternativa struct {
	et     etiqueta
	partes []string
	// correcta: the key is inside a closed <mark>. tentativa: it is inside a
	// <mark> still open; abierta: the last line left a <mark> open, so the
	// next continuation line may close it (§2.3).
	correcta, tentativa, abierta bool
}

// bloque is a question under construction.
type bloque struct {
	numero         int
	inicio, fin    int
	numeracionRota bool
	enunciado      []string
	alts           []*alternativa

	ambigua      bool // a line with the shape of a label did not open an alternative (rule 17)
	marcaAmbigua bool // ...and it carries a closed <mark>: there is a key in the range of the question
	separadas    bool // alternatives glued in one line were split
	html         bool // some HTML (or an unpaired mark) was removed
}

func nuevoBloque(numero, linea int, rota bool) *bloque {
	return &bloque{numero: numero, inicio: linea, fin: linea, numeracionRota: rota}
}

// admite says whether a label opens an alternative: a bullet always does; a
// letter only if it is A with none open, or not lower than the letter of the
// next position (a gap is relabelled). Anything else, like "E. coli" inside
// the enunciado, is continuation (contract §2.2).
func (b *bloque) admite(et etiqueta) bool {
	if et.vineta {
		return true
	}
	if len(b.alts) == 0 {
		return et.letra == 'A'
	}
	return et.letra >= rune('A'+len(b.alts))
}

// enunciadoLinea adds text to the enunciado; if the alternatives start in that
// same line, the enunciado ends there.
func (b *bloque) enunciadoLinea(linea int, l lineaLimpia) {
	t := l.t
	b.fin = linea
	b.html = b.html || l.html
	corte := len(t.r)
	if len(b.alts) == 0 {
		if c := corteEnunciado(t); c >= 0 {
			corte = c
		}
	}
	if cabeza := t.tramo(0, corte).recortar(); len(cabeza.r) > 0 {
		b.enunciado = append(b.enunciado, cabeza.String())
		// A mark in the enunciado indicates no key: it is removed like any HTML.
		b.html = b.html || cabeza.marcado()
	}
	if corte < len(t.r) {
		b.separadas = true
		b.etiquetadas(t.tramo(corte, len(t.r)), l.abierta)
	}
}

// etiquetadas adds the alternatives of a text that starts with a label,
// splitting the ones glued in the same line. abierta: the line ends with a
// <mark> still open, which belongs to the last alternative.
func (b *bloque) etiquetadas(t texto, abierta bool) {
	for len(t.r) > 0 {
		et, resto, ok := etiquetaAlternativa(t)
		if !ok {
			return
		}
		corte := et.siguiente(resto)
		propio := resto
		if corte >= 0 {
			propio = resto.tramo(0, corte)
			b.separadas = true
		}
		b.cerrarPendiente()
		alt := &alternativa{et: et,
			correcta:  et.confirmada || propio.confirmada(),
			tentativa: et.tentativa || propio.tentativa(),
			abierta:   corte < 0 && abierta}
		if p := propio.recortar(); len(p.r) > 0 {
			alt.partes = append(alt.partes, p.String())
		}
		b.alts = append(b.alts, alt)
		if corte < 0 {
			return
		}
		t = resto.tramo(corte, len(resto.r))
	}
}

// etiquetadaLinea adds the alternatives of a line that starts with a label.
func (b *bloque) etiquetadaLinea(linea int, l lineaLimpia) {
	b.fin = linea
	b.html = b.html || l.html
	b.etiquetadas(l.t, l.abierta)
}

// pendiente: the last alternative has a <mark> open that the next
// continuation line may close.
func (b *bloque) pendiente() bool {
	return len(b.alts) > 0 && b.alts[len(b.alts)-1].abierta
}

// cerrarPendiente discards the <mark> that stayed open: it does not form a mark
// and is removed like any HTML.
func (b *bloque) cerrarPendiente() {
	if b.pendiente() {
		ultima := b.alts[len(b.alts)-1]
		ultima.abierta, ultima.tentativa = false, false
		b.html = true
	}
}

// pareceEtiqueta says whether a line that did not open an alternative still
// has the shape of a label: any label with a letter, X), X., X- or (X)
// (contract §2.2, rule 17). Bullets never are. After the first alternative
// such a line is not absorbed as continuation: the question is not imported
// when the key may be elsewhere.
func (b *bloque) pareceEtiqueta(et etiqueta) bool {
	return len(b.alts) > 0 && !et.vineta
}

// ambiguaLinea records the line of a label that does not open an alternative.
func (b *bloque) ambiguaLinea(linea int, l lineaLimpia) {
	b.ambigua = true
	b.marcaAmbigua = b.marcaAmbigua || l.t.confirmada()
	b.fin = linea
	b.html = b.html || l.html
}

// continuacion adds a line without label to the last element: the last
// alternative, or the enunciado while there is none. l was cleaned knowing
// whether pendiente().
func (b *bloque) continuacion(linea int, l lineaLimpia) {
	if len(b.alts) == 0 {
		b.enunciadoLinea(linea, l)
		return
	}
	b.fin = linea
	b.html = b.html || l.html
	ultima := b.alts[len(b.alts)-1]
	if ultima.abierta && l.cerroInicial {
		ultima.correcta = ultima.correcta || ultima.tentativa
		ultima.tentativa = false
	}
	ultima.correcta = ultima.correcta || l.t.confirmada()
	ultima.tentativa = ultima.tentativa || l.t.tentativa()
	ultima.abierta = l.abierta
	ultima.partes = append(ultima.partes, l.t.String())
}

// limpio decodes the entities after the marks are known (§2.3) and collapses
// the spaces once more.
func limpio(partes []string) string {
	return strings.Join(strings.Fields(html.UnescapeString(strings.Join(partes, " "))), " ")
}

// Reasons of "con problemas" in the order of §2.4.
const (
	MotivoNumeracionRota            = "numeracion_rota"
	MotivoEnunciadoVacio            = "enunciado_vacio"
	MotivoEnunciadoLargo            = "enunciado_largo"
	MotivoAlternativasInsuficientes = "alternativas_insuficientes"
	MotivoAlternativasExcedidas     = "alternativas_excedidas"
	MotivoEtiquetasAmbiguas         = "etiquetas_ambiguas"
	MotivoAlternativaVacia          = "alternativa_vacia"
	MotivoAlternativaLarga          = "alternativa_larga"
	MotivoAlternativasDuplicadas    = "alternativas_duplicadas"
	MotivoSinClave                  = "sin_clave"
)

// Structure alerts (contract §2.3).
const (
	AlertaAlternativasSeparadas = "alternativas_separadas"
	AlertaVinetasNormalizadas   = "vinetas_normalizadas"
	AlertaHTMLEliminado         = "html_eliminado"
)

// Limits of §2.4 and of rule 5.
const (
	MaxPreguntas    = 1000
	MaxEnunciado    = 2000 // runes
	MaxAlternativa  = 500  // runes; respuestas.texto is VARCHAR(500)
	MinAlternativas = 2
	MaxAlternativas = 6
	TipoUnica       = "unica"
	TipoMultiple    = "multiple"
)

// resolver closes the block: it returns the valid question or the problem.
func (b *bloque) resolver() (Pregunta, Problema, bool) {
	b.cerrarPendiente()
	enunciado := limpio(b.enunciado)
	textos := make([]string, len(b.alts))
	correctas := 0
	for i, a := range b.alts {
		textos[i] = limpio(a.partes)
		if a.correcta {
			correctas++
		}
	}
	motivos := b.motivos(enunciado, textos, correctas)
	if len(motivos) > 0 {
		n := b.numero
		return Pregunta{}, Problema{Numero: &n, LineaInicio: b.inicio, LineaFin: b.fin, Motivos: motivos}, false
	}
	p := Pregunta{Numero: b.numero, LineaInicio: b.inicio, LineaFin: b.fin, Enunciado: enunciado,
		Tipo: TipoUnica, Alertas: b.alertas()}
	if correctas > 1 {
		p.Tipo = TipoMultiple
	}
	for i, a := range b.alts {
		p.Alternativas = append(p.Alternativas, Alternativa{Letra: string(rune('A' + i)), Texto: textos[i], Correcta: a.correcta})
	}
	return p, Problema{}, true
}

func (b *bloque) motivos(enunciado string, textos []string, correctas int) []string {
	motivos := []string{}
	anadir := func(cond bool, motivo string) {
		if cond {
			motivos = append(motivos, motivo)
		}
	}
	anadir(b.numeracionRota, MotivoNumeracionRota)
	anadir(enunciado == "", MotivoEnunciadoVacio)
	anadir(utf8.RuneCountInString(enunciado) > MaxEnunciado, MotivoEnunciadoLargo)
	anadir(len(textos) < MinAlternativas, MotivoAlternativasInsuficientes)
	anadir(len(textos) > MaxAlternativas, MotivoAlternativasExcedidas)
	anadir(b.ambigua, MotivoEtiquetasAmbiguas)
	anadir(alguna(textos, func(s string) bool { return s == "" }), MotivoAlternativaVacia)
	anadir(alguna(textos, func(s string) bool { return utf8.RuneCountInString(s) > MaxAlternativa }), MotivoAlternativaLarga)
	anadir(hayDuplicadas(textos), MotivoAlternativasDuplicadas)
	// sin_clave: no valid mark in the whole range of the question, also in the
	// lines that did not open an alternative (H5 of the review of #193).
	anadir(correctas == 0 && !b.marcaAmbigua, MotivoSinClave)
	return motivos
}

func alguna(textos []string, cond func(string) bool) bool {
	for _, s := range textos {
		if cond(s) {
			return true
		}
	}
	return false
}

func hayDuplicadas(textos []string) bool {
	vistos := make(map[string]bool, len(textos))
	for _, s := range textos {
		clave := plegado.Plegar(s)
		if vistos[clave] {
			return true
		}
		vistos[clave] = true
	}
	return false
}

// alertas lists, in a fixed order, where the parser had to intervene.
func (b *bloque) alertas() []string {
	alertas := []string{}
	if b.separadas {
		alertas = append(alertas, AlertaAlternativasSeparadas)
	}
	if b.vinetasNormalizadas() {
		alertas = append(alertas, AlertaVinetasNormalizadas)
	}
	if b.html {
		alertas = append(alertas, AlertaHTMLEliminado)
	}
	return alertas
}

// vinetasNormalizadas: a bullet without letter, mixed label styles or letters
// out of order (A, B, D are saved as A, B, C). A uniform lettered style
// ("a.", "A)") is normalised silently (P-B5).
func (b *bloque) vinetasNormalizadas() bool {
	for i, a := range b.alts {
		if a.et.vineta || a.et.estilo != b.alts[0].et.estilo || a.et.letra != rune('A'+i) {
			return true
		}
	}
	return false
}
