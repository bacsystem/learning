// cmd/migrate ejecuta goose bajo un advisory lock de Postgres, para que varias
// réplicas que arrancan a la vez no apliquen migraciones en paralelo (la
// segunda fallaba con "duplicate key ... pg_class_relname_nsp_index" y entraba
// en crash-loop). La primera réplica migra; las demás esperan el lock y luego
// goose no encuentra nada pendiente.
//
// El lock es de sesión y se toma en una conexión dedicada que vive mientras
// corre goose: si este proceso muere, Postgres libera el lock al cerrarse la
// conexión. Para que eso no deje a goose migrando sin lock, en Linux goose
// muere junto con este proceso (Pdeathsig; ver runGoose y child_linux.go).
//
// Uso:
//
//	DATABASE_URL=... migrate [-dir ./migrations] [-goose goose] [-lock-timeout 10m] [comando goose...]
//
// Sin comando ejecuta "up". La DSN se pasa a goose por GOOSE_DBSTRING (no en
// argv). GOOSE_BIN puede sustituir a -goose.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	_ "github.com/lib/pq"
)

// lockKey es una constante arbitraria y fija para "migraciones de learning".
const lockKey int64 = 0x6C6561726E696E67 // "learning"

type options struct {
	dsn         string
	dir         string
	goose       string
	lockTimeout time.Duration
	args        []string
}

func main() {
	opts, err := parseArgs(os.Args[1:], os.Getenv)
	if err != nil {
		log.Fatalf("migrate: %v", err)
	}
	if err := run(context.Background(), opts, os.Stdout, os.Stderr); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			log.Printf("migrate: goose falló: %v", err)
			os.Exit(ee.ExitCode())
		}
		log.Fatalf("migrate: %v", err)
	}
}

func parseArgs(argv []string, getenv func(string) string) (options, error) {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	o := options{dsn: getenv("DATABASE_URL")}
	gooseDefault := getenv("GOOSE_BIN")
	if gooseDefault == "" {
		gooseDefault = "goose"
	}
	fs.StringVar(&o.dir, "dir", "./migrations", "directorio de migraciones")
	fs.StringVar(&o.goose, "goose", gooseDefault, "binario de goose")
	fs.DurationVar(&o.lockTimeout, "lock-timeout", 10*time.Minute, "espera máxima por el advisory lock")
	if err := fs.Parse(argv); err != nil {
		return o, err
	}
	if o.dsn == "" {
		return o, errors.New("DATABASE_URL requerida")
	}
	o.args = fs.Args()
	if len(o.args) == 0 {
		o.args = []string{"up"}
	}
	return o, nil
}

func run(ctx context.Context, o options, stdout, stderr io.Writer) error {
	db, err := sql.Open("postgres", o.dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	conn, err := db.Conn(ctx) // conexión dedicada: el lock vive en esta sesión
	if err != nil {
		return fmt.Errorf("conectar: %w", err)
	}
	defer conn.Close()

	lockCtx, cancel := context.WithTimeout(ctx, o.lockTimeout)
	defer cancel()
	log.Printf("migrate: esperando advisory lock %d", lockKey)
	if err := acquire(lockCtx, conn); err != nil {
		return err
	}
	log.Printf("migrate: lock adquirido; ejecutando goose %v", o.args)
	defer func() {
		// Con contexto propio: liberar aunque ctx se haya cancelado.
		uctx, ucancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer ucancel()
		if _, err := conn.ExecContext(uctx, `SELECT pg_advisory_unlock($1)`, lockKey); err != nil {
			log.Printf("migrate: unlock: %v (se libera al cerrar la conexión)", err)
		}
	}()

	return runGoose(ctx, o, stdout, stderr)
}

// acquire hace polling con pg_try_advisory_lock en vez de bloquear en
// pg_advisory_lock: una sentencia bloqueada mantiene un snapshot abierto y un
// CREATE INDEX CONCURRENTLY de la réplica que migra (p.ej. 054) esperaría a esa
// sentencia, que a su vez espera el lock -> bloqueo mutuo que Postgres no
// detecta. Entre intentos la conexión queda ociosa, sin transacción.
func acquire(ctx context.Context, conn *sql.Conn) error {
	for {
		var ok bool
		if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, lockKey).Scan(&ok); err != nil {
			return fmt.Errorf("advisory lock: %w", err)
		}
		if ok {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("advisory lock: %w", ctx.Err())
		case <-time.After(time.Second):
		}
	}
}

// runGoose ejecuta goose y no vuelve hasta que el hijo termina, así que la
// sesión del lock (conn en run) sigue abierta mientras goose migra. En Linux,
// si este proceso muere (incluso con SIGKILL) el kernel mata a goose
// (Pdeathsig, child_linux.go) y el lock nunca queda libre con goose vivo
// (H95). En otros sistemas ese caso es un riesgo residual (child_other.go).
// Residual en cualquier sistema: la sentencia que goose tenía en curso sigue
// ejecutándose en el servidor hasta terminar (Postgres no detecta el cliente
// caído sin client_connection_check_interval); las migraciones normales son
// una transacción, así que al terminar no puede confirmar y se revierte, y
// mientras tanto conserva sus locks, que una segunda ejecución tendría que
// esperar. Excepción (H147): las marcadas `-- +goose NO TRANSACTION` (054,
// 058: CREATE INDEX CONCURRENTLY) no se revierten; un kill a mitad puede
// dejar un índice INVALID o una construcción aún en curso. Ver
// docs/puesta-en-marcha.md: revisar pg_stat_progress_create_index antes de
// DROP INDEX CONCURRENTLY del índice INVALID.
func runGoose(ctx context.Context, o options, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, o.goose, append([]string{"-dir", o.dir}, o.args...)...)
	cmd.Env = append(os.Environ(), "GOOSE_DRIVER=postgres", "GOOSE_DBSTRING="+o.dsn)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	configureChild(cmd)
	// Pdeathsig va ligado al hilo que hace fork: fijarlo hasta que goose termine.
	unlock := lockThread()
	defer unlock()
	return cmd.Run()
}
