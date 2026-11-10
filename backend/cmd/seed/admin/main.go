// cmd/seed/admin crea (idempotente) el usuario administrador inicial, ligado a
// una persona DEDICADA cuyo email es ADMIN_EMAIL. Nunca reutiliza una persona
// arbitraria: el email de la persona es el canal de recuperación de contraseña,
// así que ligar el admin a una persona real entregaría la cuenta a su dueño.
//
// Uso:
//
//	DATABASE_URL=... ENCRYPTION_KEY=... ADMIN_EMAIL=ops@dominio \
//	  go run ./cmd/seed/admin < archivo-con-la-contraseña
//
// Variables:
//
//	DATABASE_URL, ENCRYPTION_KEY  requeridas
//	ADMIN_EMAIL                   requerida; email de recuperación del admin
//	ADMIN_DNI                     opcional; por defecto un identificador
//	                              sintético no numérico ("admin:<username>")
//	                              que no puede chocar con un DNI real
//	ADMIN_USERNAME                opcional (por defecto "admin")
//	ADMIN_PASSWORD                opcional; si está vacía se lee UNA línea de
//	                              stdin (para no dejarla en la línea de comandos)
//	ADMIN_REBIND=1                opcional; si el usuario ya existe ligado a otra
//	                              persona o no es admin activo, lo re-liga a la
//	                              persona dedicada, fuerza rol=admin y
//	                              estado=activo, rota
//	                              la contraseña a la indicada, cierra sus sesiones
//	                              e invalida sus tokens de recuperación pendientes
//	ADMIN_INSTITUCION_ID, ADMIN_GRADO_ID, ADMIN_ESPECIALIDAD_ID
//	                              opcionales; catálogos para la persona dedicada
//	                              (por defecto el de menor id)
//
// Nunca imprime la contraseña ni PII.
package main

import (
	"bufio"
	"context"
	"crypto/cipher"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/cbaciliod/learning/cmd/seed/internal/persona"
	"github.com/cbaciliod/learning/internal/crypto"
	"github.com/cbaciliod/learning/internal/db"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptCost     = 12
	minPasswordLen = 12
	maxPasswordLen = 72 // límite de bcrypt
)

type config struct {
	Username       string
	Email          string
	DNI            string
	Password       string
	Rebind         bool
	InstitucionID  int64
	GradoID        int64
	EspecialidadID int64
}

type outcome string

const (
	outcomeCreated outcome = "created"
	outcomeExists  outcome = "exists"
	outcomeRebound outcome = "rebound"
)

func main() {
	if err := run(os.Stdin); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}

func run(stdin io.Reader) error {
	databaseURL := os.Getenv("DATABASE_URL")
	encKey := os.Getenv("ENCRYPTION_KEY")
	if databaseURL == "" || encKey == "" {
		return errors.New("DATABASE_URL y ENCRYPTION_KEY son requeridas")
	}
	cfg, err := configFromEnv(os.Getenv, stdin)
	if err != nil {
		return err
	}
	gcm, err := crypto.NewCipher(encKey)
	if err != nil {
		return fmt.Errorf("ENCRYPTION_KEY: %w", err)
	}
	rawKey, err := hex.DecodeString(encKey)
	if err != nil {
		return fmt.Errorf("ENCRYPTION_KEY: %w", err)
	}

	conn, err := db.Connect(databaseURL)
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	defer conn.Close()

	res, personaID, err := ensureAdmin(context.Background(), conn, gcm, rawKey, cfg)
	if err != nil {
		return err
	}
	switch res {
	case outcomeCreated:
		fmt.Printf("Usuario admin '%s' creado (persona dedicada id=%d).\n", cfg.Username, personaID)
	case outcomeRebound:
		fmt.Printf("Usuario admin '%s' re-ligado a la persona dedicada id=%d como admin activo; tokens de recuperación invalidados.\n", cfg.Username, personaID)
	default:
		fmt.Printf("Usuario admin '%s' ya existe ligado a su persona dedicada id=%d. Nada que hacer.\n", cfg.Username, personaID)
	}
	return nil
}

func configFromEnv(getenv func(string) string, stdin io.Reader) (config, error) {
	cfg := config{
		Username: strings.TrimSpace(getenv("ADMIN_USERNAME")),
		Email:    persona.NormalizeEmail(getenv("ADMIN_EMAIL")),
		DNI:      persona.NormalizeDNI(getenv("ADMIN_DNI")),
		Password: getenv("ADMIN_PASSWORD"),
		Rebind:   getenv("ADMIN_REBIND") == "1",
	}
	if cfg.Username == "" {
		cfg.Username = "admin"
	}
	if cfg.Email == "" || !strings.Contains(cfg.Email, "@") {
		return cfg, errors.New("ADMIN_EMAIL requerida (email de recuperación de la cuenta admin)")
	}
	if cfg.DNI == "" {
		cfg.DNI = "admin:" + cfg.Username
	}
	if cfg.Password == "" {
		line, err := bufio.NewReader(stdin).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return cfg, fmt.Errorf("leer contraseña de stdin: %w", err)
		}
		cfg.Password = strings.TrimRight(line, "\r\n")
	}
	if n := len(cfg.Password); n < minPasswordLen || n > maxPasswordLen {
		return cfg, fmt.Errorf("la contraseña debe tener entre %d y %d bytes (ADMIN_PASSWORD o una línea en stdin)", minPasswordLen, maxPasswordLen)
	}
	for k, dst := range map[string]*int64{
		"ADMIN_INSTITUCION_ID": &cfg.InstitucionID, "ADMIN_GRADO_ID": &cfg.GradoID, "ADMIN_ESPECIALIDAD_ID": &cfg.EspecialidadID,
	} {
		if v := strings.TrimSpace(getenv(k)); v != "" {
			id, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return cfg, fmt.Errorf("%s inválido", k)
			}
			*dst = id
		}
	}
	return cfg, nil
}

// ensureAdmin garantiza que exista cfg.Username con rol admin ligado a la
// persona dedicada (dni_hmac = HMAC(cfg.DNI), email_hmac = HMAC(cfg.Email)).
// Se ejecuta en una transacción.
func ensureAdmin(ctx context.Context, conn *sqlx.DB, gcm cipher.AEAD, rawKey []byte, cfg config) (outcome, int64, error) {
	tx, err := conn.BeginTxx(ctx, nil)
	if err != nil {
		return "", 0, err
	}
	defer tx.Rollback() //nolint:errcheck

	// Serializa ejecuciones concurrentes del seed.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('learning:seed-admin'))`); err != nil {
		return "", 0, err
	}

	dniHMAC := crypto.HMAC(rawKey, cfg.DNI)
	emailHMAC := crypto.HMAC(rawKey, cfg.Email)

	// Ningún OTRO registro de persona puede compartir el email del admin: el
	// flujo de recuperación busca por email_hmac.
	var clash int
	if err := tx.GetContext(ctx, &clash,
		`SELECT COUNT(*) FROM personas WHERE email_hmac = $1 AND dni_hmac <> $2`, emailHMAC, dniHMAC); err != nil {
		return "", 0, err
	}
	if clash > 0 {
		return "", 0, errors.New("ADMIN_EMAIL ya pertenece a otra persona; use un email exclusivo para el admin")
	}

	personaID, err := dedicatedPersona(ctx, tx, gcm, rawKey, cfg, dniHMAC, emailHMAC)
	if err != nil {
		return "", 0, err
	}

	var u struct {
		ID        int64  `db:"id"`
		PersonaID int64  `db:"persona_id"`
		Rol       string `db:"rol"`
		Estado    string `db:"estado"`
	}
	err = tx.GetContext(ctx, &u, `SELECT id, persona_id, rol, estado FROM usuarios WHERE username = $1 FOR UPDATE`, cfg.Username)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		hash, err := bcrypt.GenerateFromPassword([]byte(cfg.Password), bcryptCost)
		if err != nil {
			return "", 0, fmt.Errorf("bcrypt: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO usuarios (persona_id, username, password_hash, rol, estado, activated_at)
			VALUES ($1, $2, $3, 'admin', 'activo', NOW())`,
			personaID, cfg.Username, string(hash)); err != nil {
			return "", 0, fmt.Errorf("insert usuario: %w", err)
		}
		return outcomeCreated, personaID, tx.Commit()
	case err != nil:
		return "", 0, err
	case u.PersonaID == personaID && u.Rol == "admin" && u.Estado == "activo":
		return outcomeExists, personaID, tx.Commit()
	case !cfg.Rebind && u.PersonaID != personaID:
		return "", 0, fmt.Errorf(
			"el usuario '%s' existe pero está ligado a la persona id=%d, que no es la persona dedicada del admin; "+
				"vuelva a ejecutar con ADMIN_REBIND=1 para corregirlo", cfg.Username, u.PersonaID)
	case !cfg.Rebind:
		// H96: no se da por bueno un usuario con ese username que no es admin
		// activo (p.ej. 'usuario'/'pendiente'): su contraseña no es la indicada.
		return "", 0, fmt.Errorf(
			"el usuario '%s' existe con rol=%q estado=%q (se esperaba admin/activo); "+
				"vuelva a ejecutar con ADMIN_REBIND=1 para forzarlo y rotar su contraseña", cfg.Username, u.Rol, u.Estado)
	}

	// Rebind: liga el admin a su persona dedicada y fuerza rol='admin' y
	// estado='activo' (H96). Como la cuenta pudo haber sido recuperada por el
	// dueño del email anterior (o creada por otra vía), además rota la
	// contraseña a la indicada, cierra sus sesiones e invalida los tokens de
	// recuperación pendientes.
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.Password), bcryptCost)
	if err != nil {
		return "", 0, fmt.Errorf("bcrypt: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE usuarios
		SET persona_id = $1, password_hash = $2, rol = 'admin', estado = 'activo',
		    activated_at = COALESCE(activated_at, NOW()), updated_at = NOW()
		WHERE id = $3`,
		personaID, string(hash), u.ID); err != nil {
		return "", 0, fmt.Errorf("rebind: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sesiones WHERE usuario_id = $1`, u.ID); err != nil {
		return "", 0, fmt.Errorf("cerrar sesiones: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE tokens_recuperacion SET usado = TRUE WHERE usuario_id = $1 AND NOT usado`, u.ID); err != nil {
		return "", 0, fmt.Errorf("invalidar tokens: %w", err)
	}
	return outcomeRebound, personaID, tx.Commit()
}

// dedicatedPersona devuelve la persona con dni_hmac del admin, creándola si no
// existe. Si existe, su email debe ser ADMIN_EMAIL y no puede tener otro
// usuario que no sea el admin.
func dedicatedPersona(ctx context.Context, tx *sqlx.Tx, gcm cipher.AEAD, rawKey []byte, cfg config, dniHMAC, emailHMAC string) (int64, error) {
	var p struct {
		ID        int64          `db:"id"`
		EmailHMAC string         `db:"email_hmac"`
		Username  sql.NullString `db:"username"`
	}
	err := tx.GetContext(ctx, &p, `
		SELECT p.id, p.email_hmac, u.username
		FROM personas p LEFT JOIN usuarios u ON u.persona_id = p.id
		WHERE p.dni_hmac = $1`, dniHMAC)
	if err == nil {
		if p.EmailHMAC != emailHMAC {
			return 0, fmt.Errorf("la persona id=%d (ADMIN_DNI) tiene otro email; no se liga el admin a ella", p.ID)
		}
		if p.Username.Valid && p.Username.String != cfg.Username {
			return 0, fmt.Errorf("la persona id=%d ya tiene otro usuario", p.ID)
		}
		return p.ID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	inst, grado, esp := cfg.InstitucionID, cfg.GradoID, cfg.EspecialidadID
	if inst == 0 {
		if err := tx.GetContext(ctx, &inst, `SELECT id FROM instituciones ORDER BY id LIMIT 1`); err != nil {
			return 0, fmt.Errorf("no hay instituciones (cargue catálogos o fije ADMIN_INSTITUCION_ID): %w", err)
		}
	}
	if grado == 0 {
		if err := tx.GetContext(ctx, &grado, `SELECT id FROM grados WHERE institucion_id = $1 ORDER BY id LIMIT 1`, inst); err != nil {
			return 0, fmt.Errorf("no hay grados para la institución %d (fije ADMIN_GRADO_ID): %w", inst, err)
		}
	}
	if esp == 0 {
		if err := tx.GetContext(ctx, &esp, `SELECT id FROM especialidades WHERE institucion_id = $1 ORDER BY id LIMIT 1`, inst); err != nil {
			return 0, fmt.Errorf("no hay especialidades para la institución %d (fije ADMIN_ESPECIALIDAD_ID): %w", inst, err)
		}
	}

	e, err := persona.Encrypt(gcm, rawKey, persona.Plain{
		DNI: cfg.DNI, Nombres: "Administrador", ApellidoP: "Sistema", ApellidoM: "Sistema",
		Telefono: "", Email: cfg.Email,
	})
	if err != nil {
		return 0, err
	}
	if e.DNIHMAC != dniHMAC || e.EmailHMAC != emailHMAC {
		return 0, errors.New("inconsistencia de normalización de HMAC")
	}
	var id int64
	if err := tx.GetContext(ctx, &id, `
		INSERT INTO personas
			(dni, dni_hmac, nombres, apellido_paterno, apellido_materno,
			 grado_id, especialidad_id, institucion_id, telefono, email, email_hmac, estado)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'activo')
		RETURNING id`,
		e.DNI, e.DNIHMAC, e.Nombres, e.ApellidoP, e.ApellidoM,
		grado, esp, inst, e.Telefono, e.Email, e.EmailHMAC); err != nil {
		return 0, fmt.Errorf("insert persona: %w", err)
	}
	return id, nil
}
