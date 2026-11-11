package fuentes

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizarBusqueda(t *testing.T) {
	cases := []struct {
		name     string
		b        Busqueda
		consulta string
		limite   int
		err      error
	}{
		{"recorta espacios", Busqueda{Consulta: "  vigilancia  ", EspecialidadID: 2}, "vigilancia", 20, nil},
		{"límite 5", Busqueda{Consulta: "abc", EspecialidadID: 2, Limite: 5}, "abc", 5, nil},
		{"límite 25 se recorta", Busqueda{Consulta: "abc", EspecialidadID: 2, Limite: 25}, "abc", 20, nil},
		{"límite negativo = 20", Busqueda{Consulta: "abc", EspecialidadID: 2, Limite: -3}, "abc", 20, nil},
		{"2 runes", Busqueda{Consulta: "añ", EspecialidadID: 2}, "", 0, ErrConsultaInvalida},
		{"301 runes", Busqueda{Consulta: strings.Repeat("é", 301), EspecialidadID: 2}, "", 0, ErrConsultaInvalida},
		{"sin especialidad", Busqueda{Consulta: "abc"}, "", 0, ErrConsultaInvalida},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			consulta, limite, err := NormalizarBusqueda(c.b)
			if consulta != c.consulta || limite != c.limite || !errors.Is(err, c.err) {
				t.Errorf("got %q %d %v", consulta, limite, err)
			}
		})
	}
}

func TestValidarPasajes(t *testing.T) {
	sec := strings.Repeat("s", 301)
	ok := PasajeNuevo{Orden: 1, LineaInicio: 1, LineaFin: 3, Texto: "texto", Palabras: 1}
	muchos := make([]PasajeNuevo, MaxPasajes+1)
	for i := range muchos {
		muchos[i] = PasajeNuevo{Orden: i + 1, LineaInicio: 1, LineaFin: 1, Texto: "t", Palabras: 1}
	}
	cases := []struct {
		name string
		ps   []PasajeNuevo
		ok   bool
	}{
		{"válido", []PasajeNuevo{ok}, true},
		{"5000 pasajes", muchos[:MaxPasajes], true},
		{"5001 pasajes", muchos, false},
		{"vacío", nil, false},
		{"orden 0", []PasajeNuevo{{Orden: 0, LineaInicio: 1, LineaFin: 1, Texto: "t", Palabras: 1}}, false},
		{"orden repetido", []PasajeNuevo{ok, ok}, false},
		{"línea 0", []PasajeNuevo{{Orden: 1, LineaInicio: 0, LineaFin: 1, Texto: "t", Palabras: 1}}, false},
		{"fin antes que inicio", []PasajeNuevo{{Orden: 1, LineaInicio: 3, LineaFin: 2, Texto: "t", Palabras: 1}}, false},
		{"sin palabras", []PasajeNuevo{{Orden: 1, LineaInicio: 1, LineaFin: 1, Texto: "t", Palabras: 0}}, false},
		{"texto en blanco", []PasajeNuevo{{Orden: 1, LineaInicio: 1, LineaFin: 1, Texto: " \n", Palabras: 1}}, false},
		{"sección de 301", []PasajeNuevo{{Orden: 1, LineaInicio: 1, LineaFin: 1, Seccion: &sec, Texto: "t", Palabras: 1}}, false},
	}
	for _, c := range cases {
		if err := ValidarPasajes(c.ps); (err == nil) != c.ok {
			t.Errorf("%s: err = %v, want ok=%v", c.name, err, c.ok)
		}
	}
}

func TestHuella(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0}, {"a", 1}, {"a\n", 1}, {"a\nb", 2}, {"a\nb\n", 2}, {"\n", 1}, {"\n\n", 2},
		{"a\r\nb", 2}, {"a\r\nb\r\n", 2}, {"a\rb", 2}, {"a\r", 1}, {"\r\n", 1}, {"\r\r", 2},
		{"a\r\n\r\nb", 3}, {"a\n\rb", 3}, {"a\r\rb\n", 3},
	}
	for _, c := range cases {
		sha, tamano, lineas := Huella([]byte(c.in))
		if lineas != c.want || tamano != len(c.in) || len(sha) != 64 {
			t.Errorf("Huella(%q) = %s %d %d, want %d lines", c.in, sha, tamano, lineas, c.want)
		}
	}
	if sha, _, _ := Huella([]byte("abc")); sha != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Errorf("sha256(abc) = %s", sha)
	}
}
