package ia

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

// ★0: a reservation with no action left, or none at all, is a programming
// error of the consumer: nothing is called, recorded, released nor charged.
func TestServicio_SinReserva(t *testing.T) {
	tests := []struct {
		name string
		r    *Reserva
		want error
	}{
		{"no actions left", &Reserva{Actor: sistema, Acciones: 0}, ErrSinReserva},
		{"negative actions", &Reserva{Actor: sistema, Acciones: -1}, ErrSinReserva},
		{"nil reservation", nil, ErrActorInvalido},
		{"invalid actor first", &Reserva{Actor: Actor{Rol: RolUsuario}, Acciones: 0}, ErrActorInvalido},
		{"user without id", &Reserva{Actor: Actor{Rol: RolUsuario}, Acciones: 1}, ErrActorInvalido},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov := &proveedorGuion{pasos: []paso{valida(1)}}
			reg, costos := &registroMemoria{}, &costosMemoria{}
			s, _ := nuevoConCostos(prov, reg, costos)
			if _, err := s.Clasificar(context.Background(), tt.r, entradaClasificar(1)); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if prov.llamado != 0 || len(reg.usos) != 0 || costos.liberadas != 0 || costos.gasto != 0 || costos.comprobaciones != 0 {
				t.Errorf("side effects: %d calls, %d rows, %d released, %v spent, %d budget checks",
					prov.llamado, len(reg.usos), costos.liberadas, costos.gasto, costos.comprobaciones)
			}
		})
	}
}

// ★9 and ★10 in every outcome: a good call keeps its action and adds its
// cost; a failed one gives the action back (D15) and still adds what its
// attempts cost.
func TestServicio_CierreDeCostos(t *testing.T) {
	tests := []struct {
		name          string
		pasos         []paso
		prov          bool // false: disabled service
		wantErr       error
		wantAcciones  int // r.Acciones after, from 2
		wantLiberadas int
		wantGasto     float64
	}{
		{"good call consumes its action", []paso{valida(1)}, true, nil, 1, 0, 0.01},
		{"invalid output twice: both rounds are charged, the action is released", []paso{invalida, invalida}, true, ErrSalidaInvalida, 1, 1, 0.02},
		{"invalid then valid charges both calls and consumes the action", []paso{invalida, valida(1)}, true, nil, 1, 0, 0.02},
		{"provider error is released and free", []paso{{err: errors.New("401")}}, true, ErrProveedor, 1, 1, 0},
		{"disabled service releases", nil, false, ErrNoDisponible, 1, 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var prov Proveedor
			if tt.prov {
				prov = &proveedorGuion{pasos: tt.pasos}
			}
			costos := &costosMemoria{}
			s, _ := nuevoConCostos(prov, &registroMemoria{}, costos)
			r := &Reserva{Actor: sistema, Acciones: 2}
			_, err := s.Clasificar(context.Background(), r, entradaClasificar(1))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if r.Acciones != tt.wantAcciones || costos.liberadas != tt.wantLiberadas || math.Abs(costos.gasto-tt.wantGasto) > 1e-9 {
				t.Errorf("acciones %d, released %d, spent %v; want %d, %d, %v",
					r.Acciones, costos.liberadas, costos.gasto, tt.wantAcciones, tt.wantLiberadas, tt.wantGasto)
			}
		})
	}
}

// Rejections before the call (size, personal data) release the action and
// charge nothing, and never reach the budget check beyond the order of ★5.
func TestServicio_RechazosPrevios(t *testing.T) {
	tests := []struct {
		name            string
		in              EntradaClasificar
		wantErr         error
		wantComprobadas int
	}{
		{"personal data", EntradaClasificar{Items: []ItemTexto{{ID: 1, Texto: "Escribir a persona@example.com"}}, Taxonomia: entradaClasificar(1).Taxonomia}, ErrDatosPersonales, 0},
		{"input too large", EntradaClasificar{Items: []ItemTexto{{ID: 1, Texto: textoLargo()}}, Taxonomia: entradaClasificar(1).Taxonomia}, ErrEntradaDemasiadoGrande, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov := &proveedorGuion{pasos: []paso{valida(1)}}
			costos := &costosMemoria{}
			s, _ := nuevoConCostos(prov, &registroMemoria{}, costos)
			r := res(sistema)
			if _, err := s.Clasificar(context.Background(), r, tt.in); !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if prov.llamado != 0 || costos.liberadas != 1 || costos.gasto != 0 || costos.comprobaciones != tt.wantComprobadas || r.Acciones != 0 {
				t.Errorf("%d calls, %d released, %v spent, %d budget checks, acciones %d",
					prov.llamado, costos.liberadas, costos.gasto, costos.comprobaciones, r.Acciones)
			}
		})
	}
}

func textoLargo() string {
	b := make([]byte, MaxEntradaRunes+1)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}

// Escenario «Corte al 100 %», ★5: with the budget spent the provider is not
// called, the action is released, and the row says presupuesto_agotado.
func TestServicio_PresupuestoAgotado(t *testing.T) {
	prov := &proveedorGuion{pasos: []paso{valida(1)}}
	reg := &registroMemoria{}
	costos := &costosMemoria{errPresupuesto: &ErrorPresupuesto{ReiniciaAt: time.Date(2026, 12, 1, 5, 0, 0, 0, time.UTC)}}
	s, _ := nuevoConCostos(prov, reg, costos)
	r := res(sistema)
	_, err := s.Clasificar(context.Background(), r, entradaClasificar(1))
	var ep *ErrorPresupuesto
	if !errors.Is(err, ErrPresupuestoAgotado) || !errors.As(err, &ep) {
		t.Fatalf("err = %v, want *ErrorPresupuesto", err)
	}
	if prov.llamado != 0 || costos.liberadas != 1 || r.Acciones != 0 {
		t.Errorf("%d provider calls, %d released, acciones %d", prov.llamado, costos.liberadas, r.Acciones)
	}
	if u := reg.unico(t); u.Resultado != ResultadoPresupuestoAgotado || u.Intentos != 0 || u.CostoUSD != 0 {
		t.Errorf("usage %+v", u)
	}
}

// A lot of N actions: each invocation uses one, the reservation keeps the rest
// for the consumer to release after a cut (E3-04 regla 7).
func TestServicio_LoteDeAcciones(t *testing.T) {
	costos := &costosMemoria{}
	s, _ := nuevoConCostos(NewDoble(), &registroMemoria{}, costos)
	r := &Reserva{Actor: sistema, Acciones: 3}
	for i := 0; i < 2; i++ {
		if _, err := s.Clasificar(context.Background(), r, entradaClasificar(1)); err != nil {
			t.Fatal(err)
		}
	}
	if r.Acciones != 1 || costos.liberadas != 0 {
		t.Fatalf("acciones %d, released %d; want 1 and 0", r.Acciones, costos.liberadas)
	}
	if err := s.Liberar(context.Background(), r, r.Acciones); err != nil || r.Acciones != 0 || costos.liberadas != 1 {
		t.Errorf("Liberar of the rest = %v, acciones %d, released %d", err, r.Acciones, costos.liberadas)
	}
}

// A failure of the cost control never replaces the result of the call (like
// the usage row, paso 7): it is logged by op only.
func TestServicio_FalloDeCostosNoInvalidaLaSalida(t *testing.T) {
	costos := &costosMemoria{errGasto: errors.New("db down"), errLiberar: errors.New("db down")}
	s, _ := nuevoConCostos(&proveedorGuion{pasos: []paso{valida(1)}}, &registroMemoria{}, costos)
	if out, err := s.Clasificar(context.Background(), res(sistema), entradaClasificar(1)); err != nil || len(out.Resultados) != 1 {
		t.Fatalf("good call with a failing spent-amount write: %+v, %v", out, err)
	}
	s, _ = nuevoConCostos(&proveedorGuion{pasos: []paso{invalida, invalida}}, &registroMemoria{}, costos)
	if _, err := s.Clasificar(context.Background(), res(sistema), entradaClasificar(1)); !errors.Is(err, ErrSalidaInvalida) {
		t.Errorf("failed call with a failing release: err = %v, want ErrSalidaInvalida", err)
	}
}

// A consumer that cancels still gets its cancelled call settled: what the
// attempts cost is added and the action released (the real PgCostos runs both
// detached from the ctx, see TestSumarGasto and TestLiberar).
func TestServicio_CancelacionLiberaLaAccion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	costos := &costosMemoria{}
	s, _ := nuevoConCostos(&proveedorQueCancela{cancelar: cancel}, &registroMemoria{}, costos)
	r := res(sistema)
	if _, err := s.Clasificar(ctx, r, entradaClasificar(1)); !errors.Is(err, ErrCancelado) {
		t.Fatalf("err = %v", err)
	}
	if costos.liberadas != 1 || r.Acciones != 0 {
		t.Errorf("released %d, acciones %d; want 1 and 0", costos.liberadas, r.Acciones)
	}
}

// ★5 fails closed when the budget cannot be read: our own failure, recorded as error_interno, not as a provider error.
func TestServicio_FalloAlLeerElPresupuesto(t *testing.T) {
	prov := &proveedorGuion{pasos: []paso{valida(1)}}
	reg := &registroMemoria{}
	costos := &costosMemoria{errPresupuesto: errors.New("pq: connection refused")}
	s, _ := nuevoConCostos(prov, reg, costos)
	r := res(sistema)
	_, err := s.Clasificar(context.Background(), r, entradaClasificar(1))
	if !errors.Is(err, ErrCostos) {
		t.Fatalf("err = %v, want ErrCostos", err)
	}
	if prov.llamado != 0 || costos.liberadas != 1 || r.Acciones != 0 {
		t.Errorf("%d provider calls, %d released, acciones %d", prov.llamado, costos.liberadas, r.Acciones)
	}
	if u := reg.unico(t); u.Resultado != ResultadoErrorInterno {
		t.Errorf("resultado = %q, want error_interno", u.Resultado)
	}
}
