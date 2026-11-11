package ia_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/cbaciliod/learning/internal/ia"
)

type filaUso struct {
	Operacion string  `db:"operacion"`
	ActorRol  string  `db:"actor_rol"`
	ActorID   *int64  `db:"actor_id"`
	Proveedor string  `db:"proveedor"`
	Modelo    *string `db:"modelo"`
	Tokens    int     `db:"tokens_entrada"`
	Intentos  int     `db:"intentos"`
	Resultado string  `db:"resultado"`
	Cache     bool    `db:"desde_cache"`
}

func usosDesde(t *testing.T, db *sqlx.DB, desde int64) []filaUso {
	t.Helper()
	var filas []filaUso
	if err := db.Select(&filas, `SELECT operacion, actor_rol, actor_id, proveedor, modelo, tokens_entrada, intentos,
		resultado, desde_cache FROM ia_usos WHERE id > $1 ORDER BY id`, desde); err != nil {
		t.Fatal(err)
	}
	return filas
}

// A4: one ia_usos row per invocation, with its result, for every outcome,
// and for an admin actor (FK to usuarios) and the system.
func TestPgRegistro_UnaFilaPorInvocacion(t *testing.T) {
	db := dbtest.Open(t)
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	var desde int64
	if err := db.Get(&desde, `SELECT COALESCE(MAX(id), 0) FROM ia_usos`); err != nil {
		t.Fatal(err)
	}
	// Rows go before the seeded user (FK) on cleanup.
	t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM ia_usos WHERE id > $1`, desde) })

	reg := ia.NewPgRegistro(db)
	ctx := context.Background()
	in := ia.EntradaClasificar{Items: []ia.ItemTexto{{ID: 1, Texto: "Vacunas"}},
		Taxonomia: []ia.OpcionTaxonomia{{AsignaturaID: 3, Nombre: "Salud"}}}
	actorAdmin := ia.Actor{Rol: ia.RolAdmin, ID: &admin.ID}
	sistema := ia.Actor{Rol: ia.RolSistema}

	ok := ia.NewServicio(ia.NewDoble(), reg)
	if _, err := ok.Clasificar(ctx, actorAdmin, in); err != nil {
		t.Fatal(err)
	}
	if _, err := ia.NewServicio(nil, reg).Clasificar(ctx, sistema, in); !errors.Is(err, ia.ErrNoDisponible) {
		t.Fatal(err)
	}
	pii := in
	pii.Items = []ia.ItemTexto{{ID: 1, Texto: "persona@example.com"}}
	if _, err := ok.Clasificar(ctx, sistema, pii); !errors.Is(err, ia.ErrDatosPersonales) {
		t.Fatal(err)
	}
	// Timeout with the consumer's ctx already done: the row is still written.
	lento := ia.NewServicio(ia.NewDoble(ia.Lento()), reg, ia.ConTiempoLlamada(20*time.Millisecond))
	cctx, cancel := context.WithTimeout(ctx, 30*time.Millisecond)
	defer cancel()
	if _, err := lento.Clasificar(cctx, sistema, in); !errors.Is(err, ia.ErrTimeout) {
		t.Fatal(err)
	}

	filas := usosDesde(t, db, desde)
	want := []struct {
		rol, proveedor, resultado string
		intentos                  int
	}{
		{ia.RolAdmin, ia.NombreDoble, ia.ResultadoOK, 1},
		{ia.RolSistema, "", ia.ResultadoNoDisponible, 0},
		{ia.RolSistema, ia.NombreDoble, ia.ResultadoDatosPersonales, 0},
		{ia.RolSistema, ia.NombreDoble, ia.ResultadoTimeout, 1},
	}
	if len(filas) != len(want) {
		t.Fatalf("%d rows, want %d: %+v", len(filas), len(want), filas)
	}
	for i, w := range want {
		f := filas[i]
		if f.Operacion != ia.OpClasificar || f.ActorRol != w.rol || f.Proveedor != w.proveedor ||
			f.Resultado != w.resultado || f.Intentos != w.intentos || f.Cache {
			t.Errorf("row %d: %+v, want %+v", i, f, w)
		}
	}
	if filas[0].ActorID == nil || *filas[0].ActorID != admin.ID || filas[0].Tokens <= 0 || filas[0].Modelo == nil {
		t.Errorf("ok row: %+v", filas[0])
	}
	if filas[1].ActorID != nil || filas[1].Modelo != nil {
		t.Errorf("system row: %+v", filas[1])
	}
}

// ck_ia_usos_actor backs the service's actor check.
func TestMigracion066_RestriccionDelActor(t *testing.T) {
	db := dbtest.Open(t)
	for _, q := range []string{
		`INSERT INTO ia_usos (operacion, actor_rol, actor_id, proveedor, duracion_ms, resultado) VALUES ('clasificar', 'admin', NULL, 'doble', 0, 'ok')`,
		`INSERT INTO ia_usos (operacion, actor_rol, proveedor, duracion_ms, resultado) VALUES ('otra', 'sistema', 'doble', 0, 'ok')`,
		`INSERT INTO ia_usos (operacion, actor_rol, proveedor, duracion_ms, intentos, resultado) VALUES ('clasificar', 'sistema', 'doble', 0, 7, 'ok')`,
		`INSERT INTO ia_usos (operacion, actor_rol, proveedor, duracion_ms, resultado) VALUES ('clasificar', 'sistema', 'doble', 0, 'cancelado')`,
	} {
		tx, err := db.Beginx()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(q); err == nil {
			t.Errorf("accepted: %s", q)
		}
		_ = tx.Rollback()
	}
}
