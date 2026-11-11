package fuentes_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

	retirar := func(ctx context.Context, fuenteID int64) error {
		res, err := db.ExecContext(ctx, `UPDATE fuentes SET estado = 'retirada', motivo_retiro = 'prueba de contrato'
			WHERE id = $1 AND version_vigente_id IS NOT NULL`, fuenteID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fuentes.ErrNoEncontrado
		}
		return nil
	}
	nuevaVersion := func(ctx context.Context, fuenteID int64, contenido []byte) (int, error) {
		suma := sha256.Sum256(contenido)
		var n int
		err := db.GetContext(ctx, &n, `
			INSERT INTO fuente_versiones (fuente_id, numero, nombre_archivo, sha256, tamano_bytes, lineas, contenido)
			SELECT $1, COALESCE(MAX(numero), 0) + 1, 'nueva.md', $2, $3, $4, $5 FROM fuente_versiones WHERE fuente_id = $1
			RETURNING numero`, fuenteID, hex.EncodeToString(suma[:]), len(contenido), fuentes.ContarLineas(contenido), string(contenido))
		if err != nil && strings.Contains(err.Error(), "uq_fuente_versiones_sha256") {
			return 0, fuentes.ErrFuenteDuplicada
		}
		return n, err
	}
	return fuentestest.Entorno{Repo: fuentes.NewPgSource(db), Retirar: retirar, NuevaVersion: nuevaVersion,
		Especialidades: [2]int64{cat.EspecialidadID, otra}}, db
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
func publicar(t *testing.T, e fuentestest.Entorno, auditFn audit.TxFunc, textos ...string) (int64, error) {
	t.Helper()
	ctx := context.Background()
	id, v, err := e.Repo.GuardarFuente(ctx, fuentes.NuevaFuente{Titulo: "Fuente pg " + dbtest.Uniq(), Tipo: "guia",
		NombreArchivo: "pg.md", Contenido: []byte("pg " + dbtest.Uniq())})
	if err != nil {
		t.Fatal(err)
	}
	ps := make([]fuentes.PasajeNuevo, len(textos))
	for i, tx := range textos {
		ps[i] = fuentes.PasajeNuevo{Orden: i + 1, LineaInicio: 1, LineaFin: 1, Texto: tx, Palabras: len(strings.Fields(tx))}
	}
	return id, e.Repo.PublicarVersion(ctx, id, v, ps, auditFn)
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
	esp := e.Especialidades[0]
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

// PublicarVersion and MarcarVersionFallida write the audit row in their
// transaction: when it fails nothing is published or failed.
func TestPublicacion_AuditoriaEnLaTransaccion(t *testing.T) {
	e, db := entornoPg(t)
	ctx := context.Background()
	falla := func(context.Context, sqlx.ExecerContext) error { return errors.New("audit caído") }

	id, err := publicar(t, e, falla, "texto que no debe publicarse "+dbtest.Uniq())
	if err == nil {
		t.Fatal("publishing with a failing audit must fail")
	}
	var estado string
	var pasajes int
	if err := db.QueryRow(`SELECT v.estado, (SELECT COUNT(*) FROM pasajes WHERE fuente_version_id = v.id)
		FROM fuente_versiones v WHERE v.fuente_id = $1 AND v.numero = 1`, id).Scan(&estado, &pasajes); err != nil {
		t.Fatal(err)
	}
	if estado != fuentes.EstadoProcesando || pasajes != 0 {
		t.Errorf("after a failed audit: version %s with %d passages, want procesando with 0", estado, pasajes)
	}
	if err := e.Repo.MarcarVersionFallida(ctx, id, 1, fuentes.MotivoErrorInterno, falla); err == nil {
		t.Error("failing a version with a failing audit must fail")
	}

	// A working audit (actor NULL = system, as the worker) is written.
	aud := audit.New(db)
	ok := func(ctx context.Context, tx sqlx.ExecerContext) error {
		return aud.LogTx(ctx, tx, nil, "fuente.procesar", "fuente", &id, map[string]any{"version": 1}, "", "")
	}
	ps := []fuentes.PasajeNuevo{{Orden: 1, LineaInicio: 1, LineaFin: 1, Texto: "publicado " + dbtest.Uniq(), Palabras: 2}}
	if err := e.Repo.PublicarVersion(ctx, id, 1, ps, ok); err != nil {
		t.Fatal(err)
	}
	var auditados int
	if err := db.Get(&auditados, `SELECT COUNT(*) FROM audit_log WHERE accion = 'fuente.procesar' AND entidad_id = $1 AND usuario_id IS NULL`, id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM audit_log WHERE accion = 'fuente.procesar' AND entidad_id = $1`, id)
	})
	if auditados != 1 {
		t.Errorf("%d audit rows, want 1", auditados)
	}
}
