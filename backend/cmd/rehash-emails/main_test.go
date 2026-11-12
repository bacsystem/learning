package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/cbaciliod/learning/internal/auth"
	"github.com/cbaciliod/learning/internal/crypto"
	"github.com/cbaciliod/learning/internal/db/testlock"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

const testHexKey = "0000000000000000000000000000000000000000000000000000000000000001"

func TestRehash_DryRunThenApply(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	conn, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() }) // registered first → runs after the data cleanup below
	testlock.Hold(t, conn)             // released after the data cleanup below, before the pool closes
	gcm, _ := crypto.NewCipher(testHexKey)
	key, _ := hex.DecodeString(testHexKey)
	ctx := context.Background()

	sfx := fmt.Sprintf("%d%04d", time.Now().UnixNano()%1e9, rand.Intn(10000))
	var inst, grado, esp int64
	must := func(dest *int64, q string, args ...any) {
		t.Helper()
		if err := conn.QueryRow(q, args...).Scan(dest); err != nil {
			t.Fatalf("%v (%s)", err, q)
		}
	}
	must(&inst, `INSERT INTO instituciones (nombre, siglas) VALUES ($1,$2) RETURNING id`, "Inst rh "+sfx, "R"+sfx)
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

	enc := func(s string) string { v, _ := crypto.Encrypt(gcm, s); return v }
	insert := func(email string) int64 {
		dni := fmt.Sprintf("%08d", rand.Intn(1e8))
		encEmail, h := "", ""
		if email != "" {
			encEmail, h = enc(email), crypto.HMAC(key, email) // legacy: un-normalized HMAC
		}
		var id int64
		must(&id, `INSERT INTO personas (dni,dni_hmac,nombres,apellido_paterno,apellido_materno,grado_id,especialidad_id,institucion_id,telefono,email,email_hmac)
			VALUES ($1,$2,$3,$3,$3,$4,$5,$6,$3,$7,$8) RETURNING id`,
			enc(dni), crypto.HMAC(key, dni+sfx), enc("x"), grado, esp, inst, encEmail, h)
		return id
	}
	legacy := insert("  Juan@Example.COM ")
	already := insert("ana@example.com")
	empty := insert("")
	ids := []int64{legacy, already, empty}

	hmacOf := func(id int64) string {
		var h string
		if err := conn.QueryRow(`SELECT email_hmac FROM personas WHERE id=$1`, id).Scan(&h); err != nil {
			t.Fatal(err)
		}
		return h
	}
	before := hmacOf(legacy)

	st, err := rehash(ctx, conn, gcm, key, false, ids)
	if err != nil {
		t.Fatal(err)
	}
	if st.Total != 3 || st.Updated != 1 || st.Unchanged != 1 || st.Empty != 1 {
		t.Fatalf("dry-run stats %+v", st)
	}
	if hmacOf(legacy) != before {
		t.Fatal("dry-run modified data")
	}

	if st, err = rehash(ctx, conn, gcm, key, true, ids); err != nil || st.Updated != 1 {
		t.Fatalf("apply: %+v %v", st, err)
	}
	if hmacOf(legacy) != crypto.HMAC(key, "juan@example.com") {
		t.Fatal("email_hmac not recomputed from the normalized email")
	}
	// Same canonical form the app looks e-mails up with (auth.NormalizeEmail).
	if hmacOf(legacy) != crypto.HMAC(key, auth.NormalizeEmail("  Juan@Example.COM ")) {
		t.Fatal("rehash and auth.NormalizeEmail diverge")
	}
	var encEmail string
	if err := conn.QueryRow(`SELECT email FROM personas WHERE id=$1`, legacy).Scan(&encEmail); err != nil {
		t.Fatal(err)
	}
	if plain, _ := crypto.Decrypt(gcm, encEmail); plain != "juan@example.com" {
		t.Fatalf("stored email not normalized: %q", plain)
	}

	// Idempotent.
	if st, err = rehash(ctx, conn, gcm, key, true, ids); err != nil || st.Updated != 0 {
		t.Fatalf("second apply should be a no-op: %+v %v", st, err)
	}
}
