package fuentes

import "context"

// ProcesarSiguiente exposes one worker step to the integration tests of
// package fuentes_test.
func (w *IngestaWorker) ProcesarSiguiente(ctx context.Context) bool { return w.procesarSiguiente(ctx) }
