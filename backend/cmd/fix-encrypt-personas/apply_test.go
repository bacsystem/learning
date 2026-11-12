package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cbaciliod/learning/internal/crypto"
	"github.com/cbaciliod/learning/internal/db/testlock"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// H146 (integración): el SQL generado se aplica contra Postgres. Si una de las
// filas cambia entre la generación y la ejecución, el script aborta y no se
// aplica ningún UPDATE (tampoco los de las filas que no cambiaron). Sin
// cambios intermedios, aplica todo.
func TestGeneratedSQLAbortsAndRollsBackOnConcurrentChange(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	conn, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	testlock.Hold(t, conn) // released after the data cleanup below, before the pool closes
	ctx := context.Background()
	gcm, _ := crypto.NewCipher(testHexKey)
	raw, _ := hex.DecodeString(testHexKey)

	sfx := fmt.Sprintf("%d%04d", time.Now().UnixNano()%1e9, rand.Intn(10000))
	var inst, grado, esp int64
	must := func(dest *int64, q string, args ...any) {
		t.Helper()
		if err := conn.QueryRow(q, args...).Scan(dest); err != nil {
			t.Fatalf("%v (%s)", err, q)
		}
	}
	must(&inst, `INSERT INTO instituciones (nombre, siglas) VALUES ($1,$2) RETURNING id`, "Inst fx "+sfx, "F"+sfx)
	must(&grado, `INSERT INTO grados (institucion_id,nombre,abreviatura,nivel,orden) VALUES ($1,$2,'G','oficial',1) RETURNING id`, inst, "G"+sfx)
	must(&esp, `INSERT INTO especialidades (institucion_id,nombre,codigo) VALUES ($1,$2,$3) RETURNING id`, inst, "E"+sfx, "E"+sfx)
	t.Cleanup(func() {
		for _, q := range []string{
			`DELETE FROM personas WHERE institucion_id=$1`, `DELETE FROM especialidades WHERE institucion_id=$1`,
			`DELETE FROM grados WHERE institucion_id=$1`, `DELETE FROM instituciones WHERE id=$1`,
		} {
			if _, err := conn.Exec(q, inst); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
	})
	insert := func(n int) (int64, string) {
		dni := fmt.Sprintf("%08d", rand.Intn(1e8))
		var id int64
		must(&id, `INSERT INTO personas (dni, dni_hmac, nombres, apellido_paterno, apellido_materno,
			grado_id, especialidad_id, institucion_id, telefono, email, email_hmac)
			VALUES ($1, $2, 'N', 'P', 'M', $3, $4, $5, '999', $6, '') RETURNING id`,
			dni, "legacy:fx"+sfx+fmt.Sprint(n), grado, esp, inst, fmt.Sprintf("p%d-%s@x.com", n, sfx))
		return id, dni
	}
	id1, dni1 := insert(1)
	id2, dni2 := insert(2)

	generate := func() string {
		t.Helper()
		all, err := loadRows(ctx, conn)
		if err != nil {
			t.Fatal(err)
		}
		var mine []personaRow
		for _, r := range all {
			if r.ID == id1 || r.ID == id2 {
				mine = append(mine, r)
			}
		}
		rep, err := plan(gcm, raw, mine)
		if err != nil {
			t.Fatal(err)
		}
		if len(rep.Updates) != 2 {
			t.Fatalf("updates = %d, want 2", len(rep.Updates))
		}
		var buf bytes.Buffer
		if err := render(&buf, rep.Updates); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	dniOf := func(id int64) string {
		t.Helper()
		var v string
		if err := conn.Get(&v, `SELECT dni FROM personas WHERE id=$1`, id); err != nil {
			t.Fatal(err)
		}
		return v
	}

	// apply corre el script en una conexión propia, como psql: si una sentencia
	// falla, la transacción queda abortada y lo que sigue (COMMIT) no confirma.
	apply := func(sql string) error {
		t.Helper()
		c, err := conn.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		_, err = c.ExecContext(ctx, sql)
		if err != nil {
			if _, rbErr := c.ExecContext(ctx, "ROLLBACK"); rbErr != nil {
				t.Fatalf("rollback: %v", rbErr)
			}
		}
		return err
	}

	// 1) Cambio intermedio en la segunda fila: aborta y revierte también la primera.
	sql := generate()
	if _, err := conn.Exec(`UPDATE personas SET telefono='111', updated_at=NOW() WHERE id=$1`, id2); err != nil {
		t.Fatal(err)
	}
	err = apply(sql)
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("persona id=%d cambió", id2)) {
		t.Fatalf("el SQL debía abortar por la persona %d, err = %v", id2, err)
	}
	if got := dniOf(id1); got != dni1 {
		t.Fatalf("persona %d no debía modificarse (rollback), dni = %q", id1, got)
	}

	// 2) Regenerado sin cambios intermedios: aplica las dos filas.
	sql = generate()
	if err := apply(sql); err != nil {
		t.Fatalf("aplicar el SQL: %v", err)
	}
	for id, want := range map[int64]string{id1: dni1, id2: dni2} {
		pt, err := crypto.Decrypt(gcm, dniOf(id))
		if err != nil || pt != want {
			t.Fatalf("persona %d: dni cifrado inválido (%v)", id, err)
		}
	}
}
