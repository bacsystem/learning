package ia

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"github.com/cbaciliod/learning/internal/antifraude"
)

const (
	// tiempoCostos bounds the writes that must survive a cancelled consumer
	// ctx (Liberar and the spent amount): they run detached from it.
	tiempoCostos = 5 * time.Second
	// purgaCuotas is how many expired ia_cuotas rows each Reservar deletes.
	purgaCuotas = 100
)

// PgCostos implements Costos over PostgreSQL (migration 068): the counters
// are keyed by the HMAC of the actor (ADR 0002), the spent amount lives in
// ia_gasto_mes (not derived from ia_usos, ADR 0009).
type PgCostos struct {
	db     *sqlx.DB
	claves *antifraude.Claves
	lima   *time.Location
	ahora  func() time.Time
}

// OpcionCostos tunes PgCostos; tests inject the clock.
type OpcionCostos func(*PgCostos)

// ConRelojCostos sets the clock that decides the windows and the month.
func ConRelojCostos(ahora func() time.Time) OpcionCostos {
	return func(c *PgCostos) { c.ahora = ahora }
}

// NewPgCostos creates the cost control over db. It needs the tz database
// (cmd/server embeds it with time/tzdata).
func NewPgCostos(db *sqlx.DB, claves *antifraude.Claves, opciones ...OpcionCostos) (*PgCostos, error) {
	lima, err := time.LoadLocation(zonaNegocio)
	if err != nil {
		return nil, fmt.Errorf("zona %s: %w", zonaNegocio, err)
	}
	c := &PgCostos{db: db, claves: claves, lima: lima, ahora: time.Now}
	for _, o := range opciones {
		o(c)
	}
	return c, nil
}

// ventanas of the clock's now, and the first day of the month as a date.
func (c *PgCostos) ventanas() (dia, mes ventana, primerDia time.Time) {
	now := c.ahora()
	dia, mes = ventanas(now, c.lima)
	l := mes.Inicio.In(c.lima)
	return dia, mes, time.Date(l.Year(), l.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// Reservar takes n actions all or nothing in one transaction. The first
// failure is the error and rolls everything back (contract §2.2).
func (c *PgCostos) Reservar(ctx context.Context, a Actor, n int) (Reserva, error) {
	if !actorValido(a) {
		return Reserva{}, ErrActorInvalido
	}
	if n < 1 {
		return Reserva{}, fmt.Errorf("reservar %d acciones: %w", n, ErrSinReserva)
	}
	dia, mes, primerDia := c.ventanas()
	tx, err := c.db.BeginTxx(ctx, nil)
	if err != nil {
		return Reserva{}, fmt.Errorf("reservar: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	cfg, err := leerConfiguracion(ctx, tx)
	if err != nil {
		return Reserva{}, err
	}
	if agotado, err := presupuestoAgotado(ctx, tx, primerDia); err != nil {
		return Reserva{}, err
	} else if agotado {
		return Reserva{}, &ErrorPresupuesto{ReiniciaAt: mes.Fin}
	}

	r := Reserva{Actor: a, Acciones: n, Dia: dia.Inicio}
	if a.Rol != RolSistema {
		vigente, anterior := c.claves.Clave(antifraude.PrefijoCuotaIA, *a.ID)
		if a.Rol == RolUsuario {
			r.Mes = mes.Inicio
			if err := reservarVentana(ctx, tx, VentanaMes, mes, cfg.LimiteUsuarioMes, n, vigente, anterior); err != nil {
				return Reserva{}, err
			}
		}
		limiteDia := cfg.LimiteUsuarioDia
		if a.Rol == RolAdmin {
			limiteDia = cfg.LimiteAdminDia
		}
		if err := reservarVentana(ctx, tx, VentanaDia, dia, limiteDia, n, vigente, anterior); err != nil {
			return Reserva{}, err
		}
	}
	if err := purgarCuotas(ctx, tx, c.ahora()); err != nil {
		return Reserva{}, err
	}
	if err := tx.Commit(); err != nil {
		return Reserva{}, fmt.Errorf("reservar: %w", err)
	}
	return r, nil
}

// reservarVentana adds n to the counter of the window if the total, with the
// previous key's counter during a rotation, stays within limite. The check
// and the increment are one statement, so concurrent reservations cannot
// exceed the limit (contract §2.2).
func reservarVentana(ctx context.Context, tx *sqlx.Tx, nombre string, w ventana, limite, n int, vigente, anterior string) error {
	previos, err := usadosDe(ctx, tx, nombre, w, anterior)
	if err != nil {
		return err
	}
	if efectivo := limite - previos; n <= efectivo {
		var usados int
		err := tx.GetContext(ctx, &usados, `
			INSERT INTO ia_cuotas (clave, ventana, ventana_inicio, usados, expira_en)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (clave, ventana, ventana_inicio)
			DO UPDATE SET usados = ia_cuotas.usados + $4
			WHERE ia_cuotas.usados + $4 <= $6
			RETURNING usados`, vigente, nombre, w.Inicio, n, w.Fin, efectivo)
		if err == nil {
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("reservar %s: %w", nombre, err)
		}
	}
	actuales, err := usadosDe(ctx, tx, nombre, w, vigente)
	if err != nil {
		return err
	}
	return &ErrLimite{Ventana: nombre, Limite: limite, Usados: actuales + previos, ReiniciaAt: w.Fin}
}

// usadosDe reads one counter; an empty key (no rotation) or a missing row is 0.
func usadosDe(ctx context.Context, q sqlx.QueryerContext, nombre string, w ventana, clave string) (int, error) {
	if clave == "" {
		return 0, nil
	}
	var usados int
	err := sqlx.GetContext(ctx, q, &usados,
		`SELECT usados FROM ia_cuotas WHERE clave = $1 AND ventana = $2 AND ventana_inicio = $3`, clave, nombre, w.Inicio)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("leer cuota %s: %w", nombre, err)
	}
	return usados, nil
}

// purgarCuotas deletes up to purgaCuotas expired counters (lazy, like 056).
func purgarCuotas(ctx context.Context, tx *sqlx.Tx, ahora time.Time) error {
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM ia_cuotas WHERE ctid IN
		  (SELECT ctid FROM ia_cuotas WHERE expira_en < $1 LIMIT $2)`, ahora, purgaCuotas); err != nil {
		return fmt.Errorf("purgar cuotas: %w", err)
	}
	return nil
}

// Liberar gives n of the reserved actions back: it lowers the counters of
// the windows of r (never below 0) and r.Acciones. It runs detached from ctx:
// a consumer that was cancelled or timed out must still release its action.
func (c *PgCostos) Liberar(ctx context.Context, r *Reserva, n int) error {
	if r == nil || n < 1 {
		return nil
	}
	n = min(n, r.Acciones)
	if n < 1 {
		return nil
	}
	if r.Actor.Rol != RolSistema && r.Actor.ID != nil {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), tiempoCostos)
		defer cancel()
		vigente, anterior := c.claves.Clave(antifraude.PrefijoCuotaIA, *r.Actor.ID)
		ventanas := []ventanaReserva{{VentanaDia, r.Dia}}
		if !r.Mes.IsZero() {
			ventanas = append(ventanas, ventanaReserva{VentanaMes, r.Mes})
		}
		tx, err := c.db.BeginTxx(ctx, nil)
		if err != nil {
			return fmt.Errorf("liberar: %w", err)
		}
		defer func() { _ = tx.Rollback() }()
		for _, v := range ventanas {
			if err := restarUsados(ctx, tx, v.nombre, v.inicio, n, vigente, anterior); err != nil {
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("liberar: %w", err)
		}
	}
	r.Acciones -= n
	return nil
}

// ventanaReserva is a window a reservation was counted in.
type ventanaReserva struct {
	nombre string
	inicio time.Time
}

// restarUsados lowers the counter under the current key and, if that row
// does not exist (it was written before a rotation), under the previous one.
func restarUsados(ctx context.Context, tx *sqlx.Tx, nombre string, inicio time.Time, n int, vigente, anterior string) error {
	for _, clave := range []string{vigente, anterior} {
		if clave == "" {
			continue
		}
		res, err := tx.ExecContext(ctx, `
			UPDATE ia_cuotas SET usados = GREATEST(usados - $1, 0)
			WHERE clave = $2 AND ventana = $3 AND ventana_inicio = $4`, n, clave, nombre, inicio)
		if err != nil {
			return fmt.Errorf("liberar %s: %w", nombre, err)
		}
		if filas, err := res.RowsAffected(); err == nil && filas > 0 {
			return nil
		}
	}
	return nil
}

// Consumo adds the counters of the current and the previous key.
func (c *PgCostos) Consumo(ctx context.Context, a Actor) (Consumo, error) {
	if !actorValido(a) {
		return Consumo{}, ErrActorInvalido
	}
	dia, mes, primerDia := c.ventanas()
	cfg, err := leerConfiguracion(ctx, c.db)
	if err != nil {
		return Consumo{}, err
	}
	agotado, err := presupuestoAgotado(ctx, c.db, primerDia)
	if err != nil {
		return Consumo{}, err
	}
	out := Consumo{PresupuestoAgotado: agotado}
	if a.Rol == RolSistema {
		return out, nil
	}
	vigente, anterior := c.claves.Clave(antifraude.PrefijoCuotaIA, *a.ID)
	cuota := func(nombre string, w ventana, limite int) (*Cuota, error) {
		usados, err := usadosSumados(ctx, c.db, nombre, w, vigente, anterior)
		if err != nil {
			return nil, err
		}
		return &Cuota{Usados: usados, Limite: limite, Disponibles: max(limite-usados, 0), ReiniciaAt: w.Fin}, nil
	}
	limiteDia := cfg.LimiteUsuarioDia
	if a.Rol == RolAdmin {
		limiteDia = cfg.LimiteAdminDia
	}
	if out.Dia, err = cuota(VentanaDia, dia, limiteDia); err != nil {
		return Consumo{}, err
	}
	if a.Rol == RolUsuario {
		if out.Mes, err = cuota(VentanaMes, mes, cfg.LimiteUsuarioMes); err != nil {
			return Consumo{}, err
		}
	}
	return out, nil
}

func usadosSumados(ctx context.Context, q sqlx.QueryerContext, nombre string, w ventana, vigente, anterior string) (int, error) {
	total := 0
	for _, clave := range []string{vigente, anterior} {
		n, err := usadosDe(ctx, q, nombre, w, clave)
		if err != nil {
			return 0, err
		}
		total += n
	}
	return total, nil
}

// ComprobarPresupuesto answers *ErrorPresupuesto when the spent amount of the
// month is at or over the budget (D14), checked before each provider call.
func (c *PgCostos) ComprobarPresupuesto(ctx context.Context) error {
	_, mes, primerDia := c.ventanas()
	agotado, err := presupuestoAgotado(ctx, c.db, primerDia)
	if err != nil {
		return err
	}
	if agotado {
		return &ErrorPresupuesto{ReiniciaAt: mes.Fin}
	}
	return nil
}

// presupuestoAgotado compares in SQL, over NUMERIC: spent ≥ budget. A month
// without row has spent nothing.
func presupuestoAgotado(ctx context.Context, q sqlx.QueryerContext, primerDia time.Time) (bool, error) {
	var agotado bool
	err := sqlx.GetContext(ctx, q, &agotado, `
		SELECT COALESCE(g.gastado_usd, 0) >= c.presupuesto_mensual_usd
		FROM ia_configuracion c LEFT JOIN ia_gasto_mes g ON g.mes = $1
		WHERE c.id = 1`, primerDia)
	if err != nil {
		return false, fmt.Errorf("comprobar presupuesto: %w", err)
	}
	return agotado, nil
}

// SumarGasto adds usd to the spent amount of the month and evaluates the 80 %
// alert in the same transaction (contract §2.3 ★9). It runs detached from ctx
// so a cancelled call still records what it cost (D15). Costs that are not a
// positive finite number add nothing.
func (c *PgCostos) SumarGasto(ctx context.Context, usd float64) error {
	if !(usd > 0) || math.IsInf(usd, 0) {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), tiempoCostos)
	defer cancel()
	_, _, primerDia := c.ventanas()
	tx, err := c.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sumar gasto: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	// The month's row is created if it does not exist and locked, so two
	// concurrent sums serialize and only one sees the alert's threshold crossed.
	if _, err := tx.ExecContext(ctx, `INSERT INTO ia_gasto_mes (mes) VALUES ($1) ON CONFLICT (mes) DO NOTHING`, primerDia); err != nil {
		return fmt.Errorf("sumar gasto: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `SELECT 1 FROM ia_gasto_mes WHERE mes = $1 FOR UPDATE`, primerDia); err != nil {
		return fmt.Errorf("sumar gasto: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE ia_gasto_mes SET gastado_usd = gastado_usd + $2::numeric, updated_at = NOW() WHERE mes = $1`,
		primerDia, strconv.FormatFloat(usd, 'f', 6, 64)); err != nil {
		return fmt.Errorf("sumar gasto: %w", err)
	}
	if err := evaluarAlerta80(ctx, tx, primerDia); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sumar gasto: %w", err)
	}
	return nil
}

var mesesES = [...]string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}

// evaluarAlerta80 sends the alert of the 80 % once per value of the budget
// (contract §2.4): when spent ≥ 0.8 × budget and the budget differs from the
// one already notified. The comparison and the mark are one statement over
// NUMERIC; the notification goes in the same transaction. The caller holds
// the month's row lock. It is also used when the admin changes the budget.
func evaluarAlerta80(ctx context.Context, tx *sqlx.Tx, primerDia time.Time) error {
	var f struct {
		Gastado     string `db:"gastado"`
		Presupuesto string `db:"presupuesto"`
	}
	err := tx.GetContext(ctx, &f, `
		UPDATE ia_gasto_mes g
		SET alerta_80_presupuesto_usd = c.presupuesto_mensual_usd, alerta_80_at = NOW()
		FROM ia_configuracion c
		WHERE g.mes = $1 AND c.id = 1
		  AND g.gastado_usd >= 0.8 * c.presupuesto_mensual_usd
		  AND g.alerta_80_presupuesto_usd IS DISTINCT FROM c.presupuesto_mensual_usd
		RETURNING to_char(g.gastado_usd, 'FM999999990.00') AS gastado,
		          to_char(c.presupuesto_mensual_usd, 'FM999999990.00') AS presupuesto`, primerDia)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("evaluar alerta de presupuesto: %w", err)
	}
	cuerpo := fmt.Sprintf("Se gastaron %s USD de %s USD en %s de %d.",
		coma(f.Gastado), coma(f.Presupuesto), mesesES[primerDia.Month()-1], primerDia.Year())
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO notificaciones (titulo, cuerpo, tipo, destinatario, creado_por)
		VALUES ('Presupuesto de IA al 80 %', $1, 'sistema', 'rol:admin', NULL)`, cuerpo); err != nil {
		return fmt.Errorf("notificar presupuesto: %w", err)
	}
	log.Warn().Str("op", "ia.alerta_presupuesto").Msg("ia: budget at 80 %")
	return nil
}

// coma writes a decimal number with the Spanish decimal comma.
func coma(n string) string {
	b := []byte(n)
	for i := range b {
		if b[i] == '.' {
			b[i] = ','
		}
	}
	return string(b)
}

// Configuracion reads the current limits and budget.
func (c *PgCostos) Configuracion(ctx context.Context) (Configuracion, error) {
	return leerConfiguracion(ctx, c.db)
}
