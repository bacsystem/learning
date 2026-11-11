package salud

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// relojFijo is a clock the test moves by hand.
type relojFijo struct {
	mu sync.Mutex
	t  time.Time
}

func (r *relojFijo) ahora() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.t
}

func (r *relojFijo) avanzar(d time.Duration) {
	r.mu.Lock()
	r.t = r.t.Add(d)
	r.mu.Unlock()
}

var inicio = time.Date(2026, 11, 11, 15, 4, 5, 0, time.UTC)

func categoria(s string) *string { return &s }

// sonda is an isolated check (doble de prueba) that counts its calls.
func sonda(id string, llamadas *atomic.Int32, f func(ctx context.Context) Servicio) Comprobacion {
	return Comprobacion{ID: id, SinResultado: map[string]any{"dato": nil}, Comprobar: func(ctx context.Context) Servicio {
		if llamadas != nil {
			llamadas.Add(1)
		}
		return f(ctx)
	}}
}

func operativa(id string) func(context.Context) Servicio {
	return func(context.Context) Servicio {
		return Servicio{ID: id, Estado: EstadoOperativo, Datos: map[string]any{"dato": 1}}
	}
}

func TestEstadoGlobal(t *testing.T) {
	cases := []struct {
		estados []string
		want    string
	}{
		{[]string{EstadoOperativo, EstadoOperativo}, EstadoOperativo},
		{[]string{EstadoOperativo, EstadoDegradado}, EstadoDegradado},
		{[]string{EstadoOperativo, EstadoDesconocido}, EstadoDesconocido},
		{[]string{EstadoDesconocido, EstadoDegradado}, EstadoDegradado},
		{[]string{EstadoDegradado, EstadoCaido, EstadoOperativo}, EstadoCaido},
		{[]string{EstadoNoConfigurado, EstadoOperativo}, EstadoOperativo},
		{[]string{EstadoNoConfigurado, EstadoNoConfigurado}, EstadoOperativo},
		{nil, EstadoOperativo},
	}
	for _, c := range cases {
		svs := make([]Servicio, len(c.estados))
		for i, e := range c.estados {
			svs[i] = Servicio{Estado: e}
		}
		if got := estadoGlobal(svs); got != c.want {
			t.Errorf("estadoGlobal(%v) = %s, want %s", c.estados, got, c.want)
		}
	}
}

// Comprobación que no termina: the answer comes in < 3 s with tiempo_agotado,
// both for a check that honours ctx and for one that ignores it.
func TestEstado_TiempoAgotado(t *testing.T) {
	liberar := make(chan struct{})
	defer close(liberar)
	cases := []struct {
		name  string
		sonda func(ctx context.Context) Servicio
	}{
		{"honours ctx", func(ctx context.Context) Servicio { <-ctx.Done(); return Servicio{ID: "x", Estado: EstadoOperativo} }},
		{"ignores ctx", func(context.Context) Servicio { <-liberar; return Servicio{ID: "x", Estado: EstadoOperativo} }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := &relojFijo{t: inicio}
			svc := NewService([]Comprobacion{sonda(IDAPI, nil, operativa(IDAPI)), sonda(IDBaseDatos, nil, c.sonda)}, r.ahora)
			desde := time.Now()
			got := svc.Estado(context.Background())
			if d := time.Since(desde); d >= 3*time.Second {
				t.Fatalf("answer took %v, want < 3 s", d)
			}
			bd := got.Servicios[1]
			if bd.ID != IDBaseDatos || bd.Estado != EstadoCaido || bd.Categoria == nil || *bd.Categoria != CategoriaTiempoAgotado {
				t.Errorf("base_datos %+v, want caido/tiempo_agotado", bd)
			}
			if v, ok := bd.Datos["dato"]; !ok || v != nil {
				t.Errorf("datos %v, want the keys with null", bd.Datos)
			}
			if got.Estado != EstadoCaido || got.Servicios[0].Estado != EstadoOperativo {
				t.Errorf("global %s, api %s", got.Estado, got.Servicios[0].Estado)
			}
		})
	}
}

// A panic inside a check is caido / error_interno for that service only.
func TestEstado_Panico(t *testing.T) {
	r := &relojFijo{t: inicio}
	svc := NewService([]Comprobacion{
		sonda(IDAPI, nil, operativa(IDAPI)),
		sonda(IDBaseDatos, nil, func(context.Context) Servicio { panic("dial tcp 10.0.0.5:5432 user=admin") }),
	}, r.ahora)
	got := svc.Estado(context.Background())
	bd := got.Servicios[1]
	if bd.Estado != EstadoCaido || bd.Categoria == nil || *bd.Categoria != CategoriaErrorInterno || bd.Datos == nil {
		t.Errorf("base_datos %+v, want caido/error_interno with datos", bd)
	}
	if got.Servicios[0].Estado != EstadoOperativo {
		t.Errorf("api %+v", got.Servicios[0])
	}
}

// Caché de 15 s: same comprobado_en and no new probe within it; a new run after it.
func TestEstado_Cache(t *testing.T) {
	r := &relojFijo{t: inicio}
	var llamadas atomic.Int32
	svc := NewService([]Comprobacion{sonda(IDBaseDatos, &llamadas, operativa(IDBaseDatos))}, r.ahora)

	primera := svc.Estado(context.Background())
	r.avanzar(5 * time.Second)
	segunda := svc.Estado(context.Background())
	if !segunda.ComprobadoEn.Equal(primera.ComprobadoEn) || llamadas.Load() != 1 {
		t.Fatalf("within 15 s: comprobado_en %v vs %v, %d probes; want the same and 1", segunda.ComprobadoEn, primera.ComprobadoEn, llamadas.Load())
	}
	r.avanzar(10 * time.Second) // 15 s after the run: expired
	tercera := svc.Estado(context.Background())
	if !tercera.ComprobadoEn.Equal(inicio.Add(15*time.Second)) || llamadas.Load() != 2 {
		t.Errorf("after 15 s: comprobado_en %v, %d probes; want a new run", tercera.ComprobadoEn, llamadas.Load())
	}
}

// Concurrent requests with an expired cache share one run.
func TestEstado_CacheConcurrente(t *testing.T) {
	r := &relojFijo{t: inicio}
	var llamadas atomic.Int32
	svc := NewService([]Comprobacion{sonda(IDBaseDatos, &llamadas, func(context.Context) Servicio {
		time.Sleep(20 * time.Millisecond)
		return Servicio{ID: IDBaseDatos, Estado: EstadoOperativo, Datos: map[string]any{}}
	})}, r.ahora)
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc.Estado(context.Background())
		}()
	}
	wg.Wait()
	if n := llamadas.Load(); n != 1 {
		t.Errorf("%d probes, want 1", n)
	}
}

// A client that hangs up does not cache a timeout for everyone.
func TestEstado_CancelacionDelClienteNoSeCachea(t *testing.T) {
	r := &relojFijo{t: inicio}
	svc := NewService([]Comprobacion{sonda(IDBaseDatos, nil, func(ctx context.Context) Servicio {
		if ctx.Err() != nil {
			return Servicio{ID: IDBaseDatos, Estado: EstadoCaido, Categoria: categoria(CategoriaTiempoAgotado), Datos: map[string]any{}}
		}
		return Servicio{ID: IDBaseDatos, Estado: EstadoOperativo, Datos: map[string]any{}}
	})}, r.ahora)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := svc.Estado(ctx); got.Servicios[0].Estado != EstadoOperativo {
		t.Errorf("with the request cancelled: %+v, want operativo", got.Servicios[0])
	}
}

func TestComprobarBD(t *testing.T) {
	errDSN := errors.New(`dial tcp 10.0.0.5:5432: password authentication failed for user "admin" (postgres://admin:s3cr3t@db.internal:5432/app)`)
	cases := []struct {
		name          string
		latencia      time.Duration
		err           error
		ctxVencido    bool
		wantEstado    string
		wantCategoria *string
		wantLatencia  any
	}{
		{"20 ms is operativo", 20 * time.Millisecond, nil, false, EstadoOperativo, nil, int64(20)},
		{"500 ms is still operativo", 500 * time.Millisecond, nil, false, EstadoOperativo, nil, int64(500)},
		{"800 ms is degradado", 800 * time.Millisecond, nil, false, EstadoDegradado, nil, int64(800)},
		{"sub-millisecond latency truncates to 0", 400 * time.Microsecond, nil, false, EstadoOperativo, nil, int64(0)},
		{"connection error is sin_conexion", 0, errDSN, false, EstadoCaido, categoria(CategoriaSinConexion), nil},
		{"error after the deadline is tiempo_agotado (57014)", 0, errors.New("pq: canceling statement due to user request"), true, EstadoCaido, categoria(CategoriaTiempoAgotado), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := &relojFijo{t: inicio}
			comp := comprobarBD(func(context.Context) error { r.avanzar(c.latencia); return c.err }, r.ahora)
			ctx := context.Background()
			if c.ctxVencido {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			got := comp.Comprobar(ctx)
			if got.ID != IDBaseDatos || got.Estado != c.wantEstado || (got.Categoria == nil) != (c.wantCategoria == nil) ||
				(c.wantCategoria != nil && *got.Categoria != *c.wantCategoria) {
				t.Fatalf("got %+v, want %s/%v", got, c.wantEstado, c.wantCategoria)
			}
			if v, ok := got.Datos["latencia_ms"]; !ok || v != c.wantLatencia {
				t.Errorf("latencia_ms %v (present %v), want %v", v, ok, c.wantLatencia)
			}
		})
	}
}

func TestComprobarAPI(t *testing.T) {
	r := &relojFijo{t: inicio.Add(86400*time.Second + 900*time.Millisecond)}
	got := ComprobarAPI("1.4.0", inicio, r.ahora).Comprobar(context.Background())
	if got.ID != IDAPI || got.Estado != EstadoOperativo || got.Categoria != nil ||
		got.Datos["version"] != "1.4.0" || got.Datos["en_marcha_s"] != int64(86400) {
		t.Errorf("got %+v", got)
	}
}

// Sin secretos ni datos internos: a database error with host, user and DSN
// leaves only the category in the response body.
func TestObtener_SinSecretos(t *testing.T) {
	r := &relojFijo{t: inicio}
	dsn := errors.New(`dial tcp 10.0.0.5:5432: password authentication failed for user "admin" (postgres://admin:s3cr3t@db.internal:5432/app)`)
	svc := NewService([]Comprobacion{
		ComprobarAPI("1.4.0", inicio, r.ahora),
		comprobarBD(func(context.Context) error { return dsn }, r.ahora),
	}, r.ahora)
	w := httptest.NewRecorder()
	NewHandler(svc).Obtener(w, httptest.NewRequest(http.MethodGet, "/admin/salud", nil))
	body := w.Body.String()
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("code %d, content type %q", w.Code, w.Header().Get("Content-Type"))
	}
	for _, secreto := range []string{"10.0.0.5", "5432", "admin", "s3cr3t", "db.internal", "postgres://", "password", "dial"} {
		if strings.Contains(body, secreto) {
			t.Errorf("body leaks %q: %s", secreto, body)
		}
	}
	var got struct {
		Estado       string `json:"estado"`
		ComprobadoEn string `json:"comprobado_en"`
		Servicios    []struct {
			ID        string         `json:"id"`
			Estado    string         `json:"estado"`
			Categoria *string        `json:"categoria"`
			Datos     map[string]any `json:"datos"`
		} `json:"servicios"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	bd := got.Servicios[1]
	if got.Estado != EstadoCaido || got.ComprobadoEn != "2026-11-11T15:04:05Z" || bd.Categoria == nil || *bd.Categoria != CategoriaSinConexion {
		t.Errorf("unexpected body %s", body)
	}
	if v, ok := bd.Datos["latencia_ms"]; !ok || v != nil {
		t.Errorf(`datos %v, want {"latencia_ms": null}`, bd.Datos)
	}
	if !strings.Contains(body, `"categoria":null`) {
		t.Errorf("api categoria must be an explicit null: %s", body)
	}
}
