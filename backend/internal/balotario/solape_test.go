package balotario

import (
	"fmt"
	"testing"

	"github.com/cbaciliod/learning/internal/fuentes"
)

func TestPasajesQueSolapan(t *testing.T) {
	// Passages 1–4, 5–8 and 9–12.
	rangos := []fuentes.RangoPasaje{{PasajeID: 10, LineaInicio: 1, LineaFin: 4}, {PasajeID: 20, LineaInicio: 5, LineaFin: 8}, {PasajeID: 30, LineaInicio: 9, LineaFin: 12}}
	tests := []struct {
		name        string
		inicio, fin int
		want        string
	}{
		{"dentro de un pasaje", 2, 3, "[10]"},
		{"igual a un pasaje", 5, 8, "[20]"},
		{"toca por el final del pasaje: su última línea es la primera de la pregunta", 4, 6, "[10 20]"},
		{"toca por el inicio del pasaje: su primera línea es la última de la pregunta", 3, 5, "[10 20]"},
		{"una sola línea en el borde inferior", 5, 5, "[20]"},
		{"una sola línea en el borde superior", 8, 8, "[20]"},
		{"abarca varios", 1, 12, "[10 20 30]"},
		{"empieza justo después de un pasaje", 5, 5, "[20]"},
		{"antes del primero", 0, 0, "[]"},
		{"después del último", 13, 14, "[]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desde := 0
			if got := fmt.Sprint(pasajesQueSolapan(rangos, &desde, tt.inicio, tt.fin)); got != tt.want {
				t.Errorf("[%d, %d] = %s, want %s", tt.inicio, tt.fin, got, tt.want)
			}
		})
	}

	t.Run("hueco entre pasajes: una pregunta en el hueco no cita ninguno", func(t *testing.T) {
		huecos := []fuentes.RangoPasaje{{PasajeID: 1, LineaInicio: 1, LineaFin: 2}, {PasajeID: 2, LineaInicio: 6, LineaFin: 8}}
		desde := 0
		if got := pasajesQueSolapan(huecos, &desde, 3, 5); len(got) != 0 {
			t.Errorf("got %v", got)
		}
	})

	t.Run("preguntas seguidas comparten el recorrido", func(t *testing.T) {
		desde := 0
		var got []string
		for _, q := range [][2]int{{1, 4}, {5, 8}, {9, 12}, {13, 14}} {
			got = append(got, fmt.Sprint(pasajesQueSolapan(rangos, &desde, q[0], q[1])))
		}
		if fmt.Sprint(got) != "[[10] [20] [30] []]" {
			t.Errorf("got %v", got)
		}
	})

	t.Run("pasajes que se pisan: el largo de antes sigue contando", func(t *testing.T) {
		pisados := []fuentes.RangoPasaje{{PasajeID: 1, LineaInicio: 1, LineaFin: 10}, {PasajeID: 2, LineaInicio: 3, LineaFin: 4}}
		desde := 0
		if got := fmt.Sprint(pasajesQueSolapan(pisados, &desde, 5, 6)); got != "[1]" {
			t.Errorf("got %s", got)
		}
	})
}
