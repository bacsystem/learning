package texto

import "testing"

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
		{"á", "á"}, {"é", "é"}, {"í", "í"}, {"ó", "ó"}, {"ú", "ú"},
		{"à", "à"}, {"è", "è"}, {"ì", "ì"}, {"ò", "ò"}, {"ù", "ù"},
		{"ü", "ü"},
		{"Á", "Á"}, {"É", "É"}, {"Í", "Í"}, {"Ó", "Ó"}, {"Ú", "Ú"},
		{"À", "À"}, {"È", "È"}, {"Ì", "Ì"}, {"Ò", "Ò"}, {"Ù", "Ù"},
		{"Ü", "Ü"},
		{"ñ", "ñ"}, {"Ñ", "Ñ"},
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
	nfd := "año"
	if got := Plegar(nfd); got != "año" || got != Plegar("año") {
		t.Errorf("Plegar(año NFD) = %q, want año", got)
	}
	if Plegar(nfd) == Plegar("ano") {
		t.Error("año in NFD must differ from ano")
	}
	if got := Plegar("EPIDEMIOLOGÍA"); got != "epidemiologia" {
		t.Errorf("Plegar(NFD uppercase) = %q", got)
	}
}
