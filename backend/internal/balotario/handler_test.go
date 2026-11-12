package balotario

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/fuentes"
	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
	"github.com/jmoiron/sqlx"
)

// fx is a mux with one admin and one plain usuario, and a way to publish fuentes.
type fx struct {
	f       *qt.Fixture
	db      *sqlx.DB
	mux     *http.ServeMux
	admin   string
	usuario string
	src     *fuentes.PgSource
}

func nuevoFx(t *testing.T) *fx {
	t.Helper()
	db := qt.DB(t)
	f := qt.Seed(t, db, qt.Options{Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 1, Peso: 100, Unicas: 1}}})
	var desde int64
	if err := db.Get(&desde, `SELECT COALESCE(MAX(id), 0) FROM fuentes`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { limpiarFuentes(t, db, desde) })
	mux := http.NewServeMux()
	Mount(mux, NewService(NewRepository(db)), qt.JWTSecret, qt.AllowSession)
	return &fx{f: f, db: db, mux: mux, admin: f.Token(t, f.UsuarioID, "admin"), usuario: f.Token(t, f.UsuarioID, "usuario"), src: fuentes.NewPgSource(db)}
}

// limpiarFuentes removes what the tests above desde created, imports first.
func limpiarFuentes(t *testing.T, db *sqlx.DB, desde int64) {
	t.Helper()
	for _, q := range []string{
		`DELETE FROM pregunta_balotario WHERE importacion_id IN (SELECT id FROM balotario_importaciones WHERE fuente_id > $1)`,
		`DELETE FROM balotario_importaciones WHERE fuente_id > $1`,
		`UPDATE fuentes SET version_vigente_id = NULL, estado = 'procesando', motivo_retiro = NULL WHERE id > $1`,
		`DELETE FROM pasajes WHERE fuente_version_id IN (SELECT id FROM fuente_versiones WHERE fuente_id > $1)`,
		`DELETE FROM fuente_versiones WHERE fuente_id > $1`,
		`DELETE FROM fuente_especialidades WHERE fuente_id > $1`,
		`DELETE FROM fuentes WHERE id > $1`,
	} {
		if _, err := db.Exec(q, desde); err != nil {
			t.Errorf("cleanup %q: %v", q, err)
		}
	}
}

// publicar stores a fuente of the tipo and publishes it with the passages given
// as ranges of lines.
func (x *fx) publicar(t *testing.T, tipo, contenido string, rangos [][2]int) int64 {
	t.Helper()
	ctx := context.Background()
	id, v, err := x.src.GuardarFuente(ctx, fuentes.NuevaFuente{Titulo: "Fuente " + fmt.Sprint(len(contenido)) + tipo, Tipo: tipo,
		NombreArchivo: "secreto.md", Contenido: []byte(contenido)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ps := make([]fuentes.PasajeNuevo, len(rangos))
	for i, r := range rangos {
		ps[i] = fuentes.PasajeNuevo{Orden: i + 1, LineaInicio: r[0], LineaFin: r[1], Texto: fmt.Sprintf("pasaje %d", i+1), Palabras: 2}
	}
	if err := x.src.PublicarVersion(ctx, id, v, ps, nil); err != nil {
		t.Fatal(err)
	}
	return id
}

// bloques builds n valid questions of 4 lines each (enunciado and 3 alternatives):
// question i occupies the lines 4(i-1)+1 … 4i.
func bloques(n int) string {
	var md strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&md, "%d. Enunciado %d\nA) Uno\n<mark>B) Dos</mark>\nC) Tres\n", i, i)
	}
	return md.String()
}

// sinClave builds n blocks with no key, numbered from desde.
func sinClave(desde, n int) string {
	var md strings.Builder
	for i := desde; i < desde+n; i++ {
		fmt.Fprintf(&md, "%d. Sin clave %d\nA) Uno\nB) Dos\n", i, i)
	}
	return md.String()
}

func (x *fx) get(path, token string) (int, map[string]any) {
	w := qt.Do(x.mux, "GET", path, token, "")
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return w.Code, body
}

func rutaPrevia(id int64, query string) string {
	return fmt.Sprintf("/admin/fuentes/%d/balotario%s", id, query)
}

func TestVistaPrevia_Validaciones(t *testing.T) {
	x := nuevoFx(t)
	todo := bloques(2)
	activo := x.publicar(t, "balotario", todo, [][2]int{{1, 8}})
	guia := x.publicar(t, "guia", todo+" ", [][2]int{{1, 8}})
	retirada := x.publicar(t, "balotario", todo+"  ", [][2]int{{1, 8}})
	if _, err := x.db.Exec(`UPDATE fuentes SET estado = 'retirada', motivo_retiro = 'prueba' WHERE id = $1`, retirada); err != nil {
		t.Fatal(err)
	}
	procesando, _, err := x.src.GuardarFuente(context.Background(), fuentes.NuevaFuente{Titulo: "En proceso", Tipo: "balotario",
		NombreArchivo: "p.md", Contenido: []byte("proceso " + todo)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	enorme := x.publicar(t, "balotario", bloques(1005), [][2]int{{1, 4020}})

	tests := []struct {
		name    string
		path    string
		token   string
		status  int
		code    string
		detalle map[string]any
	}{
		{"id no numerico", "/admin/fuentes/abc/balotario", x.admin, 400, "parametro_invalido", nil},
		{"id cero", "/admin/fuentes/0/balotario", x.admin, 400, "parametro_invalido", nil},
		{"page cero", rutaPrevia(activo, "?page=0"), x.admin, 400, "parametro_invalido", nil},
		{"page no numerico", rutaPrevia(activo, "?page=x"), x.admin, 400, "parametro_invalido", nil},
		{"per_page negativo", rutaPrevia(activo, "?per_page=-1"), x.admin, 400, "parametro_invalido", nil},
		{"fuente inexistente", rutaPrevia(987654321, ""), x.admin, 404, "no_encontrado", nil},
		{"no es balotario", rutaPrevia(guia, ""), x.admin, 422, "tipo_fuente_invalido", nil},
		{"retirada", rutaPrevia(retirada, ""), x.admin, 422, "fuente_no_activa", map[string]any{"estado": "retirada"}},
		{"procesando", rutaPrevia(procesando, ""), x.admin, 422, "fuente_no_activa", map[string]any{"estado": "procesando"}},
		{"mas de 1000 preguntas", rutaPrevia(enorme, ""), x.admin, 422, "limite_preguntas", map[string]any{"maximo": float64(1000), "detectadas": float64(1005)}},
		{"sin token", rutaPrevia(activo, ""), "", 401, "", nil},
		{"usuario no admin", rutaPrevia(activo, ""), x.usuario, 403, "", nil},
		{"activa", rutaPrevia(activo, ""), x.admin, 200, "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := x.get(tt.path, tt.token)
			if status != tt.status {
				t.Fatalf("status = %d, want %d (%v)", status, tt.status, body)
			}
			if tt.code != "" && body["error"] != tt.code {
				t.Errorf("error = %v, want %s", body["error"], tt.code)
			}
			if tt.detalle != nil && fmt.Sprint(body["detalle"]) != fmt.Sprint(tt.detalle) {
				t.Errorf("detalle = %v, want %v", body["detalle"], tt.detalle)
			}
		})
	}
}

func TestVistaPrevia_Contenido(t *testing.T) {
	x := nuevoFx(t)
	// 25 valid questions (lines 1–100) and 25 without key (lines 101–175).
	id := x.publicar(t, "balotario", bloques(25)+sinClave(26, 25), [][2]int{{1, 175}})

	status, body := x.get(rutaPrevia(id, ""), x.admin)
	if status != 200 {
		t.Fatalf("status = %d (%v)", status, body)
	}
	if got := fmt.Sprint(body["totales"]); got != "map[con_alertas:0 con_problemas:25 multiple:0 unica:25 validas:25]" {
		t.Errorf("totales = %s", got)
	}
	if n := len(body["preguntas"].([]any)); n != PreguntasVistaPrevia {
		t.Errorf("preguntas = %d, want %d", n, PreguntasVistaPrevia)
	}
	primera := body["preguntas"].([]any)[0].(map[string]any)
	if primera["numero"] != float64(1) || primera["linea_inicio"] != float64(1) || primera["linea_fin"] != float64(4) || primera["tipo"] != "unica" ||
		primera["enunciado"] != "Enunciado 1" || len(primera["alternativas"].([]any)) != 3 {
		t.Errorf("primera = %v", primera)
	}
	if body["ruta_revision"] != RutaVerificacionClave || body["importacion"] != nil || body["puede_confirmar"] != true || body["motivo_no_confirmable"] != nil {
		t.Errorf("ruta=%v importacion=%v puede=%v motivo=%v", body["ruta_revision"], body["importacion"], body["puede_confirmar"], body["motivo_no_confirmable"])
	}
	if fmt.Sprint(body["otras_importaciones"]) != "[]" {
		t.Errorf("otras_importaciones = %v", body["otras_importaciones"])
	}

	t.Run("los problemas se paginan y no llevan texto", func(t *testing.T) {
		_, body := x.get(rutaPrevia(id, "?page=3&per_page=10"), x.admin)
		p := body["problemas"].(map[string]any)
		data := p["data"].([]any)
		if p["total"] != float64(25) || p["page"] != float64(3) || p["per_page"] != float64(10) || len(data) != 5 {
			t.Fatalf("problemas = %v", p)
		}
		ultimo := data[4].(map[string]any)
		if ultimo["numero"] != float64(50) || ultimo["linea_inicio"] != float64(173) || ultimo["linea_fin"] != float64(175) ||
			fmt.Sprint(ultimo["motivos"]) != "[sin_clave]" || ultimo["enunciado"] != nil {
			t.Errorf("ultimo = %v", ultimo)
		}
		if _, body := x.get(rutaPrevia(id, "?page=9"), x.admin); len(body["problemas"].(map[string]any)["data"].([]any)) != 0 {
			t.Errorf("una página más allá del final debe ir vacía: %v", body["problemas"])
		}
		if _, body := x.get(rutaPrevia(id, "?per_page=500"), x.admin); body["problemas"].(map[string]any)["per_page"] != float64(PerPageMax) {
			t.Errorf("per_page debe topar en %d: %v", PerPageMax, body["problemas"])
		}
	})

	t.Run("nunca trae el archivo ni su huella", func(t *testing.T) {
		w := qt.Do(x.mux, "GET", rutaPrevia(id, ""), x.admin, "")
		for _, prohibido := range []string{"nombre_archivo", "secreto.md", "sha256", "contenido", "tamano_bytes"} {
			if strings.Contains(w.Body.String(), prohibido) {
				t.Errorf("la respuesta contiene %q", prohibido)
			}
		}
	})
}

func TestVistaPrevia_SinPasaje(t *testing.T) {
	x := nuevoFx(t)
	// The passages only cover the lines 1–6: question 1 (1–4) and question 2 (5–8, which overlaps).
	// Question 3 (9–12) has no passage.
	id := x.publicar(t, "balotario", bloques(3), [][2]int{{1, 6}})
	_, body := x.get(rutaPrevia(id, ""), x.admin)
	if got := fmt.Sprint(body["totales"]); got != "map[con_alertas:0 con_problemas:1 multiple:0 unica:2 validas:2]" {
		t.Errorf("totales = %s", got)
	}
	data := body["problemas"].(map[string]any)["data"].([]any)
	if len(data) != 1 || data[0].(map[string]any)["numero"] != float64(3) || fmt.Sprint(data[0].(map[string]any)["motivos"]) != "[sin_pasaje]" {
		t.Errorf("problemas = %v", data)
	}
}

func TestVistaPrevia_OficialEImportaciones(t *testing.T) {
	x := nuevoFx(t)
	id := x.publicar(t, "balotario", bloques(2), [][2]int{{1, 8}})
	if _, err := x.db.Exec(`UPDATE fuente_versiones SET oficial_verificada = true, oficial_entidad = 'MINSA', oficial_nota = 'ok', oficial_at = NOW()
		WHERE fuente_id = $1`, id); err != nil {
		t.Fatal(err)
	}
	var versionID int64
	if err := x.db.Get(&versionID, `SELECT version_vigente_id FROM fuentes WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}

	_, body := x.get(rutaPrevia(id, ""), x.admin)
	fuente := body["fuente"].(map[string]any)
	if body["ruta_revision"] != RutaEstructural || fuente["oficial_verificada"] != true || fuente["oficial_entidad"] != "MINSA" || fuente["version"] != float64(1) {
		t.Errorf("oficial: ruta=%v fuente=%v", body["ruta_revision"], fuente)
	}

	// An import of an older version only warns; an import of this version blocks the confirmation.
	t.Run("otras importaciones avisan", func(t *testing.T) {
		nueva := int64(0)
		if err := x.db.Get(&nueva, `INSERT INTO fuente_versiones (fuente_id, numero, nombre_archivo, sha256, tamano_bytes, lineas, contenido, estado)
			VALUES ($1, 2, 'v2.md', repeat('a', 64), 1, 1, 'x', 'lista') RETURNING id`, id); err != nil {
			t.Fatal(err)
		}
		insertarImportacion(t, x, id, nueva, 7)
		_, body := x.get(rutaPrevia(id, ""), x.admin)
		otras := body["otras_importaciones"].([]any)
		if len(otras) != 1 || otras[0].(map[string]any)["version"] != float64(2) || otras[0].(map[string]any)["creadas"] != float64(7) ||
			body["importacion"] != nil || body["puede_confirmar"] != true {
			t.Errorf("otras=%v importacion=%v puede=%v", otras, body["importacion"], body["puede_confirmar"])
		}
	})
	t.Run("ya importada bloquea la confirmacion", func(t *testing.T) {
		insertarImportacion(t, x, id, versionID, 2)
		_, body := x.get(rutaPrevia(id, ""), x.admin)
		imp := body["importacion"].(map[string]any)
		if imp["creadas"] != float64(2) || imp["id"] == nil || imp["created_at"] == nil ||
			body["puede_confirmar"] != false || body["motivo_no_confirmable"] != "ya_importado" {
			t.Errorf("importacion=%v puede=%v motivo=%v", imp, body["puede_confirmar"], body["motivo_no_confirmable"])
		}
	})
}

func TestVistaPrevia_SinValidas(t *testing.T) {
	x := nuevoFx(t)
	id := x.publicar(t, "balotario", sinClave(1, 3), [][2]int{{1, 9}})
	_, body := x.get(rutaPrevia(id, ""), x.admin)
	if body["puede_confirmar"] != false || body["motivo_no_confirmable"] != "sin_validas" || fmt.Sprint(body["preguntas"]) != "[]" {
		t.Errorf("puede=%v motivo=%v preguntas=%v", body["puede_confirmar"], body["motivo_no_confirmable"], body["preguntas"])
	}
}

// insertarImportacion registers an import of a version, as the confirmation does.
func insertarImportacion(t *testing.T, x *fx, fuenteID, versionID int64, creadas int) {
	t.Helper()
	if _, err := x.db.Exec(`INSERT INTO balotario_importaciones (fuente_id, fuente_version_id, proceso_id, especialidad_id, asignatura_id, creadas, con_problemas)
		VALUES ($1, $2, $3, $4, $5, $6, 0)`, fuenteID, versionID, x.f.ProcesoID, x.f.EspecialidadID, x.f.Asignaturas[0], creadas); err != nil {
		t.Fatal(err)
	}
}
