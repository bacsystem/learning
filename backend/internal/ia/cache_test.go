package ia

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
)

// cacheMemoria is the cache in memory; err makes both operations fail.
type cacheMemoria struct {
	mu       sync.Mutex
	entradas map[string][]byte
	fuentes  map[string][]int64
	err      error
	lecturas int
}

func nuevaCacheMemoria() *cacheMemoria {
	return &cacheMemoria{entradas: map[string][]byte{}, fuentes: map[string][]int64{}}
}

func (c *cacheMemoria) Buscar(_ context.Context, clave string) ([]byte, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lecturas++
	if c.err != nil {
		return nil, false, c.err
	}
	raw, ok := c.entradas[clave]
	return raw, ok, nil
}

func (c *cacheMemoria) Guardar(_ context.Context, clave, _ string, salida []byte, fuentes []int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return c.err
	}
	c.entradas[clave], c.fuentes[clave] = salida, fuentes
	return nil
}

// La clave: contract E6-02 §4.
func TestClaveCache(t *testing.T) {
	base := entradaClasificar(2).solicitud()
	modificar := func(f func(*Solicitud)) Solicitud {
		s := entradaClasificar(2).solicitud()
		f(&s)
		return s
	}
	iguales := []struct {
		name string
		sol  Solicitud
	}{
		{"same request", base},
		{"whitespace of the texts is collapsed", modificar(func(s *Solicitud) {
			s.Datos = append([]Bloque(nil), s.Datos...)
			s.Datos[0].Texto = "  " + strings.ReplaceAll(s.Datos[0].Texto, " ", "\t \n") + "  "
		})},
		{"the sources do not enter the key", modificar(func(s *Solicitud) { s.Fuentes = []int64{1, 2, 3} })},
	}
	for _, tt := range iguales {
		t.Run("same key: "+tt.name, func(t *testing.T) {
			if claveCache(tt.sol) != claveCache(base) {
				t.Error("the key changed")
			}
		})
	}
	distintas := []struct {
		name string
		sol  Solicitud
	}{
		{"another operation", modificar(func(s *Solicitud) { s.Operacion = OpGenerarMaterial })},
		{"another output schema", modificar(func(s *Solicitud) { s.EsquemaSalida = "clasificar.v2" })},
		{"other instructions (a new prompt)", modificar(func(s *Solicitud) { s.Instrucciones += " Sé breve." })},
		{"another text", modificar(func(s *Solicitud) {
			s.Datos = append([]Bloque(nil), s.Datos...)
			s.Datos[0].Texto += "?"
		})},
		{"another label", modificar(func(s *Solicitud) {
			s.Datos = append([]Bloque(nil), s.Datos...)
			s.Datos[0].Etiqueta += "9"
		})},
		{"fewer blocks", modificar(func(s *Solicitud) { s.Datos = s.Datos[:len(s.Datos)-1] })},
		{"the boundary between label and text matters", modificar(func(s *Solicitud) {
			s.Datos = []Bloque{{Etiqueta: "a", Texto: "bc"}}
		})},
	}
	for _, tt := range distintas {
		t.Run("another key: "+tt.name, func(t *testing.T) {
			if claveCache(tt.sol) == claveCache(base) {
				t.Error("the key did not change")
			}
		})
	}
	if a, b := claveCache(Solicitud{Datos: []Bloque{{Etiqueta: "a", Texto: "bc"}}}), claveCache(Solicitud{Datos: []Bloque{{Etiqueta: "ab", Texto: "c"}}}); a == b {
		t.Error("label and text can be shifted without changing the key")
	}
	if len(claveCache(base)) != 64 {
		t.Errorf("key length = %d, want 64 hex", len(claveCache(base)))
	}
}

func llamadasDelProveedor(p *proveedorGuion) int { return p.llamado }

// ★1 and «Caché»: the second identical request is served from the cache, with
// no provider call, its action given back and a cache row without cost.
func TestServicio_Cache(t *testing.T) {
	prov := &proveedorGuion{pasos: []paso{valida(1)}}
	reg, costos, cache := &registroMemoria{}, &costosMemoria{}, nuevaCacheMemoria()
	s, _ := nuevoConCostos(prov, reg, costos, ConCache(cache))

	r1 := &Reserva{Actor: sistema, Acciones: 2}
	primera, err := s.Clasificar(context.Background(), r1, entradaClasificar(1))
	if err != nil || len(cache.entradas) != 1 || r1.Acciones != 1 {
		t.Fatalf("first call: %+v, %v, %d entries, acciones %d", primera, err, len(cache.entradas), r1.Acciones)
	}
	segunda, err := s.Clasificar(context.Background(), r1, entradaClasificar(1))
	if err != nil {
		t.Fatal(err)
	}
	if llamadasDelProveedor(prov) != 1 {
		t.Errorf("%d provider calls, want 1", prov.llamado)
	}
	if costos.liberadas != 1 || r1.Acciones != 0 {
		t.Errorf("released %d, acciones %d; want the hit to give its action back (1 and 0)", costos.liberadas, r1.Acciones)
	}
	if len(segunda.Resultados) != 1 || segunda.Resultados[0] != primera.Resultados[0] {
		t.Errorf("cached output %+v differs from %+v", segunda, primera)
	}
	if len(reg.usos) != 2 {
		t.Fatalf("%d usage rows, want 2", len(reg.usos))
	}
	hit := reg.usos[1]
	if !hit.DesdeCache || hit.Resultado != ResultadoOK || hit.Intentos != 0 || hit.CostoUSD != 0 || hit.Proveedor != "" ||
		hit.Modelo != "" || hit.TokensEntrada != 0 || hit.TokensSalida != 0 {
		t.Errorf("cache hit row = %+v", hit)
	}
	if reg.usos[0].DesdeCache {
		t.Error("the first call is not a cache hit")
	}
	if costos.gasto != 0.01 {
		t.Errorf("spent %v, want only the first call's 0.01", costos.gasto)
	}
}

// Escenarios «Caché con presupuesto agotado» and the disabled service: ★1 is
// before «habilitado» and before the budget check (D16).
func TestServicio_CacheSeSirveConTodoAgotado(t *testing.T) {
	cache := nuevaCacheMemoria()
	llenar, _ := nuevoConCostos(&proveedorGuion{pasos: []paso{valida(1)}}, &registroMemoria{}, &costosMemoria{}, ConCache(cache))
	if _, err := llenar.Clasificar(context.Background(), res(sistema), entradaClasificar(1)); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		prov  Proveedor
		costo *costosMemoria
	}{
		{"service disabled (no IA_PROVEEDOR)", nil, &costosMemoria{}},
		{"budget spent", &proveedorGuion{}, &costosMemoria{errPresupuesto: &ErrorPresupuesto{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := nuevoConCostos(tt.prov, &registroMemoria{}, tt.costo, ConCache(cache))
			out, err := s.Clasificar(context.Background(), res(sistema), entradaClasificar(1))
			if err != nil || len(out.Resultados) != 1 {
				t.Fatalf("got %+v, %v; want the cached output", out, err)
			}
			if tt.costo.comprobaciones != 0 {
				t.Error("the budget was checked before the cache")
			}
		})
	}
}

// A corrupt or stale entry, or a cache that fails, is a miss: the call goes to
// the provider as usual.
func TestServicio_CacheInutilizableEsUnFallo(t *testing.T) {
	llaveDe := func() string { return claveCache(entradaClasificar(1).solicitud()) }
	tests := []struct {
		name   string
		cache  func() *cacheMemoria
		wantEn int // provider calls
	}{
		{"corrupt JSON", func() *cacheMemoria {
			c := nuevaCacheMemoria()
			c.entradas[llaveDe()] = []byte(`{"resultados": [`)
			return c
		}, 1},
		{"unknown fields", func() *cacheMemoria {
			c := nuevaCacheMemoria()
			c.entradas[llaveDe()] = []byte(`{"resultados":[{"id":1,"asignatura_id":3,"tema_id":null,"confianza":0}],"extra":1}`)
			return c
		}, 1},
		{"output of another input (wrong id)", func() *cacheMemoria {
			c := nuevaCacheMemoria()
			c.entradas[llaveDe()] = []byte(`{"resultados":[{"id":99,"asignatura_id":3,"tema_id":null,"confianza":0}]}`)
			return c
		}, 1},
		{"the cache fails", func() *cacheMemoria {
			c := nuevaCacheMemoria()
			c.err = errors.New("db down")
			return c
		}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov := &proveedorGuion{pasos: []paso{valida(1)}}
			s, _ := nuevoConCostos(prov, &registroMemoria{}, &costosMemoria{}, ConCache(tt.cache()))
			if out, err := s.Clasificar(context.Background(), res(sistema), entradaClasificar(1)); err != nil || len(out.Resultados) != 1 {
				t.Fatalf("got %+v, %v", out, err)
			}
			if prov.llamado != tt.wantEn {
				t.Errorf("%d provider calls, want %d", prov.llamado, tt.wantEn)
			}
		})
	}
}

// Only good outputs are cached, and the sources reach the cache.
func TestServicio_SoloSeGuardaLoBueno(t *testing.T) {
	cache := nuevaCacheMemoria()
	s, _ := nuevoConCostos(&proveedorGuion{pasos: []paso{invalida, invalida}}, &registroMemoria{}, &costosMemoria{}, ConCache(cache))
	if _, err := s.Clasificar(context.Background(), res(sistema), entradaClasificar(1)); !errors.Is(err, ErrSalidaInvalida) {
		t.Fatal(err)
	}
	if len(cache.entradas) != 0 {
		t.Errorf("%d entries cached from a failed call", len(cache.entradas))
	}
	in := entradaClasificar(1)
	in.Fuentes = []int64{7, 9}
	s, _ = nuevoConCostos(&proveedorGuion{pasos: []paso{valida(1)}}, &registroMemoria{}, &costosMemoria{}, ConCache(cache))
	if _, err := s.Clasificar(context.Background(), res(sistema), in); err != nil {
		t.Fatal(err)
	}
	for _, f := range cache.fuentes {
		if len(f) != 2 || f[0] != 7 || f[1] != 9 {
			t.Errorf("sources stored = %v, want [7 9]", f)
		}
	}
}

// BuscarEnCache is the request-time check: a hit writes its row for the actor
// and spends nothing; a miss writes nothing.
func TestBuscarEnCache(t *testing.T) {
	cache := nuevaCacheMemoria()
	llenar, _ := nuevoConCostos(&proveedorGuion{pasos: []paso{valida(1)}}, &registroMemoria{}, &costosMemoria{}, ConCache(cache))
	if _, err := llenar.Clasificar(context.Background(), res(sistema), entradaClasificar(1)); err != nil {
		t.Fatal(err)
	}
	id := int64(5)
	admin := Actor{Rol: RolAdmin, ID: &id}

	reg, costos := &registroMemoria{}, &costosMemoria{}
	s, _ := nuevoConCostos(nil, reg, costos, ConCache(cache))
	out, ok, err := BuscarEnCache[SalidaClasificar](context.Background(), s, admin, entradaClasificar(1))
	if err != nil || !ok || len(out.Resultados) != 1 {
		t.Fatalf("hit = %+v, %v, %v", out, ok, err)
	}
	u := reg.unico(t)
	if !u.DesdeCache || u.Actor.Rol != RolAdmin || u.Actor.ID == nil || *u.Actor.ID != 5 || u.Resultado != ResultadoOK {
		t.Errorf("hit row = %+v", u)
	}
	if costos.liberadas != 0 || costos.gasto != 0 || costos.comprobaciones != 0 {
		t.Errorf("a request-time hit touched the cost control: %+v", costos)
	}

	reg2 := &registroMemoria{}
	s2, _ := nuevoConCostos(nil, reg2, &costosMemoria{}, ConCache(cache))
	if _, ok, err := BuscarEnCache[SalidaClasificar](context.Background(), s2, admin, entradaClasificar(2)); ok || err != nil || len(reg2.usos) != 0 {
		t.Errorf("miss = %v, %v, %d rows; want no hit, no error, no row", ok, err, len(reg2.usos))
	}
	if _, _, err := BuscarEnCache[SalidaClasificar](context.Background(), s2, Actor{Rol: RolUsuario}, entradaClasificar(1)); !errors.Is(err, ErrActorInvalido) {
		t.Errorf("invalid actor: %v", err)
	}
	sinCache, _ := nuevoConCostos(nil, &registroMemoria{}, &costosMemoria{})
	if _, ok, err := BuscarEnCache[SalidaClasificar](context.Background(), sinCache, admin, entradaClasificar(1)); ok || err != nil {
		t.Errorf("without a cache: %v, %v", ok, err)
	}
}

// the stored JSON is the validated output, with no actor nor input.
func TestCache_NoGuardaActorNiEntrada(t *testing.T) {
	cache := nuevaCacheMemoria()
	id := int64(41)
	s, _ := nuevoConCostos(NewDoble(), &registroMemoria{}, &costosMemoria{}, ConCache(cache))
	if _, err := s.Clasificar(context.Background(), &Reserva{Actor: Actor{Rol: RolAdmin, ID: &id}, Acciones: 1}, entradaClasificar(1)); err != nil {
		t.Fatal(err)
	}
	for clave, raw := range cache.entradas {
		var campos map[string]json.RawMessage
		if err := json.Unmarshal(raw, &campos); err != nil || len(campos) != 1 {
			t.Errorf("entry %s fields = %v (%v), want only the output", clave, campos, err)
		}
		if strings.Contains(string(raw), "Enunciado de la pregunta") || strings.Contains(string(raw), "admin") {
			t.Errorf("entry holds input or actor: %s", raw)
		}
	}
}
