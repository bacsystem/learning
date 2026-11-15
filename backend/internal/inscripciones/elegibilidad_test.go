package inscripciones

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/jmoiron/sqlx"
)

// PLT-06: a postulante only sees and joins the procesos of their institution.

func procesoConNivel(t *testing.T, db *sqlx.DB, c dbtest.Catalog, estado, nivel string) int64 {
	t.Helper()
	id := dbtest.SeedProceso(t, db, c, testAnio, estado)
	if _, err := db.Exec(`UPDATE procesos SET nivel = NULLIF($2, '') WHERE id = $1`, id, nivel); err != nil {
		t.Fatal(err)
	}
	return id
}

func decodeProcesos(t *testing.T, rr *httptest.ResponseRecorder) []ProcesoDTO {
	t.Helper()
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", rr.Code, rr.Body)
	}
	var out []ProcesoDTO
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("body is not a JSON array: %v (%s)", err, rr.Body)
	}
	return out
}

func listarProcesos(t *testing.T, h *Handler, userID int64, rol string) []ProcesoDTO {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/procesos", nil)
	return decodeProcesos(t, dbtest.Serve(t, h.ListProcesos, req, userID, rol))
}

func idsDe(list []ProcesoDTO) map[int64]bool {
	m := map[int64]bool{}
	for _, p := range list {
		m[p.ID] = true
	}
	return m
}

func TestListProcesos_SoloInstitucionYNivel(t *testing.T) {
	db, h, _ := setup(t)
	a, b := dbtest.SeedCatalog(t, db), dbtest.SeedCatalog(t, db)
	user := dbtest.SeedUser(t, db, a, "usuario", "activo")
	admin := dbtest.SeedUser(t, db, a, "admin", "activo")
	sinNivel := procesoConNivel(t, db, a, "activo", "")
	mismoNivel := procesoConNivel(t, db, a, "activo", "suboficial")
	otroNivel := procesoConNivel(t, db, a, "activo", "oficial")
	inactivo := procesoConNivel(t, db, a, "inactivo", "")
	deB := procesoConNivel(t, db, b, "activo", "")

	roles := []struct {
		name string
		id   int64
		rol  string
	}{{"usuario", user.ID, "usuario"}, {"admin", admin.ID, "admin"}}
	for _, r := range roles {
		got := idsDe(listarProcesos(t, h, r.id, r.rol))
		if !got[sinNivel] || !got[mismoNivel] {
			t.Errorf("%s: missing own procesos: %v", r.name, got)
		}
		for name, id := range map[string]int64{"otro nivel": otroNivel, "inactivo": inactivo, "otra institucion": deB} {
			if got[id] {
				t.Errorf("%s: sees the %s proceso", r.name, name)
			}
		}
	}
}

func TestListProcesos_SinProcesosEsArrayVacio(t *testing.T) {
	db, h, _ := setup(t)
	a, b := dbtest.SeedCatalog(t, db), dbtest.SeedCatalog(t, db)
	user := dbtest.SeedUser(t, db, a, "usuario", "activo")
	procesoConNivel(t, db, b, "activo", "")
	req := httptest.NewRequest(http.MethodGet, "/procesos", nil)
	rr := dbtest.Serve(t, h.ListProcesos, req, user.ID, "usuario")
	if got := decodeProcesos(t, rr); len(got) != 0 {
		t.Fatalf("want no procesos, got %v", got)
	}
	if body := bytes.TrimSpace(rr.Body.Bytes()); string(body) != "[]" {
		t.Fatalf("want [], got %s", body)
	}
}

// The public landing keeps listing every active proceso.
func TestListProcesosPublico_NoCambia(t *testing.T) {
	db, h, _ := setup(t)
	b := dbtest.SeedCatalog(t, db)
	deB := procesoConNivel(t, db, b, "activo", "")
	rr := httptest.NewRecorder()
	h.ListProcesosPublico(rr, httptest.NewRequest(http.MethodGet, "/procesos/publico", nil))
	if !idsDe(decodeProcesos(t, rr))[deB] {
		t.Fatal("the public list must keep every institution")
	}
}

type errorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func countInscripciones(t *testing.T, db *sqlx.DB, userID int64) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM inscripciones WHERE usuario_id=$1`, userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestInscribir_Elegibilidad(t *testing.T) {
	db, h, _ := setup(t)
	setConfigPago(t, db, testAnio, "10")
	a, b := dbtest.SeedCatalog(t, db), dbtest.SeedCatalog(t, db)
	user := dbtest.SeedUser(t, db, a, "usuario", "activo")
	ok := procesoConNivel(t, db, a, "activo", "")
	deB := procesoConNivel(t, db, b, "activo", "")
	otroNivel := procesoConNivel(t, db, a, "activo", "oficial")
	deBOtroNivel := procesoConNivel(t, db, b, "activo", "oficial")
	inactivoDeB := procesoConNivel(t, db, b, "inactivo", "")
	inactivo := procesoConNivel(t, db, a, "inactivo", "")

	// Cases run in order: "ya inscrito" needs the row of "ok".
	cases := []struct {
		name    string
		proceso int64
		status  int
		code    string // "" = success body
		newRows int
	}{
		{"ok", ok, http.StatusCreated, "", 1},
		{"ya inscrito", ok, http.StatusConflict, "ya_inscrito", 0},
		{"otra institucion", deB, http.StatusForbidden, "institucion_no_permitida", 0},
		{"otro nivel", otroNivel, http.StatusForbidden, "nivel_no_permitido", 0},
		{"institucion gana al nivel", deBOtroNivel, http.StatusForbidden, "institucion_no_permitida", 0},
		{"inactivo gana a la institucion", inactivoDeB, http.StatusNotFound, "no_encontrado", 0},
		{"inactivo", inactivo, http.StatusNotFound, "no_encontrado", 0},
		{"proceso_id negativo", -5, http.StatusBadRequest, "cuerpo_invalido", 0},
		{"inexistente positivo", 1 << 40, http.StatusNotFound, "no_encontrado", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			antes := countInscripciones(t, db, user.ID)
			rr := inscribir(t, h, user.ID, c.proceso)
			if rr.Code != c.status {
				t.Fatalf("status = %d, want %d (%s)", rr.Code, c.status, rr.Body)
			}
			if c.code != "" {
				var e errorBody
				if err := json.Unmarshal(rr.Body.Bytes(), &e); err != nil || e.Error != c.code || e.Message == "" {
					t.Fatalf("body = %s, want error code %q with a message", rr.Body, c.code)
				}
			}
			if got := countInscripciones(t, db, user.ID) - antes; got != c.newRows {
				t.Fatalf("new inscripciones = %d, want %d", got, c.newRows)
			}
		})
	}
}

func TestInscribir_SinMontoTieneSuCodigo(t *testing.T) {
	db, h, _ := setup(t)
	a := dbtest.SeedCatalog(t, db)
	user := dbtest.SeedUser(t, db, a, "usuario", "activo")
	proc := dbtest.SeedProceso(t, db, a, testAnio+1, "activo") // year without configuracion_pago
	rr := inscribir(t, h, user.ID, proc)
	var e errorBody
	_ = json.Unmarshal(rr.Body.Bytes(), &e)
	if rr.Code != http.StatusConflict || e.Error != "sin_monto_configurado" {
		t.Fatalf("got %d %s, want 409 sin_monto_configurado", rr.Code, rr.Body)
	}
}

func TestInscribir_CuerpoInvalido(t *testing.T) {
	db, h, _ := setup(t)
	a := dbtest.SeedCatalog(t, db)
	user := dbtest.SeedUser(t, db, a, "usuario", "activo")
	for _, body := range []string{`{`, `{}`, `{"proceso_id":0}`, `{"proceso_id":-3}`} {
		req := httptest.NewRequest(http.MethodPost, "/inscripciones", bytes.NewBufferString(body))
		rr := dbtest.Serve(t, h.Inscribir, req, user.ID, "usuario")
		var e errorBody
		_ = json.Unmarshal(rr.Body.Bytes(), &e)
		if rr.Code != http.StatusBadRequest || e.Error != "cuerpo_invalido" {
			t.Fatalf("%s: got %d %s, want 400 cuerpo_invalido", body, rr.Code, rr.Body)
		}
	}
}
