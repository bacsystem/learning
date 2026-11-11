package fuentes_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/cbaciliod/learning/internal/fuentes"
)

// publicarTextos publishes the version with one passage per text.
func (e *subidaEnv) publicarTextos(t *testing.T, id int64, numero int, textos ...string) {
	t.Helper()
	pasajes := make([]fuentes.PasajeNuevo, len(textos))
	for i, tx := range textos {
		pasajes[i] = fuentes.PasajeNuevo{Orden: i + 1, LineaInicio: i + 1, LineaFin: i + 1, Texto: tx, Palabras: len(strings.Fields(tx))}
	}
	if err := e.publicar.PublicarVersion(context.Background(), id, numero, pasajes, nil); err != nil {
		t.Fatal(err)
	}
}

func (e *subidaEnv) buscarPasajes(t *testing.T, q, bearer string) (int, map[string]any) {
	t.Helper()
	return e.get(t, "/admin/pasajes?q="+url.QueryEscape(q), bearer)
}

func titulos(body map[string]any) []string {
	var res []string
	for _, it := range body["data"].([]any) {
		res = append(res, it.(map[string]any)["fuente_titulo"].(string))
	}
	return res
}

// E3-07 §2 and ADR 0007: only the current version of 'activa' fuentes, by
// relevance, with the title, the extract and no file data.
func TestBuscarPasajes_Admin(t *testing.T) {
	e := nuevoSubidaEnv(t, nil)
	marca := "zq" + dbtest.Uniq()

	activa := e.crearCon(t, "Activa "+marca, "guia")
	largo := marca + " " + strings.Repeat("palabra\n\n  extensa ", 60)
	e.publicarTextos(t, activa, 1, largo)

	conV2 := e.crearCon(t, "ConV2 "+marca, "guia")
	e.publicarTextos(t, conV2, 1, marca+" texto de la versión uno")
	if status, body := e.version(t, conV2, contenidoUnico()); status != http.StatusAccepted {
		t.Fatalf("v2: %d %v", status, body)
	}
	e.publicarTextos(t, conV2, 2, marca+" texto de la versión dos")

	denso := e.crearCon(t, "Denso "+marca, "guia")
	e.publicarTextos(t, denso, 1, marca+" "+marca+" "+marca+" "+marca)

	retirada := e.crearCon(t, "Retirada "+marca, "guia")
	e.publicarTextos(t, retirada, 1, marca+" retirada")
	if status, body := e.estado(t, retirada, `{"estado":"retirada","motivo":"Retirada de prueba"}`); status != http.StatusOK {
		t.Fatalf("retirar: %d %v", status, body)
	}
	e.crearCon(t, "Procesando "+marca, "guia") // never published

	status, body := e.buscarPasajes(t, marca, e.admin)
	if status != http.StatusOK {
		t.Fatalf("got %d %v", status, body)
	}
	got := titulos(body)
	sort.Strings(got)
	want := []string{"Activa " + marca, "ConV2 " + marca, "Denso " + marca}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fuentes %v, want %v (retired, processing and old versions out)", got, want)
	}

	data := body["data"].([]any)
	if first := data[0].(map[string]any)["fuente_titulo"]; first != "Denso "+marca {
		t.Errorf("first hit %v, want the densest passage", first)
	}
	for _, it := range data {
		m := it.(map[string]any)
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		wantKeys := []string{"extracto", "fuente_id", "fuente_titulo", "linea_fin", "linea_inicio", "pasaje_id", "seccion", "version"}
		if !reflect.DeepEqual(keys, wantKeys) {
			t.Errorf("fields %v, want %v", keys, wantKeys)
		}
		switch m["fuente_titulo"] {
		case "ConV2 " + marca:
			if m["version"] != 2.0 || !strings.Contains(m["extracto"].(string), "dos") {
				t.Errorf("ConV2 hit = %v, want version 2", m)
			}
		case "Activa " + marca:
			ex := m["extracto"].(string)
			if r := []rune(ex); len(r) != 301 || r[300] != '…' || strings.ContainsAny(ex, "\n\t") || strings.Contains(ex, "  ") {
				t.Errorf("extracto %q: want 300 runes plus … on one line", ex)
			}
			if m["seccion"] != nil {
				t.Errorf("seccion = %v, want null", m["seccion"])
			}
		}
	}
}

func TestBuscarPasajes_Limite20YSinTerminos(t *testing.T) {
	e := nuevoSubidaEnv(t, nil)
	marca := "zq" + dbtest.Uniq()
	id := e.crearCon(t, "Muchos "+marca, "guia")
	textos := make([]string, 25)
	for i := range textos {
		textos[i] = fmt.Sprintf("%s pasaje %d", marca, i)
	}
	e.publicarTextos(t, id, 1, textos...)

	if status, body := e.buscarPasajes(t, marca, e.admin); status != http.StatusOK || len(body["data"].([]any)) != fuentes.MaxResultados {
		t.Errorf("got %d with %d hits, want %d", status, len(body["data"].([]any)), fuentes.MaxResultados)
	}
	// Only stopwords: no searchable term, an empty list and not an error.
	if status, body := e.buscarPasajes(t, "de la el", e.admin); status != http.StatusOK || len(body["data"].([]any)) != 0 {
		t.Errorf("stopwords: %d %v", status, body)
	}
}

func TestBuscarPasajes_ErroresYPermisos(t *testing.T) {
	e := nuevoSubidaEnv(t, nil)
	cases := []struct {
		name, q, bearer string
		status          int
		code            string
	}{
		{"2 runes", "ñé", e.admin, 422, "consulta_invalida"},
		{"2 after trimming", "  ab  ", e.admin, 422, "consulta_invalida"},
		{"empty", "", e.admin, 422, "consulta_invalida"},
		{"301 runes", strings.Repeat("é", 301), e.admin, 422, "consulta_invalida"},
		{"3 runes", "abc", e.admin, 200, ""},
		{"300 runes", strings.Repeat("é", 300), e.admin, 200, ""},
		{"not admin", "abc", e.usuario, 403, ""},
		{"no session", "abc", "", 401, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, body := e.buscarPasajes(t, c.q, c.bearer)
			if status != c.status {
				t.Fatalf("got %d %v, want %d", status, body, c.status)
			}
			if c.code != "" {
				det, _ := body["detalle"].(map[string]any)
				if body["error"] != c.code || det["min"] != 3.0 || det["max"] != 300.0 {
					t.Errorf("body = %v, want %s with min 3 and max 300", body, c.code)
				}
			}
		})
	}
}
