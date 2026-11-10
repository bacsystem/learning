// cmd/seed seeds the personas table from the SQL seed file, encrypting PII fields
// and computing a deterministic HMAC for indexed DNI/email lookups.
//
// It never logs PII: problems are reported by line number of the seed file.
// It exits non-zero if any line could not be parsed (nothing is inserted in that
// case) or if any row failed to insert.
package main

import (
	"bufio"
	"context"
	"crypto/cipher"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cbaciliod/learning/cmd/seed/internal/persona"
	"github.com/cbaciliod/learning/internal/crypto"
	"github.com/cbaciliod/learning/internal/db"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var insertRE = regexp.MustCompile(`^INSERT INTO personas\s*\(([^)]*)\)\s*VALUES\s*\((.+)\)\s*ON CONFLICT`)

// expectedColumns is the column order the parser maps to personaRecord.
var expectedColumns = []string{
	"dni", "nombres", "apellido_paterno", "apellido_materno",
	"grado_id", "especialidad_id", "institucion_id", "telefono", "email", "estado",
}

type personaRecord struct {
	Line           int
	DNI            string
	Nombres        string
	ApellidoP      string
	ApellidoM      string
	GradoID        int64
	EspecialidadID int64
	InstitucionID  int64
	Telefono       string
	Email          string
	Estado         string
}

// lineError describes a seed-file line that could not be used. Reason never
// contains field values (they are PII).
type lineError struct {
	Line   int
	Reason string
}

func main() {
	os.Exit(run())
}

func run() int {
	databaseURL := mustEnv("DATABASE_URL")
	encKey := mustEnv("ENCRYPTION_KEY")

	gcm, err := crypto.NewCipher(encKey)
	if err != nil {
		log.Printf("crypto: %v", err)
		return 1
	}
	rawKey, err := hex.DecodeString(encKey)
	if err != nil {
		log.Printf("ENCRYPTION_KEY: %v", err)
		return 1
	}

	seedFile := "assets/data/sql/001_seed_personas.sql"
	if len(os.Args) > 1 {
		seedFile = os.Args[1]
	}

	f, err := os.Open(seedFile)
	if err != nil {
		log.Printf("open: %v", err)
		return 1
	}
	records, skipped, err := parseSeed(f)
	f.Close()
	if err != nil {
		log.Printf("parse: %v", err)
		return 1
	}
	log.Printf("parsed %d rows from %s, skipped %d lines", len(records), seedFile, len(skipped))
	if len(skipped) > 0 {
		for _, s := range skipped {
			log.Printf("  skipped line %d: %s", s.Line, s.Reason)
		}
		log.Printf("aborting: fix the seed file (nothing was inserted)")
		return 1
	}

	conn, err := db.Connect(databaseURL)
	if err != nil {
		log.Printf("db: %v", err)
		return 1
	}
	defer conn.Close()

	inserted, duplicates, errs := seed(context.Background(), conn, gcm, rawKey, records)
	log.Printf("done: inserted=%d duplicates=%d errors=%d", inserted, duplicates, len(errs))
	for _, e := range errs {
		log.Printf("  error line %d: %s", e.Line, e.Reason)
	}
	if len(errs) > 0 {
		return 1
	}
	return 0
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("env %s is required", k)
	}
	return v
}

// parseSeed reads the seed SQL. Blank lines, `--` comments and BEGIN/COMMIT
// are ignored; every other line must be a well-formed INSERT INTO personas or
// it is reported in skipped (with its line number).
func parseSeed(r io.Reader) (records []personaRecord, skipped []lineError, err error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		upper := strings.ToUpper(strings.TrimSuffix(line, ";"))
		if line == "" || strings.HasPrefix(line, "--") || upper == "BEGIN" || upper == "COMMIT" {
			continue
		}
		rec, perr := parseLine(line)
		if perr != nil {
			skipped = append(skipped, lineError{Line: lineNo, Reason: perr.Error()})
			continue
		}
		rec.Line = lineNo
		records = append(records, rec)
	}
	return records, skipped, scanner.Err()
}

func parseLine(line string) (personaRecord, error) {
	m := insertRE.FindStringSubmatch(line)
	if m == nil {
		return personaRecord{}, errors.New("not an `INSERT INTO personas (...) VALUES (...) ON CONFLICT` statement")
	}
	cols := strings.Split(m[1], ",")
	if len(cols) < len(expectedColumns) {
		return personaRecord{}, fmt.Errorf("expected at least %d columns, got %d", len(expectedColumns), len(cols))
	}
	for i, want := range expectedColumns {
		if got := strings.TrimSpace(cols[i]); got != want {
			return personaRecord{}, fmt.Errorf("column %d is %q, expected %q", i+1, got, want)
		}
	}
	fields, err := splitValues(m[2])
	if err != nil {
		return personaRecord{}, err
	}
	if len(fields) != len(cols) {
		return personaRecord{}, fmt.Errorf("%d columns but %d values", len(cols), len(fields))
	}
	ids := make([]int64, 3)
	for i, idx := range []int{4, 5, 6} {
		v, err := strconv.ParseInt(fields[idx], 10, 64)
		if err != nil {
			return personaRecord{}, fmt.Errorf("column %s is not an integer", expectedColumns[idx])
		}
		ids[i] = v
	}
	return personaRecord{
		DNI:            fields[0],
		Nombres:        fields[1],
		ApellidoP:      fields[2],
		ApellidoM:      fields[3],
		GradoID:        ids[0],
		EspecialidadID: ids[1],
		InstitucionID:  ids[2],
		Telefono:       fields[7],
		Email:          fields[8],
		Estado:         fields[9],
	}, nil
}

var errUnterminated = errors.New("unterminated quoted string")

// splitValues splits a SQL VALUES list on top-level commas. Single-quoted
// strings may contain commas and SQL-escaped quotes (a doubled single quote). Whitespace outside
// quotes is dropped; whitespace inside quotes is kept.
func splitValues(s string) ([]string, error) {
	var fields []string
	var cur strings.Builder
	inQuote := false
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		ch := rs[i]
		switch {
		case inQuote && ch == '\'':
			if i+1 < len(rs) && rs[i+1] == '\'' {
				cur.WriteRune('\'')
				i++
			} else {
				inQuote = false
			}
		case inQuote:
			cur.WriteRune(ch)
		case ch == '\'':
			inQuote = true
		case ch == ',':
			fields = append(fields, cur.String())
			cur.Reset()
		case ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n':
			// whitespace outside quotes
		default:
			cur.WriteRune(ch)
		}
	}
	if inQuote {
		return nil, errUnterminated
	}
	fields = append(fields, cur.String())
	return fields, nil
}

func seed(ctx context.Context, conn *sqlx.DB, gcm cipher.AEAD, rawKey []byte, records []personaRecord) (inserted, duplicates int, errs []lineError) {
	total := len(records)
	start := time.Now()

	for i, r := range records {
		if (i+1)%500 == 0 {
			log.Printf("  progress: %d/%d (%s)", i+1, total, time.Since(start).Round(time.Second))
		}

		e, err := persona.Encrypt(gcm, rawKey, persona.Plain{
			DNI: r.DNI, Nombres: r.Nombres, ApellidoP: r.ApellidoP, ApellidoM: r.ApellidoM,
			Telefono: r.Telefono, Email: r.Email,
		})
		if err != nil {
			errs = append(errs, lineError{Line: r.Line, Reason: err.Error()})
			continue
		}
		res, err := conn.ExecContext(ctx, `
			INSERT INTO personas
				(dni, dni_hmac, nombres, apellido_paterno, apellido_materno,
				 grado_id, especialidad_id, institucion_id, telefono, email, email_hmac, estado)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
			ON CONFLICT (dni_hmac) DO NOTHING`,
			e.DNI, e.DNIHMAC, e.Nombres, e.ApellidoP, e.ApellidoM,
			r.GradoID, r.EspecialidadID, r.InstitucionID,
			e.Telefono, e.Email, e.EmailHMAC, r.Estado,
		)
		if err != nil {
			errs = append(errs, lineError{Line: r.Line, Reason: dbErrReason(err)})
			continue
		}
		n, err := res.RowsAffected()
		if err != nil {
			errs = append(errs, lineError{Line: r.Line, Reason: dbErrReason(err)})
			continue
		}
		if n > 0 {
			inserted++
		} else {
			duplicates++
		}
	}
	return
}

// dbErrReason returns a DB error message. Values are bound parameters (and
// encrypted), so pq errors don't echo plaintext PII; Detail is dropped anyway
// because Postgres may include key values there.
func dbErrReason(err error) string {
	var pe *pq.Error
	if errors.As(err, &pe) {
		return fmt.Sprintf("%s (SQLSTATE %s)", pe.Message, pe.Code)
	}
	return err.Error()
}
