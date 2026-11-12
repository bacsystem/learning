package ia

import "context"

// Solicitud is what a provider receives. Instrucciones is a fixed template of
// the code; Datos is academic content that the adapter sends delimited as
// data, never concatenated to the instructions. Ids and numbers go in
// Etiqueta, never in Texto, so they are not taken for personal data.
type Solicitud struct {
	Operacion     string
	Instrucciones string
	Datos         []Bloque
	EsquemaSalida string // e.g. "clasificar.v1"
	// Fuentes are the fuente_id whose content goes in Datos. The consumer
	// fills them; the cache uses them to know when an answer went stale
	// (SIA-E6-02 §4). They are never sent to the provider.
	Fuentes []int64
}

// Bloque is one delimited piece of data.
type Bloque struct{ Etiqueta, Texto string }

// Respuesta is the provider's raw JSON answer and its usage.
type Respuesta struct {
	JSON          []byte
	TokensEntrada int
	TokensSalida  int
	Modelo        string
}

// Proveedor is the port every adapter implements (ADR 0006). Completar must
// honour ctx and mark retryable failures (network, 429, 5xx) as
// *ErrorTransitorio.
type Proveedor interface {
	Nombre() string
	Completar(ctx context.Context, s Solicitud) (Respuesta, error)
	CostoUSD(r Respuesta) float64 // the adapter's price table; 0 in the double
}

// ErrorTransitorio marks a provider failure worth retrying.
type ErrorTransitorio struct{ Causa error }

func (e *ErrorTransitorio) Error() string {
	return "error transitorio del proveedor: " + e.Causa.Error()
}

func (e *ErrorTransitorio) Unwrap() error { return e.Causa }
