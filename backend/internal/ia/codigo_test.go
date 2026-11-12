package ia

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"reflect"
	"testing"
	"time"
)

// Contract E6-02 §5: the code and status of the new errors, and the detail
// that goes with them; wrapped errors map the same.
func TestCodigoYDetalle_ControlDeCostos(t *testing.T) {
	reinicia := time.Date(2026, 12, 1, 5, 0, 0, 0, time.UTC)
	dia := &ErrLimite{Ventana: VentanaDia, Limite: 5, Usados: 5, ReiniciaAt: reinicia}
	mes := &ErrLimite{Ventana: VentanaMes, Limite: 60, Usados: 60, ReiniciaAt: reinicia}
	tests := []struct {
		name        string
		err         error
		wantCodigo  string
		wantStatus  int
		wantDetalle map[string]any
	}{
		{"budget spent", &ErrorPresupuesto{ReiniciaAt: reinicia}, "presupuesto_agotado", http.StatusTooManyRequests,
			map[string]any{"reinicia_at": reinicia}},
		{"daily quota", dia, "limite_ia", http.StatusTooManyRequests,
			map[string]any{"limite": 5, "usados": 5, "reinicia_at": reinicia}},
		{"monthly quota", mes, "limite_ia_mensual", http.StatusTooManyRequests,
			map[string]any{"limite": 60, "usados": 60, "reinicia_at": reinicia}},
		{"wrapped daily quota", fmt.Errorf("reservar: %w", dia), "limite_ia", http.StatusTooManyRequests,
			map[string]any{"limite": 5, "usados": 5, "reinicia_at": reinicia}},
		{"wrapped budget", fmt.Errorf("reservar: %w", &ErrorPresupuesto{ReiniciaAt: reinicia}), "presupuesto_agotado", http.StatusTooManyRequests,
			map[string]any{"reinicia_at": reinicia}},
		{"no reservation is an internal error", ErrSinReserva, "error_interno", http.StatusInternalServerError, nil},
		{"invalid actor is an internal error", ErrActorInvalido, "error_interno", http.StatusInternalServerError, nil},
		{"cancelled maps to nothing", fmt.Errorf("%w: %w", ErrCancelado, context.Canceled), "", 0, nil},
		{"no error", nil, "", 0, nil},
		{"foreign error", fmt.Errorf("otro"), "", 0, nil},
		{"timeout keeps its code and has no detail", ErrTimeout, "ia_timeout", http.StatusGatewayTimeout, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			codigo, status := Codigo(tt.err)
			if codigo != tt.wantCodigo || status != tt.wantStatus {
				t.Errorf("Codigo = %q %d, want %q %d", codigo, status, tt.wantCodigo, tt.wantStatus)
			}
			if got := Detalle(tt.err); !reflect.DeepEqual(got, tt.wantDetalle) {
				t.Errorf("Detalle = %v, want %v", got, tt.wantDetalle)
			}
		})
	}
}

// validar is the rule set of the PUT, without a database: the edges of D19.
func TestActualizarConfiguracion_Validar(t *testing.T) {
	entero := func(v int) *int { return &v }
	decimal := func(v float64) *float64 { return &v }
	valida := ActualizarConfiguracion{entero(5), entero(60), entero(200), entero(20), entero(20), decimal(40)}
	if c, err := valida.validar(); err != nil || c.PresupuestoMensualUSD != 40 || c.LimiteUsuarioMes != 60 {
		t.Fatalf("valid body: %+v, %v", c, err)
	}
	casos := []struct {
		name   string
		mutar  func(*ActualizarConfiguracion)
		campos map[string]string
	}{
		{"day equal to month is valid", func(r *ActualizarConfiguracion) { r.LimiteUsuarioDia, r.LimiteUsuarioMes = entero(7), entero(7) }, nil},
		{"day one over the month", func(r *ActualizarConfiguracion) { r.LimiteUsuarioDia, r.LimiteUsuarioMes = entero(8), entero(7) }, map[string]string{"limite_usuario_dia": "dia_mayor_que_mes"}},
		{"cents are valid", func(r *ActualizarConfiguracion) { r.PresupuestoMensualUSD = decimal(12.34) }, nil},
		{"0.1 + 0.2 style floats are valid", func(r *ActualizarConfiguracion) { r.PresupuestoMensualUSD = decimal(0.1 + 0.2 + 1) }, nil},
		{"three decimals", func(r *ActualizarConfiguracion) { r.PresupuestoMensualUSD = decimal(1.005) }, map[string]string{"presupuesto_mensual_usd": "dec2"}},
		{"NaN budget", func(r *ActualizarConfiguracion) { r.PresupuestoMensualUSD = decimal(math.NaN()) }, map[string]string{"presupuesto_mensual_usd": "min"}},
		{"nothing at all", func(r *ActualizarConfiguracion) { *r = ActualizarConfiguracion{} }, map[string]string{
			"limite_usuario_dia": "requerido", "limite_usuario_mes": "requerido", "limite_admin_dia": "requerido",
			"max_preguntas_solicitud": "requerido", "lote_clasificacion": "requerido", "presupuesto_mensual_usd": "requerido"}},
	}
	for _, tt := range casos {
		t.Run(tt.name, func(t *testing.T) {
			r := valida
			tt.mutar(&r)
			_, err := r.validar()
			if tt.campos == nil {
				if err != nil {
					t.Fatalf("err = %v, want valid", err)
				}
				return
			}
			var verr *ValidacionError
			if !errors.As(err, &verr) || !reflect.DeepEqual(verr.Campos, tt.campos) {
				t.Errorf("err = %v, want campos %v", err, tt.campos)
			}
		})
	}
}
