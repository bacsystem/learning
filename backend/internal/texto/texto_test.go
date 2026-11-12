package texto

import (
	"strings"
	"testing"
)

func TestPlegar(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"vacío", "", ""},
		{"minúsculas", "Vigilancia Epidemiológica", "vigilancia epidemiologica"},
		{"todas las tildes agudas", "ÁÉÍÓÚáéíóú", "aeiouaeiou"},
		{"tildes graves", "ÀÈÌÒÙàèìòù", "aeiouaeiou"},
		{"diéresis", "Pingüino PINGÜINO", "pinguino pinguino"},
		{"la ñ se conserva", "Año AÑO", "año año"},
		{"año y ano siguen distintos", "ano", "ano"},
		{"no recorta ni colapsa espacios", "  a   b ", "  a   b "},
		{"otros símbolos intactos", "Salud-Pública 2026 (I)", "salud-publica 2026 (i)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Plegar(c.in); got != c.want {
				t.Errorf("Plegar(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// ADR 0003 § Plegado: every letter of the set folds the same in NFC and NFD,
// upper and lower case.
func TestPlegar_NFCyNFDIguales(t *testing.T) {
	cases := []struct {
		letra, nfd string
	}{
		{"á", "a\u0301"}, {"é", "e\u0301"}, {"í", "i\u0301"}, {"ó", "o\u0301"}, {"ú", "u\u0301"},
		{"à", "a\u0300"}, {"è", "e\u0300"}, {"ì", "i\u0300"}, {"ò", "o\u0300"}, {"ù", "u\u0300"},
		{"ü", "u\u0308"},
		{"Á", "A\u0301"}, {"É", "E\u0301"}, {"Í", "I\u0301"}, {"Ó", "O\u0301"}, {"Ú", "U\u0301"},
		{"À", "A\u0300"}, {"È", "E\u0300"}, {"Ì", "I\u0300"}, {"Ò", "O\u0300"}, {"Ù", "U\u0300"},
		{"Ü", "U\u0308"},
		{"ñ", "n\u0303"}, {"Ñ", "N\u0303"},
	}
	for _, c := range cases {
		t.Run(c.letra, func(t *testing.T) {
			if nfc, nfd := Plegar(c.letra), Plegar(c.nfd); nfc != nfd {
				t.Errorf("Plegar(NFC %q) = %q, Plegar(NFD %q) = %q", c.letra, nfc, c.nfd, nfd)
			}
		})
	}
}

func TestPlegar_EnieDescompuesta(t *testing.T) {
	nfd := "an\u0303o"
	if got := Plegar(nfd); got != "año" || got != Plegar("año") {
		t.Errorf("Plegar(año NFD) = %q, want año", got)
	}
	if Plegar(nfd) == Plegar("ano") {
		t.Error("año in NFD must differ from ano")
	}
	if got := Plegar("EPIDEMIOLOGI\u0301A"); got != "epidemiologia" {
		t.Errorf("Plegar(NFD uppercase) = %q", got)
	}
}

func TestExtracto(t *testing.T) {
	largo := strings.Repeat("ñ", 300)
	cases := []struct {
		name, in, want string
	}{
		{"vacío", "", ""},
		{"solo espacios", " \n\t ", ""},
		{"colapsa espacios y saltos", "  uno\n\ndos \t tres  ", "uno dos tres"},
		{"300 runes exactos no se cortan", largo, largo},
		{"301 runes se cortan en 300", largo + "ñ", largo + "…"},
		{"multibyte cuenta runes, no bytes", strings.Repeat("😀", 301), strings.Repeat("😀", 300) + "…"},
		{"el corte va después de colapsar", strings.Repeat("a \n ", 200), strings.Repeat("a ", 150) + "…"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Extracto(c.in); got != c.want {
				t.Errorf("Extracto = %q, want %q", got, c.want)
			}
		})
	}
}
