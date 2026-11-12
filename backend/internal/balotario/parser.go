// Package balotario parses a balotario (.md) into questions with their
// alternatives, the key marked with <mark> and the lines of origin
// (SIA-E3-01, contract docs/api/serums-ia-e3-01-e3-03-balotario.md §2).
//
// Parsear is pure and deterministic: the same content gives the same result.
package balotario

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/cbaciliod/learning/internal/fuentes"
)

// ErrLimitePreguntas is the only error of Parsear: the file has more than
// MaxPreguntas question blocks (valid and with problems). No partial result.
var ErrLimitePreguntas = errors.New("el balotario supera el máximo de preguntas")

// LimiteError is ErrLimitePreguntas with the number of blocks detected, for the
// detail of the API error (contract §3, step 5).
type LimiteError struct{ Detectadas int }

func (e *LimiteError) Error() string        { return ErrLimitePreguntas.Error() }
func (e *LimiteError) Is(target error) bool { return target == ErrLimitePreguntas }

const bom = "\xef\xbb\xbf"

// Resultado is what Parsear found, in order of appearance.
type Resultado struct {
	Validas   []Pregunta
	Problemas []Problema
}

// Pregunta is a valid question.
type Pregunta struct {
	Numero                int
	LineaInicio, LineaFin int // 1-based, numbered like pasajes.linea_* (fuentes.DividirLineas)
	Enunciado             string
	Tipo                  string // TipoUnica | TipoMultiple
	Alternativas          []Alternativa
	Alertas               []string // never nil
}

// Alternativa is labelled A, B, C… by position.
type Alternativa struct {
	Letra, Texto string
	Correcta     bool
}

// Problema is a block the parser could not accept; it is never dropped silently.
type Problema struct {
	Numero                *int
	LineaInicio, LineaFin int
	Motivos               []string // ≥ 1, in the order of §2.4
}

// Totales is the summary of a Resultado.
type Totales struct {
	Validas, ConProblemas, Unica, Multiple, ConAlertas int
}

// Totales counts the valid ones, the ones with problems, their type and the
// valid ones with structure alerts.
func (r Resultado) Totales() Totales {
	t := Totales{Validas: len(r.Validas), ConProblemas: len(r.Problemas)}
	for _, p := range r.Validas {
		if p.Tipo == TipoMultiple {
			t.Multiple++
		} else {
			t.Unica++
		}
		if len(p.Alertas) > 0 {
			t.ConAlertas++
		}
	}
	return t
}

// Parsear reads the original content of a fuente version.
func Parsear(contenido []byte) (Resultado, error) {
	contenido = bytes.TrimPrefix(contenido, []byte(bom))
	limpio := aplanarScripts(strings.ToValidUTF8(string(contenido), string(utf8.RuneError)))
	lineas := fuentes.DividirLineas([]byte(limpio))

	var p lector
	for i, cruda := range lineas {
		p.linea(i+1, cruda)
	}
	p.cerrar()
	if p.detectados > MaxPreguntas {
		return Resultado{}, &LimiteError{Detectadas: p.detectados}
	}
	return p.res, nil
}

// placeholderScript stands for a removed block in each of its lines: a line with
// only removed HTML is not a blank line for the grammar.
const placeholderScript = "<script></script>"

var reSaltoDeLinea = regexp.MustCompile(`\r\n|\r|\n`)

// aplanarScripts removes the content of <script> and <style> blocks, also the
// ones that span several lines (rule 16), before the content is split into
// lines. The line breaks of the block stay, so the numbers of line do not
// move, and every one of its lines keeps an empty <script></script> to raise
// the html_eliminado alert.
func aplanarScripts(contenido string) string {
	return reScriptStyle.ReplaceAllStringFunc(contenido, func(bloque string) string {
		var sb strings.Builder
		sb.WriteString(placeholderScript)
		for _, salto := range reSaltoDeLinea.FindAllString(bloque, -1) {
			sb.WriteString(salto)
			sb.WriteString(placeholderScript)
		}
		return sb.String()
	})
}

// lector is the state machine of the grammar (§2.2).
type lector struct {
	res Resultado
	cur *bloque // open question; nil outside of one

	detectados     int
	hayAnterior    bool
	numeroAnterior int
	trasEncabezado bool
	enBlanco       bool
}

func (p *lector) linea(n int, cruda string) {
	l := limpiar(cruda, false)
	t := l.t
	if len(t.r) == 0 {
		if l.html { // only removed HTML (a multi-line script): neither blank nor content
			if p.cur != nil {
				p.cur.html = true
			}
			return
		}
		p.enBlanco = p.cur != nil
		return
	}
	enBlanco := p.enBlanco
	p.enBlanco = false

	if t.r[0] == '#' {
		p.cerrar()
		p.trasEncabezado = true
		return
	}
	if numero, resto, ok := inicioPregunta(t); ok && p.abre(numero) {
		p.cerrar()
		p.detectados++
		l.t = resto
		p.abrir(n, numero, l)
		return
	}
	if p.cur == nil {
		return // outside of the questions: headers, footers (rule 7)
	}
	if et, _, ok := etiquetaAlternativa(t); ok {
		if p.cur.admite(et) {
			p.cur.etiquetadaLinea(n, l)
			return
		}
		// A letter G–Z after a blank line is text that follows the question (a
		// reference like "S. Pérez, 2020"): the blank line closes it below.
		if p.cur.pareceEtiqueta(et) && (!enBlanco || et.letra <= 'F') {
			p.cur.ambiguaLinea(n, l)
			return
		}
	}
	if enBlanco && len(p.cur.alts) > 0 {
		p.cerrar() // the text after the last alternative is not part of the question
		return
	}
	if p.cur.pendiente() {
		l = limpiar(cruda, true) // a <mark> left open in the alternative may close here
	}
	p.cur.continuacion(n, l)
}

// abre decides whether a line that looks like "N." starts a question: when
// none is open; when the open one already has an alternative; or when N is the
// next expected number (so a question without alternatives does not swallow
// the next one). Otherwise it is part of the enunciado ("1. … 3. …").
func (p *lector) abre(numero int) bool {
	return p.cur == nil || len(p.cur.alts) > 0 || numero == p.cur.numero+1
}

func (p *lector) abrir(linea, numero int, l lineaLimpia) {
	esperado := numero == p.numeroAnterior+1 || (p.trasEncabezado && numero == 1)
	rota := p.hayAnterior && !esperado
	p.hayAnterior, p.numeroAnterior, p.trasEncabezado = true, numero, false
	p.cur = nuevoBloque(numero, linea, rota)
	p.cur.enunciadoLinea(linea, l)
}

func (p *lector) cerrar() {
	if p.cur == nil {
		return
	}
	if p.detectados > MaxPreguntas {
		// Past the limit the result is discarded (ErrLimitePreguntas): the
		// blocks are only counted, so a huge file does not keep every question in memory.
		p.cur = nil
		return
	}
	if pregunta, problema, valida := p.cur.resolver(); valida {
		p.res.Validas = append(p.res.Validas, pregunta)
	} else {
		p.res.Problemas = append(p.res.Problemas, problema)
	}
	p.cur = nil
}
