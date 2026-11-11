package fuentes

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/cbaciliod/learning/internal/audit"
)

// Worker settings (ADR 0003 § Worker de ingesta).
const (
	// intervaloSondeo: the UI refreshes every 5 s (SIA-E2-07 regla 6), so a
	// change shows up on its next refresh.
	intervaloSondeo = 2 * time.Second
	// maxIntentos: transient failures retry until the third attempt, then the
	// version fails with error_interno.
	maxIntentos = 3
	// tiempoOperacion bounds normalising and publishing one version (≤ 2 MiB,
	// ≤ 5 000 passages). It is detached from the worker's lifetime so shutdown
	// lets it finish, and stays below the server's workerDrainTimeout (30 s).
	// If it is cut anyway the transaction rolls back and the claim retries.
	tiempoOperacion = 25 * time.Second
)

// colaIngesta hands out the next version to process.
type colaIngesta interface {
	Reclamar(ctx context.Context) (VersionReclamada, bool, error)
}

// publicador is the part of SourceRepository the worker writes with.
type publicador interface {
	PublicarVersion(ctx context.Context, fuenteID int64, version int, pasajes []PasajeNuevo, aud Auditor) error
	MarcarVersionFallida(ctx context.Context, fuenteID int64, version int, motivo string, aud Auditor) error
}

// IngestaWorker normalises every claimed 'procesando' version and publishes
// its passages, or marks it failed:
//   - deterministic failures (sin_contenido, limite_pasajes, and an invalid
//     publication, which is a programming error) fail on the first attempt;
//   - transient ones (database, panic) leave the claim to expire and retry,
//     and fail with error_interno on the third attempt.
//
// Logs carry only the operation, fuente id, version and attempt, never the
// content (SIA-E2-02 § Seguridad).
type IngestaWorker struct {
	cola       colaIngesta
	repo       publicador
	aud        Auditor
	normalizar func([]byte) ([]PasajeNuevo, error) // Normalizar; replaceable in tests
	intervalo  time.Duration
}

// NewIngestaWorker creates the worker; aud audits as the system (actor nil).
func NewIngestaWorker(cola colaIngesta, repo publicador, aud Auditor) *IngestaWorker {
	return &IngestaWorker{cola: cola, repo: repo, aud: aud, normalizar: Normalizar, intervalo: intervaloSondeo}
}

// AuditorSistema writes the worker's audit rows with audit.LogTx, with no
// actor, IP or user agent (actor nil = system).
func AuditorSistema(l *audit.Logger) Auditor {
	return func(ctx context.Context, tx sqlx.ExecerContext, ev Evento) error {
		id := ev.EntidadID
		return l.LogTx(ctx, tx, nil, ev.Accion, ev.Entidad, &id, ev.Detalle, "", "")
	}
}

// Run polls every intervalo until ctx is cancelled. Each tick drains the queue;
// the version in progress when ctx is cancelled is finished before Run returns.
func (w *IngestaWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.intervalo)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for ctx.Err() == nil && w.procesarSiguiente(ctx) {
			}
		}
	}
}

// procesarSiguiente claims and processes one version; it reports whether one
// was claimed, so Run keeps draining.
func (w *IngestaWorker) procesarSiguiente(ctx context.Context) bool {
	v, ok, err := w.cola.Reclamar(ctx)
	if err != nil {
		log.Error().Err(err).Str("op", "fuentes.reclamar").Msg("ingesta: claim failed")
		return false
	}
	if !ok {
		return false
	}
	c, cancel := context.WithTimeout(context.WithoutCancel(ctx), tiempoOperacion)
	defer cancel()
	w.procesar(c, v)
	return true
}

// procesar decides the outcome of one claimed version.
func (w *IngestaWorker) procesar(ctx context.Context, v VersionReclamada) {
	if v.Intentos > maxIntentos { // a claim that expired after the last attempt
		w.fallar(ctx, v, MotivoErrorInterno)
		return
	}
	pasajes, err := w.normalizarSeguro(v.Contenido)
	switch {
	case errors.Is(err, ErrSinContenido):
		w.fallar(ctx, v, MotivoSinContenido)
		return
	case errors.Is(err, ErrLimitePasajes):
		w.fallar(ctx, v, MotivoLimitePasajes)
		return
	case err != nil:
		w.transitorio(ctx, v, "fuentes.normalizar", err)
		return
	}
	err = w.repo.PublicarVersion(ctx, v.FuenteID, v.Version, pasajes, w.aud)
	switch {
	case err == nil:
		registrar(v).Int("pasajes", len(pasajes)).Msg("ingesta: version published")
	case errors.Is(err, ErrPublicacionInvalida):
		registrarError(v, "fuentes.publicar", err).Msg("ingesta: invalid publication")
		w.fallar(ctx, v, MotivoErrorInterno)
	case errors.Is(err, ErrNoEncontrado):
		registrar(v).Msg("ingesta: version no longer in process, skipped")
	default:
		w.transitorio(ctx, v, "fuentes.publicar", err)
	}
}

// transitorio leaves the claim to expire so the version retries, unless this
// was the last attempt.
func (w *IngestaWorker) transitorio(ctx context.Context, v VersionReclamada, op string, err error) {
	registrarError(v, op, err).Msg("ingesta: transient failure")
	if v.Intentos >= maxIntentos {
		w.fallar(ctx, v, MotivoErrorInterno)
	}
}

// fallar marks the version failed. If that cannot be recorded, the claim
// expires and the next attempt fails it again.
func (w *IngestaWorker) fallar(ctx context.Context, v VersionReclamada, motivo string) {
	err := w.repo.MarcarVersionFallida(ctx, v.FuenteID, v.Version, motivo, w.aud)
	switch {
	case err == nil:
		registrar(v).Str("motivo", motivo).Msg("ingesta: version failed")
	case errors.Is(err, ErrNoEncontrado):
		registrar(v).Msg("ingesta: version no longer in process, skipped")
	default:
		registrarError(v, "fuentes.fallar", err).Str("motivo", motivo).Msg("ingesta: could not mark the version failed")
	}
}

// normalizarSeguro turns a panic of the normaliser into a transient error.
func (w *IngestaWorker) normalizarSeguro(contenido []byte) (pasajes []PasajeNuevo, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic al normalizar: %v", p)
		}
	}()
	return w.normalizar(contenido)
}

// registrar and registrarError start a log line with the version's ids only.
func registrar(v VersionReclamada) *zerolog.Event {
	return log.Info().Int64("fuente_id", v.FuenteID).Int("version", v.Version).Int("intento", v.Intentos)
}

func registrarError(v VersionReclamada, op string, err error) *zerolog.Event {
	return log.Warn().Err(err).Str("op", op).Int64("fuente_id", v.FuenteID).Int("version", v.Version).Int("intento", v.Intentos)
}
