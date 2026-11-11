package fuentes_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/cbaciliod/learning/internal/fuentes"
)

func (e *subidaEnv) get(t *testing.T, path, bearer string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if bearer != "" {
		req.Header.Set("Authorization", bearer)
	}
	rr := httptest.NewRecorder()
	e.h.ServeHTTP(rr, req)
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("GET %s: %d %s", path, rr.Code, rr.Body)
	}
	return rr.Code, out
}

// crearCon creates a fuente through S1 with its own title and tipo.
func (e *subidaEnv) crearCon(t *testing.T, titulo, tipo string, esp ...int64) int64 {
	t.Helper()
	campos := map[string][]string{"titulo": {titulo}, "tipo": {tipo}}
	for _, id := range esp {
		campos["especialidad_ids"] = append(campos["especialidad_ids"], fmt.Sprint(id))
	}
	status, body := e.subir(t, fuentes.RutaCrear, e.admin, contenidoUnico(), campos)
	if status != http.StatusAccepted {
		t.Fatalf("crear %q: %d %v", titulo, status, body)
	}
	return int64(body["id"].(float64))
}

func (e *subidaEnv) publicarV(t *testing.T, id int64, numero int) {
	t.Helper()
	if err := e.publicar.PublicarVersion(context.Background(), id, numero,
		[]fuentes.PasajeNuevo{{Orden: 1, LineaInicio: 1, LineaFin: 1, Texto: "Vigilancia", Palabras: 1}}, nil); err != nil {
		t.Fatal(err)
	}
}

func (e *subidaEnv) fallarV(t *testing.T, id int64, numero int) {
	t.Helper()
	if err := e.publicar.MarcarVersionFallida(context.Background(), id, numero, fuentes.MotivoLimitePasajes, nil); err != nil {
		t.Fatal(err)
	}
}

// listadoSembrado is the fixture of S3/S4: one fuente per case, all titles
// prefixed with a marker unique to the test (the test DB is shared).
type listadoSembrado struct {
	marca                                                              string
	oficial, procesando, fallida, conV2, conV2Fallida, ret, recuperada int64
}

func sembrarListado(t *testing.T, e *subidaEnv) listadoSembrado {
	t.Helper()
	m := "Lst" + dbtest.Uniq()
	s := listadoSembrado{marca: m}
	// Activa A, marked oficial.
	s.oficial = e.crearCon(t, m+" alfa", "guia", e.espA)
	e.publicarV(t, s.oficial, 1)
	e.exec(t, `UPDATE fuente_versiones SET oficial_verificada = true, oficial_entidad = 'MINSA', oficial_nota = 'Descargado de la web oficial',
		oficial_at = NOW() WHERE fuente_id = $1 AND numero = 1`, s.oficial)
	// Procesando, for every especialidad.
	s.procesando = e.crearCon(t, m+" beta", "norma")
	// Fallida B: no current version.
	s.fallida = e.crearCon(t, m+" gamma", "balotario", e.espB)
	e.fallarV(t, s.fallida, 1)
	// Activa B with v2 in process.
	s.conV2 = e.crearCon(t, m+" delta", "guia", e.espB)
	e.publicarV(t, s.conV2, 1)
	if status, body := e.version(t, s.conV2, contenidoUnico()); status != http.StatusAccepted {
		t.Fatalf("v2: %d %v", status, body)
	}
	// Activa A with v2 failed and v3 archived: version_en_proceso is v2.
	s.conV2Fallida = e.crearCon(t, m+" 50% dto_x", "otro", e.espA)
	e.publicarV(t, s.conV2Fallida, 1)
	if status, body := e.version(t, s.conV2Fallida, contenidoUnico()); status != http.StatusAccepted {
		t.Fatalf("v2: %d %v", status, body)
	}
	e.fallarV(t, s.conV2Fallida, 2)
	e.exec(t, `INSERT INTO fuente_versiones (fuente_id, numero, estado, nombre_archivo, sha256, tamano_bytes, lineas)
		VALUES ($1, 3, 'archivada', 'v3.md', encode(sha256(convert_to($2, 'UTF8')), 'hex'), 1, 1)`, s.conV2Fallida, m+" v3")
	// Retirada A.
	s.ret = e.crearCon(t, m+" omega", "guia", e.espA)
	e.publicarV(t, s.ret, 1)
	e.exec(t, `UPDATE fuentes SET estado = 'retirada', motivo_retiro = 'Prueba del listado', updated_at = NOW() WHERE id = $1`, s.ret)
	// Recovered B: v1 failed, v2 published; a failure before the current
	// version is not in process.
	s.recuperada = e.crearCon(t, m+" kappa", "guia", e.espB)
	e.fallarV(t, s.recuperada, 1)
	if status, body := e.version(t, s.recuperada, contenidoUnico()); status != http.StatusAccepted {
		t.Fatalf("v2: %d %v", status, body)
	}
	e.publicarV(t, s.recuperada, 2)
	return s
}

func idsDe(t *testing.T, body map[string]any) []int64 {
	t.Helper()
	data, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("data is not a list: %v", body)
	}
	ids := make([]int64, len(data))
	for i, d := range data {
		ids[i] = int64(d.(map[string]any)["id"].(float64))
	}
	return ids
}

func mismos(a, b []int64) bool {
	x, y := append([]int64{}, a...), append([]int64{}, b...)
	sort.Slice(x, func(i, j int) bool { return x[i] < x[j] })
	sort.Slice(y, func(i, j int) bool { return y[i] < y[j] })
	return fmt.Sprint(x) == fmt.Sprint(y)
}

// Escenario «Listado» (E2-07 reglas 1–2): every filter, alone and combined
// with the title search, the order and the access.
func TestS3_Filtros(t *testing.T) {
	e := nuevoSubidaEnv(t, nil)
	s := sembrarListado(t, e)
	q := "&q=" + url.QueryEscape(s.marca)
	todos := []int64{s.oficial, s.procesando, s.fallida, s.conV2, s.conV2Fallida, s.ret, s.recuperada}
	cases := []struct {
		name  string
		query string
		want  []int64
	}{
		{"solo la marca", strings.TrimPrefix(q, "&"), todos},
		{"estado activa", "estado=activa" + q, []int64{s.oficial, s.conV2, s.conV2Fallida, s.recuperada}},
		{"estado procesando", "estado=procesando" + q, []int64{s.procesando}},
		{"estado fallida", "estado=fallida" + q, []int64{s.fallida}},
		{"estado retirada", "estado=retirada" + q, []int64{s.ret}},
		{"tipo norma", "tipo=norma" + q, []int64{s.procesando}},
		{"especialidad A incluye las de todas", fmt.Sprintf("especialidad_id=%d", e.espA) + q,
			[]int64{s.oficial, s.conV2Fallida, s.ret, s.procesando}},
		{"especialidad B", fmt.Sprintf("especialidad_id=%d", e.espB) + q, []int64{s.fallida, s.conV2, s.recuperada, s.procesando}},
		{"oficial true", "oficial=true" + q, []int64{s.oficial}},
		{"oficial false incluye las sin vigente", "oficial=false" + q,
			[]int64{s.procesando, s.fallida, s.conV2, s.conV2Fallida, s.ret, s.recuperada}},
		{"combinados", fmt.Sprintf("estado=activa&tipo=guia&especialidad_id=%d&oficial=false", e.espB) + q, []int64{s.conV2, s.recuperada}},
		{"título sin mayúsculas", "q=" + url.QueryEscape(strings.ToUpper(s.marca)+" ALFA"), []int64{s.oficial}},
		{"% es literal", "q=" + url.QueryEscape(s.marca+" 50%"), []int64{s.conV2Fallida}},
		{"_ es literal", "q=" + url.QueryEscape("dto_x"), []int64{s.conV2Fallida}},
		{"_ no es comodín", "q=" + url.QueryEscape(s.marca+" _lfa"), []int64{}},
		{"q con espacios alrededor", "q=" + url.QueryEscape("  "+s.marca+" omega  "), []int64{s.ret}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, body := e.get(t, "/admin/fuentes?"+c.query, e.admin)
			if status != http.StatusOK {
				t.Fatalf("%d %v", status, body)
			}
			if got := idsDe(t, body); !mismos(got, c.want) || body["total"] != float64(len(c.want)) {
				t.Errorf("ids = %v (total %v), want %v", got, body["total"], c.want)
			}
		})
	}

	t.Run("orden updated_at DESC, id DESC", func(t *testing.T) {
		_, body := e.get(t, "/admin/fuentes?per_page=100"+q, e.admin)
		var claves []string
		for _, d := range body["data"].([]any) {
			f := d.(map[string]any)
			at, err := time.Parse(time.RFC3339Nano, f["updated_at"].(string))
			if err != nil {
				t.Fatal(err)
			}
			claves = append(claves, fmt.Sprintf("%020d-%020d", at.UnixNano(), int64(f["id"].(float64))))
		}
		if !sort.SliceIsSorted(claves, func(i, j int) bool { return claves[i] > claves[j] }) {
			t.Errorf("not sorted by updated_at, id descending: %v", claves)
		}
	})

	for _, c := range []struct {
		name   string
		bearer string
		status int
	}{{"anónimo", "", http.StatusUnauthorized}, {"no admin", e.usuario, http.StatusForbidden}} {
		t.Run(c.name, func(t *testing.T) {
			if status, body := e.get(t, "/admin/fuentes", c.bearer); status != c.status {
				t.Errorf("S3: %d %v, want %d", status, body, c.status)
			}
			if status, body := e.get(t, fmt.Sprintf("/admin/fuentes/%d", s.oficial), c.bearer); status != c.status {
				t.Errorf("S4: %d %v, want %d", status, body, c.status)
			}
		})
	}
}

// Pagination and the title search length (contract S3).
func TestS3_PaginacionYConsulta(t *testing.T) {
	e := nuevoSubidaEnv(t, nil)
	s := sembrarListado(t, e)
	q := "&q=" + url.QueryEscape(s.marca)
	cases := []struct {
		name   string
		query  string
		status int
		filas  int
	}{
		{"página 2 de 2 en 2", "?per_page=2&page=2" + q, 200, 2},
		{"última página parcial", "?per_page=4&page=2" + q, 200, 3},
		{"página vacía", "?per_page=2&page=5" + q, 200, 0},
		{"q de 100 caracteres", "?q=" + strings.Repeat("ñ", 100), 200, 0},
		{"q de 101 caracteres", "?q=" + strings.Repeat("ñ", 101), 422, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, body := e.get(t, "/admin/fuentes"+c.query, e.admin)
			if status != c.status {
				t.Fatalf("%d %v, want %d", status, body, c.status)
			}
			if c.status == 422 {
				if codigoDe(body) != "validacion" || body["detalle"].(map[string]any)["campos"].(map[string]any)["q"] != "max" {
					t.Errorf("body = %v", body)
				}
				return
			}
			if got := idsDe(t, body); len(got) != c.filas {
				t.Errorf("%d rows, want %d: %v", len(got), c.filas, body)
			}
			if strings.Contains(c.query, s.marca) && body["total"] != 7.0 {
				t.Errorf("total = %v, want 7", body["total"])
			}
		})
	}
}

// Every FuenteResumen field per state, and the same summary in S4 (one SQL
// for both): version_vigente, version_en_proceso (an archived version after
// the failed one is skipped), especialidades by name, aplica_a_todas.
func TestS3_S4_Resumen(t *testing.T) {
	e := nuevoSubidaEnv(t, nil)
	s := sembrarListado(t, e)
	_, lista := e.get(t, "/admin/fuentes?per_page=100&q="+url.QueryEscape(s.marca), e.admin)
	filas := map[int64]map[string]any{}
	for _, d := range lista["data"].([]any) {
		f := d.(map[string]any)
		filas[int64(f["id"].(float64))] = f
	}
	var nombreA, nombreB string
	if err := e.db.QueryRow(`SELECT (SELECT nombre FROM especialidades WHERE id = $1), (SELECT nombre FROM especialidades WHERE id = $2)`,
		e.espA, e.espB).Scan(&nombreA, &nombreB); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name       string
		id         int64
		estado     string
		vigente    string // fmt of version_vigente
		enProceso  string // fmt of version_en_proceso
		todas      bool
		especialid string
	}{
		{"activa con marca", s.oficial, "activa", "map[numero:1 oficial_verificada:true pasajes_count:1]", "<nil>", false, nombreA},
		{"procesando para todas", s.procesando, "procesando", "<nil>", "map[estado:procesando motivo_fallo:<nil> numero:1]", true, ""},
		{"fallida sin vigente", s.fallida, "fallida", "<nil>", "map[estado:fallida motivo_fallo:limite_pasajes numero:1]", false, nombreB},
		{"activa con v2 en proceso", s.conV2, "activa", "map[numero:1 oficial_verificada:false pasajes_count:1]",
			"map[estado:procesando motivo_fallo:<nil> numero:2]", false, nombreB},
		{"activa con v2 fallida y v3 archivada", s.conV2Fallida, "activa", "map[numero:1 oficial_verificada:false pasajes_count:1]",
			"map[estado:fallida motivo_fallo:limite_pasajes numero:2]", false, nombreA},
		{"retirada", s.ret, "retirada", "map[numero:1 oficial_verificada:false pasajes_count:1]", "<nil>", false, nombreA},
		{"recuperada: la fallida anterior a la vigente no está en proceso", s.recuperada, "activa",
			"map[numero:2 oficial_verificada:false pasajes_count:1]", "<nil>", false, nombreB},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fila := filas[c.id]
			if fila == nil {
				t.Fatalf("fuente %d not listed", c.id)
			}
			var nombres []string
			for _, es := range fila["especialidades"].([]any) {
				nombres = append(nombres, es.(map[string]any)["nombre"].(string))
			}
			if fila["estado"] != c.estado || fmt.Sprint(fila["version_vigente"]) != c.vigente || fmt.Sprint(fila["version_en_proceso"]) != c.enProceso ||
				fila["aplica_a_todas"] != c.todas || strings.Join(nombres, ",") != c.especialid {
				t.Errorf("row = %v", fila)
			}
			status, detalle := e.get(t, fmt.Sprintf("/admin/fuentes/%d", c.id), e.admin)
			if status != http.StatusOK {
				t.Fatalf("S4: %d %v", status, detalle)
			}
			for k, v := range fila {
				if fmt.Sprint(detalle[k]) != fmt.Sprint(v) {
					t.Errorf("S4 %s = %v, S3 says %v", k, detalle[k], v)
				}
			}
		})
	}

	t.Run("detalle completo", func(t *testing.T) {
		_, d := e.get(t, fmt.Sprintf("/admin/fuentes/%d", s.oficial), e.admin)
		oficial, _ := d["oficial"].(map[string]any)
		if oficial["entidad_emisora"] != "MINSA" || oficial["nota"] != "Descargado de la web oficial" || d["contenidos_citantes"] != 0.0 ||
			len(d["versiones"].([]any)) != 1 || d["motivo_retiro"] != nil {
			t.Errorf("detalle = %v", d)
		}
		_, d = e.get(t, fmt.Sprintf("/admin/fuentes/%d", s.conV2Fallida), e.admin)
		var numeros []string
		for _, v := range d["versiones"].([]any) {
			vm := v.(map[string]any)
			numeros = append(numeros, fmt.Sprintf("%v %v", vm["numero"], vm["estado"]))
		}
		if strings.Join(numeros, ",") != "3 archivada,2 fallida,1 lista" || d["oficial"] != nil {
			t.Errorf("versiones = %v, oficial = %v", numeros, d["oficial"])
		}
		_, d = e.get(t, fmt.Sprintf("/admin/fuentes/%d", s.ret), e.admin)
		if d["motivo_retiro"] != "Prueba del listado" {
			t.Errorf("motivo_retiro = %v", d["motivo_retiro"])
		}
	})
	t.Run("inexistente", func(t *testing.T) {
		if status, body := e.get(t, "/admin/fuentes/999999999", e.admin); status != http.StatusNotFound || codigoDe(body) != "no_encontrado" {
			t.Errorf("%d %v", status, body)
		}
	})
}
