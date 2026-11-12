package ia_test

import (
	"context"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/cbaciliod/learning/internal/ia"
)

func entornoCache(t *testing.T) *sqlx.DB {
	t.Helper()
	db := dbtest.Open(t)
	limpiar := func() {
		if _, err := db.Exec(`DELETE FROM ia_cache`); err != nil {
			t.Errorf("limpiar la caché: %v", err)
		}
	}
	limpiar()
	t.Cleanup(limpiar)
	return db
}

// fuenteActiva inserts a source in estado activa with one ready version, and
// removes everything after the test. It returns the source id.
func fuenteActiva(t *testing.T, db *sqlx.DB) int64 {
	t.Helper()
	var id int64
	if err := db.Get(&id, `INSERT INTO fuentes (titulo, tipo) VALUES ('Fuente de la caché '||$1::text, 'guia') RETURNING id`, dbtest.Uniq()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`UPDATE fuentes SET estado = 'procesando', version_vigente_id = NULL, motivo_retiro = NULL WHERE id = $1`, id)
		_, _ = db.Exec(`DELETE FROM fuente_versiones WHERE fuente_id = $1`, id)
		_, _ = db.Exec(`DELETE FROM fuentes WHERE id = $1`, id)
	})
	activarVersion(t, db, id, nuevaVersion(t, db, id))
	return id
}

// nuevaVersion adds a ready version to the source and returns its id.
func nuevaVersion(t *testing.T, db *sqlx.DB, fuenteID int64) int64 {
	t.Helper()
	var v int64
	if err := db.Get(&v, `INSERT INTO fuente_versiones (fuente_id, numero, estado, nombre_archivo, sha256, tamano_bytes, lineas, contenido)
		VALUES ($1, (SELECT COALESCE(MAX(numero), 0) + 1 FROM fuente_versiones WHERE fuente_id = $1), 'lista', 'f.txt',
		        encode(sha256(($2 || clock_timestamp()::text)::bytea), 'hex'), 1, 1, 'texto')
		RETURNING id`, fuenteID, dbtest.Uniq()); err != nil {
		t.Fatal(err)
	}
	return v
}

// activarVersion makes version the current one of an active source.
func activarVersion(t *testing.T, db *sqlx.DB, fuenteID, version int64) {
	t.Helper()
	if _, err := db.Exec(`UPDATE fuentes SET estado = 'activa', version_vigente_id = $2 WHERE id = $1`, fuenteID, version); err != nil {
		t.Fatal(err)
	}
}

func buscar(t *testing.T, c *ia.PgCache, clave string) (string, bool) {
	t.Helper()
	raw, ok, err := c.Buscar(context.Background(), clave)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw), ok
}

func guardar(t *testing.T, c *ia.PgCache, clave, salida string, fuentes ...int64) {
	t.Helper()
	if err := c.Guardar(context.Background(), clave, ia.OpClasificar, []byte(salida), fuentes); err != nil {
		t.Fatal(err)
	}
}

func filasCache(t *testing.T, db *sqlx.DB) (n int) {
	t.Helper()
	if err := db.Get(&n, `SELECT COUNT(*) FROM ia_cache`); err != nil {
		t.Fatal(err)
	}
	return n
}

// Round trip with and without sources; the entry keeps the first writer's output.
func TestPgCache_GuardarYBuscar(t *testing.T) {
	db := entornoCache(t)
	c := ia.NewPgCache(db)
	if _, ok := buscar(t, c, "k1"); ok {
		t.Fatal("hit on an empty cache")
	}
	guardar(t, c, "k1", `{"a":1}`)
	if raw, ok := buscar(t, c, "k1"); !ok || raw != `{"a": 1}` { // JSONB normalizes the spacing
		t.Errorf("got %q, %v", raw, ok)
	}
	guardar(t, c, "k1", `{"a":2}`)
	if raw, _ := buscar(t, c, "k1"); raw != `{"a": 1}` {
		t.Errorf("the second writer replaced the entry: %q", raw)
	}
	if n := filasCache(t, db); n != 1 {
		t.Errorf("%d rows, want 1", n)
	}
}

// Escenario «Invalidación»: a new current version, or a retired source, makes
// the entry stale; the stale entry is deleted.
func TestPgCache_InvalidacionPorFuente(t *testing.T) {
	tests := []struct {
		name    string
		cambiar string
		stale   bool
	}{
		{"nothing changed", ``, false},
		{"new current version", `UPDATE fuentes SET version_vigente_id = (SELECT id FROM fuente_versiones WHERE fuente_id = $1 AND id <> version_vigente_id LIMIT 1) WHERE id = $1`, true},
		{"source retired", `UPDATE fuentes SET estado = 'retirada', motivo_retiro = 'Obsoleta' WHERE id = $1`, true},
		{"source back to processing", `UPDATE fuentes SET estado = 'procesando', version_vigente_id = NULL WHERE id = $1`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := entornoCache(t)
			c := ia.NewPgCache(db)
			f1, f2 := fuenteActiva(t, db), fuenteActiva(t, db)
			nuevaVersion(t, db, f2) // a second version to switch to
			guardar(t, c, "k", `{"x":true}`, f1, f2)
			if tt.cambiar != "" {
				if _, err := db.Exec(tt.cambiar, f2); err != nil {
					t.Fatal(err)
				}
			}
			if _, ok := buscar(t, c, "k"); ok == tt.stale {
				t.Errorf("hit = %v, want %v", ok, !tt.stale)
			}
			if n := filasCache(t, db); (n == 0) != tt.stale {
				t.Errorf("%d rows left, stale entry should be deleted = %v", n, tt.stale)
			}
		})
	}
}

// The entry pins the CURRENT version when written, not the version of the
// content: an explanation over a passage of an older cited version is valid
// as long as the current version does not change (contract §4).
func TestPgCache_FijaLaVersionVigenteAlEscribir(t *testing.T) {
	db := entornoCache(t)
	c := ia.NewPgCache(db)
	f := fuenteActiva(t, db)
	vieja := nuevaVersion(t, db, f) // an older version that a citation may point to
	var vigente int64
	if err := db.Get(&vigente, `SELECT version_vigente_id FROM fuentes WHERE id = $1`, f); err != nil || vigente == vieja {
		t.Fatalf("setup: current %d, older %d (%v)", vigente, vieja, err)
	}
	guardar(t, c, "expl", `{"e":1}`, f)
	var vigentes []int64
	if err := db.Select(&vigentes, `SELECT unnest(vigente_ids) FROM ia_cache WHERE clave = 'expl'`); err != nil || len(vigentes) != 1 || vigentes[0] != vigente {
		t.Fatalf("pinned versions = %v (%v), want the current one [%d]", vigentes, err, vigente)
	}
	if _, ok := buscar(t, c, "expl"); !ok {
		t.Error("an entry over an older cited version was born stale")
	}
}

// A source that is not activa now cannot be pinned: nothing is stored.
func TestPgCache_NoGuardaConUnaFuenteNoActiva(t *testing.T) {
	db := entornoCache(t)
	c := ia.NewPgCache(db)
	f := fuenteActiva(t, db)
	if _, err := db.Exec(`UPDATE fuentes SET estado = 'retirada', motivo_retiro = 'x' WHERE id = $1`, f); err != nil {
		t.Fatal(err)
	}
	guardar(t, c, "k", `{}`, f)
	guardar(t, c, "k2", `{}`, 99999999) // unknown source
	if n := filasCache(t, db); n != 0 {
		t.Errorf("%d rows stored over inactive sources", n)
	}
}

// The entry lives 30 days, to the instant (contract §4).
func TestPgCache_Vence(t *testing.T) {
	db := entornoCache(t)
	t0 := time.Date(2026, 11, 12, 15, 0, 0, 0, time.UTC)
	escribe := ia.NewPgCache(db, ia.ConRelojCache(func() time.Time { return t0 }))
	guardar(t, escribe, "k", `{}`)
	tests := []struct {
		name string
		at   time.Time
		want bool
	}{
		{"a second before 30 days", t0.Add(30*24*time.Hour - time.Second), true},
		{"at 30 days exactly", t0.Add(30 * 24 * time.Hour), false},
		{"after 30 days", t0.Add(31 * 24 * time.Hour), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guardar(t, escribe, "k", `{}`) // re-insert (no-op if it still exists)
			lee := ia.NewPgCache(db, ia.ConRelojCache(func() time.Time { return tt.at }))
			if _, ok := buscar(t, lee, "k"); ok != tt.want {
				t.Errorf("hit = %v, want %v", ok, tt.want)
			}
		})
	}
}

// Each write purges up to 100 expired entries.
func TestPgCache_PurgaVencidas(t *testing.T) {
	db := entornoCache(t)
	if _, err := db.Exec(`INSERT INTO ia_cache (clave, operacion, salida, expira_en)
		SELECT lpad(g::text, 64, '0'), 'clasificar', '{}', NOW() - INTERVAL '1 day' FROM generate_series(1, 150) g`); err != nil {
		t.Fatal(err)
	}
	guardar(t, ia.NewPgCache(db), "nueva", `{}`)
	if n := filasCache(t, db); n != 51 {
		t.Errorf("%d entries left, want 51 (150 expired - 100 purged + the new one)", n)
	}
}

// Whole path with the real stores: a good call is cached, the second one is a
// hit that gives back its action, and a source change makes it a miss again.
func TestServicioConCacheReal(t *testing.T) {
	db := entornoCache(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	var desde int64
	if err := db.Get(&desde, `SELECT COALESCE(MAX(id), 0) FROM ia_usos`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM ia_usos WHERE id > $1`, desde) })
	f := fuenteActiva(t, db)

	costos := nuevoPgCostos(t, db, reloj(mediodia))
	s := ia.NewServicio(ia.NewDoble(), ia.NewPgRegistro(db), costos, ia.ConCache(ia.NewPgCache(db)))
	a := ia.Actor{Rol: ia.RolAdmin, ID: &admin.ID}
	in := ia.EntradaClasificar{Items: []ia.ItemTexto{{ID: 1, Texto: "Vacunas"}}, Taxonomia: []ia.OpcionTaxonomia{{AsignaturaID: 3, Nombre: "Salud"}}, Fuentes: []int64{f}}

	for i, want := range []bool{false, true} {
		r, err := costos.Reservar(context.Background(), a, 1)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Clasificar(context.Background(), &r, in); err != nil {
			t.Fatal(err)
		}
		filas := usosDesde(t, db, desde)
		if got := filas[len(filas)-1].Cache; got != want {
			t.Errorf("call %d: desde_cache = %v, want %v", i+1, got, want)
		}
	}
	if consumo, _ := costos.Consumo(context.Background(), a); consumo.Dia.Usados != 1 {
		t.Errorf("usados = %d, want 1 (the hit gave its action back)", consumo.Dia.Usados)
	}
	activarVersion(t, db, f, nuevaVersion(t, db, f))
	r, _ := costos.Reservar(context.Background(), a, 1)
	if _, err := s.Clasificar(context.Background(), &r, in); err != nil {
		t.Fatal(err)
	}
	if filas := usosDesde(t, db, desde); filas[len(filas)-1].Cache {
		t.Error("a new current version still served the cached output")
	}
}
