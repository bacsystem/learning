package balotario

import (
	"sort"

	"github.com/cbaciliod/learning/internal/fuentes"
)

// importable is a valid question with the passages it cites.
type importable struct {
	Pregunta
	PasajeIDs []int64
}

// analisis is what the preview shows and the import saves: the valid questions
// with their citations, and the problems (the parser's plus sin_pasaje).
type analisis struct {
	Validas   []importable
	Problemas []Problema
}

// analizar parses the content and cites each valid question to the passages
// whose lines overlap its range (D8). A valid question with no passage cannot
// be cited, so it becomes a problem with the motivo sin_pasaje (rule 9). The
// problems stay in order of appearance.
func analizar(contenido string, rangos []fuentes.RangoPasaje) (analisis, error) {
	res, err := Parsear([]byte(contenido))
	if err != nil {
		return analisis{}, err
	}
	out := analisis{Problemas: res.Problemas}
	desde := 0
	for _, p := range res.Validas {
		ids := pasajesQueSolapan(rangos, &desde, p.LineaInicio, p.LineaFin)
		if len(ids) == 0 {
			n := p.Numero
			out.Problemas = append(out.Problemas, Problema{Numero: &n, LineaInicio: p.LineaInicio, LineaFin: p.LineaFin,
				Motivos: []string{MotivoSinPasaje}})
			continue
		}
		out.Validas = append(out.Validas, importable{Pregunta: p, PasajeIDs: ids})
	}
	sort.SliceStable(out.Problemas, func(i, j int) bool { return out.Problemas[i].LineaInicio < out.Problemas[j].LineaInicio })
	return out, nil
}

// totales counts after sin_pasaje is applied.
func (a analisis) totales() Totales {
	t := Totales{Validas: len(a.Validas), ConProblemas: len(a.Problemas)}
	for _, v := range a.Validas {
		if v.Tipo == TipoMultiple {
			t.Multiple++
		} else {
			t.Unica++
		}
		if len(v.Alertas) > 0 {
			t.ConAlertas++
		}
	}
	return t
}
