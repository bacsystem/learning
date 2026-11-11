package fuentes

import "context"

// ProcesarSiguiente exposes one worker step to the integration tests of
// package fuentes_test.
func (w *IngestaWorker) ProcesarSiguiente(ctx context.Context) bool { return w.procesarSiguiente(ctx) }

// Procesar exposes the processing of an already claimed version, to replay a
// stale worker in the integration tests.
func (w *IngestaWorker) Procesar(ctx context.Context, v VersionReclamada) { w.procesar(ctx, v) }
