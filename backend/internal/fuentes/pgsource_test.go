package fuentes_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/audit"
	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/cbaciliod/learning/internal/fuentes"
	"github.com/cbaciliod/learning/internal/fuentes/fuentestest"
	"github.com/jmoiron/sqlx"
)

// entornoPg builds the Postgres adapter over the test DB with two real
// especialidades (FKs) and the two admin hooks, done in SQL as S2/S5 will do.
// Every fuente created by the test is removed at the end.
func entornoPg(t *testing.T) (fuentestest.Entorno, *sqlx.DB) {
	t.Helper()
	db := dbtest.Open(t)
	cat := dbtest.SeedCatalog(t, db)
	var otra int64
	u := dbtest.Uniq()
	if err := db.QueryRow(`INSERT INTO especialidades (institucion_id, nombre, codigo) VALUES ($1, $2, $3) RETURNING id`,
		cat.InstitucionID, "Esp "+u, "S"+u).Scan(&otra); err != nil {
		t.Fatal(err)
	}
	var desde int64
	if err := db.Get(&desde, `SELECT COALESCE(MAX(id), 0) FROM fuentes`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { limpiarFuentes(t, db, desde) })

	// Contract hooks, in SQL as S5 and S2 will do (E2-06); not in the port.
	retirar := func(t *testing.T, fuenteID int64) {
		t.Helper()
		res, err := db.Exec(`UPDATE fuentes SET estado = 'retirada', motivo_retiro = 'prueba de contrato', updated_at = NOW()
			WHERE id = $1 AND version_vigente_id IS NOT NULL`, fuenteID)
		if err != nil {
			t.Fatalf("Retirar: %v", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			t.Fatalf("Retirar: fuente %d has no current version", fuenteID)
		}
	}
	nuevaVersion := func(t *testing.T, fuenteID int64, contenido []byte) int {
		t.Helper()
		sha, tamano, lineas := fuentes.Huella(contenido)
		var n int
		if err := db.Get(&n, `
			INSERT INTO fuente_versiones (fuente_id, numero, nombre_archivo, sha256, tamano_bytes, lineas, contenido)
			SELECT $1, COALESCE(MAX(numero), 0) + 1, 'nueva.md', $2, $3, $4, $5 FROM fuente_versiones WHERE fuente_id = $1
			RETURNING numero`, fuenteID, sha, tamano, lineas, string(contenido)); err != nil {
			t.Fatalf("NuevaVersion: %v", err)
		}
		return n
	}
	return fuentestest.Entorno{Repo: fuentes.NewPgSource(db), EspecialidadA: cat.EspecialidadID, EspecialidadB: otra,
		Retirar: retirar, NuevaVersion: nuevaVersion}, db
}

// limpiarFuentes removes the fuentes above desde; the current version is
// unlinked first because fuentes and fuente_versiones reference each other.
func limpiarFuentes(t *testing.T, db *sqlx.DB, desde int64) {
	for _, q := range []string{
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

// Escenario «Intercambiabilidad»: the own adapter passes the contract that the
// in-memory adapter passes (fuentestest/memoria_test.go).
func TestContrato_Postgres(t *testing.T) {
	fuentestest.RunContract(t, func(t *testing.T) fuentestest.Entorno {
		e, _ := entornoPg(t)
		return e
	})
}

// publicar stores and publishes one fuente with the given passage texts.
func publicar(t *testing.T, e fuentestest.Entorno, aud fuentes.Auditor, textos ...string) (int64, error) {
	t.Helper()
	ctx := context.Background()
	id, v, err := e.Repo.GuardarFuente(ctx, fuentes.NuevaFuente{Titulo: "Fuente pg " + dbtest.Uniq(), Tipo: "guia",
		NombreArchivo: "pg.md", Contenido: []byte("pg " + dbtest.Uniq())}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ps := make([]fuentes.PasajeNuevo, len(textos))
	for i, tx := range textos {
		ps[i] = fuentes.PasajeNuevo{Orden: i + 1, LineaInicio: 1, LineaFin: 1, Texto: tx, Palabras: len(strings.Fields(tx))}
	}
	return id, e.Repo.PublicarVersion(ctx, id, v, ps, aud)
}

// The Postgres search folds case and accents in Go (NFC and NFD), stems in
// Spanish, and never fails on arbitrary input (ADR 0003 § Recuperación).
func TestBuscarPasajes_FTSPostgres(t *testing.T) {
	e, db := entornoPg(t)
	mk := fmt.Sprintf("mk%s9", dbtest.Uniq())
	id, err := publicar(t, e, nil, "Vigilancia Epidemiológica de brotes "+mk, "Año de la salud "+mk)
	if err != nil {
		t.Fatal(err)
	}
	esp := e.EspecialidadA
	cases := []struct {
		name, consulta string
		want           int
	}{
		{"sin tildes ni mayúsculas", "vigilancia epidemiologica " + mk, 1},
		{"plural (stemming)", "vigilancias epidemiológicas " + mk, 1},
		{"tilde en NFD", "epidemiolo\u0301gica " + mk, 1},
		{"la ñ se distingue", "ano " + mk, 0},
		{"la ñ en NFD", "an\u0303o " + mk, 1},
		{"solo stopwords: lista vacía", "de la los", 0},
		// An open quote turns the rest into a phrase: only "no error" is required.
		{"sintaxis arbitraria no falla", `"` + mk + ` -brotes or (`, -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := e.Repo.BuscarPasajes(context.Background(), fuentes.Busqueda{Consulta: c.consulta, EspecialidadID: esp})
			if err != nil || (c.want >= 0 && len(got) != c.want) {
				t.Errorf("got %d hits, %v; want %d", len(got), err, c.want)
			}
		})
	}
	// The original text is kept as is; only tsv is folded.
	var textoGuardado, tsv string
	if err := db.QueryRow(`SELECT p.texto, p.tsv::text FROM pasajes p JOIN fuente_versiones v ON v.id = p.fuente_version_id
		WHERE v.fuente_id = $1 AND p.orden = 1`, id).Scan(&textoGuardado, &tsv); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(textoGuardado, "Vigilancia Epidemiológica") || strings.Contains(tsv, "ó") || !strings.Contains(tsv, "epidemiolog") {
		t.Errorf("texto = %q, tsv = %s", textoGuardado, tsv)
	}
}

// The adapter builds the audit event inside its transaction (ADR 0003 §
// Auditoría en el puerto): fuente.crear with the new id, fuente.procesar and
// fuente.fallar. When the Auditor fails, nothing is created, published or
// failed.
func TestAuditoriaEnLaTransaccion(t *testing.T) {
	e, db := entornoPg(t)
	ctx := context.Background()
	falla := func(context.Context, sqlx.ExecerContext, fuentes.Evento) error { return errors.New("audit caído") }
	aud := audit.New(db)
	sistema := func(ctx context.Context, tx sqlx.ExecerContext, ev fuentes.Evento) error {
		return aud.LogTx(ctx, tx, nil, ev.Accion, ev.Entidad, &ev.EntidadID, ev.Detalle, "", "")
	}
	nf := func() fuentes.NuevaFuente {
		return fuentes.NuevaFuente{Titulo: "Auditada " + dbtest.Uniq(), Tipo: "norma", EspecialidadIDs: []int64{e.EspecialidadB, e.EspecialidadA},
			NombreArchivo: "a.md", Contenido: []byte("auditada " + dbtest.Uniq())}
	}
	var desde int64
	if err := db.Get(&desde, `SELECT COALESCE(MAX(id), 0) FROM audit_log`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM audit_log WHERE id > $1 AND entidad = 'fuente'`, desde) })

	// GuardarFuente: a failing Auditor creates nothing.
	var antes int
	if err := db.Get(&antes, `SELECT COUNT(*) FROM fuentes`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.Repo.GuardarFuente(ctx, nf(), falla); err == nil {
		t.Fatal("creating with a failing audit must fail")
	}
	var despues int
	if err := db.Get(&despues, `SELECT COUNT(*) FROM fuentes`); err != nil || despues != antes {
		t.Fatalf("fuentes %d → %d after a failed audit", antes, despues)
	}

	// fuente.crear carries the new id and its detalle.
	f := nf()
	id, _, err := e.Repo.GuardarFuente(ctx, f, sistema)
	if err != nil {
		t.Fatal(err)
	}
	detalle := detalleAudit(t, db, desde, fuentes.AccionCrear, id)
	sha, tamano, _ := fuentes.Huella(f.Contenido)
	esps := fmt.Sprint([]any{float64(f.EspecialidadIDs[0]), float64(f.EspecialidadIDs[1])})
	if detalle["version"] != 1.0 || detalle["sha256"] != sha || detalle["tamano_bytes"] != float64(tamano) ||
		detalle["tipo"] != "norma" || fmt.Sprint(detalle["especialidad_ids"]) != esps || detalle["estado_nuevo"] != "procesando" {
		t.Errorf("fuente.crear detalle = %v", detalle)
	}

	// PublicarVersion: a failing Auditor publishes nothing.
	ps := []fuentes.PasajeNuevo{{Orden: 1, LineaInicio: 1, LineaFin: 1, Texto: "publicado " + dbtest.Uniq(), Palabras: 2}}
	if err := e.Repo.PublicarVersion(ctx, id, 1, ps, falla); err == nil {
		t.Fatal("publishing with a failing audit must fail")
	}
	var estado string
	var pasajes int
	if err := db.QueryRow(`SELECT v.estado, (SELECT COUNT(*) FROM pasajes WHERE fuente_version_id = v.id)
		FROM fuente_versiones v WHERE v.fuente_id = $1 AND v.numero = 1`, id).Scan(&estado, &pasajes); err != nil {
		t.Fatal(err)
	}
	if estado != fuentes.EstadoProcesando || pasajes != 0 {
		t.Fatalf("after a failed audit: version %s with %d passages", estado, pasajes)
	}
	if err := e.Repo.PublicarVersion(ctx, id, 1, ps, sistema); err != nil {
		t.Fatal(err)
	}
	detalle = detalleAudit(t, db, desde, fuentes.AccionProcesar, id)
	if detalle["version"] != 1.0 || detalle["pasajes_count"] != 1.0 || detalle["estado_anterior"] != "procesando" || detalle["estado_nuevo"] != "activa" {
		t.Errorf("fuente.procesar detalle = %v", detalle)
	}

	// MarcarVersionFallida on a new version: the fuente keeps 'activa'.
	n := e.NuevaVersion(t, id, []byte("v2 "+dbtest.Uniq()))
	if err := e.Repo.MarcarVersionFallida(ctx, id, n, fuentes.MotivoErrorInterno, falla); err == nil {
		t.Fatal("failing with a failing audit must fail")
	}
	if err := e.Repo.MarcarVersionFallida(ctx, id, n, fuentes.MotivoErrorInterno, sistema); err != nil {
		t.Fatal(err)
	}
	detalle = detalleAudit(t, db, desde, fuentes.AccionFallar, id)
	if detalle["version"] != float64(n) || detalle["motivo_fallo"] != "error_interno" || detalle["estado_anterior"] != "activa" || detalle["estado_nuevo"] != "activa" {
		t.Errorf("fuente.fallar detalle = %v", detalle)
	}
}

// detalleAudit returns the detalle of the only row of accion for the fuente
// written after desde, by the system (usuario_id NULL).
func detalleAudit(t *testing.T, db *sqlx.DB, desde int64, accion string, fuenteID int64) map[string]any {
	t.Helper()
	var raw [][]byte
	if err := db.Select(&raw, `SELECT detalle FROM audit_log WHERE id > $1 AND accion = $2 AND entidad = 'fuente'
		AND entidad_id = $3 AND usuario_id IS NULL`, desde, accion, fuenteID); err != nil {
		t.Fatal(err)
	}
	if len(raw) != 1 {
		t.Fatalf("%s: %d audit rows, want 1", accion, len(raw))
	}
	var d map[string]any
	if err := json.Unmarshal(raw[0], &d); err != nil {
		t.Fatal(err)
	}
	return d
}

// estado_anterior is the fuente's state when the worker's transaction gets
// the fuente, not a snapshot taken before waiting for its lock: another
// transaction holds the fuente, changes its state and commits while the
// worker waits (READ COMMITTED would otherwise audit the stale state).
func TestEstadoAnteriorTrasEsperarElBloqueo(t *testing.T) {
	cases := []struct {
		name   string
		accion string
		correr func(ctx context.Context, repo *fuentes.PgSource, id int64, n int, aud fuentes.Auditor) error
	}{
		{"publicar", fuentes.AccionProcesar, func(ctx context.Context, repo *fuentes.PgSource, id int64, n int, aud fuentes.Auditor) error {
			ps := []fuentes.PasajeNuevo{{Orden: 1, LineaInicio: 1, LineaFin: 1, Texto: "tras el bloqueo " + dbtest.Uniq(), Palabras: 3}}
			return repo.PublicarVersion(ctx, id, n, ps, aud)
		}},
		{"fallar", fuentes.AccionFallar, func(ctx context.Context, repo *fuentes.PgSource, id int64, n int, aud fuentes.Auditor) error {
			return repo.MarcarVersionFallida(ctx, id, n, fuentes.MotivoErrorInterno, aud)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, db := entornoPg(t)
			ctx := context.Background()
			aud := audit.New(db)
			sistema := func(ctx context.Context, tx sqlx.ExecerContext, ev fuentes.Evento) error {
				return aud.LogTx(ctx, tx, nil, ev.Accion, ev.Entidad, &ev.EntidadID, ev.Detalle, "", "")
			}
			var desde int64
			if err := db.Get(&desde, `SELECT COALESCE(MAX(id), 0) FROM audit_log`); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM audit_log WHERE id > $1 AND entidad = 'fuente'`, desde) })
			id, err := publicar(t, e, nil, "vigente "+dbtest.Uniq())
			if err != nil {
				t.Fatal(err)
			}
			n := e.NuevaVersion(t, id, []byte("en proceso "+dbtest.Uniq()))

			otra, err := db.Beginx()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = otra.Rollback() }()
			if _, err := otra.Exec(`SELECT 1 FROM fuentes WHERE id = $1 FOR UPDATE`, id); err != nil {
				t.Fatal(err)
			}
			if _, err := otra.Exec(`UPDATE fuentes SET estado = 'retirada', motivo_retiro = 'mientras espera' WHERE id = $1`, id); err != nil {
				t.Fatal(err)
			}
			pid := dbtest.BackendPID(t, otra)
			resultado := make(chan int, 1)
			errs := make(chan error, 1)
			go func() {
				errs <- c.correr(ctx, e.Repo.(*fuentes.PgSource), id, n, sistema)
				resultado <- 1
			}()
			if !dbtest.BlockedBy(t, db, pid, resultado) {
				t.Fatal("the worker did not wait for the fuente's lock")
			}
			if err := otra.Commit(); err != nil {
				t.Fatal(err)
			}
			<-resultado
			if err := <-errs; err != nil {
				t.Fatal(err)
			}
			d := detalleAudit(t, db, desde, c.accion, id)
			if d["estado_anterior"] != fuentes.EstadoRetirada || d["estado_nuevo"] != fuentes.EstadoRetirada {
				t.Errorf("%s detalle = %v, want retirada → retirada", c.accion, d)
			}
		})
	}
}
