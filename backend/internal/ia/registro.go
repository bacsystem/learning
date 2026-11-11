package ia

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// Results of ia_usos.resultado (contract §5).
const (
	ResultadoOK                     = "ok"
	ResultadoNoDisponible           = "no_disponible"
	ResultadoEntradaDemasiadoGrande = "entrada_demasiado_grande"
	ResultadoDatosPersonales        = "datos_personales"
	ResultadoTimeout                = "timeout"
	ResultadoSalidaInvalida         = "salida_invalida"
	ResultadoErrorProveedor         = "error_proveedor"
)

// tiempoRegistro bounds the usage INSERT, which runs detached from the
// caller's ctx: on a timeout that ctx is already done and the row is still
// required.
const tiempoRegistro = 5 * time.Second

// Uso is one ia_usos row: one per invocation, also for rejections. It never
// holds the prompt, the answer, the detected text nor error messages; tokens
// and cost add up every attempt.
type Uso struct {
	Operacion     string
	Actor         Actor
	Proveedor     string // "" when the service is disabled
	Modelo        string
	TokensEntrada int
	TokensSalida  int
	CostoUSD      float64
	Duracion      time.Duration
	Intentos      int
	Resultado     string
}

// PgRegistro writes ia_usos with the service's own pool, outside any
// transaction of the consumer, so the usage stays recorded even if the
// consumer rolls its operation back.
type PgRegistro struct {
	db *sqlx.DB
}

// NewPgRegistro creates the usage store over db.
func NewPgRegistro(db *sqlx.DB) *PgRegistro { return &PgRegistro{db: db} }

// Registrar inserts u on a context detached from ctx with its own deadline.
func (r *PgRegistro) Registrar(ctx context.Context, u Uso) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), tiempoRegistro)
	defer cancel()
	var modelo *string
	if u.Modelo != "" {
		modelo = &u.Modelo
	}
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO ia_usos (operacion, actor_rol, actor_id, proveedor, modelo, tokens_entrada, tokens_salida,
		                     costo_usd, duracion_ms, intentos, resultado, desde_cache)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, false)`,
		u.Operacion, u.Actor.Rol, u.Actor.ID, u.Proveedor, modelo, u.TokensEntrada, u.TokensSalida,
		u.CostoUSD, u.Duracion.Milliseconds(), u.Intentos, u.Resultado); err != nil {
		return fmt.Errorf("registrar uso de %s: %w", u.Operacion, err)
	}
	return nil
}
