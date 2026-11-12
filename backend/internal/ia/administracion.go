package ia

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// Audit of the configuration change (contract E6-02 §3.3).
const (
	AccionConfigurar     = "ia.configurar"
	EntidadConfiguracion = "ia_configuracion"
	idConfiguracion      = int64(1)
)

// Evento is one audit_log row of a configuration change.
type Evento struct {
	Accion    string
	Entidad   string
	EntidadID int64
	Detalle   map[string]any
}

// Auditor writes ev inside the change's transaction; its error rolls the
// change back. The handler builds it with the admin, IP and user agent.
type Auditor func(ctx context.Context, tx sqlx.ExecerContext, ev Evento) error

// ActualizarConfiguracion is the body of PUT /admin/ia/configuracion: all the
// fields, as a full replacement. A pointer tells an absent field from a 0.
type ActualizarConfiguracion struct {
	LimiteUsuarioDia      *int     `json:"limite_usuario_dia"`
	LimiteUsuarioMes      *int     `json:"limite_usuario_mes"`
	LimiteAdminDia        *int     `json:"limite_admin_dia"`
	MaxPreguntasSolicitud *int     `json:"max_preguntas_solicitud"`
	LoteClasificacion     *int     `json:"lote_clasificacion"`
	PresupuestoMensualUSD *float64 `json:"presupuesto_mensual_usd"`
}

// Rules of detalle.campos in the 422 validacion (README §3).
const (
	reglaRequerido = "requerido"
	reglaMin       = "min"
	reglaMax       = "max"
	reglaDecimales = "dec2"
	reglaDiaMes    = "dia_mayor_que_mes"
)

// ErrValidacion is the 422 validacion; ValidacionError matches it.
var ErrValidacion = errors.New("validacion")

// ValidacionError lists the fields that broke a rule (field → rule).
type ValidacionError struct{ Campos map[string]string }

func (e *ValidacionError) Error() string {
	nombres := make([]string, 0, len(e.Campos))
	for campo := range e.Campos {
		nombres = append(nombres, campo)
	}
	sort.Strings(nombres)
	return ErrValidacion.Error() + ": " + strings.Join(nombres, ", ")
}

func (e *ValidacionError) Unwrap() error { return ErrValidacion }

// rango is the allowed interval of an integer field (D19).
type rango struct{ min, max int }

var rangos = map[string]rango{
	"limite_usuario_dia":      {0, 50},
	"limite_usuario_mes":      {0, 1000},
	"limite_admin_dia":        {1, 1000},
	"max_preguntas_solicitud": {1, 50},
	"lote_clasificacion":      {1, 50},
}

const (
	presupuestoMin = 1.0
	presupuestoMax = 1000.0
)

// validar checks every field with all of them present, and returns the new
// configuration. The error lists every broken field, not only the first.
func (r ActualizarConfiguracion) validar() (Configuracion, error) {
	campos := map[string]string{}
	entero := func(nombre string, v *int) int {
		if v == nil {
			campos[nombre] = reglaRequerido
			return 0
		}
		switch lim := rangos[nombre]; {
		case *v < lim.min:
			campos[nombre] = reglaMin
		case *v > lim.max:
			campos[nombre] = reglaMax
		}
		return *v
	}
	c := Configuracion{
		LimiteUsuarioDia:      entero("limite_usuario_dia", r.LimiteUsuarioDia),
		LimiteUsuarioMes:      entero("limite_usuario_mes", r.LimiteUsuarioMes),
		LimiteAdminDia:        entero("limite_admin_dia", r.LimiteAdminDia),
		MaxPreguntasSolicitud: entero("max_preguntas_solicitud", r.MaxPreguntasSolicitud),
		LoteClasificacion:     entero("lote_clasificacion", r.LoteClasificacion),
	}
	switch p := r.PresupuestoMensualUSD; {
	case p == nil:
		campos["presupuesto_mensual_usd"] = reglaRequerido
	case !(*p >= presupuestoMin):
		campos["presupuesto_mensual_usd"] = reglaMin
	case *p > presupuestoMax:
		campos["presupuesto_mensual_usd"] = reglaMax
	case !tieneDosDecimales(*p):
		campos["presupuesto_mensual_usd"] = reglaDecimales
	default:
		c.PresupuestoMensualUSD = math.Round(*p*100) / 100
	}
	_, diaRoto := campos["limite_usuario_dia"]
	_, mesRoto := campos["limite_usuario_mes"]
	if !diaRoto && !mesRoto && c.LimiteUsuarioDia > c.LimiteUsuarioMes {
		campos["limite_usuario_dia"] = reglaDiaMes
	}
	if len(campos) > 0 {
		return Configuracion{}, &ValidacionError{Campos: campos}
	}
	return c, nil
}

// tieneDosDecimales: the value has at most 2 decimals (same rule as dec2).
func tieneDosDecimales(v float64) bool {
	return math.Abs(v*100-math.Round(v*100)) < 1e-6
}

// ActualizarConfiguracion replaces the configuration (contract §3.3): it
// locks the row, audits only what changed in the same transaction, and, if
// the budget changed, evaluates the 80 % alert for the new value (regla 18).
// With nothing changed it answers the current configuration and audits nothing.
func (c *PgCostos) ActualizarConfiguracion(ctx context.Context, adminID int64, req ActualizarConfiguracion, aud Auditor) (Configuracion, error) {
	nueva, err := req.validar()
	if err != nil {
		return Configuracion{}, err
	}
	tx, err := c.db.BeginTxx(ctx, nil)
	if err != nil {
		return Configuracion{}, fmt.Errorf("actualizar configuración de ia: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var antes Configuracion
	if err := tx.GetContext(ctx, &antes, `SELECT `+columnasConfiguracion+` FROM ia_configuracion WHERE id = 1 FOR UPDATE`); err != nil {
		return Configuracion{}, fmt.Errorf("bloquear la configuración de ia: %w", err)
	}
	cambios := diferencias(antes, nueva)
	if len(cambios) == 0 {
		return antes, nil
	}
	var actual Configuracion
	if err := tx.GetContext(ctx, &actual, `
		UPDATE ia_configuracion SET limite_usuario_dia = $1, limite_usuario_mes = $2, limite_admin_dia = $3,
		       max_preguntas_solicitud = $4, lote_clasificacion = $5, presupuesto_mensual_usd = $6,
		       updated_by = $7, updated_at = NOW()
		WHERE id = 1 RETURNING `+columnasConfiguracion,
		nueva.LimiteUsuarioDia, nueva.LimiteUsuarioMes, nueva.LimiteAdminDia, nueva.MaxPreguntasSolicitud,
		nueva.LoteClasificacion, nueva.PresupuestoMensualUSD, adminID); err != nil {
		return Configuracion{}, fmt.Errorf("guardar la configuración de ia: %w", err)
	}
	if _, cambioPresupuesto := cambios["presupuesto_mensual_usd"]; cambioPresupuesto {
		if err := c.rearmarAlerta(ctx, tx); err != nil {
			return Configuracion{}, err
		}
	}
	if aud != nil {
		ev := Evento{Accion: AccionConfigurar, Entidad: EntidadConfiguracion, EntidadID: idConfiguracion,
			Detalle: map[string]any{"cambios": cambios}}
		if err := aud(ctx, tx, ev); err != nil {
			return Configuracion{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Configuracion{}, fmt.Errorf("actualizar configuración de ia: %w", err)
	}
	return actual, nil
}

// rearmarAlerta locks the month's spent amount, if there is any, and
// evaluates the 80 % alert against the new budget (contract §2.4).
func (c *PgCostos) rearmarAlerta(ctx context.Context, tx *sqlx.Tx) error {
	_, _, primerDia := c.ventanas()
	var existe int
	err := tx.GetContext(ctx, &existe, `SELECT 1 FROM ia_gasto_mes WHERE mes = $1 FOR UPDATE`, primerDia)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // nothing spent this month: nothing to alert about
	}
	if err != nil {
		return fmt.Errorf("bloquear el gasto del mes: %w", err)
	}
	return evaluarAlerta80(ctx, tx, primerDia)
}

// diferencias lists the fields that changed as {campo: {antes, despues}}.
func diferencias(antes, despues Configuracion) map[string]map[string]any {
	cambios := map[string]map[string]any{}
	enteros := []struct {
		nombre         string
		antes, despues int
	}{
		{"limite_usuario_dia", antes.LimiteUsuarioDia, despues.LimiteUsuarioDia},
		{"limite_usuario_mes", antes.LimiteUsuarioMes, despues.LimiteUsuarioMes},
		{"limite_admin_dia", antes.LimiteAdminDia, despues.LimiteAdminDia},
		{"max_preguntas_solicitud", antes.MaxPreguntasSolicitud, despues.MaxPreguntasSolicitud},
		{"lote_clasificacion", antes.LoteClasificacion, despues.LoteClasificacion},
	}
	for _, e := range enteros {
		if e.antes != e.despues {
			cambios[e.nombre] = map[string]any{"antes": e.antes, "despues": e.despues}
		}
	}
	if antes.PresupuestoMensualUSD != despues.PresupuestoMensualUSD {
		cambios["presupuesto_mensual_usd"] = map[string]any{"antes": antes.PresupuestoMensualUSD, "despues": despues.PresupuestoMensualUSD}
	}
	return cambios
}

// Gasto is GET /admin/ia/gasto: the month in course (contract E6-02 §3.4).
// No figure is per user.
type Gasto struct {
	Mes            string           `json:"mes"`
	PresupuestoUSD float64          `json:"presupuesto_usd"`
	GastadoUSD     float64          `json:"gastado_usd"`
	Porcentaje     float64          `json:"porcentaje"`
	Alerta80At     *time.Time       `json:"alerta_80_at"`
	Agotado        bool             `json:"agotado"`
	ReiniciaAt     time.Time        `json:"reinicia_at"`
	PorOperacion   []GastoOperacion `json:"por_operacion"`
}

// GastoOperacion aggregates ia_usos of the month for one operation.
type GastoOperacion struct {
	Operacion    string  `db:"operacion" json:"operacion"`
	Invocaciones int     `db:"invocaciones" json:"invocaciones"`
	DesdeCache   int     `db:"desde_cache" json:"desde_cache"`
	CostoUSD     float64 `db:"costo_usd" json:"costo_usd"`
}

// Gasto reads the spent amount of the month (ia_gasto_mes is the value that
// rules) and the detail by operation (ia_usos, in Lima time).
func (c *PgCostos) Gasto(ctx context.Context) (Gasto, error) {
	_, mes, primerDia := c.ventanas()
	var f struct {
		Presupuesto float64    `db:"presupuesto"`
		Gastado     float64    `db:"gastado"`
		Porcentaje  float64    `db:"porcentaje"`
		Alerta80At  *time.Time `db:"alerta_80_at"`
		Agotado     bool       `db:"agotado"`
	}
	err := c.db.GetContext(ctx, &f, `
		SELECT c.presupuesto_mensual_usd::float8 AS presupuesto,
		       ROUND(COALESCE(g.gastado_usd, 0), 2)::float8 AS gastado,
		       ROUND(COALESCE(g.gastado_usd, 0) * 100 / c.presupuesto_mensual_usd, 2)::float8 AS porcentaje,
		       g.alerta_80_at,
		       COALESCE(g.gastado_usd, 0) >= c.presupuesto_mensual_usd AS agotado
		FROM ia_configuracion c LEFT JOIN ia_gasto_mes g ON g.mes = $1
		WHERE c.id = 1`, primerDia)
	if err != nil {
		return Gasto{}, fmt.Errorf("leer el gasto de ia: %w", err)
	}
	ops := []GastoOperacion{}
	if err := c.db.SelectContext(ctx, &ops, `
		SELECT operacion, COUNT(*)::int AS invocaciones, COUNT(*) FILTER (WHERE desde_cache)::int AS desde_cache,
		       ROUND(COALESCE(SUM(costo_usd), 0), 2)::float8 AS costo_usd
		FROM ia_usos WHERE created_at >= $1 AND created_at < $2
		GROUP BY operacion ORDER BY operacion`, mes.Inicio, mes.Fin); err != nil {
		return Gasto{}, fmt.Errorf("leer el uso de ia por operación: %w", err)
	}
	return Gasto{
		Mes:            primerDia.Format("2006-01"),
		PresupuestoUSD: f.Presupuesto, GastadoUSD: f.Gastado, Porcentaje: f.Porcentaje,
		Alerta80At: f.Alerta80At, Agotado: f.Agotado, ReiniciaAt: mes.Fin, PorOperacion: ops,
	}, nil
}
