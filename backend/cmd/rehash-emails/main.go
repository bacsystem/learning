// rehash-emails normaliza los emails de personas (trim + minúsculas) y
// recalcula personas.email_hmac a partir del email descifrado.
//
// Las filas anteriores a la normalización (H49) se guardaron con el HMAC del
// email tal cual se escribió, por lo que "Juan@X.com" y "juan@x.com" no
// coincidían en las búsquedas. Este comando se ejecuta UNA vez tras el deploy.
//
// Por defecto es un dry-run: solo cuenta lo que cambiaría. Con -apply escribe
// los cambios dentro de una única transacción (todo o nada).
//
// Solo imprime CONTEOS: nunca emails, DNIs ni ningún otro dato personal.
//
// Uso:
//
//	DATABASE_URL=... ENCRYPTION_KEY=... go run ./cmd/rehash-emails          # dry-run
//	DATABASE_URL=... ENCRYPTION_KEY=... go run ./cmd/rehash-emails -apply   # aplica
package main

import (
	"context"
	"crypto/cipher"
	"encoding/hex"
	"flag"
	"fmt"
	"os"

	"github.com/cbaciliod/learning/internal/auth"
	"github.com/cbaciliod/learning/internal/crypto"
	"github.com/cbaciliod/learning/internal/db"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type stats struct {
	Total         int // personas examinadas
	Empty         int // sin email
	DecryptErrors int // email que no se pudo descifrar (no se toca)
	Unchanged     int // ya normalizadas
	Updated       int // email y/o email_hmac recalculados
	DuplicateHMAC int // valores de email_hmac compartidos por >1 persona tras normalizar
}

// rehash recomputes email/email_hmac for every persona (or only onlyIDs when
// not empty). With apply=false nothing is written (the transaction is rolled
// back).
func rehash(ctx context.Context, conn *sqlx.DB, gcm cipher.AEAD, key []byte, apply bool, onlyIDs []int64) (stats, error) {
	var st stats
	tx, err := conn.BeginTxx(ctx, nil)
	if err != nil {
		return st, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	q := `SELECT id, email, email_hmac FROM personas`
	args := []any{}
	if len(onlyIDs) > 0 {
		q += ` WHERE id = ANY($1)`
		args = append(args, pq.Array(onlyIDs))
	}
	q += ` ORDER BY id FOR UPDATE`

	var rows []struct {
		ID        int64  `db:"id"`
		Email     string `db:"email"`
		EmailHMAC string `db:"email_hmac"`
	}
	if err := tx.SelectContext(ctx, &rows, q, args...); err != nil {
		return st, err
	}

	hmacCount := map[string]int{}
	for _, row := range rows {
		st.Total++
		if row.Email == "" {
			st.Empty++
			continue
		}
		plain, err := crypto.Decrypt(gcm, row.Email)
		if err != nil {
			st.DecryptErrors++
			continue
		}
		norm := auth.NormalizeEmail(plain) // same canonical form as the app
		if norm == "" {
			st.Empty++
			continue
		}
		newHMAC := crypto.HMAC(key, norm)
		hmacCount[newHMAC]++
		if norm == plain && newHMAC == row.EmailHMAC {
			st.Unchanged++
			continue
		}
		st.Updated++
		if !apply {
			continue
		}
		encEmail := row.Email
		if norm != plain {
			if encEmail, err = crypto.Encrypt(gcm, norm); err != nil {
				return st, fmt.Errorf("encrypt persona %d: %w", row.ID, err)
			}
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE personas SET email=$1, email_hmac=$2, updated_at=NOW() WHERE id=$3`,
			encEmail, newHMAC, row.ID); err != nil {
			return st, fmt.Errorf("update persona %d: %w", row.ID, err)
		}
	}
	for _, n := range hmacCount {
		if n > 1 {
			st.DuplicateHMAC++
		}
	}
	if apply {
		if err := tx.Commit(); err != nil {
			return st, err
		}
	}
	return st, nil
}

func main() {
	apply := flag.Bool("apply", false, "escribir los cambios (por defecto: dry-run)")
	flag.Parse()

	encKeyHex := os.Getenv("ENCRYPTION_KEY")
	dsn := os.Getenv("DATABASE_URL")
	if encKeyHex == "" || dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL y ENCRYPTION_KEY son obligatorios")
		os.Exit(2)
	}
	gcm, err := crypto.NewCipher(encKeyHex)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ENCRYPTION_KEY inválida: %v\n", err)
		os.Exit(2)
	}
	key, err := hex.DecodeString(encKeyHex)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ENCRYPTION_KEY inválida: %v\n", err)
		os.Exit(2)
	}

	conn, err := db.Connect(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	st, err := rehash(context.Background(), conn, gcm, key, *apply, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rehash falló (sin cambios): %v\n", err)
		conn.Close()
		os.Exit(1)
	}

	mode := "DRY-RUN (sin cambios; use -apply para escribir)"
	if *apply {
		mode = "APLICADO"
	}
	fmt.Printf("modo: %s\n", mode)
	fmt.Printf("personas examinadas:        %d\n", st.Total)
	fmt.Printf("sin email:                  %d\n", st.Empty)
	fmt.Printf("errores de descifrado:      %d\n", st.DecryptErrors)
	fmt.Printf("ya normalizadas:            %d\n", st.Unchanged)
	fmt.Printf("actualizadas/a actualizar:  %d\n", st.Updated)
	fmt.Printf("email_hmac duplicados:      %d (emails compartidos por >1 persona)\n", st.DuplicateHMAC)
}
