package ia

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
)

// Call limits (reglas 1 and 4, contract §2): at most 2 validation rounds × 3
// transport attempts = 6 provider calls per invocation.
const (
	tiempoLlamadaDefecto = 60 * time.Second
	rondasValidacion     = 2
)

// esperasDefecto are the waits before each transient retry (2 retries).
var esperasDefecto = []time.Duration{time.Second, 2 * time.Second}

// Registrador stores the usage rows (PgRegistro in production).
type Registrador interface {
	Registrar(ctx context.Context, u Uso) error
}

// Opcion tunes the service; the defaults are the contract's. Tests inject
// short times so they never wait.
type Opcion func(*servicio)

// ConTiempoLlamada sets the deadline of each provider call (default 60 s).
func ConTiempoLlamada(d time.Duration) Opcion { return func(s *servicio) { s.tiempoLlamada = d } }

// ConEsperas sets the waits before each transient retry (default 1 s, 2 s);
// their number is the number of retries.
func ConEsperas(esperas ...time.Duration) Opcion {
	return func(s *servicio) { s.esperas = esperas }
}

// ConReloj sets the clock that measures the duration of each invocation.
func ConReloj(ahora func() time.Time) Opcion { return func(s *servicio) { s.ahora = ahora } }

type servicio struct {
	prov          Proveedor // nil = disabled (no IA_PROVEEDOR)
	reg           Registrador
	costos        Costos
	validate      *validator.Validate
	tiempoLlamada time.Duration
	esperas       []time.Duration
	ahora         func() time.Time
	dormir        func(ctx context.Context, d time.Duration) bool // dormirReal; tests record the waits
}

// NewServicio creates the service over prov, recording usage with reg and
// controlling quota and budget with costos. A nil
// prov is the disabled service: every operation is ErrNoDisponible and the
// rest of the application keeps working (regla 5).
func NewServicio(prov Proveedor, reg Registrador, costos Costos, opciones ...Opcion) Servicio {
	s := &servicio{
		prov: prov, reg: reg, costos: costos,
		validate:      validator.New(validator.WithRequiredStructEnabled()),
		tiempoLlamada: tiempoLlamadaDefecto,
		esperas:       esperasDefecto,
		ahora:         time.Now,
		dormir:        dormirReal,
	}
	for _, o := range opciones {
		o(s)
	}
	return s
}

func (s *servicio) Clasificar(ctx context.Context, r *Reserva, in EntradaClasificar) (SalidaClasificar, error) {
	return ejecutar(ctx, s, r, in.solicitud(), func(o SalidaClasificar) error { return o.validar(in) })
}

func (s *servicio) GenerarPreguntas(ctx context.Context, r *Reserva, in EntradaGenerarPreguntas) (SalidaGenerarPreguntas, error) {
	return ejecutar(ctx, s, r, in.solicitud(), func(o SalidaGenerarPreguntas) error { return o.validar(in) })
}

func (s *servicio) GenerarMaterial(ctx context.Context, r *Reserva, in EntradaGenerarMaterial) (SalidaGenerarMaterial, error) {
	return ejecutar(ctx, s, r, in.solicitud(), func(o SalidaGenerarMaterial) error { return o.validar(in) })
}

func (s *servicio) GenerarExplicacion(ctx context.Context, r *Reserva, in EntradaGenerarExplicacion) (SalidaGenerarExplicacion, error) {
	return ejecutar(ctx, s, r, in.solicitud(), func(o SalidaGenerarExplicacion) error { return o.validar(in) })
}

// ejecutar runs one invocation of the reservation r and records its usage,
// whatever the outcome. An invalid actor is a programming error of the
// consumer: it would break ck_ia_usos_actor, so nothing is called nor
// recorded; the same for a reservation with no action left (contract E6-02
// §2.3 ★0).
func ejecutar[S any](ctx context.Context, s *servicio, r *Reserva, sol Solicitud, validar func(S) error) (S, error) {
	var cero S
	if r == nil || !actorValido(r.Actor) {
		return cero, ErrActorInvalido
	}
	if r.Acciones < 1 {
		return cero, ErrSinReserva
	}
	inicio := s.ahora()
	uso := Uso{Operacion: sol.Operacion, Actor: r.Actor}
	out, err := llamar(ctx, s, sol, &uso, validar)
	uso.Duracion = s.ahora().Sub(inicio)
	uso.Resultado = resultadoDe(err)
	s.cerrarCostos(ctx, r, uso, err)
	s.registrar(ctx, uso)
	if err != nil {
		return cero, err
	}
	return out, nil
}

// cerrarCostos settles the invocation (contract E6-02 §2.3 ★9 and ★10): what
// the attempts cost is added to the month's spent amount even if the call
// failed (D15); a failed call gives its action back, a good one keeps it. A
// failure here is logged by op and never replaces the result of the call.
func (s *servicio) cerrarCostos(ctx context.Context, r *Reserva, uso Uso, err error) {
	if uso.CostoUSD > 0 {
		if gerr := s.costos.SumarGasto(ctx, uso.CostoUSD); gerr != nil {
			log.Error().Str("op", "ia.sumar_gasto").Str("operacion", uso.Operacion).Msg("ia: spent amount not recorded")
		}
	}
	if err == nil {
		r.Acciones--
		return
	}
	if lerr := s.costos.Liberar(ctx, r, 1); lerr != nil {
		log.Error().Str("op", "ia.liberar").Str("operacion", uso.Operacion).Msg("ia: action not released")
	}
}

func (s *servicio) Reservar(ctx context.Context, a Actor, n int) (Reserva, error) {
	return s.costos.Reservar(ctx, a, n)
}

func (s *servicio) Liberar(ctx context.Context, r *Reserva, n int) error {
	return s.costos.Liberar(ctx, r, n)
}

func (s *servicio) Consumo(ctx context.Context, a Actor) (Consumo, error) {
	return s.costos.Consumo(ctx, a)
}

// llamar applies the flow of contract §2: enabled, size, personal data,
// budget (E6-02 ★5), then up to 2 validation rounds of up to 3 transport attempts each.
func llamar[S any](ctx context.Context, s *servicio, sol Solicitud, uso *Uso, validar func(S) error) (S, error) {
	var cero S
	if s.prov == nil {
		return cero, ErrNoDisponible
	}
	uso.Proveedor = s.prov.Nombre()
	if runesDeDatos(sol) > MaxEntradaRunes {
		return cero, ErrEntradaDemasiadoGrande
	}
	if err := buscarDatosPersonales(sol); err != nil {
		return cero, err
	}
	if err := s.costos.ComprobarPresupuesto(ctx); err != nil { // ★5
		// A consumer that gave up makes the check fail with its own ctx error.
		if fin := finDelConsumidor(ctx); fin != nil {
			return cero, fin
		}
		return cero, err
	}
	for range rondasValidacion {
		resp, err := s.transporte(ctx, sol, uso)
		if err != nil {
			return cero, err
		}
		if out, ok := decodificar(s.validate, resp.JSON, validar); ok {
			return out, nil
		}
	}
	return cero, ErrSalidaInvalida
}

// transporte calls the provider with a deadline per attempt and retries only
// transient errors, after a growing wait. The deadline is decided by the
// attempt's ctx before looking at the error, so a timeout that an adapter
// wraps as transient is still a timeout and is not retried. Tokens and cost
// of every answer received add up in uso.
func (s *servicio) transporte(ctx context.Context, sol Solicitud, uso *Uso) (Respuesta, error) {
	for intento := 0; ; intento++ {
		if err := finDelConsumidor(ctx); err != nil {
			return Respuesta{}, err
		}
		llamadaCtx, cancel := context.WithTimeout(ctx, s.tiempoLlamada)
		resp, err := s.prov.Completar(llamadaCtx, sol)
		vencido := llamadaCtx.Err() != nil
		cancel()
		uso.Intentos++
		if err == nil {
			uso.TokensEntrada += resp.TokensEntrada
			uso.TokensSalida += resp.TokensSalida
			uso.CostoUSD += s.prov.CostoUSD(resp)
			uso.Modelo = resp.Modelo
			return resp, nil
		}
		if cerr := finDelConsumidor(ctx); cerr != nil {
			return Respuesta{}, cerr
		}
		if vencido {
			return Respuesta{}, ErrTimeout
		}
		var transitorio *ErrorTransitorio
		if !errors.As(err, &transitorio) || intento >= len(s.esperas) {
			return Respuesta{}, ErrProveedor
		}
		if !s.dormir(ctx, s.esperas[intento]) {
			return Respuesta{}, finDelConsumidor(ctx)
		}
	}
}

// finDelConsumidor says why the consumer's ctx ended (contract §5): a
// cancellation is ErrCancelado, which wraps context.Canceled; an expired
// deadline is ErrTimeout (no answer in the time it had). It is nil while ctx
// is alive.
func finDelConsumidor(ctx context.Context) error {
	switch err := ctx.Err(); {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled):
		return fmt.Errorf("%w: %w", ErrCancelado, err)
	default:
		return ErrTimeout
	}
}

// decodificar reads the JSON strictly (unknown fields and trailing data are
// rejected) and validates it. The decode error is dropped on purpose: it
// quotes the provider's output.
func decodificar[S any](v *validator.Validate, data []byte, validar func(S) error) (S, bool) {
	var out S
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return out, false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return out, false
	}
	if err := v.Struct(out); err != nil {
		return out, false
	}
	return out, validar(out) == nil
}

// dormirReal waits d unless ctx ends first.
func dormirReal(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// registrar writes the usage row; a failure is logged by op only and never
// replaces the result of the call (contract §2, paso 7).
func (s *servicio) registrar(ctx context.Context, uso Uso) {
	if uso.Resultado != ResultadoOK {
		log.Warn().Str("op", "ia."+uso.Operacion).Str("resultado", uso.Resultado).Int("intentos", uso.Intentos).
			Msg("ia: call not completed")
	}
	if err := s.reg.Registrar(ctx, uso); err != nil {
		log.Error().Str("op", "ia.registrar").Str("operacion", uso.Operacion).Msg("ia: usage not recorded")
	}
}

func actorValido(a Actor) bool {
	switch a.Rol {
	case RolSistema:
		return a.ID == nil
	case RolAdmin, RolUsuario:
		return a.ID != nil && *a.ID > 0
	default:
		return false
	}
}

func resultadoDe(err error) string {
	switch {
	case err == nil:
		return ResultadoOK
	case errors.Is(err, ErrNoDisponible):
		return ResultadoNoDisponible
	case errors.Is(err, ErrEntradaDemasiadoGrande):
		return ResultadoEntradaDemasiadoGrande
	case errors.Is(err, ErrDatosPersonales):
		return ResultadoDatosPersonales
	case errors.Is(err, ErrTimeout):
		return ResultadoTimeout
	case errors.Is(err, ErrCancelado):
		return ResultadoCancelado
	case errors.Is(err, ErrPresupuestoAgotado):
		return ResultadoPresupuestoAgotado
	case errors.Is(err, ErrSalidaInvalida):
		return ResultadoSalidaInvalida
	default:
		return ResultadoErrorProveedor
	}
}
