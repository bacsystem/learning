package ia_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/db/dbtest"
)

const migracion068 = "../../migrations/068_create_ia_control_costos.sql"

// seccionesGoose splits a goose file into its Up and Down SQL.
func seccionesGoose(t *testing.T, ruta string) (up, down string) {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	_, resto, ok := strings.Cut(string(b), "-- +goose Up")
	if !ok {
		t.Fatal("no Up section")
	}
	up, down, ok = strings.Cut(resto, "-- +goose Down")
	if !ok {
		t.Fatal("no Down section")
	}
	return up, down
}

// Escenario «Migración reversible»: the Down of 068 drops the four tables,
// maps presupuesto_agotado to no_disponible and brings back the unnamed CHECK
// of 066 (ia_usos_resultado_check); the Up again recreates everything with the
// default configuration row. It runs in a transaction that is rolled back, so
// the shared test database stays at 068.
func TestMigracion068_DownYUp(t *testing.T) {
	db := dbtest.Open(t)
	ctx := context.Background()
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, resultado := range []string{"presupuesto_agotado", "error_interno"} {
		if _, err := tx.ExecContext(ctx, `INSERT INTO ia_usos (operacion, actor_rol, proveedor, duracion_ms, resultado)
			VALUES ('clasificar', 'sistema', 'doble', 0, $1)`, resultado); err != nil {
			t.Fatalf("068 must accept %s: %v", resultado, err)
		}
	}
	up, down := seccionesGoose(t, migracion068)
	if _, err := tx.ExecContext(ctx, down); err != nil {
		t.Fatalf("Down: %v", err)
	}
	for _, tabla := range []string{"ia_configuracion", "ia_cuotas", "ia_gasto_mes", "ia_cache"} {
		var existe bool
		if err := tx.GetContext(ctx, &existe, `SELECT to_regclass($1) IS NOT NULL`, tabla); err != nil || existe {
			t.Errorf("after Down, %s exists = %v (%v)", tabla, existe, err)
		}
	}
	var resultados []string
	if err := tx.SelectContext(ctx, &resultados, `SELECT resultado FROM ia_usos ORDER BY id DESC LIMIT 2`); err != nil ||
		len(resultados) != 2 || resultados[0] != "error_proveedor" || resultados[1] != "no_disponible" {
		t.Errorf("after Down, resultados = %v (%v), want [error_proveedor no_disponible]", resultados, err)
	}
	checks := func(momento, quiere, noQuiere string) {
		t.Helper()
		var nombres []string
		if err := tx.SelectContext(ctx, &nombres, `SELECT conname FROM pg_constraint WHERE conrelid = 'ia_usos'::regclass AND conname IN ($1, $2)`, quiere, noQuiere); err != nil {
			t.Fatal(err)
		}
		if len(nombres) != 1 || nombres[0] != quiere {
			t.Errorf("%s: constraints %v, want only %s", momento, nombres, quiere)
		}
	}
	checks("after Down", "ia_usos_resultado_check", "ck_ia_usos_resultado")
	if _, err := tx.ExecContext(ctx, `SAVEPOINT s`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO ia_usos (operacion, actor_rol, proveedor, duracion_ms, resultado)
		VALUES ('clasificar', 'sistema', 'doble', 0, 'presupuesto_agotado')`); err == nil {
		t.Error("after Down, presupuesto_agotado is still accepted")
	}
	if _, err := tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT s`); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.ExecContext(ctx, up); err != nil {
		t.Fatalf("Up again: %v", err)
	}
	checks("after Up", "ck_ia_usos_resultado", "ia_usos_resultado_check")
	var limiteDia int
	var presupuesto string
	if err := tx.QueryRowxContext(ctx, `SELECT limite_usuario_dia, presupuesto_mensual_usd::text FROM ia_configuracion WHERE id = 1`).Scan(&limiteDia, &presupuesto); err != nil ||
		limiteDia != 5 || presupuesto != "40.00" {
		t.Errorf("default configuration = %d, %s (%v)", limiteDia, presupuesto, err)
	}
}

// ia_cuotas.clave must start with the literal prefix: in a LIKE the `_` would
// be a wildcard and accept «iaXcuota:».
func TestMigracion068_PrefijoExactoDeLaClave(t *testing.T) {
	db := dbtest.Open(t)
	tests := []struct {
		clave string
		ok    bool
	}{
		{"ia_cuota:abc", true},
		{"iaXcuota:abc", false},
		{"ia_cuotaX", false},
		{"otra:abc", false},
	}
	for _, tt := range tests {
		tx, err := db.Beginx()
		if err != nil {
			t.Fatal(err)
		}
		_, err = tx.Exec(`INSERT INTO ia_cuotas (clave, ventana, ventana_inicio, usados, expira_en) VALUES ($1, 'dia', NOW(), 1, NOW())`, tt.clave)
		if (err == nil) != tt.ok {
			t.Errorf("clave %q: err = %v, want accepted = %v", tt.clave, err, tt.ok)
		}
		_ = tx.Rollback()
	}
}

// updated_by must not stop the deletion of an admin: the configuration keeps
// its values and forgets who saved them.
func TestMigracion068_UpdatedByNoImpideBorrarAlAdmin(t *testing.T) {
	db := dbtest.Open(t)
	admin := dbtest.SeedUser(t, db, dbtest.SeedCatalog(t, db), "admin", "activo")
	if _, err := db.Exec(`UPDATE ia_configuracion SET updated_by = $1 WHERE id = 1`, admin.ID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(`UPDATE ia_configuracion SET updated_by = NULL WHERE id = 1`) })
	var fk string
	if err := db.Get(&fk, `SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname = 'ia_configuracion_updated_by_fkey'`); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fk, "ON DELETE SET NULL") {
		t.Errorf("updated_by constraint = %s, want ON DELETE SET NULL", fk)
	}
}
