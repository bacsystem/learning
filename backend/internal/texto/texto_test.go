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
