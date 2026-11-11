package salud

import (
	"context"
	"maps"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	// tiempoComprobacion bounds every check (PLT-01 reglas 3–4).
	tiempoComprobacion = 2 * time.Second
	// vigenciaCache: within it every request gets the same result and the
	// same comprobado_en (PLT-01 regla 6).
	vigenciaCache = 15 * time.Second
)

// Comprobacion checks one service. Comprobar must honour ctx. SinResultado is
// the Datos of the service when the check does not finish or panics: the
// same keys with null values, so Datos is never null.
type Comprobacion struct {
	ID           string
	Comprobar    func(ctx context.Context) Servicio
	SinResultado map[string]any
}

// Service runs the checks in parallel and caches the answer for 15 s.
type Service struct {
	comprobaciones []Comprobacion
	reloj          func() time.Time
	tiempo         time.Duration

	mu         sync.Mutex
	cache      *Respuesta
	cacheHasta time.Time
}

// NewService creates the service; comprobaciones keep the contract order
// (api, base_datos, correo, worker_ingesta, copia_r2).
func NewService(comprobaciones []Comprobacion, reloj func() time.Time) *Service {
	return &Service{comprobaciones: comprobaciones, reloj: reloj, tiempo: tiempoComprobacion}
}

// Estado returns the cached answer or runs the checks. Concurrent requests
// with an expired cache share one run (mutex with a double check). The checks
// are detached from the request, so a client that hangs up does not cache a
// timeout for everyone.
func (s *Service) Estado(ctx context.Context) Respuesta {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cache != nil && s.reloj().Before(s.cacheHasta) {
		return *s.cache
	}
	r := s.ejecutar(context.WithoutCancel(ctx))
	s.cache, s.cacheHasta = &r, r.ComprobadoEn.Add(vigenciaCache)
	return r
}

// ejecutar runs every check in parallel, each under its own deadline.
func (s *Service) ejecutar(ctx context.Context) Respuesta {
	// RFC 3339 to the second, as in the contract example.
	comprobadoEn := s.reloj().UTC().Truncate(time.Second)
	servicios := make([]Servicio, len(s.comprobaciones))
	var wg sync.WaitGroup
	for i, c := range s.comprobaciones {
		wg.Add(1)
		go func() {
			defer wg.Done()
			servicios[i] = s.comprobar(ctx, c)
		}()
	}
	wg.Wait()
	return Respuesta{Estado: estadoGlobal(servicios), ComprobadoEn: comprobadoEn, Servicios: servicios}
}

// comprobar runs c without waiting past its deadline: a check that does not
// finish is caido / tiempo_agotado, and a panic is caido / error_interno. The
// buffered channel lets a late check end without blocking; it honours ctx, so
// it ends soon after the deadline.
func (s *Service) comprobar(ctx context.Context, c Comprobacion) Servicio {
	ctx, cancel := context.WithTimeout(ctx, s.tiempo)
	defer cancel()
	resultado := make(chan Servicio, 1)
	go func() {
		defer func() {
			if recover() != nil {
				// The panic value may quote internal details: log the op only.
				log.Error().Str("op", "salud.comprobar").Str("servicio", c.ID).Msg("service check panicked")
				resultado <- fallido(c, CategoriaErrorInterno)
			}
		}()
		resultado <- c.Comprobar(ctx)
	}()
	var sv Servicio
	select {
	case sv = <-resultado:
		// The id and a non-null Datos are the service's, whatever the check returns.
		sv.ID = c.ID
		if sv.Datos == nil {
			sv.Datos = copiar(c.SinResultado)
		}
	case <-ctx.Done():
		sv = fallido(c, CategoriaTiempoAgotado)
	}
	if sv.Categoria != nil {
		log.Warn().Str("op", "salud.comprobar").Str("servicio", c.ID).Str("estado", sv.Estado).
			Str("categoria", *sv.Categoria).Msg("service check not operational")
	}
	return sv
}

// fallido is a caido service with its category and the null data.
func fallido(c Comprobacion, categoria string) Servicio {
	return Servicio{ID: c.ID, Estado: EstadoCaido, Categoria: &categoria, Datos: copiar(c.SinResultado)}
}

// copiar gives each answer its own Datos map (never nil), so a cached answer
// is not shared mutable state.
func copiar(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	maps.Copy(out, m)
	return out
}

// gravedad orders the states for the global one: caido > degradado =
// desconocido > operativo; no_configurado does not count (PLT-01 regla 5).
func gravedad(estado string) (nivel int, cuenta bool) {
	switch estado {
	case EstadoOperativo:
		return 0, true
	case EstadoDegradado, EstadoDesconocido:
		return 1, true
	case EstadoCaido:
		return 2, true
	default: // no_configurado
		return 0, false
	}
}

// estadoGlobal is the worst individual state; on a tie between degradado and
// desconocido, degradado. With no counting service it is operativo.
func estadoGlobal(servicios []Servicio) string {
	peor, nivelPeor := EstadoOperativo, 0
	for _, sv := range servicios {
		nivel, cuenta := gravedad(sv.Estado)
		if !cuenta {
			continue
		}
		if nivel > nivelPeor || (nivel == nivelPeor && sv.Estado == EstadoDegradado) {
			peor, nivelPeor = sv.Estado, nivel
		}
	}
	return peor
}
