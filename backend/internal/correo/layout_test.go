package correo

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cbaciliod/learning/internal/locale"
)

func TestWrap_MarcaPieEIdioma(t *testing.T) {
	for _, loc := range []string{locale.ES, locale.EN} {
		got := Wrap(loc, "Asunto", "<p>contenido</p>")
		for _, want := range []string{
			fmt.Sprintf(`<html lang="%s">`, loc), ">" + Brand + "</p>", "<p>contenido</p>", AutoNotice(loc),
			fmt.Sprintf("© %d %s", time.Now().Year(), Brand),
		} {
			if !strings.Contains(got, want) {
				t.Errorf("%s: layout lacks %q", loc, want)
			}
		}
	}
	if AutoNotice(locale.ES) == AutoNotice(locale.EN) {
		t.Error("the footer must differ between es and en")
	}
}

func TestBtnPrimary_EscapaLaURL(t *testing.T) {
	got := BtnPrimary(locale.EN, "Go", `https://x/?a=1&b="2"`)
	if strings.Contains(got, `"2"`) || !strings.Contains(got, "&amp;b=&#34;2&#34;") {
		t.Fatalf("url not escaped: %s", got)
	}
	if !strings.Contains(got, ButtonFallback(locale.EN)) {
		t.Fatal("button fallback line missing")
	}
}
