package fuentes

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// colaFalsa hands out its versions in order; vacia is closed (if set) when a
// claim finds the queue empty.
type colaFalsa struct {
	mu        sync.Mutex
	versiones []VersionReclamada
	err       error
	vacia     chan struct{}
}

func (c *colaFalsa) Reclamar(context.Context) (VersionReclamada, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return VersionReclamada{}, false, c.err
	}
	if len(c.versiones) == 0 {
		if c.vacia != nil {
			close(c.vacia)
			c.vacia = nil
		}
		return VersionReclamada{}, false, nil
	}
	v := c.versiones[0]
	c.versiones = c.versiones[1:]
	return v, true, nil
}

// publicadorFalso records what the worker did with each version.
type publicadorFalso struct {
	errPublicar, errFallar error
	publicados             []int // passages per published version
	fallidos               []string
}

func (p *publicadorFalso) PublicarVersion(_ context.Context, _ int64, _ int, pasajes []PasajeNuevo, _ Auditor) error {
	if p.errPublicar != nil {
		return p.errPublicar
	}
	p.publicados = append(p.publicados, len(pasajes))
	return nil
}

func (p *publicadorFalso) MarcarVersionFallida(_ context.Context, _ int64, _ int, motivo string, _ Auditor) error {
	if p.errFallar != nil {
		return p.errFallar
	}
	p.fallidos = append(p.fallidos, motivo)
	return nil
}

func TestIngestaWorker_Procesar(t *testing.T) {
	texto := []byte(strings.Repeat("palabra ", 300))
	var limite strings.Builder
	for range MaxPasajes + 1 {
		limite.WriteString("# S\n\n" + strings.Repeat("p ", MinPalabrasPasaje) + "\n\n")
	}
	errBD := errors.New("conexión perdida")
	cases := []struct {
		name          string
		contenido     []byte
		intentos      int
		errPublicar   error
		wantPublicado bool
		wantFallo     string // "" = not marked failed
	}{
		{"publishes the passages", texto, 1, nil, true, ""},
		{"empty content fails at once", []byte("  \n "), 1, nil, false, MotivoSinContenido},
		{"too many passages fail at once", []byte(limite.String()), 1, nil, false, MotivoLimitePasajes},
		{"invalid publication is error_interno at once", texto, 1, ErrPublicacionInvalida, false, MotivoErrorInterno},
		{"version no longer in process is skipped", texto, 1, ErrNoEncontrado, false, ""},
		{"transient error retries on attempt 1", texto, 1, errBD, false, ""},
		{"transient error retries on attempt 2", texto, 2, errBD, false, ""},
		{"transient error on attempt 3 is error_interno", texto, maxIntentos, errBD, false, MotivoErrorInterno},
		{"expired claim after the last attempt is error_interno", texto, maxIntentos + 1, nil, false, MotivoErrorInterno},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pub := &publicadorFalso{errPublicar: c.errPublicar}
			cola := &colaFalsa{versiones: []VersionReclamada{{FuenteID: 1, Version: 1, Contenido: c.contenido, Intentos: c.intentos}}}
			w := NewIngestaWorker(cola, pub, nil)
			if !w.procesarSiguiente(context.Background()) {
				t.Fatal("procesarSiguiente: want a claimed version")
			}
			if got := len(pub.publicados) == 1; got != c.wantPublicado {
				t.Errorf("published %v, want %v", got, c.wantPublicado)
			}
			gotFallo := strings.Join(pub.fallidos, ",")
			if gotFallo != c.wantFallo {
				t.Errorf("failed with %q, want %q", gotFallo, c.wantFallo)
			}
		})
	}
}

func TestIngestaWorker_ColaVaciaOConError(t *testing.T) {
	for _, cola := range []*colaFalsa{{}, {err: errors.New("bd caída")}} {
		pub := &publicadorFalso{}
		if NewIngestaWorker(cola, pub, nil).procesarSiguiente(context.Background()) {
			t.Errorf("cola %+v: want nothing processed", cola)
		}
		if len(pub.publicados)+len(pub.fallidos) != 0 {
			t.Errorf("cola %+v: the worker wrote %+v", cola, pub)
		}
	}
}

// If marking the version failed does not work, the worker moves on: the
// claim expires and the next attempt fails it again.
func TestIngestaWorker_FallarSinRegistrar(t *testing.T) {
	pub := &publicadorFalso{errFallar: errors.New("bd caída")}
	cola := &colaFalsa{versiones: []VersionReclamada{{FuenteID: 1, Version: 1, Contenido: nil, Intentos: 1}}}
	if !NewIngestaWorker(cola, pub, nil).procesarSiguiente(context.Background()) {
		t.Fatal("want a claimed version")
	}
	if len(pub.fallidos) != 0 {
		t.Errorf("fallidos %v", pub.fallidos)
	}
}

// A panic while normalising is a transient failure: retried, and error_interno
// on the last attempt.
func TestIngestaWorker_PanicoAlNormalizar(t *testing.T) {
	for _, c := range []struct {
		intentos  int
		wantFallo string
	}{{1, ""}, {maxIntentos, MotivoErrorInterno}} {
		pub := &publicadorFalso{}
		cola := &colaFalsa{versiones: []VersionReclamada{{FuenteID: 1, Version: 1, Intentos: c.intentos}}}
		w := NewIngestaWorker(cola, pub, nil)
		w.normalizar = func([]byte) ([]PasajeNuevo, error) { panic("fallo interno") }
		if !w.procesarSiguiente(context.Background()) {
			t.Fatal("want a claimed version")
		}
		if got := strings.Join(pub.fallidos, ","); got != c.wantFallo || len(pub.publicados) != 0 {
			t.Errorf("intento %d: fallidos %q publicados %v, want %q", c.intentos, got, pub.publicados, c.wantFallo)
		}
	}
}

// Run drains the queue on a tick and returns once ctx is cancelled.
func TestIngestaWorker_Run(t *testing.T) {
	texto := []byte(strings.Repeat("palabra ", 200))
	cola := &colaFalsa{vacia: make(chan struct{}), versiones: []VersionReclamada{
		{FuenteID: 1, Version: 1, Contenido: texto, Intentos: 1},
		{FuenteID: 2, Version: 1, Contenido: texto, Intentos: 1},
	}}
	vacia := cola.vacia
	pub := &publicadorFalso{}
	w := NewIngestaWorker(cola, pub, nil)
	w.intervalo = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	hecho := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(hecho)
	}()
	select {
	case <-vacia:
	case <-time.After(5 * time.Second):
		t.Fatal("the worker did not drain the queue")
	}
	cancel()
	<-hecho
	if len(pub.publicados) != 2 {
		t.Errorf("published %d versions, want 2", len(pub.publicados))
	}
}
