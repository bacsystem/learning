package ia_test

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/cbaciliod/learning/internal/ia"
)

func gastar(t *testing.T, c *ia.PgCostos, usd float64) {
	t.Helper()
	if err := c.SumarGasto(context.Background(), usd); err != nil {
		t.Fatalf("SumarGasto(%v): %v", usd, err)
	}
}

func gastado(t *testing.T, db *sqlx.DB) string {
	t.Helper()
	var g string
	if err := db.Get(&g, `SELECT COALESCE(SUM(gastado_usd), 0)::text FROM ia_gasto_mes`); err != nil {
		t.Fatal(err)
	}
	return g
}

type alerta struct {
	Cuerpo       string `db:"cuerpo"`
	Tipo         string `db:"tipo"`
	Destinatario string `db:"destinatario"`
	CreadoPor    *int64 `db:"creado_por"`
}

func alertas(t *testing.T, db *sqlx.DB) []alerta {
	t.Helper()
	var as []alerta
	if err := db.Select(&as, `SELECT cuerpo, tipo, destinatario, creado_por FROM notificaciones WHERE titulo = $1 ORDER BY id`, tituloAlerta); err != nil {
		t.Fatal(err)
	}
	return as
}

func esPresupuesto(t *testing.T, err error) *ia.ErrorPresupuesto {
	t.Helper()
	var ep *ia.ErrorPresupuesto
	if !errors.As(err, &ep) || !errors.Is(err, ia.ErrPresupuestoAgotado) {
		t.Fatalf("err = %v, want *ErrorPresupuesto", err)
	}
	return ep
}

// Escenario «Corte al 100 %»: the cut is spent ≥ budget, so the exact amount
// already cuts; below it, a cent short, it does not.
func TestPresupuesto_CorteAlCien(t *testing.T) {
	db := entornoCostos(t)
	c := nuevoPgCostos(t, db, reloj(mediodia))
	gastar(t, c, 39.99)
	if err := c.ComprobarPresupuesto(context.Background()); err != nil {
		t.Fatalf("39.99 of 40: %v", err)
	}
	gastar(t, c, 0.01)
	ep := esPresupuesto(t, c.ComprobarPresupuesto(context.Background()))
	if !ep.ReiniciaAt.Equal(time.Date(2026, 12, 1, 5, 0, 0, 0, time.UTC)) {
		t.Errorf("reinicia_at = %v", ep.ReiniciaAt)
	}
	// Every reservation is refused, also the system's and a user with quota left.
	for _, a := range []ia.Actor{{Rol: ia.RolSistema}, usuarioDePrueba(t, db, ia.RolUsuario), usuarioDePrueba(t, db, ia.RolAdmin)} {
		_, err := c.Reservar(context.Background(), a, 1)
		_ = esPresupuesto(t, err)
	}
	// D14: what was already in flight may pass the budget.
	gastar(t, c, 5)
	if g := gastado(t, db); g != "45.000000" {
		t.Errorf("spent = %s, want 45.000000", g)
	}
}

// D16: the budget is the first check, before the month and the day.
func TestPresupuesto_PrecedenciaSobreLosLimites(t *testing.T) {
	db := entornoCostos(t)
	configurar(t, db, `limite_usuario_dia = 1, limite_usuario_mes = 1`)
	c := nuevoPgCostos(t, db, reloj(mediodia))
	a := usuarioDePrueba(t, db, ia.RolUsuario)
	reservar(t, c, a, 1)
	_, err := c.Reservar(context.Background(), a, 1)
	_ = esLimite(t, err, ia.VentanaMes)
	gastar(t, c, 40)
	_, err = c.Reservar(context.Background(), a, 1)
	_ = esPresupuesto(t, err)
}

// Regla 18 and D19: raising the budget reopens the calls at once, lowering it
// to the spent amount or below cuts them at once.
func TestPresupuesto_CambiarElPresupuesto(t *testing.T) {
	db := entornoCostos(t)
	c := nuevoPgCostos(t, db, reloj(mediodia))
	gastar(t, c, 40)
	_ = esPresupuesto(t, c.ComprobarPresupuesto(context.Background()))
	configurar(t, db, `presupuesto_mensual_usd = 50.00`)
	if err := c.ComprobarPresupuesto(context.Background()); err != nil {
		t.Errorf("after raising to 50: %v", err)
	}
	configurar(t, db, `presupuesto_mensual_usd = 40.00`)
	_ = esPresupuesto(t, c.ComprobarPresupuesto(context.Background()))
	configurar(t, db, `presupuesto_mensual_usd = 24.00`)
	_ = esPresupuesto(t, c.ComprobarPresupuesto(context.Background()))
}

// Escenario «Reinicio mensual»: a new month starts at 0, at Lima midnight.
func TestPresupuesto_ReinicioMensual(t *testing.T) {
	db := entornoCostos(t)
	antes := nuevoPgCostos(t, db, reloj(time.Date(2026, 12, 1, 4, 59, 0, 0, time.UTC)))
	gastar(t, antes, 40)
	_ = esPresupuesto(t, antes.ComprobarPresupuesto(context.Background()))
	despues := nuevoPgCostos(t, db, reloj(time.Date(2026, 12, 1, 5, 0, 0, 0, time.UTC)))
	if err := despues.ComprobarPresupuesto(context.Background()); err != nil {
		t.Errorf("first instant of the month: %v", err)
	}
	gastar(t, despues, 1)
	var meses int
	_ = db.Get(&meses, `SELECT COUNT(*) FROM ia_gasto_mes`)
	if meses != 2 {
		t.Errorf("%d month rows, want 2 (November and December)", meses)
	}
}

// The sums are exact (NUMERIC), not binary floats; costs that are not a
// positive finite number add nothing; a cancelled ctx still records (D15);
// and the amount does not depend on ia_usos (D18).
func TestSumarGasto_ExactitudYBordes(t *testing.T) {
	db := entornoCostos(t)
	c := nuevoPgCostos(t, db, reloj(mediodia))
	for i := 0; i < 10; i++ {
		gastar(t, c, 0.1)
	}
	if g := gastado(t, db); g != "1.000000" {
		t.Errorf("ten times 0.1 = %s, want 1.000000", g)
	}
	for _, usd := range []float64{0, -3, math.NaN(), math.Inf(1)} {
		gastar(t, c, usd)
	}
	if g := gastado(t, db); g != "1.000000" {
		t.Errorf("invalid costs changed the amount to %s", g)
	}
	cancelado, cancelar := context.WithCancel(context.Background())
	cancelar()
	if err := c.SumarGasto(cancelado, 0.5); err != nil {
		t.Fatalf("SumarGasto with a cancelled ctx: %v", err)
	}
	// The amount does not depend on ia_usos: a usage row of this test is
	// inserted and deleted (only its own), and the amount stays.
	var uso int64
	if err := db.Get(&uso, `INSERT INTO ia_usos (operacion, actor_rol, proveedor, duracion_ms, costo_usd, resultado)
		VALUES ('clasificar', 'sistema', 'doble', 1, 0.5, 'ok') RETURNING id`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM ia_usos WHERE id = $1`, uso); err != nil {
		t.Fatal(err)
	}
	if g := gastado(t, db); g != "1.500000" {
		t.Errorf("amount = %s, want 1.500000 (cancelled ctx recorded, ia_usos deletion ignored)", g)
	}
}

// Escenario «Alerta al 80 %»: 31.90 → 32.10 sends one notification, with the
// exact text, to the admins and from the system; more spending does not repeat it.
func TestAlerta80_UnaSolaVez(t *testing.T) {
	db := entornoCostos(t)
	c := nuevoPgCostos(t, db, reloj(mediodia))
	gastar(t, c, 31.90)
	if n := len(alertas(t, db)); n != 0 {
		t.Fatalf("%d alerts at 79.75 %%", n)
	}
	gastar(t, c, 0.20)
	as := alertas(t, db)
	if len(as) != 1 {
		t.Fatalf("%d alerts after crossing 80 %%, want 1", len(as))
	}
	if as[0].Cuerpo != "Se gastaron 32,10 USD de 40,00 USD en noviembre de 2026." ||
		as[0].Tipo != "sistema" || as[0].Destinatario != "rol:admin" || as[0].CreadoPor != nil {
		t.Errorf("alert = %+v", as[0])
	}
	gastar(t, c, 3)
	gastar(t, c, 10)
	if n := len(alertas(t, db)); n != 1 {
		t.Errorf("%d alerts after more spending, want still 1", n)
	}
}

// The threshold is ≥ 80 %: 31.999999 does not alert, 32.000000 does.
func TestAlerta80_Borde(t *testing.T) {
	db := entornoCostos(t)
	c := nuevoPgCostos(t, db, reloj(mediodia))
	gastar(t, c, 31.999999)
	if n := len(alertas(t, db)); n != 0 {
		t.Fatalf("%d alerts just under 80 %%", n)
	}
	gastar(t, c, 0.000001)
	if n := len(alertas(t, db)); n != 1 {
		t.Errorf("%d alerts at exactly 80 %%, want 1", n)
	}
}

// Two calls whose costs cross the threshold together send one alert.
func TestAlerta80_Concurrencia(t *testing.T) {
	db := entornoCostos(t)
	c := nuevoPgCostos(t, db, reloj(mediodia))
	gastar(t, c, 31.90)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.SumarGasto(context.Background(), 0.2); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if n := len(alertas(t, db)); n != 1 {
		t.Errorf("%d alerts from concurrent sums, want 1", n)
	}
	if g := gastado(t, db); g != "32.700000" {
		t.Errorf("amount = %s, want 32.700000 (no sum lost)", g)
	}
}

// D19, regla 18: changing the budget re-arms the alert for the new value.
func TestAlerta80_UnaPorValorDelPresupuesto(t *testing.T) {
	db := entornoCostos(t)
	c := nuevoPgCostos(t, db, reloj(mediodia))
	gastar(t, c, 33)
	if n := len(alertas(t, db)); n != 1 {
		t.Fatalf("%d alerts, want 1", n)
	}
	configurar(t, db, `presupuesto_mensual_usd = 41.00`) // 80 % = 32.80: already over
	gastar(t, c, 0.5)
	as := alertas(t, db)
	if len(as) != 2 || as[1].Cuerpo != "Se gastaron 33,50 USD de 41,00 USD en noviembre de 2026." {
		t.Errorf("alerts after changing the budget = %+v", as)
	}
	gastar(t, c, 0.5)
	if n := len(alertas(t, db)); n != 2 {
		t.Errorf("%d alerts, want 2 (one per budget value)", n)
	}
}
