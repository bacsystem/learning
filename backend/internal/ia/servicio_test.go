package ia

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

// registroMemoria keeps the usage rows; err makes Registrar fail.
type registroMemoria struct {
	mu   sync.Mutex
	usos []Uso
	err  error
}

func (r *registroMemoria) Registrar(_ context.Context, u Uso) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.usos = append(r.usos, u)
	return r.err
}

func (r *registroMemoria) unico(t *testing.T) Uso {
	t.Helper()
	if len(r.usos) != 1 {
		t.Fatalf("%d usage rows, want 1", len(r.usos))
	}
	return r.usos[0]
}

// paso is one scripted answer of proveedorGuion.
type paso struct {
	json        string
	err         error
	tokens      int
	vencerAntes bool // wait for the attempt's ctx to end before answering err
}

// proveedorGuion answers its steps in order and counts the calls.
type proveedorGuion struct {
	mu      sync.Mutex
	pasos   []paso
	llamado int
}

func (p *proveedorGuion) Nombre() string { return "guion" }

func (p *proveedorGuion) CostoUSD(r Respuesta) float64 { return float64(r.TokensEntrada) / 1000 }

func (p *proveedorGuion) Completar(ctx context.Context, _ Solicitud) (Respuesta, error) {
	p.mu.Lock()
	ps := p.pasos[p.llamado]
	p.llamado++
	p.mu.Unlock()
	if ps.vencerAntes {
		<-ctx.Done()
	}
	if ps.err != nil {
		return Respuesta{}, ps.err
	}
	return Respuesta{JSON: []byte(ps.json), TokensEntrada: ps.tokens, TokensSalida: 1, Modelo: "m1"}, nil
}

var (
	transitorio = paso{err: &ErrorTransitorio{Causa: errors.New("503")}}
	invalida    = paso{json: `{"otra":1}`, tokens: 10}
)

func valida(id int64) paso {
	return paso{json: fmt.Sprintf(`{"resultados":[{"id":%d,"asignatura_id":3,"tema_id":null,"confianza":0}]}`, id), tokens: 10}
}

var sistema = Actor{Rol: RolSistema}

func entradaClasificar(n int) EntradaClasificar {
	in := EntradaClasificar{Taxonomia: []OpcionTaxonomia{{AsignaturaID: 3, Nombre: "Salud pública"}}}
	for i := 1; i <= n; i++ {
		in.Items = append(in.Items, ItemTexto{ID: int64(i), Texto: fmt.Sprintf("Enunciado de la pregunta %d sobre vacunas", i)})
	}
	return in
}

// nuevo returns a service with short times and the waits it would sleep.
func nuevo(prov Proveedor, reg Registrador, opciones ...Opcion) (*servicio, *[]time.Duration) {
	s := NewServicio(prov, reg, append([]Opcion{ConTiempoLlamada(50 * time.Millisecond)}, opciones...)...).(*servicio)
	var esperas []time.Duration
	s.dormir = func(_ context.Context, d time.Duration) bool { esperas = append(esperas, d); return true }
	return s, &esperas
}

// Escenario «Llamada exitosa»: 10 questions, a valid output and a usage row
// with tokens and cost, without the content.
func TestServicio_LlamadaExitosa(t *testing.T) {
	reg := &registroMemoria{}
	s, _ := nuevo(NewDoble(), reg)
	out, err := s.Clasificar(context.Background(), sistema, entradaClasificar(10))
	if err != nil || len(out.Resultados) != 10 {
		t.Fatalf("got %d results, err %v", len(out.Resultados), err)
	}
	u := reg.unico(t)
	if u.Resultado != ResultadoOK || u.Intentos != 1 || u.TokensEntrada <= 0 || u.TokensSalida <= 0 ||
		u.Proveedor != NombreDoble || u.Modelo != NombreDoble || u.Operacion != OpClasificar || u.CostoUSD != 0 {
		t.Errorf("usage %+v", u)
	}
}

// The flow of contract §2 and the worst case of 6 calls, with a scripted
// provider: calls made, usage attempts and result of each sequence.
func TestServicio_Reintentos(t *testing.T) {
	cases := []struct {
		name         string
		pasos        []paso
		wantErr      error
		wantLlamadas int
		wantEsperas  []time.Duration
	}{
		{"first answer valid", []paso{valida(1)}, nil, 1, nil},
		{"invalid once, then valid", []paso{invalida, valida(1)}, nil, 2, nil},
		{"invalid twice is salida_invalida", []paso{invalida, invalida}, ErrSalidaInvalida, 2, nil},
		{"two transients, then valid", []paso{transitorio, transitorio, valida(1)}, nil, 3, []time.Duration{time.Second, 2 * time.Second}},
		{"three transients is ia_error", []paso{transitorio, transitorio, transitorio}, ErrProveedor, 3, []time.Duration{time.Second, 2 * time.Second}},
		{"non-transient error is not retried", []paso{{err: errors.New("401")}}, ErrProveedor, 1, nil},
		{"wrong id is invalid output", []paso{valida(99), valida(1)}, nil, 2, nil},
		{"trailing data is invalid output", []paso{{json: `{"resultados":[{"id":1,"asignatura_id":3,"confianza":0}]} {}`}, valida(1)}, nil, 2, nil},
		{"empty results is invalid output", []paso{{json: `{"resultados":[]}`}, invalida}, ErrSalidaInvalida, 2, nil},
		{
			"worst case: 6 calls, waits restart each round",
			[]paso{transitorio, transitorio, invalida, transitorio, transitorio, invalida},
			ErrSalidaInvalida, 6, []time.Duration{time.Second, 2 * time.Second, time.Second, 2 * time.Second},
		},
		{"timeout wrapped as transient is not retried", []paso{{err: &ErrorTransitorio{Causa: errors.New("x")}, vencerAntes: true}}, ErrTimeout, 1, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prov := &proveedorGuion{pasos: c.pasos}
			reg := &registroMemoria{}
			s, esperas := nuevo(prov, reg)
			_, err := s.Clasificar(context.Background(), sistema, entradaClasificar(1))
			if !errors.Is(err, c.wantErr) || (c.wantErr == nil && err != nil) {
				t.Fatalf("err %v, want %v", err, c.wantErr)
			}
			u := reg.unico(t)
			if prov.llamado != c.wantLlamadas || u.Intentos != c.wantLlamadas {
				t.Errorf("calls %d, usage intentos %d, want %d", prov.llamado, u.Intentos, c.wantLlamadas)
			}
			if fmt.Sprint(*esperas) != fmt.Sprint(c.wantEsperas) {
				t.Errorf("waits %v, want %v", *esperas, c.wantEsperas)
			}
			if u.Resultado != resultadoDe(c.wantErr) {
				t.Errorf("resultado %q, want %q", u.Resultado, resultadoDe(c.wantErr))
			}
		})
	}
}

// Tokens and cost add up every answer received, the invalid ones too.
func TestServicio_SumaTokensDeTodosLosIntentos(t *testing.T) {
	reg := &registroMemoria{}
	s, _ := nuevo(&proveedorGuion{pasos: []paso{invalida, valida(1)}}, reg)
	if _, err := s.Clasificar(context.Background(), sistema, entradaClasificar(1)); err != nil {
		t.Fatal(err)
	}
	if u := reg.unico(t); u.TokensEntrada != 20 || u.TokensSalida != 2 || u.CostoUSD != 0.02 {
		t.Errorf("usage %+v, want 20/2 tokens and 0.02 USD", u)
	}
}

// Escenario «Salida inválida» with the double.
func TestServicio_SalidaInvalidaConElDoble(t *testing.T) {
	for _, c := range []struct {
		n       int
		wantErr error
	}{{1, nil}, {2, ErrSalidaInvalida}} {
		reg := &registroMemoria{}
		s, _ := nuevo(NewDoble(SalidaInvalida(c.n)), reg)
		_, err := s.Clasificar(context.Background(), sistema, entradaClasificar(3))
		if !errors.Is(err, c.wantErr) || (c.wantErr == nil && err != nil) {
			t.Errorf("SalidaInvalida(%d): err %v, want %v", c.n, err, c.wantErr)
		}
		if u := reg.unico(t); u.Intentos != 2 {
			t.Errorf("SalidaInvalida(%d): %d attempts, want 2 (one retry)", c.n, u.Intentos)
		}
	}
}

// Escenarios «Datos personales», «Servicio deshabilitado», «Entrada
// demasiado grande» y «Timeout»: the error, its HTTP mapping and the usage
// row; the rejections never call the provider.
func TestServicio_Rechazos(t *testing.T) {
	grande := entradaClasificar(1)
	grande.Items[0].Texto = strings.Repeat("a", MaxEntradaRunes)
	limite := entradaClasificar(1)
	limite.Items[0].Texto = strings.Repeat("á", MaxEntradaRunes-utf8.RuneCountInString("Salud pública"))
	conEmail := entradaClasificar(2)
	conEmail.Items[1].Texto = "Escribe a persona@example.com"
	conDNI := entradaClasificar(1)
	conDNI.Items[0].Texto = "El paciente 45678912 llegó"
	cases := []struct {
		name         string
		prov         Proveedor
		in           EntradaClasificar
		wantErr      error
		wantStatus   int
		wantLlamadas bool
	}{
		{"disabled", nil, entradaClasificar(1), ErrNoDisponible, http.StatusServiceUnavailable, false},
		{"over 30 000 runes", NewDoble(), grande, ErrEntradaDemasiadoGrande, http.StatusUnprocessableEntity, false},
		{"exactly 30 000 runes is accepted", NewDoble(), limite, nil, 0, true},
		{"e-mail", NewDoble(), conEmail, ErrDatosPersonales, http.StatusUnprocessableEntity, false},
		{"isolated 8 digits as DNI", NewDoble(), conDNI, ErrDatosPersonales, http.StatusUnprocessableEntity, false},
		{"provider slower than the deadline", NewDoble(Lento()), entradaClasificar(1), ErrTimeout, http.StatusGatewayTimeout, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reg := &registroMemoria{}
			s, _ := nuevo(c.prov, reg)
			_, err := s.Clasificar(context.Background(), sistema, c.in)
			if !errors.Is(err, c.wantErr) || (c.wantErr == nil && err != nil) {
				t.Fatalf("err %v, want %v", err, c.wantErr)
			}
			if c.wantErr != nil {
				if _, status := Codigo(err); status != c.wantStatus {
					t.Errorf("Codigo: status %d, want %d", status, c.wantStatus)
				}
			}
			u := reg.unico(t)
			if (u.Intentos > 0) != c.wantLlamadas || u.Resultado != resultadoDe(c.wantErr) {
				t.Errorf("usage %+v", u)
			}
			if c.prov == nil && u.Proveedor != "" {
				t.Errorf("disabled: proveedor %q, want empty", u.Proveedor)
			}
		})
	}
}

// The personal-data error names the type and the block, never the text.
func TestServicio_DatosPersonalesSinElTexto(t *testing.T) {
	in := entradaClasificar(2)
	in.Items[1].Texto = "Llamar al 987 654 321"
	s, _ := nuevo(NewDoble(), &registroMemoria{})
	_, err := s.Clasificar(context.Background(), sistema, in)
	var dp *ErrorDatosPersonales
	if !errors.As(err, &dp) || dp.Tipo != TipoTelefono || dp.Bloque != 1 {
		t.Fatalf("got %v, want telefono in block 1", err)
	}
	if strings.Contains(err.Error(), "987") {
		t.Errorf("error quotes the match: %v", err)
	}
}

// Ids travel in the labels: an 8-digit id is not taken for a DNI.
func TestServicio_IdDeOchoDigitosNoEsDNI(t *testing.T) {
	in := EntradaClasificar{
		Items:     []ItemTexto{{ID: 12345678, Texto: "Pregunta sin datos personales"}},
		Taxonomia: []OpcionTaxonomia{{AsignaturaID: 87654321, Nombre: "Salud pública"}},
	}
	s, _ := nuevo(NewDoble(), &registroMemoria{})
	out, err := s.Clasificar(context.Background(), sistema, in)
	if err != nil || out.Resultados[0].ID != 12345678 || out.Resultados[0].AsignaturaID != 87654321 {
		t.Fatalf("got %+v, %v", out, err)
	}
}

// A consumer that cancels its ctx gets ErrCancelado (wrapping context.Canceled)
// and its usage is still recorded; an expired deadline of its ctx is a timeout
// (contract §5).
func TestServicio_FinDelCtxDelConsumidor(t *testing.T) {
	vencido := func() (context.Context, context.CancelFunc) {
		return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	}
	cancelado := func() (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx, cancel
	}
	tests := []struct {
		name          string
		ctx           func() (context.Context, context.CancelFunc)
		wantErr       error
		wantCanceled  bool
		wantResultado string
	}{
		{"cancelled before the call", cancelado, ErrCancelado, true, ResultadoCancelado},
		{"deadline expired before the call", vencido, ErrTimeout, false, ResultadoTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := tt.ctx()
			defer cancel()
			reg := &registroMemoria{}
			s, _ := nuevo(NewDoble(), reg)
			_, err := s.Clasificar(ctx, sistema, entradaClasificar(1))
			if !errors.Is(err, tt.wantErr) || errors.Is(err, context.Canceled) != tt.wantCanceled {
				t.Fatalf("err %v, want %v (wraps Canceled: %v)", err, tt.wantErr, tt.wantCanceled)
			}
			if u := reg.unico(t); u.Resultado != tt.wantResultado || u.Intentos != 0 {
				t.Errorf("usage %+v", u)
			}
		})
	}
}

// proveedorQueCancela cancels the consumer's ctx from inside the call and
// fails like an adapter that lost its connection.
type proveedorQueCancela struct {
	proveedorGuion
	cancelar context.CancelFunc
}

func (p *proveedorQueCancela) Completar(context.Context, Solicitud) (Respuesta, error) {
	p.cancelar()
	return Respuesta{}, &ErrorTransitorio{Causa: errors.New("connection lost")}
}

// The consumer cancels while the provider is working: the attempt ends as
// cancelado, not as ia_error nor ia_timeout, and it is not retried.
func TestServicio_CancelaDuranteLaLlamada(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	prov := &proveedorQueCancela{cancelar: cancel}
	reg := &registroMemoria{}
	s, esperas := nuevo(prov, reg)
	_, err := s.Clasificar(ctx, sistema, entradaClasificar(1))
	if !errors.Is(err, ErrCancelado) || !errors.Is(err, context.Canceled) {
		t.Fatalf("err %v, want ErrCancelado wrapping context.Canceled", err)
	}
	if u := reg.unico(t); u.Resultado != ResultadoCancelado || u.Intentos != 1 {
		t.Errorf("usage %+v", u)
	}
	if len(*esperas) != 0 {
		t.Errorf("waited %v, want no retry", *esperas)
	}
}

func TestServicio_ActorInvalido(t *testing.T) {
	id := int64(7)
	for _, a := range []Actor{{Rol: RolAdmin}, {Rol: RolUsuario}, {Rol: RolSistema, ID: &id}, {Rol: "otro", ID: &id}, {}} {
		reg := &registroMemoria{}
		prov := &proveedorGuion{pasos: []paso{valida(1)}}
		s, _ := nuevo(prov, reg)
		if _, err := s.Clasificar(context.Background(), a, entradaClasificar(1)); !errors.Is(err, ErrActorInvalido) {
			t.Errorf("actor %+v: err %v, want ErrActorInvalido", a, err)
		}
		if prov.llamado != 0 || len(reg.usos) != 0 {
			t.Errorf("actor %+v: %d calls, %d rows; want none", a, prov.llamado, len(reg.usos))
		}
	}
	for _, a := range []Actor{{Rol: RolAdmin, ID: &id}, {Rol: RolUsuario, ID: &id}, sistema} {
		s, _ := nuevo(NewDoble(), &registroMemoria{})
		if _, err := s.Clasificar(context.Background(), a, entradaClasificar(1)); err != nil {
			t.Errorf("actor %+v: %v", a, err)
		}
	}
}

// A usage row that cannot be written never replaces the result (paso 7).
func TestServicio_FalloDelRegistroNoInvalidaLaSalida(t *testing.T) {
	s, _ := nuevo(NewDoble(), &registroMemoria{err: errors.New("db down")})
	if out, err := s.Clasificar(context.Background(), sistema, entradaClasificar(2)); err != nil || len(out.Resultados) != 2 {
		t.Fatalf("got %+v, %v", out, err)
	}
}

// Escenario «Doble de prueba»: every operation answers valid, deterministic
// output without network.
func TestDoble_CuatroOperacionesDeterministas(t *testing.T) {
	tema := int64(4)
	pasajes := []Pasaje{{ID: 11, Texto: "Pasaje uno"}, {ID: 12, Texto: "Pasaje dos"}}
	correr := func() []string {
		s, _ := nuevo(NewDoble(), &registroMemoria{})
		ctx := context.Background()
		c, err1 := s.Clasificar(ctx, sistema, EntradaClasificar{Items: []ItemTexto{{ID: 1, Texto: "x"}},
			Taxonomia: []OpcionTaxonomia{{AsignaturaID: 3, TemaID: &tema, Nombre: "Vacunas"}, {AsignaturaID: 5, Nombre: "Otra"}}})
		p, err2 := s.GenerarPreguntas(ctx, sistema, EntradaGenerarPreguntas{TemaID: 4, Cantidad: 3, Pasajes: pasajes})
		m, err3 := s.GenerarMaterial(ctx, sistema, EntradaGenerarMaterial{TemaID: 4, Pasajes: pasajes})
		e, err4 := s.GenerarExplicacion(ctx, sistema, EntradaGenerarExplicacion{PreguntaID: 9, Enunciado: "¿Cuál?",
			Alternativas: []string{"a", "b"}, Correcta: 1, Pasajes: pasajes})
		if err := errors.Join(err1, err2, err3, err4); err != nil {
			t.Fatal(err)
		}
		if *c.Resultados[0].TemaID != 4 || c.Resultados[0].AsignaturaID != 3 || len(p.Preguntas) != 3 {
			t.Errorf("clasificar %+v, preguntas %d", c, len(p.Preguntas))
		}
		j, err := json.Marshal([]any{c, p, m, e}) // JSON, not fmt: TemaID is a pointer
		if err != nil {
			t.Fatal(err)
		}
		return []string{string(j)}
	}
	if a, b := correr(), correr(); fmt.Sprint(a) != fmt.Sprint(b) {
		t.Errorf("two runs differ:\n%v\n%v", a, b)
	}
}

// The double's failure counters are safe across goroutines.
func TestDoble_TransitorioConcurrente(t *testing.T) {
	d := NewDoble(Transitorio(5))
	var wg sync.WaitGroup
	var mu sync.Mutex
	fallos := 0
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := d.Completar(context.Background(), entradaClasificar(1).solicitud()); err != nil {
				mu.Lock()
				fallos++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if fallos != 5 {
		t.Errorf("%d transient failures, want 5", fallos)
	}
}

func TestProveedorDesdeNombre(t *testing.T) {
	if p, err := ProveedorDesdeNombre(""); p != nil || err != nil {
		t.Errorf("empty: %v, %v; want disabled", p, err)
	}
	if p, err := ProveedorDesdeNombre(NombreDoble); err != nil || p.Nombre() != NombreDoble {
		t.Errorf("doble: %v, %v", p, err)
	}
	if _, err := ProveedorDesdeNombre("openai"); err == nil {
		t.Error("unknown provider accepted")
	}
}

func TestCodigo(t *testing.T) {
	cases := []struct {
		err    error
		codigo string
		status int
	}{
		{ErrNoDisponible, "ia_no_disponible", 503},
		{ErrEntradaDemasiadoGrande, "entrada_demasiado_grande", 422},
		{&ErrorDatosPersonales{Tipo: TipoDNI}, "datos_personales", 422},
		{ErrTimeout, "ia_timeout", 504},
		{ErrSalidaInvalida, "salida_invalida", 502},
		{ErrProveedor, "ia_error", 502},
		{nil, "", 0},
		{fmt.Errorf("%w: %w", ErrCancelado, context.Canceled), "", 0},
	}
	for _, c := range cases {
		if codigo, status := Codigo(c.err); codigo != c.codigo || status != c.status {
			t.Errorf("Codigo(%v) = %q %d", c.err, codigo, status)
		}
	}
	if codigo, status := Codigo(errors.New("otro")); codigo != "" || status != 0 {
		t.Error("an unrelated error mapped")
	}
}
