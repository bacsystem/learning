package ia

import (
	"context"
	"errors"
	"time"
)

// Reserva is what Reservar hands to the consumer's job: the actions it may
// still use and the windows they were counted in. It carries no HMAC key
// (ADR 0009); Liberar derives the keys again.
type Reserva struct {
	Actor    Actor
	Acciones int       // reserved and not yet used
	Dia, Mes time.Time // start of each window (Lima midnight); Mes is zero for admin and sistema
}

// Cuota is the consumption of one window.
type Cuota struct {
	Usados      int       `json:"usados"`
	Limite      int       `json:"limite"`
	Disponibles int       `json:"disponibles"`
	ReiniciaAt  time.Time `json:"reinicia_at"`
}

// Consumo is what an actor has left (contract E6-02 §3.1). Mes is nil for
// admin and sistema, which have no monthly limit.
type Consumo struct {
	Dia                *Cuota `json:"dia"`
	Mes                *Cuota `json:"mes"`
	PresupuestoAgotado bool   `json:"presupuesto_agotado"`
}

// Errors of the cost control (contract E6-02 §5).
var (
	// ErrPresupuestoAgotado: the spent amount of the month reached the budget.
	ErrPresupuestoAgotado = errors.New("presupuesto_agotado")
	// ErrSinReserva is a programming error of the consumer: an invocation
	// without actions left in its reservation. Nothing is called nor recorded.
	ErrSinReserva = errors.New("sin_reserva")
)

// ErrorPresupuesto is ErrPresupuestoAgotado with the instant the budget
// restarts (the start of next month in Lima).
type ErrorPresupuesto struct{ ReiniciaAt time.Time }

func (e *ErrorPresupuesto) Error() string { return ErrPresupuestoAgotado.Error() }

func (e *ErrorPresupuesto) Unwrap() error { return ErrPresupuestoAgotado }

// ErrLimite is the quota of a window exhausted: Ventana is VentanaDia or
// VentanaMes. With Limite and Usados the consumer works out how many actions
// still fit.
type ErrLimite struct {
	Ventana    string
	Limite     int
	Usados     int
	ReiniciaAt time.Time
}

func (e *ErrLimite) Error() string {
	if e.Ventana == VentanaMes {
		return "limite_ia_mensual"
	}
	return "limite_ia"
}

// Costos is what the service needs from the cost control: the budget check
// before each call, the spent amount after it, and the quota of the consumers
// (PgCostos in production).
type Costos interface {
	// Reservar takes n actions all or nothing, in the order of the contract:
	// budget, month, day.
	Reservar(ctx context.Context, a Actor, n int) (Reserva, error)
	// Liberar returns n actions to the windows of r and removes them from it.
	Liberar(ctx context.Context, r *Reserva, n int) error
	Consumo(ctx context.Context, a Actor) (Consumo, error)
	// ComprobarPresupuesto answers *ErrorPresupuesto when spent ≥ budget.
	ComprobarPresupuesto(ctx context.Context) error
	// SumarGasto adds usd to the month's spent amount and evaluates the 80 % alert.
	SumarGasto(ctx context.Context, usd float64) error
}
