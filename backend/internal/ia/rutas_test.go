package ia_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/audit/audittest"
	"github.com/cbaciliod/learning/internal/auth"
	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/cbaciliod/learning/internal/ia"
	jwtpkg "github.com/cbaciliod/learning/internal/jwt"
)

const secretoDePrueba = "ia-rutas-secret-32-bytes-exactly!"

// servidorIA mounts the routes with a real JWT check; the session checker
// answers the role registered for each token, as a live session would.
type servidorIA struct {
	t        *testing.T
	db       *sqlx.DB
	costos   *ia.PgCostos
	mux      *http.ServeMux
	sesiones map[string]*auth.Session
}

func nuevoServidorIA(t *testing.T) *servidorIA {
	t.Helper()
	db := entornoCostos(t)
	s := &servidorIA{t: t, db: db, costos: nuevoPgCostos(t, db, reloj(mediodia)), mux: http.NewServeMux(), sesiones: map[string]*auth.Session{}}
	check := func(_ context.Context, hash string) (*auth.Session, error) { return s.sesiones[hash], nil }
	ia.Mount(s.mux, s.costos, audit.New(db), secretoDePrueba, check)
	return s
}

// entrar issues a token for an existing user and registers its live session.
func (s *servidorIA) entrar(userID int64, rol string) string {
	s.t.Helper()
	tok, _, err := jwtpkg.Issue(secretoDePrueba, userID, rol, "", 0, 0, 0)
	if err != nil {
		s.t.Fatal(err)
	}
	s.sesiones[fmt.Sprintf("%x", sha256.Sum256([]byte(tok)))] = &auth.Session{UserID: userID, Rol: rol, Estado: "activo"}
	return tok
}

func (s *servidorIA) pedir(token, metodo, ruta, cuerpo string) *httptest.ResponseRecorder {
	s.t.Helper()
	r := httptest.NewRequest(metodo, ruta, strings.NewReader(cuerpo))
	if cuerpo != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, r)
	return w
}

func decodificar(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("body %q: %v", w.Body.String(), err)
	}
	return m
}

const cuerpoValido = `{"limite_usuario_dia": 5, "limite_usuario_mes": 60, "limite_admin_dia": 200,
	"max_preguntas_solicitud": 20, "lote_clasificacion": 20, "presupuesto_mensual_usd": 40.00}`

// cuerpoCon is the valid body with the pairs (field, JSON value) replaced; an
// empty value removes the field.
func cuerpoCon(pares ...string) string {
	var m map[string]json.RawMessage
	_ = json.Unmarshal([]byte(cuerpoValido), &m)
	for i := 0; i < len(pares); i += 2 {
		if pares[i+1] == "" {
			delete(m, pares[i])
		} else {
			m[pares[i]] = json.RawMessage(pares[i+1])
		}
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// Authorization of the four routes: 401 without session, 403 for a usuario on
// the admin ones, 200 otherwise (README §2).
func TestRutas_Autorizacion(t *testing.T) {
	s := nuevoServidorIA(t)
	admin := dbtest.SeedUser(t, s.db, dbtest.SeedCatalog(t, s.db), "admin", "activo")
	usuario := dbtest.SeedUser(t, s.db, dbtest.SeedCatalog(t, s.db), "usuario", "activo")
	tokAdmin, tokUsuario := s.entrar(admin.ID, "admin"), s.entrar(usuario.ID, "usuario")
	rutas := []struct {
		metodo, ruta, cuerpo string
		soloAdmin            bool
	}{
		{"GET", "/serums-ia/ia/consumo", "", false},
		{"GET", "/admin/ia/configuracion", "", true},
		{"PUT", "/admin/ia/configuracion", cuerpoValido, true},
		{"GET", "/admin/ia/gasto", "", true},
	}
	for _, r := range rutas {
		t.Run(r.metodo+" "+r.ruta, func(t *testing.T) {
			if w := s.pedir("", r.metodo, r.ruta, r.cuerpo); w.Code != http.StatusUnauthorized {
				t.Errorf("anonymous: %d, want 401", w.Code)
			}
			want := http.StatusOK
			if r.soloAdmin {
				want = http.StatusForbidden
			}
			if w := s.pedir(tokUsuario, r.metodo, r.ruta, r.cuerpo); w.Code != want {
				t.Errorf("usuario: %d, want %d (%s)", w.Code, want, w.Body)
			}
			if w := s.pedir(tokAdmin, r.metodo, r.ruta, r.cuerpo); w.Code != http.StatusOK {
				t.Errorf("admin: %d, want 200 (%s)", w.Code, w.Body)
			}
		})
	}
}

// §3.1: the consumption comes from the claims; an admin has no month.
func TestConsumo_DelUsuarioAutenticado(t *testing.T) {
	s := nuevoServidorIA(t)
	cat := dbtest.SeedCatalog(t, s.db)
	usuario := dbtest.SeedUser(t, s.db, cat, "usuario", "activo")
	otro := dbtest.SeedUser(t, s.db, cat, "usuario", "activo")
	admin := dbtest.SeedUser(t, s.db, cat, "admin", "activo")
	reservar(t, s.costos, ia.Actor{Rol: ia.RolUsuario, ID: &usuario.ID}, 3)
	reservar(t, s.costos, ia.Actor{Rol: ia.RolUsuario, ID: &otro.ID}, 1)

	got := decodificar(t, s.pedir(s.entrar(usuario.ID, "usuario"), "GET", "/serums-ia/ia/consumo?usuario_id="+fmt.Sprint(otro.ID), ""))
	dia := got["dia"].(map[string]any)
	mes := got["mes"].(map[string]any)
	if dia["usados"] != 3.0 || dia["limite"] != 5.0 || dia["disponibles"] != 2.0 || dia["reinicia_at"] != "2026-11-13T05:00:00Z" ||
		mes["usados"] != 3.0 || mes["limite"] != 60.0 || mes["disponibles"] != 57.0 || mes["reinicia_at"] != "2026-12-01T05:00:00Z" ||
		got["presupuesto_agotado"] != false {
		t.Errorf("consumo = %v (a usuario_id in the query must be ignored)", got)
	}
	adm := decodificar(t, s.pedir(s.entrar(admin.ID, "admin"), "GET", "/serums-ia/ia/consumo", ""))
	if m, ok := adm["mes"]; !ok || m != nil {
		t.Errorf("admin mes = %v (present = %v), want null", m, ok)
	}
	if adm["dia"].(map[string]any)["limite"] != 200.0 {
		t.Errorf("admin day = %v", adm["dia"])
	}
}

func TestConfiguracion_Leer(t *testing.T) {
	s := nuevoServidorIA(t)
	admin := dbtest.SeedUser(t, s.db, dbtest.SeedCatalog(t, s.db), "admin", "activo")
	got := decodificar(t, s.pedir(s.entrar(admin.ID, "admin"), "GET", "/admin/ia/configuracion", ""))
	want := map[string]any{"limite_usuario_dia": 5.0, "limite_usuario_mes": 60.0, "limite_admin_dia": 200.0,
		"max_preguntas_solicitud": 20.0, "lote_clasificacion": 20.0, "presupuesto_mensual_usd": 40.0}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %v", k, got[k], v)
		}
	}
	if _, ok := got["updated_at"]; !ok {
		t.Error("updated_at is missing")
	}
}

// §3.3 validation, in order: the body first (400/413), then the ranges (D19)
// with every broken field listed.
func TestConfigurar_Validacion(t *testing.T) {
	s := nuevoServidorIA(t)
	admin := dbtest.SeedUser(t, s.db, dbtest.SeedCatalog(t, s.db), "admin", "activo")
	tok := s.entrar(admin.ID, "admin")
	tests := []struct {
		name   string
		cuerpo string
		status int
		code   string
		campos map[string]string
	}{
		{"valid at the lower edges", `{"limite_usuario_dia":0,"limite_usuario_mes":0,"limite_admin_dia":1,"max_preguntas_solicitud":1,"lote_clasificacion":1,"presupuesto_mensual_usd":1}`, 200, "", nil},
		{"valid at the upper edges", `{"limite_usuario_dia":50,"limite_usuario_mes":1000,"limite_admin_dia":1000,"max_preguntas_solicitud":50,"lote_clasificacion":50,"presupuesto_mensual_usd":1000}`, 200, "", nil},
		{"invalid JSON", `{"limite_usuario_dia":`, 400, "cuerpo_invalido", nil},
		{"unknown field", cuerpoCon("otro", "1"), 400, "cuerpo_invalido", nil},
		{"usuario day over 50", cuerpoCon("limite_usuario_dia", "51"), 422, "validacion", map[string]string{"limite_usuario_dia": "max"}},
		{"usuario day negative", cuerpoCon("limite_usuario_dia", "-1"), 422, "validacion", map[string]string{"limite_usuario_dia": "min"}},
		{"usuario month over 1000", cuerpoCon("limite_usuario_mes", "1001"), 422, "validacion", map[string]string{"limite_usuario_mes": "max"}},
		{"admin day 0", cuerpoCon("limite_admin_dia", "0"), 422, "validacion", map[string]string{"limite_admin_dia": "min"}},
		{"admin day over 1000", cuerpoCon("limite_admin_dia", "1001"), 422, "validacion", map[string]string{"limite_admin_dia": "max"}},
		{"max questions 0", cuerpoCon("max_preguntas_solicitud", "0"), 422, "validacion", map[string]string{"max_preguntas_solicitud": "min"}},
		{"batch over 50", cuerpoCon("lote_clasificacion", "51"), 422, "validacion", map[string]string{"lote_clasificacion": "max"}},
		{"budget under 1", cuerpoCon("presupuesto_mensual_usd", "0.99"), 422, "validacion", map[string]string{"presupuesto_mensual_usd": "min"}},
		{"budget 0", cuerpoCon("presupuesto_mensual_usd", "0"), 422, "validacion", map[string]string{"presupuesto_mensual_usd": "min"}},
		{"budget over 1000", cuerpoCon("presupuesto_mensual_usd", "1000.01"), 422, "validacion", map[string]string{"presupuesto_mensual_usd": "max"}},
		{"budget with 3 decimals", cuerpoCon("presupuesto_mensual_usd", "12.345"), 422, "validacion", map[string]string{"presupuesto_mensual_usd": "dec2"}},
		{"day over month", `{"limite_usuario_dia":10,"limite_usuario_mes":5,"limite_admin_dia":200,"max_preguntas_solicitud":20,"lote_clasificacion":20,"presupuesto_mensual_usd":40}`, 422, "validacion", map[string]string{"limite_usuario_dia": "dia_mayor_que_mes"}},
		{"a missing field", cuerpoCon("limite_admin_dia", ""), 422, "validacion", map[string]string{"limite_admin_dia": "requerido"}},
		{"a null field", cuerpoCon("lote_clasificacion", "null"), 422, "validacion", map[string]string{"lote_clasificacion": "requerido"}},
		{"every broken field is listed", `{"limite_usuario_dia":99,"limite_usuario_mes":5000,"presupuesto_mensual_usd":0}`, 422, "validacion", map[string]string{
			"limite_usuario_dia": "max", "limite_usuario_mes": "max", "limite_admin_dia": "requerido",
			"max_preguntas_solicitud": "requerido", "lote_clasificacion": "requerido", "presupuesto_mensual_usd": "min"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := s.pedir(tok, "PUT", "/admin/ia/configuracion", tt.cuerpo)
			if w.Code != tt.status {
				t.Fatalf("status %d, want %d (%s)", w.Code, tt.status, w.Body)
			}
			if tt.status == 200 {
				return
			}
			got := decodificar(t, w)
			if got["error"] != tt.code || got["message"] == nil || got["message"] == "" {
				t.Errorf("error body = %v, want code %s with message", got, tt.code)
			}
			if tt.campos != nil {
				campos := got["detalle"].(map[string]any)["campos"].(map[string]any)
				if len(campos) != len(tt.campos) {
					t.Errorf("campos = %v, want %v", campos, tt.campos)
				}
				for k, v := range tt.campos {
					if campos[k] != v {
						t.Errorf("campos[%s] = %v, want %s", k, campos[k], v)
					}
				}
			}
		})
	}
}

type filaAuditoria struct {
	Accion    string `db:"accion"`
	Entidad   string `db:"entidad"`
	EntidadID *int64 `db:"entidad_id"`
	UsuarioID *int64 `db:"usuario_id"`
	Detalle   []byte `db:"detalle"`
}

func auditoriasDeConfiguracion(t *testing.T, db *sqlx.DB, adminID int64) []filaAuditoria {
	t.Helper()
	var filas []filaAuditoria
	if err := db.Select(&filas, `SELECT accion, entidad, entidad_id, usuario_id, detalle FROM audit_log
		WHERE accion = 'ia.configurar' AND usuario_id = $1 ORDER BY id`, adminID); err != nil {
		t.Fatal(err)
	}
	return filas
}

// §3.3 audit: only the fields that changed, with before and after, in the same
// transaction; no change: 200 and no row; a failed audit rolls the change back.
func TestConfigurar_Auditoria(t *testing.T) {
	s := nuevoServidorIA(t)
	admin := dbtest.SeedUser(t, s.db, dbtest.SeedCatalog(t, s.db), "admin", "activo")
	tok := s.entrar(admin.ID, "admin")
	t.Cleanup(func() { _, _ = s.db.Exec(`DELETE FROM audit_log WHERE usuario_id = $1`, admin.ID) })

	cuerpo := cuerpoCon("limite_usuario_dia", "3", "presupuesto_mensual_usd", "60.5")
	w := s.pedir(tok, "PUT", "/admin/ia/configuracion", cuerpo)
	if w.Code != 200 {
		t.Fatalf("PUT: %d %s", w.Code, w.Body)
	}
	got := decodificar(t, w)
	if got["limite_usuario_dia"] != 3.0 || got["presupuesto_mensual_usd"] != 60.5 {
		t.Errorf("response = %v", got)
	}
	var persistida ia.Configuracion
	if err := s.db.Get(&persistida, `SELECT limite_usuario_dia, limite_usuario_mes, limite_admin_dia, max_preguntas_solicitud,
		lote_clasificacion, presupuesto_mensual_usd::float8 AS presupuesto_mensual_usd, updated_at FROM ia_configuracion`); err != nil ||
		persistida.LimiteUsuarioDia != 3 || persistida.PresupuestoMensualUSD != 60.5 {
		t.Errorf("stored = %+v (%v)", persistida, err)
	}

	filas := auditoriasDeConfiguracion(t, s.db, admin.ID)
	if len(filas) != 1 || filas[0].Entidad != "ia_configuracion" || filas[0].EntidadID == nil || *filas[0].EntidadID != 1 {
		t.Fatalf("audit rows = %+v", filas)
	}
	var det struct {
		Cambios map[string]map[string]any `json:"cambios"`
	}
	if err := json.Unmarshal(filas[0].Detalle, &det); err != nil {
		t.Fatal(err)
	}
	if len(det.Cambios) != 2 || det.Cambios["limite_usuario_dia"]["antes"] != 5.0 || det.Cambios["limite_usuario_dia"]["despues"] != 3.0 ||
		det.Cambios["presupuesto_mensual_usd"]["antes"] != 40.0 || det.Cambios["presupuesto_mensual_usd"]["despues"] != 60.5 {
		t.Errorf("cambios = %v, want only the two that changed", det.Cambios)
	}

	// The same body again changes nothing: 200 and no new row.
	if w := s.pedir(tok, "PUT", "/admin/ia/configuracion", cuerpo); w.Code != 200 {
		t.Fatalf("PUT again: %d", w.Code)
	}
	if n := len(auditoriasDeConfiguracion(t, s.db, admin.ID)); n != 1 {
		t.Errorf("%d audit rows after a no-op PUT, want 1", n)
	}

	// If the audit cannot be written, the change rolls back.
	audittest.FailInsertsBy(t, s.db, admin.ID)
	if w := s.pedir(tok, "PUT", "/admin/ia/configuracion", cuerpoCon("lote_clasificacion", "7")); w.Code != http.StatusInternalServerError {
		t.Errorf("PUT with a failing audit: %d, want 500", w.Code)
	}
	var lote int
	_ = s.db.Get(&lote, `SELECT lote_clasificacion FROM ia_configuracion`)
	if lote != 20 {
		t.Errorf("lote_clasificacion = %d after a failed audit, want 20 (rolled back)", lote)
	}
}

// Regla 18, escenario «Bajar el presupuesto»: from 40 to 24 with 20 spent the
// alert goes out when saving; lowering to the spent amount cuts the calls at
// once; raising it without crossing 80 % sends nothing.
func TestConfigurar_PresupuestoRearmaLaAlerta(t *testing.T) {
	s := nuevoServidorIA(t)
	admin := dbtest.SeedUser(t, s.db, dbtest.SeedCatalog(t, s.db), "admin", "activo")
	tok := s.entrar(admin.ID, "admin")
	t.Cleanup(func() { _, _ = s.db.Exec(`DELETE FROM audit_log WHERE usuario_id = $1`, admin.ID) })
	gastar(t, s.costos, 20)
	if n := len(alertas(t, s.db)); n != 0 {
		t.Fatalf("%d alerts at 50 %%", n)
	}

	if w := s.pedir(tok, "PUT", "/admin/ia/configuracion", cuerpoCon("presupuesto_mensual_usd", "24")); w.Code != 200 {
		t.Fatalf("PUT 24: %d %s", w.Code, w.Body)
	}
	as := alertas(t, s.db)
	if len(as) != 1 || as[0].Cuerpo != "Se gastaron 20,00 USD de 24,00 USD en noviembre de 2026." {
		t.Fatalf("alerts after lowering to 24 = %+v", as)
	}
	if err := s.costos.ComprobarPresupuesto(context.Background()); err != nil {
		t.Errorf("20 of 24 must still allow calls: %v", err)
	}

	if w := s.pedir(tok, "PUT", "/admin/ia/configuracion", cuerpoCon("presupuesto_mensual_usd", "20")); w.Code != 200 {
		t.Fatal(w.Body)
	}
	_ = esPresupuesto(t, s.costos.ComprobarPresupuesto(context.Background())) // cut at the moment of saving
	if n := len(alertas(t, s.db)); n != 2 {
		t.Errorf("%d alerts after changing to 20, want 2 (one per budget value)", n)
	}

	if w := s.pedir(tok, "PUT", "/admin/ia/configuracion", cuerpoCon("presupuesto_mensual_usd", "100")); w.Code != 200 {
		t.Fatal(w.Body)
	}
	if err := s.costos.ComprobarPresupuesto(context.Background()); err != nil {
		t.Errorf("raising the budget must reopen the calls: %v", err)
	}
	if n := len(alertas(t, s.db)); n != 2 {
		t.Errorf("%d alerts after raising to 100, want still 2", n)
	}
}

// §3.4: the month's spent amount, the share, the alert instant and the
// detail by operation; nothing per user.
func TestGasto_Mensual(t *testing.T) {
	s := nuevoServidorIA(t)
	cat := dbtest.SeedCatalog(t, s.db)
	admin := dbtest.SeedUser(t, s.db, cat, "admin", "activo")
	tok := s.entrar(admin.ID, "admin")
	var desde int64
	_ = s.db.Get(&desde, `SELECT COALESCE(MAX(id), 0) FROM ia_usos`)
	t.Cleanup(func() { _, _ = s.db.Exec(`DELETE FROM ia_usos WHERE id > $1`, desde) })

	got := decodificar(t, s.pedir(tok, "GET", "/admin/ia/gasto", ""))
	if got["mes"] != "2026-11" || got["gastado_usd"] != 0.0 || got["porcentaje"] != 0.0 || got["agotado"] != false ||
		got["alerta_80_at"] != nil || got["reinicia_at"] != "2026-12-01T05:00:00Z" {
		t.Errorf("empty month = %v", got)
	}
	if ops, ok := got["por_operacion"].([]any); !ok || len(ops) != 0 {
		t.Errorf("por_operacion = %v, want []", got["por_operacion"])
	}

	if _, err := s.db.Exec(`INSERT INTO ia_usos (operacion, actor_rol, actor_id, proveedor, duracion_ms, costo_usd, resultado, desde_cache, created_at) VALUES
		('clasificar', 'admin', $1, 'doble', 1, 0.5, 'ok', false, $2),
		('clasificar', 'admin', $1, 'doble', 1, 0,   'ok', true,  $2),
		('generar_material', 'sistema', NULL, 'doble', 1, 1.236, 'ok', false, $2),
		('clasificar', 'admin', $1, 'doble', 1, 9, 'ok', false, '2026-10-31T12:00:00Z'),
		('clasificar', 'admin', $1, 'doble', 1, 9, 'ok', false, '2026-12-01T04:59:59Z')`,
		admin.ID, mediodia.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	gastar(t, s.costos, 32.105)
	got = decodificar(t, s.pedir(tok, "GET", "/admin/ia/gasto", ""))
	if got["presupuesto_usd"] != 40.0 || got["gastado_usd"] != 32.11 || got["porcentaje"] != 80.26 || got["agotado"] != false || got["alerta_80_at"] == nil {
		t.Errorf("month with spending = %v", got)
	}
	ops := got["por_operacion"].([]any)
	if len(ops) != 2 {
		t.Fatalf("por_operacion = %v, want 2 operations (the 31/10 Lima row is October)", ops)
	}
	// 30/11 23:59:59 Lima (04:59:59Z of 1/12) is still November; sorted by operation.
	clasificar, material := ops[0].(map[string]any), ops[1].(map[string]any)
	if clasificar["operacion"] != "clasificar" || clasificar["invocaciones"] != 3.0 || clasificar["desde_cache"] != 1.0 || clasificar["costo_usd"] != 9.5 ||
		material["operacion"] != "generar_material" || material["invocaciones"] != 1.0 || material["desde_cache"] != 0.0 || material["costo_usd"] != 1.24 {
		t.Errorf("por_operacion = %v", ops)
	}
	gastar(t, s.costos, 10)
	got = decodificar(t, s.pedir(tok, "GET", "/admin/ia/gasto", ""))
	if got["agotado"] != true || got["porcentaje"].(float64) <= 100 {
		t.Errorf("over the budget = %v: want agotado and a share above 100 (D14)", got)
	}
	for k := range got {
		if strings.Contains(k, "usuario") || strings.Contains(k, "actor") {
			t.Errorf("the response has a per-user field %q", k)
		}
	}
}
