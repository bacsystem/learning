package ia

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// Configuracion is the single row of ia_configuracion (contract E6-02 §3.2).
// The budget has two decimals; every comparison against the spent amount is
// done in SQL over NUMERIC, never over this float.
type Configuracion struct {
	LimiteUsuarioDia      int       `db:"limite_usuario_dia" json:"limite_usuario_dia"`
	LimiteUsuarioMes      int       `db:"limite_usuario_mes" json:"limite_usuario_mes"`
	LimiteAdminDia        int       `db:"limite_admin_dia" json:"limite_admin_dia"`
	MaxPreguntasSolicitud int       `db:"max_preguntas_solicitud" json:"max_preguntas_solicitud"`
	LoteClasificacion     int       `db:"lote_clasificacion" json:"lote_clasificacion"`
	PresupuestoMensualUSD float64   `db:"presupuesto_mensual_usd" json:"presupuesto_mensual_usd"`
	UpdatedAt             time.Time `db:"updated_at" json:"updated_at"`
}

const columnasConfiguracion = `limite_usuario_dia, limite_usuario_mes, limite_admin_dia, max_preguntas_solicitud,
	lote_clasificacion, presupuesto_mensual_usd::float8 AS presupuesto_mensual_usd, updated_at`

// leerConfiguracion reads the row; q is the pool or the transaction.
func leerConfiguracion(ctx context.Context, q sqlx.QueryerContext) (Configuracion, error) {
	var c Configuracion
	if err := sqlx.GetContext(ctx, q, &c, `SELECT `+columnasConfiguracion+` FROM ia_configuracion WHERE id = 1`); err != nil {
		return Configuracion{}, fmt.Errorf("leer configuración de ia: %w", err)
	}
	return c, nil
}
