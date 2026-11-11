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
	// tiempoFallo bounds recording a failure (one small transaction).
	tiempoFallo = 5 * time.Second
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
// was claimed, so Run keeps draining. A panic anywhere in the processing is
// logged and leaves the claim to expire, so it never stops the server.
func (w *IngestaWorker) procesarSiguiente(ctx context.Context) (reclamada bool) {
	v, ok, err := w.cola.Reclamar(ctx)
	if err != nil {
		log.Error().Err(err).Str("op", "fuentes.reclamar").Msg("ingesta: claim failed")
		return false
	}
	if !ok {
		return false
	}
	defer func() {
		if p := recover(); p != nil {
			registrarError(v, "fuentes.procesar", fmt.Errorf("panic: %v", p)).Msg("ingesta: processing panicked")
			reclamada = true
		}
	}()
	w.procesar(ctx, v)
	return true
}

// procesar publishes one claimed version or records why it failed. Every
// write runs on a context detached from the worker's lifetime, so a version
// in progress at shutdown is finished.
func (w *IngestaWorker) procesar(ctx context.Context, v VersionReclamada) {
	c, cancel := context.WithTimeout(context.WithoutCancel(ctx), tiempoOperacion)
	motivo, err := w.publicar(c, v)
	cancel()
	switch {
	case motivo != "":
		w.fallar(ctx, v, motivo)
	case err != nil:
		registrarError(v, "fuentes.publicar", err).Msg("ingesta: transient failure")
		if v.Intentos >= maxIntentos {
			w.fallar(ctx, v, MotivoErrorInterno)
		}
	}
}

// publicar normalises and publishes v. It returns the failure reason of a
// deterministic failure, or the error of a transient one (database, panic).
func (w *IngestaWorker) publicar(ctx context.Context, v VersionReclamada) (motivo string, err error) {
	defer func() {
		if p := recover(); p != nil {
			motivo, err = "", fmt.Errorf("panic: %v", p)
		}
	}()
	if v.Intentos > maxIntentos { // a claim that expired after the last attempt
		return MotivoErrorInterno, nil
	}
	pasajes, err := w.normalizar(v.Contenido)
	switch {
	case errors.Is(err, ErrSinContenido):
		return MotivoSinContenido, nil
	case errors.Is(err, ErrLimitePasajes):
		return MotivoLimitePasajes, nil
	case err != nil:
		return "", err
	}
	err = w.repo.PublicarVersion(ctx, v.FuenteID, v.Version, pasajes, w.aud)
	switch {
	case err == nil:
		registrar(v).Int("pasajes", len(pasajes)).Msg("ingesta: version published")
		return "", nil
	case errors.Is(err, ErrPublicacionInvalida): // a programming error: retrying cannot help
		registrarError(v, "fuentes.publicar", err).Msg("ingesta: invalid publication")
		return MotivoErrorInterno, nil
	case errors.Is(err, ErrNoEncontrado):
		registrar(v).Msg("ingesta: version no longer in process, skipped")
		return "", nil
	default:
		return "", err
	}
}

// fallar marks the version failed, on its own deadline so a timed-out
// publication can still be recorded. If that fails too, the claim expires and
// the next attempt fails it again.
func (w *IngestaWorker) fallar(ctx context.Context, v VersionReclamada, motivo string) {
	c, cancel := context.WithTimeout(context.WithoutCancel(ctx), tiempoFallo)
	defer cancel()
	err := w.repo.MarcarVersionFallida(c, v.FuenteID, v.Version, motivo, w.aud)
	switch {
	case err == nil:
		registrar(v).Str("motivo", motivo).Msg("ingesta: version failed")
	case errors.Is(err, ErrNoEncontrado):
		registrar(v).Msg("ingesta: version no longer in process, skipped")
	default:
		registrarError(v, "fuentes.fallar", err).Str("motivo", motivo).Msg("ingesta: could not mark the version failed")
	}
}

// registrar and registrarError start a log line with the version's ids only.
func registrar(v VersionReclamada) *zerolog.Event {
	return log.Info().Int64("fuente_id", v.FuenteID).Int("version", v.Version).Int("intento", v.Intentos)
}

func registrarError(v VersionReclamada, op string, err error) *zerolog.Event {
	return log.Warn().Err(err).Str("op", op).Int64("fuente_id", v.FuenteID).Int("version", v.Version).Int("intento", v.Intentos)
}
