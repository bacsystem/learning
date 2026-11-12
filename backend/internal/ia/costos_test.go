package ia_test

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/cbaciliod/learning/internal/antifraude"
	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/cbaciliod/learning/internal/ia"
)

// tituloAlerta is the title of the 80 % alert, only created by this feature.
const tituloAlerta = "Presupuesto de IA al 80 %"

// mediodia is 10:00 in Lima (15:00 UTC) of 12/11/2026, far from every border.
var mediodia = time.Date(2026, 11, 12, 15, 0, 0, 0, time.UTC)

func reloj(t time.Time) ia.OpcionCostos { return ia.ConRelojCostos(func() time.Time { return t }) }

// entornoCostos leaves ia_cuotas, ia_gasto_mes, the configuration and the
// notifications of the budget alert as the migration creates them, before and
// after the test.
func entornoCostos(t *testing.T) *sqlx.DB {
	t.Helper()
	db := dbtest.Open(t)
	limpiar := func() {
		for _, q := range []string{
			`DELETE FROM ia_cuotas`, `DELETE FROM ia_gasto_mes`,
			`UPDATE ia_configuracion SET limite_usuario_dia = 5, limite_usuario_mes = 60, limite_admin_dia = 200,
				max_preguntas_solicitud = 20, lote_clasificacion = 20, presupuesto_mensual_usd = 40.00 WHERE id = 1`,
		} {
			if _, err := db.Exec(q); err != nil {
				t.Errorf("limpiar: %v", err)
			}
		}
		if _, err := db.Exec(`DELETE FROM notificaciones WHERE titulo = $1`, tituloAlerta); err != nil {
			t.Errorf("limpiar notificaciones: %v", err)
		}
	}
	limpiar()
	t.Cleanup(limpiar)
	return db
}

func configurar(t *testing.T, db *sqlx.DB, asignacion string, args ...any) {
	t.Helper()
	if _, err := db.Exec(`UPDATE ia_configuracion SET `+asignacion+` WHERE id = 1`, args...); err != nil {
		t.Fatal(err)
	}
}

func usuarioDePrueba(t *testing.T, db *sqlx.DB, rol string) ia.Actor {
	t.Helper()
	u := dbtest.SeedUser(t, db, dbtest.SeedCatalog(t, db), map[string]string{ia.RolUsuario: "usuario", ia.RolAdmin: "admin"}[rol], "activo")
	return ia.Actor{Rol: rol, ID: &u.ID}
}

func reservar(t *testing.T, c *ia.PgCostos, a ia.Actor, n int) ia.Reserva {
	t.Helper()
	r, err := c.Reservar(context.Background(), a, n)
	if err != nil {
		t.Fatalf("Reservar(%d): %v", n, err)
	}
	return r
}

func esLimite(t *testing.T, err error, ventana string) *ia.ErrLimite {
	t.Helper()
	var lim *ia.ErrLimite
	if !errors.As(err, &lim) || lim.Ventana != ventana {
		t.Fatalf("err = %v, want *ErrLimite{%s}", err, ventana)
	}
	return lim
}

// Escenarios «Dentro del límite» y «Límite diario»: 5 actions a day for a
// usuario, then limite_ia with the data the consumer needs, and the reset at
// the next Lima midnight.
func TestReservar_LimiteDiario(t *testing.T) {
	db := entornoCostos(t)
	c := nuevoPgCostos(t, db, reloj(mediodia))
	a := usuarioDePrueba(t, db, ia.RolUsuario)
	for i := 0; i < 5; i++ {
		reservar(t, c, a, 1)
	}
	_, err := c.Reservar(context.Background(), a, 1)
	lim := esLimite(t, err, ia.VentanaDia)
	if lim.Limite != 5 || lim.Usados != 5 || !lim.ReiniciaAt.Equal(time.Date(2026, 11, 13, 5, 0, 0, 0, time.UTC)) {
		t.Errorf("limit data = %+v", lim)
	}
	if lim.Error() != "limite_ia" {
		t.Errorf("code = %q", lim.Error())
	}
	consumo, err := c.Consumo(context.Background(), a)
	if err != nil || consumo.Dia.Usados != 5 || consumo.Dia.Disponibles != 0 || consumo.Mes.Usados != 5 || consumo.Mes.Disponibles != 55 {
		t.Errorf("consumo = %+v, %+v (%v)", consumo.Dia, consumo.Mes, err)
	}
}

// Escenario «Límite mensual»: the month is checked before the day, so a
// usuario with both exhausted gets limite_ia_mensual.
func TestReservar_LimiteMensualAntesQueElDiario(t *testing.T) {
	db := entornoCostos(t)
	configurar(t, db, `limite_usuario_dia = 3, limite_usuario_mes = 3`)
	c := nuevoPgCostos(t, db, reloj(mediodia))
	a := usuarioDePrueba(t, db, ia.RolUsuario)
	reservar(t, c, a, 3)
	_, err := c.Reservar(context.Background(), a, 1)
	lim := esLimite(t, err, ia.VentanaMes)
	if lim.Error() != "limite_ia_mensual" || !lim.ReiniciaAt.Equal(time.Date(2026, 12, 1, 5, 0, 0, 0, time.UTC)) {
		t.Errorf("limit = %+v (%s)", lim, lim.Error())
	}
}

// D17: all or nothing. A batch that does not fit reserves nothing, not even
// the part of the window that did fit; and a batch over the whole limit fails.
func TestReservar_TodoONada(t *testing.T) {
	db := entornoCostos(t)
	c := nuevoPgCostos(t, db, reloj(mediodia))
	a := usuarioDePrueba(t, db, ia.RolUsuario)
	reservar(t, c, a, 3)
	for _, n := range []int{3, 6} {
		_, err := c.Reservar(context.Background(), a, n)
		_ = esLimite(t, err, ia.VentanaDia)
	}
	consumo, _ := c.Consumo(context.Background(), a)
	if consumo.Dia.Usados != 3 || consumo.Mes.Usados != 3 {
		t.Errorf("a failed reservation left usados day %d, month %d; want 3 and 3", consumo.Dia.Usados, consumo.Mes.Usados)
	}
	reservar(t, c, a, 2) // what fits still fits
}

// The day fails after the month was incremented: the month is rolled back.
func TestReservar_FalloDelDiaRevierteElMes(t *testing.T) {
	db := entornoCostos(t)
	configurar(t, db, `limite_usuario_dia = 1, limite_usuario_mes = 10`)
	c := nuevoPgCostos(t, db, reloj(mediodia))
	a := usuarioDePrueba(t, db, ia.RolUsuario)
	reservar(t, c, a, 1)
	_, err := c.Reservar(context.Background(), a, 1)
	_ = esLimite(t, err, ia.VentanaDia)
	if consumo, _ := c.Consumo(context.Background(), a); consumo.Mes.Usados != 1 {
		t.Errorf("month usados = %d, want 1 (the failed day rolled back the month)", consumo.Mes.Usados)
	}
}

// Escenario «Límite de admin» and the actors without month or counter.
func TestReservar_AdminYSistema(t *testing.T) {
	db := entornoCostos(t)
	configurar(t, db, `limite_admin_dia = 2`)
	c := nuevoPgCostos(t, db, reloj(mediodia))
	admin := usuarioDePrueba(t, db, ia.RolAdmin)
	r := reservar(t, c, admin, 2)
	if !r.Mes.IsZero() || r.Dia.IsZero() || r.Acciones != 2 {
		t.Errorf("admin reservation = %+v: want a day and no month", r)
	}
	_, err := c.Reservar(context.Background(), admin, 1)
	_ = esLimite(t, err, ia.VentanaDia)
	if consumo, _ := c.Consumo(context.Background(), admin); consumo.Mes != nil || consumo.Dia.Limite != 2 {
		t.Errorf("admin consumo = %+v", consumo)
	}

	var antes int
	_ = db.Get(&antes, `SELECT COUNT(*) FROM ia_cuotas`)
	rs := reservar(t, c, ia.Actor{Rol: ia.RolSistema}, 50) // no counter, no limit
	if rs.Acciones != 50 || !rs.Mes.IsZero() {
		t.Errorf("system reservation = %+v", rs)
	}
	var despues int
	_ = db.Get(&despues, `SELECT COUNT(*) FROM ia_cuotas`)
	if antes != despues {
		t.Errorf("the system wrote %d counters", despues-antes)
	}
}

func TestReservar_EntradasInvalidas(t *testing.T) {
	db := entornoCostos(t)
	c := nuevoPgCostos(t, db, reloj(mediodia))
	id := int64(9)
	tests := []struct {
		name string
		a    ia.Actor
		n    int
		want error
	}{
		{"actor without id", ia.Actor{Rol: ia.RolUsuario}, 1, ia.ErrActorInvalido},
		{"unknown role", ia.Actor{Rol: "otro", ID: &id}, 1, ia.ErrActorInvalido},
		{"zero actions", ia.Actor{Rol: ia.RolSistema}, 0, ia.ErrSinReserva},
		{"negative actions", ia.Actor{Rol: ia.RolSistema}, -1, ia.ErrSinReserva},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := c.Reservar(context.Background(), tt.a, tt.n); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

// A limit of 0 allows nothing (limite_usuario_dia may be 0, D19).
func TestReservar_LimiteCero(t *testing.T) {
	db := entornoCostos(t)
	configurar(t, db, `limite_usuario_dia = 0`)
	c := nuevoPgCostos(t, db, reloj(mediodia))
	_, err := c.Reservar(context.Background(), usuarioDePrueba(t, db, ia.RolUsuario), 1)
	_ = esLimite(t, err, ia.VentanaDia)
}

// Escenario «Concurrencia»: 3 requests race for the last action; one gets it
// and the other two get the limit error, never an overshoot.
func TestReservar_Concurrencia(t *testing.T) {
	db := entornoCostos(t)
	c := nuevoPgCostos(t, db, reloj(mediodia))
	a := usuarioDePrueba(t, db, ia.RolUsuario)
	reservar(t, c, a, 4)
	var wg sync.WaitGroup
	errs := make([]error, 3)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = c.Reservar(context.Background(), a, 1)
		}()
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
			continue
		}
		_ = esLimite(t, err, ia.VentanaDia)
	}
	if ok != 1 {
		t.Fatalf("%d of 3 concurrent reservations succeeded, want 1 (%v)", ok, errs)
	}
	if consumo, _ := c.Consumo(context.Background(), a); consumo.Dia.Usados != 5 {
		t.Errorf("usados = %d, want 5", consumo.Dia.Usados)
	}
}

// Escenario «Reinicio mensual» and the daily one, with the clock on the
// borders of the Lima day (UTC-5): 04:59Z is still yesterday, 05:00Z is today.
func TestReservar_ReinicioEnLima(t *testing.T) {
	tests := []struct {
		name    string
		antes   time.Time
		despues time.Time
		ventana string
	}{
		{"the day resets at Lima midnight", time.Date(2026, 11, 12, 4, 59, 0, 0, time.UTC), time.Date(2026, 11, 12, 5, 0, 0, 0, time.UTC), ia.VentanaDia},
		{"the month resets on the 1st at Lima midnight", time.Date(2026, 12, 1, 4, 59, 0, 0, time.UTC), time.Date(2026, 12, 1, 5, 0, 0, 0, time.UTC), ia.VentanaMes},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := entornoCostos(t)
			configurar(t, db, `limite_usuario_dia = 2, limite_usuario_mes = 2`)
			if tt.ventana == ia.VentanaDia {
				configurar(t, db, `limite_usuario_mes = 60`)
			}
			a := usuarioDePrueba(t, db, ia.RolUsuario)
			antes := nuevoPgCostos(t, db, reloj(tt.antes))
			reservar(t, antes, a, 2)
			_, err := antes.Reservar(context.Background(), a, 1)
			lim := esLimite(t, err, tt.ventana)
			if !lim.ReiniciaAt.Equal(tt.despues) {
				t.Errorf("reinicia_at = %v, want %v", lim.ReiniciaAt, tt.despues)
			}
			if _, err := nuevoPgCostos(t, db, reloj(tt.despues)).Reservar(context.Background(), a, 1); err != nil {
				t.Errorf("at the reset instant: %v", err)
			}
		})
	}
}

// Liberar gives the actions back, never below 0, and it works with the
// consumer's ctx already cancelled: a timed-out job must still release.
func TestLiberar(t *testing.T) {
	db := entornoCostos(t)
	c := nuevoPgCostos(t, db, reloj(mediodia))
	a := usuarioDePrueba(t, db, ia.RolUsuario)
	r := reservar(t, c, a, 3)

	cancelado, cancelar := context.WithCancel(context.Background())
	cancelar()
	if err := c.Liberar(cancelado, &r, 2); err != nil {
		t.Fatalf("Liberar with a cancelled ctx: %v", err)
	}
	if r.Acciones != 1 {
		t.Errorf("r.Acciones = %d, want 1", r.Acciones)
	}
	if consumo, _ := c.Consumo(context.Background(), a); consumo.Dia.Usados != 1 || consumo.Mes.Usados != 1 {
		t.Errorf("after releasing 2 of 3: day %d, month %d; want 1 and 1", consumo.Dia.Usados, consumo.Mes.Usados)
	}
	if err := c.Liberar(context.Background(), &r, 10); err != nil || r.Acciones != 0 { // more than it holds
		t.Errorf("Liberar(10) = %v, r.Acciones = %d", err, r.Acciones)
	}
	if err := c.Liberar(context.Background(), &r, 1); err != nil { // nothing left to release
		t.Error(err)
	}
	if consumo, _ := c.Consumo(context.Background(), a); consumo.Dia.Usados != 0 || consumo.Mes.Usados != 0 {
		t.Errorf("counters = %d / %d, want 0 / 0", consumo.Dia.Usados, consumo.Mes.Usados)
	}
	// A counter never goes below 0, even if other actions of the actor were released first.
	other := reservar(t, c, a, 1)
	other.Acciones = 5
	if err := c.Liberar(context.Background(), &other, 5); err != nil {
		t.Fatal(err)
	}
	var negativos int
	_ = db.Get(&negativos, `SELECT COUNT(*) FROM ia_cuotas WHERE usados < 0`)
	if negativos != 0 {
		t.Errorf("%d counters below 0", negativos)
	}

	rs := ia.Reserva{Actor: ia.Actor{Rol: ia.RolSistema}, Acciones: 2}
	if err := c.Liberar(context.Background(), &rs, 1); err != nil || rs.Acciones != 1 {
		t.Errorf("system release = %v, %d", err, rs.Acciones)
	}
}

// ADR 0002, rotation: the previous key's counter counts against the limit,
// and Liberar falls back to it when the current key has no row.
func TestRotacionDeClaves(t *testing.T) {
	db := entornoCostos(t)
	a := usuarioDePrueba(t, db, ia.RolUsuario)
	b := usuarioDePrueba(t, db, ia.RolUsuario)
	vieja := nuevoPgCostos(t, db, reloj(mediodia)) // writes with key 1
	reservar(t, vieja, a, 3)
	rb := reservar(t, vieja, b, 2)

	rotada, err := ia.NewPgCostos(db, antifraude.New(bytes.Repeat([]byte{3}, 32), bytes.Repeat([]byte{1}, 32)), reloj(mediodia))
	if err != nil {
		t.Fatal(err)
	}
	if consumo, _ := rotada.Consumo(context.Background(), a); consumo.Dia.Usados != 3 || consumo.Dia.Disponibles != 2 {
		t.Errorf("consumo after the rotation = %+v, want 3 used (the previous key counts)", consumo.Dia)
	}
	reservar(t, rotada, a, 2) // 3 + 2 = the limit of 5
	_, err = rotada.Reservar(context.Background(), a, 1)
	_ = esLimite(t, err, ia.VentanaDia)

	// b has rows only under the previous key: Liberar finds them there.
	if err := rotada.Liberar(context.Background(), &rb, 2); err != nil {
		t.Fatal(err)
	}
	if consumo, _ := rotada.Consumo(context.Background(), b); consumo.Dia.Usados != 0 || consumo.Mes.Usados != 0 {
		t.Errorf("b after releasing through the fallback = %+v / %+v", consumo.Dia, consumo.Mes)
	}
}

// Escenarios «Contador sin datos personales» and «Contador tras borrado de
// datos»: the table has no usuario_id nor FK, the key does not hold the id,
// and deleting the actor's ia_usos rows leaves the counter alone.
func TestCuotas_SinDatosPersonalesNiBorrado(t *testing.T) {
	db := entornoCostos(t)
	c := nuevoPgCostos(t, db, reloj(mediodia))
	a := usuarioDePrueba(t, db, ia.RolUsuario)
	reservar(t, c, a, 2)

	var columnas []string
	if err := db.Select(&columnas, `SELECT column_name FROM information_schema.columns WHERE table_name IN ('ia_cuotas', 'ia_gasto_mes') ORDER BY 1`); err != nil {
		t.Fatal(err)
	}
	for _, col := range columnas {
		if col == "usuario_id" || col == "actor_id" {
			t.Errorf("column %s identifies the user", col)
		}
	}
	var fks int
	_ = db.Get(&fks, `SELECT COUNT(*) FROM pg_constraint WHERE contype = 'f' AND conrelid IN ('ia_cuotas'::regclass, 'ia_gasto_mes'::regclass)`)
	if fks != 0 {
		t.Errorf("%d foreign keys on the counter tables", fks)
	}
	var claves []string
	_ = db.Select(&claves, `SELECT clave FROM ia_cuotas`)
	if len(claves) != 2 {
		t.Fatalf("%d counters, want 2", len(claves))
	}

	if _, err := db.Exec(`DELETE FROM ia_usos WHERE actor_id = $1`, *a.ID); err != nil {
		t.Fatal(err)
	}
	if consumo, _ := c.Consumo(context.Background(), a); consumo.Dia.Usados != 2 {
		t.Errorf("usados after deleting the user's ia_usos = %d, want 2", consumo.Dia.Usados)
	}
}

// Lazy purge: each Reservar deletes up to 100 expired counters.
func TestReservar_PurgaCuotasVencidas(t *testing.T) {
	db := entornoCostos(t)
	if _, err := db.Exec(`INSERT INTO ia_cuotas (clave, ventana, ventana_inicio, usados, expira_en)
		SELECT 'ia_cuota:viejo' || g, 'dia', '2026-01-01'::timestamptz + g * INTERVAL '1 day', 1, '2026-01-02'::timestamptz + g * INTERVAL '1 day'
		FROM generate_series(1, 150) g`); err != nil {
		t.Fatal(err)
	}
	c := nuevoPgCostos(t, db, reloj(mediodia))
	reservar(t, c, usuarioDePrueba(t, db, ia.RolUsuario), 1)
	var vencidas int
	_ = db.Get(&vencidas, `SELECT COUNT(*) FROM ia_cuotas WHERE clave LIKE 'ia_cuota:viejo%'`)
	if vencidas != 50 {
		t.Errorf("%d expired counters left, want 50 (100 purged)", vencidas)
	}
}

// Consumo of the system and with the budget spent.
func TestConsumo_PresupuestoYSistema(t *testing.T) {
	db := entornoCostos(t)
	c := nuevoPgCostos(t, db, reloj(mediodia))
	consumo, err := c.Consumo(context.Background(), ia.Actor{Rol: ia.RolSistema})
	if err != nil || consumo.Dia != nil || consumo.Mes != nil || consumo.PresupuestoAgotado {
		t.Errorf("system consumo = %+v (%v)", consumo, err)
	}
	if err := c.SumarGasto(context.Background(), 40); err != nil {
		t.Fatal(err)
	}
	if consumo, _ := c.Consumo(context.Background(), ia.Actor{Rol: ia.RolSistema}); !consumo.PresupuestoAgotado {
		t.Error("budget spent but consumo says it is not")
	}
	if _, err := c.Consumo(context.Background(), ia.Actor{Rol: ia.RolUsuario}); !errors.Is(err, ia.ErrActorInvalido) {
		t.Errorf("invalid actor: %v", err)
	}
}
