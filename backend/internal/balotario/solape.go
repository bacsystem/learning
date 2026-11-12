package balotario

import "github.com/cbaciliod/learning/internal/fuentes"

// pasajesQueSolapan returns the ids of the passages whose lines overlap
// [inicio, fin] (D8): pa.linea_inicio <= fin AND pa.linea_fin >= inicio.
//
// rangos comes ordered by orden, so by line; desde is the index where the
// previous question left off. The questions also arrive by line, so one pass
// over both lists is enough (linear, not nested).
func pasajesQueSolapan(rangos []fuentes.RangoPasaje, desde *int, inicio, fin int) []int64 {
	for *desde < len(rangos) && rangos[*desde].LineaFin < inicio {
		*desde++
	}
	var ids []int64
	for j := *desde; j < len(rangos) && rangos[j].LineaInicio <= fin; j++ {
		if rangos[j].LineaFin >= inicio {
			ids = append(ids, rangos[j].PasajeID)
		}
	}
	return ids
}
