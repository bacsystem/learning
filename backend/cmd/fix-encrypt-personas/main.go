// fix-encrypt-personas genera (no ejecuta) el SQL para cifrar los campos PII de
// personas que sigan en texto plano y para recalcular dni_hmac/email_hmac.
//
// Cada campo se evalúa por separado: si descifra con ENCRYPTION_KEY ya está
// cifrado y se deja igual; si no, se cifra. Excepción (H121): si el valor
// tiene forma de ciphertext (ver looksLikeCiphertext) y no descifra, la
// herramienta aborta: casi seguro ENCRYPTION_KEY es otra y seguir generaría
// el SQL para cifrar dos veces todo el padrón. Así una fila con el DNI en claro
// pero el resto ya cifrado no se cifra dos veces. Los HMAC se calculan sobre
// el valor en claro normalizado (DNI trim; email lower+trim, igual que auth).
//
// Las filas cuyo dni_hmac resultante colisionaría con el de otra persona se
// reportan por id y se omiten. El SQL va envuelto en BEGIN/COMMIT. Nunca se
// imprime PII en claro: solo ids, nombres de columnas, ciphertext y HMAC.
//
// H146: cada UPDATE solo aplica si la fila sigue como se leyó: WHERE id AND
// updated_at AND dni_hmac AND email_hmac iguales a los valores leídos. Los
// campos en texto plano no se comparan directamente porque eso imprimiría PII
// en el SQL; los cubre updated_at, que todo UPDATE de personas en la app (y en
// cmd/rehash-emails) fija a NOW(). Cada UPDATE va en un bloque DO que lanza
// una excepción si no afectó exactamente una fila: la transacción queda
// abortada y el COMMIT final se convierte en ROLLBACK (no se aplica nada).
//
// Uso:
//
//	DATABASE_URL=... ENCRYPTION_KEY=... ./fix-encrypt-personas > fix.sql
package main

import (
	"context"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/cbaciliod/learning/internal/crypto"
	"github.com/cbaciliod/learning/internal/db"
	"github.com/jmoiron/sqlx"
)

type personaRow struct {
	ID              int64  `db:"id"`
	DNI             string `db:"dni"`
	DNIHMAC         string `db:"dni_hmac"`
	Nombres         string `db:"nombres"`
	ApellidoPaterno string `db:"apellido_paterno"`
	ApellidoMaterno string `db:"apellido_materno"`
	Telefono        string `db:"telefono"`
	Email           string `db:"email"`
	EmailHMAC       string `db:"email_hmac"`
	UpdatedAt       string `db:"updated_at"` // updated_at::text, exact to the microsecond
}

type column struct {
	Name  string
	Value string // ciphertext or hex HMAC — never plaintext
}

type update struct {
	ID        int64
	Columns   []column
	Guard     []column // column = value read; all of them non-PII
	Encrypted []string // names of fields that were plaintext
}

type report struct {
	Updates    []update
	Collisions [][]int64 // groups of persona ids that would share a dni_hmac
}

func main() {
	if err := run(os.Stdout); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}

func run(out io.Writer) error {
	encKeyHex := os.Getenv("ENCRYPTION_KEY")
	dsn := os.Getenv("DATABASE_URL")
	if encKeyHex == "" || dsn == "" {
		return fmt.Errorf("DATABASE_URL y ENCRYPTION_KEY son requeridas")
	}
	gcm, err := crypto.NewCipher(encKeyHex)
	if err != nil {
		return fmt.Errorf("ENCRYPTION_KEY inválida: %w", err)
	}
	rawKey, err := hex.DecodeString(encKeyHex)
	if err != nil {
		return fmt.Errorf("ENCRYPTION_KEY inválida: %w", err)
	}

	conn, err := db.Connect(dsn)
	if err != nil {
		return fmt.Errorf("db connect: %w", err)
	}
	defer conn.Close()

	rows, err := loadRows(context.Background(), conn)
	if err != nil {
		return err
	}

	rep, err := plan(gcm, rawKey, rows)
	if err != nil {
		return err
	}
	for _, g := range rep.Collisions {
		log.Printf("COLISIÓN dni_hmac entre personas ids=%v: omitidas, revisar a mano", g)
	}
	if len(rep.Updates) == 0 {
		log.Println("No hay personas que corregir.")
		return nil
	}
	if err := render(out, rep.Updates); err != nil {
		return err
	}
	log.Printf("%d persona(s) a corregir, %d grupo(s) en colisión omitidos. Revise y ejecute el SQL generado.",
		len(rep.Updates), len(rep.Collisions))
	return nil
}

func loadRows(ctx context.Context, conn *sqlx.DB) ([]personaRow, error) {
	var rows []personaRow
	if err := conn.SelectContext(ctx, &rows,
		`SELECT id, dni, dni_hmac, nombres, apellido_paterno, apellido_materno, telefono, email, email_hmac,
		        updated_at::text AS updated_at
		 FROM personas ORDER BY id`); err != nil {
		return nil, fmt.Errorf("select: %w", err)
	}
	return rows, nil
}

// plan decides, per row and per field, what has to change.
func plan(gcm cipher.AEAD, rawKey []byte, rows []personaRow) (report, error) {
	type pending struct {
		u        update
		dniHMAC  string
		modified bool
	}
	var all []pending
	byHMAC := map[string][]int{} // target dni_hmac -> indices into all

	for _, p := range rows {
		u := update{ID: p.ID, Guard: []column{
			{"updated_at", p.UpdatedAt}, {"dni_hmac", p.DNIHMAC}, {"email_hmac", p.EmailHMAC},
		}}
		plain := map[string]string{}
		for _, f := range []struct{ name, val string }{
			{"dni", p.DNI}, {"nombres", p.Nombres}, {"apellido_paterno", p.ApellidoPaterno},
			{"apellido_materno", p.ApellidoMaterno}, {"telefono", p.Telefono}, {"email", p.Email},
		} {
			if pt, err := crypto.Decrypt(gcm, f.val); err == nil {
				plain[f.name] = pt
				continue
			}
			if looksLikeCiphertext(gcm, f.val) {
				// Nunca el valor: solo id y campo.
				return report{}, fmt.Errorf("persona id=%d: el campo %s tiene forma de ciphertext pero no descifra "+
					"(¿ENCRYPTION_KEY distinta de la que lo cifró?); abortado sin generar SQL", p.ID, f.name)
			}
			pt := f.val
			if f.name == "email" {
				pt = strings.ToLower(strings.TrimSpace(pt))
			}
			if f.name == "dni" {
				pt = strings.TrimSpace(pt)
			}
			ct, err := crypto.Encrypt(gcm, pt)
			if err != nil {
				return report{}, fmt.Errorf("persona id=%d: cifrar %s: %w", p.ID, f.name, err)
			}
			plain[f.name] = pt
			u.Columns = append(u.Columns, column{f.name, ct})
			u.Encrypted = append(u.Encrypted, f.name)
		}
		dniHMAC := crypto.HMAC(rawKey, strings.TrimSpace(plain["dni"]))
		emailHMAC := crypto.HMAC(rawKey, strings.ToLower(strings.TrimSpace(plain["email"])))
		if dniHMAC != p.DNIHMAC {
			u.Columns = append(u.Columns, column{"dni_hmac", dniHMAC})
		}
		if emailHMAC != p.EmailHMAC {
			u.Columns = append(u.Columns, column{"email_hmac", emailHMAC})
		}
		byHMAC[dniHMAC] = append(byHMAC[dniHMAC], len(all))
		all = append(all, pending{u: u, dniHMAC: dniHMAC, modified: len(u.Columns) > 0})
	}

	var rep report
	collided := map[int]bool{}
	keys := make([]string, 0, len(byHMAC))
	for k := range byHMAC {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		idx := byHMAC[k]
		if len(idx) < 2 {
			continue
		}
		var ids []int64
		for _, i := range idx {
			ids = append(ids, all[i].u.ID)
			collided[i] = true
		}
		sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] })
		rep.Collisions = append(rep.Collisions, ids)
	}
	for i, p := range all {
		if p.modified && !collided[i] {
			rep.Updates = append(rep.Updates, p.u)
		}
	}
	return rep, nil
}

// looksLikeCiphertext reports whether v has the shape crypto.Encrypt produces:
// strict standard base64 (with padding) decoding to at least nonce+tag bytes
// (12+16 = 28 bytes, i.e. ≥ 40 base64 characters). Real plaintext in personas
// does not reach that: an 8-digit DNI is valid base64 but decodes to 6 bytes;
// emails contain '@', multi-word names contain spaces and phones are far
// shorter than 40 characters, all of which fail one of the two checks. A
// false positive only makes the tool abort (fail closed), never corrupt data.
func looksLikeCiphertext(gcm cipher.AEAD, v string) bool {
	data, err := base64.StdEncoding.Strict().DecodeString(v)
	return err == nil && len(data) >= gcm.NonceSize()+gcm.Overhead()
}

// quote returns v as a SQL string literal. The values are ciphertext (base64),
// hex HMACs or timestamps, never containing the "$fix$" delimiter of the DO
// block; doubling single quotes is still required inside it.
func quote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}

func render(w io.Writer, updates []update) error {
	var b strings.Builder
	b.WriteString("BEGIN;\n\n")
	for _, u := range updates {
		names := make([]string, len(u.Columns))
		for i, c := range u.Columns {
			names[i] = c.Name
		}
		fmt.Fprintf(&b, "-- persona id=%d: cifrar=[%s] columnas=[%s]\n",
			u.ID, strings.Join(u.Encrypted, ","), strings.Join(names, ","))
		b.WriteString("DO $fix$\nDECLARE n bigint;\nBEGIN\nUPDATE personas SET\n")
		for _, c := range u.Columns {
			fmt.Fprintf(&b, "  %s=%s,\n", c.Name, quote(c.Value))
		}
		fmt.Fprintf(&b, "  updated_at=NOW()\nWHERE id=%d", u.ID)
		for _, g := range u.Guard {
			if g.Name == "updated_at" {
				fmt.Fprintf(&b, "\n  AND updated_at=%s::timestamptz", quote(g.Value))
				continue
			}
			fmt.Fprintf(&b, "\n  AND %s=%s", g.Name, quote(g.Value))
		}
		fmt.Fprintf(&b, ";\nGET DIAGNOSTICS n = ROW_COUNT;\n"+
			"IF n <> 1 THEN\n"+
			"  RAISE EXCEPTION 'persona id=%d cambió desde que se generó este SQL (%% fila(s) afectadas): no se aplica nada; vuelva a generarlo', n;\n"+
			"END IF;\nEND\n$fix$;\n\n", u.ID)
	}
	b.WriteString("COMMIT;\n")
	_, err := io.WriteString(w, b.String())
	return err
}
