// cmd/seed/demo siembra datos 100 % sintéticos e idempotentes para la
// preproducción y el desarrollo local (deploy/local): catálogos, procesos con
// áreas que suman 100, pesos, preguntas, configuración de simulacro y de pago,
// usuarios ya verificados y activos con cada rol, e inscripciones.
//
// Uso:
//
//	ENV=preprod DATABASE_URL=... ENCRYPTION_KEY=... \
//	DEMO_ADMIN_PASSWORD=... DEMO_POSTULANTE_PASSWORD=... \
//	  go run ./cmd/seed/demo
//
// Guardias: solo corre con ENV exactamente "preprod" o "development" (aborta
// con production, test, vacío o cualquier otro valor) antes de abrir la base;
// las contraseñas salen de variables, sin valores por defecto; no lee ningún
// archivo (nunca assets/). No imprime contraseñas ni PII.
//
// Es idempotente: una corrida nueva completa lo que falte sin duplicar y fija
// la contraseña de los usuarios demo al valor de las variables.
package main

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/cbaciliod/learning/internal/crypto"
	"github.com/cbaciliod/learning/internal/db"
)

const (
	minPasswordLen = 12
	maxPasswordLen = 72 // límite de bcrypt
	anioPorDefecto = 2026
	minAnio        = 2024
	maxAnio        = 2099
	timeoutSiembra = 2 * time.Minute
)

// entornosPermitidos es la lista blanca exacta de ENV (sin recortar ni
// normalizar): un ENV mal escrito tiene que fallar cerrado.
var entornosPermitidos = map[string]bool{"preprod": true, "development": true}

type config struct {
	Env                string
	DatabaseURL        string
	EncryptionKey      string
	AdminPassword      string
	PostulantePassword string
	Anio               int
}

func main() {
	if err := run(); err != nil {
		log.Printf("seed/demo: %v", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := configFromEnv(os.Getenv)
	if err != nil {
		return err
	}
	gcm, err := crypto.NewCipher(cfg.EncryptionKey)
	if err != nil {
		return fmt.Errorf("ENCRYPTION_KEY: %w", err)
	}
	rawKey, err := hex.DecodeString(cfg.EncryptionKey)
	if err != nil {
		return fmt.Errorf("ENCRYPTION_KEY: %w", err)
	}
	conn, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("conectar a la base: %w", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeoutSiembra)
	defer cancel()
	res, err := sembrar(ctx, conn, gcm, rawKey, cfg)
	if err != nil {
		return err
	}
	log.Printf("seed/demo: listo (%s)", res)
	return nil
}

// configFromEnv validates every input before anything touches the database.
func configFromEnv(getenv func(string) string) (config, error) {
	cfg := config{
		Env:                getenv("ENV"),
		DatabaseURL:        getenv("DATABASE_URL"),
		EncryptionKey:      getenv("ENCRYPTION_KEY"),
		AdminPassword:      getenv("DEMO_ADMIN_PASSWORD"),
		PostulantePassword: getenv("DEMO_POSTULANTE_PASSWORD"),
		Anio:               anioPorDefecto,
	}
	if !entornosPermitidos[cfg.Env] {
		return config{}, fmt.Errorf("ENV=%q no permitido: seed/demo solo corre con ENV=preprod o ENV=development", cfg.Env)
	}
	if cfg.DatabaseURL == "" || cfg.EncryptionKey == "" {
		return config{}, errors.New("DATABASE_URL y ENCRYPTION_KEY son obligatorias")
	}
	for nombre, valor := range map[string]string{"DEMO_ADMIN_PASSWORD": cfg.AdminPassword, "DEMO_POSTULANTE_PASSWORD": cfg.PostulantePassword} {
		if n := len(valor); n < minPasswordLen || n > maxPasswordLen {
			return config{}, fmt.Errorf("%s es obligatoria y debe tener entre %d y %d caracteres", nombre, minPasswordLen, maxPasswordLen)
		}
	}
	if s := getenv("DEMO_ANIO"); s != "" {
		anio, err := strconv.Atoi(s)
		if err != nil || anio < minAnio || anio > maxAnio {
			return config{}, fmt.Errorf("DEMO_ANIO=%q debe ser un año entre %d y %d", s, minAnio, maxAnio)
		}
		cfg.Anio = anio
	}
	return cfg, nil
}
