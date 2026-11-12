// Package ia is the provider-agnostic AI service of SERUMS IA (SIA-E6-01,
// docs/api/serums-ia-e6-01-servicio-ia.md, ADR 0006): four typed operations
// that reject personal data and oversized input before calling the provider,
// bound every call, retry only transient errors and invalid output, validate
// the output and record one usage row per invocation, never its content.
// The calls run in the consumers' jobs, never in a request. SIA-E6-02 adds the
// control of costs (docs/api/serums-ia-e6-02-control-costos.md, ADR 0009): the
// quota of actions reserved all or nothing, the monthly budget, the cache, and
// the routes of the consumption and of the admin's configuration and spend.
package ia

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Operations (regla 1); they are also ia_usos.operacion.
const (
	OpClasificar         = "clasificar"
	OpGenerarPreguntas   = "generar_preguntas"
	OpGenerarMaterial    = "generar_material"
	OpGenerarExplicacion = "generar_explicacion"
)

// Actor roles of ia_usos.actor_rol.
const (
	RolAdmin   = "admin"
	RolUsuario = "usuario"
	RolSistema = "sistema"
)

// MaxEntradaRunes is the input limit per call, over the texts of the data
// blocks (regla 6).
const MaxEntradaRunes = 30000

// Actor identifies who asked for the call, by opaque id only: never a name,
// e-mail or DNI. ID is nil for the system and set for admin and usuario.
type Actor struct {
	Rol string
	ID  *int64
}

// Servicio is what consumers use. Every operation spends one action of the
// reservation r (SIA-E6-02) and records one usage row. Reservar, Liberar and
// Consumo are the quota API of the consumers (contract E6-02 §2.2).
type Servicio interface {
	Clasificar(ctx context.Context, r *Reserva, in EntradaClasificar) (SalidaClasificar, error)
	GenerarPreguntas(ctx context.Context, r *Reserva, in EntradaGenerarPreguntas) (SalidaGenerarPreguntas, error)
	GenerarMaterial(ctx context.Context, r *Reserva, in EntradaGenerarMaterial) (SalidaGenerarMaterial, error)
	GenerarExplicacion(ctx context.Context, r *Reserva, in EntradaGenerarExplicacion) (SalidaGenerarExplicacion, error)

	// Reservar takes n actions all or nothing (D17), in the order budget,
	// month, day. Errors: *ErrorPresupuesto, *ErrLimite.
	Reservar(ctx context.Context, a Actor, n int) (Reserva, error)
	// Liberar gives n actions back; the consumer releases what it does not use.
	Liberar(ctx context.Context, r *Reserva, n int) error
	Consumo(ctx context.Context, a Actor) (Consumo, error)
	Configuracion(ctx context.Context) (Configuracion, error)
	// BuscarCache is the building block of BuscarEnCache, which consumers
	// call: acepta decides whether the stored output is usable before the
	// cache hit is recorded for a.
	BuscarCache(ctx context.Context, a Actor, sol Solicitud, acepta func(json.RawMessage) bool) (json.RawMessage, bool, error)
}

// Errors of the service (contract §5). Their text is the stable code.
var (
	ErrNoDisponible           = errors.New("ia_no_disponible")
	ErrEntradaDemasiadoGrande = errors.New("entrada_demasiado_grande")
	ErrDatosPersonales        = errors.New("datos_personales")
	ErrTimeout                = errors.New("ia_timeout")
	ErrSalidaInvalida         = errors.New("salida_invalida")
	ErrProveedor              = errors.New("ia_error")
	// ErrCancelado wraps context.Canceled: the consumer cancelled its own ctx,
	// so it is recorded but never mapped to HTTP (Codigo reports ok=false).
	ErrCancelado = errors.New("cancelado")
	// ErrCostos wraps a failure of the cost control itself (the database),
	// not of the provider: it is recorded as error_interno and answered 500.
	ErrCostos = errors.New("error_costos")
	// ErrActorInvalido is a programming error of the consumer: the actor
	// would break ck_ia_usos_actor, so nothing is called nor recorded.
	ErrActorInvalido = errors.New("actor_invalido")
)

// Personal data types reported by ErrorDatosPersonales.
const (
	TipoDNI      = "dni"
	TipoEmail    = "email"
	TipoTelefono = "telefono"
)

// BloqueInstrucciones is the Bloque index reported when the match is in the
// instructions rather than in a data block.
const BloqueInstrucciones = -1

// ErrorDatosPersonales says which kind of personal data was found and in
// which block, so the consumer knows what to review. It never carries the
// matched text. It is ErrDatosPersonales for errors.Is.
type ErrorDatosPersonales struct {
	Tipo   string
	Bloque int
}

func (e *ErrorDatosPersonales) Error() string {
	return fmt.Sprintf("%s: %s en el bloque %d", ErrDatosPersonales, e.Tipo, e.Bloque)
}

func (e *ErrorDatosPersonales) Unwrap() error { return ErrDatosPersonales }

// Codigo maps an error of the service to the code (format 2) and the HTTP
// status a consumer must use to expose it (contract §5; E6-02 §5). It returns
// ("", 0) for nil, for ErrCancelado (the consumer caused it) and for an error
// that is not of the service. Detalle gives the data that goes with it.
func Codigo(err error) (codigo string, status int) {
	var lim *ErrLimite
	if errors.As(err, &lim) {
		return lim.Error(), http.StatusTooManyRequests
	}
	for _, m := range mapaErrores {
		if errors.Is(err, m.err) {
			return m.codigo(), m.status
		}
	}
	return "", 0
}

// Detalle is the `detalle` object of the error response of Codigo, or nil
// when the error carries none: the instant the budget or the quota restarts
// and, for a quota, its limit and what was used (contract E6-02 §5).
func Detalle(err error) map[string]any {
	var lim *ErrLimite
	var ep *ErrorPresupuesto
	switch {
	case errors.As(err, &lim):
		return map[string]any{"limite": lim.Limite, "usados": lim.Usados, "reinicia_at": lim.ReiniciaAt}
	case errors.As(err, &ep):
		return map[string]any{"reinicia_at": ep.ReiniciaAt}
	}
	return nil
}

// errorMapeado is the status and code of one error of the service.
type errorMapeado struct {
	err    error
	status int
	cod    string // when the code is not the error's text
}

func (m errorMapeado) codigo() string {
	if m.cod != "" {
		return m.cod
	}
	return m.err.Error()
}

var mapaErrores = []errorMapeado{
	{ErrNoDisponible, http.StatusServiceUnavailable, ""},
	{ErrEntradaDemasiadoGrande, http.StatusUnprocessableEntity, ""},
	{ErrDatosPersonales, http.StatusUnprocessableEntity, ""},
	{ErrTimeout, http.StatusGatewayTimeout, ""},
	{ErrSalidaInvalida, http.StatusBadGateway, ""},
	{ErrProveedor, http.StatusBadGateway, ""},
	{ErrPresupuestoAgotado, http.StatusTooManyRequests, ""},
	{ErrSinReserva, http.StatusInternalServerError, "error_interno"},
	{ErrActorInvalido, http.StatusInternalServerError, "error_interno"},
}
