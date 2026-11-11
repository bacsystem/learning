package fuentestest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cbaciliod/learning/internal/fuentes"
)

// Entorno is what the contract needs from an adapter (ADR 0003): the port,
// two existing especialidad ids (Postgres needs them for the FKs; memory does
// not check) and two hooks for the admin life cycle that is not in the port,
// as S5 and S2 will do in E2-06: retire, and add the next version in process.
type Entorno struct {
	Repo          fuentes.SourceRepository
	EspecialidadA int64
	EspecialidadB int64
	Retirar       func(t *testing.T, fuenteID int64)
	NuevaVersion  func(t *testing.T, fuenteID int64, contenido []byte) (version int)
}

var seq atomic.Int64

// marcador returns a word unique to this run. It ends in a digit so the
// Spanish stemmer leaves it intact, and the search is global, so every test
// searches only its own marker.
func marcador() string {
	return fmt.Sprintf("mk%x%x9", time.Now().UnixNano()%0xfffffff, seq.Add(1))
}

// RunContract runs the SourceRepository contract (SIA-E2-02) on the adapter
// built by nuevo. Every adapter must pass the same cases.
func RunContract(t *testing.T, nuevo func(t *testing.T) Entorno) {
	t.Run("BuscarPasajesPorRelevancia", func(t *testing.T) { buscarPorRelevancia(t, nuevo(t)) })
	t.Run("LimiteDeResultados", func(t *testing.T) { limiteDeResultados(t, nuevo(t)) })
	t.Run("FiltroPorEspecialidad", func(t *testing.T) { filtroPorEspecialidad(t, nuevo(t)) })
	t.Run("SoloFuentesActivasYVersionVigente", func(t *testing.T) { soloActivasYVigente(t, nuevo(t)) })
	t.Run("AreaYTemaNoFiltran", func(t *testing.T) { areaYTemaNoFiltran(t, nuevo(t)) })
	t.Run("ConsultaFueraDeLimite", func(t *testing.T) { consultaFueraDeLimite(t, nuevo(t)) })
	t.Run("ResolverCitaHistorica", func(t *testing.T) { resolverCitaHistorica(t, nuevo(t)) })
	t.Run("CitaQueNoCoincide", func(t *testing.T) { citaQueNoCoincide(t, nuevo(t)) })
	t.Run("PublicacionAtomica", func(t *testing.T) { publicacionAtomica(t, nuevo(t)) })
	t.Run("VersionFallida", func(t *testing.T) { versionFallida(t, nuevo(t)) })
	t.Run("PublicarEnRetiradaNoLaReactiva", func(t *testing.T) { publicarEnRetirada(t, nuevo(t)) })
	t.Run("ObtenerPasajesPaginado", func(t *testing.T) { obtenerPasajes(t, nuevo(t)) })
	t.Run("HashDuplicado", func(t *testing.T) { hashDuplicado(t, nuevo(t)) })
	t.Run("ListarFuentes", func(t *testing.T) { listarFuentes(t, nuevo(t)) })
}

// ── Seeding through the port ──────────────────────────────────────────────────

type sembrada struct {
	fuenteID int64
	version  int
	pasajes  []int64 // ids by orden
}

func guardar(t *testing.T, e Entorno, especialidades []int64) (int64, int) {
	t.Helper()
	contenido := []byte("# Fuente de contrato " + marcador() + "\nTexto.\n")
	id, v, err := e.Repo.GuardarFuente(context.Background(), fuentes.NuevaFuente{
		Titulo: "Fuente contrato " + marcador(), Tipo: "guia", EspecialidadIDs: especialidades,
		NombreArchivo: "fuente.md", Contenido: contenido}, nil)
	if err != nil {
		t.Fatalf("GuardarFuente: %v", err)
	}
	return id, v
}

func nuevos(textos ...string) []fuentes.PasajeNuevo {
	out := make([]fuentes.PasajeNuevo, len(textos))
	for i, tx := range textos {
		sec := fmt.Sprintf("Sección %d", i+1)
		out[i] = fuentes.PasajeNuevo{Orden: i + 1, LineaInicio: i*10 + 1, LineaFin: i*10 + 9, Seccion: &sec,
			Texto: tx, Palabras: len(strings.Fields(tx))}
	}
	return out
}

func (s sembrada) publicar(t *testing.T, e Entorno, textos ...string) sembrada {
	t.Helper()
	ctx := context.Background()
	if err := e.Repo.PublicarVersion(ctx, s.fuenteID, s.version, nuevos(textos...), nil); err != nil {
		t.Fatalf("PublicarVersion(%d, v%d): %v", s.fuenteID, s.version, err)
	}
	pag, err := e.Repo.ObtenerPasajes(ctx, s.fuenteID, s.version, 1, 100)
	if err != nil {
		t.Fatalf("ObtenerPasajes: %v", err)
	}
	s.pasajes = nil
	for _, p := range pag.Data {
		s.pasajes = append(s.pasajes, p.PasajeID)
	}
	return s
}

// activa creates and publishes a fuente with the given passages.
func activa(t *testing.T, e Entorno, especialidades []int64, textos ...string) sembrada {
	t.Helper()
	id, v := guardar(t, e, especialidades)
	return sembrada{fuenteID: id, version: v}.publicar(t, e, textos...)
}

func buscar(t *testing.T, e Entorno, b fuentes.Busqueda) []fuentes.PasajeEncontrado {
	t.Helper()
	r, err := e.Repo.BuscarPasajes(context.Background(), b)
	if err != nil {
		t.Fatalf("BuscarPasajes: %v", err)
	}
	return r
}

func ids(ps []fuentes.PasajeEncontrado) []int64 {
	out := make([]int64, len(ps))
	for i, p := range ps {
		out[i] = p.PasajeID
	}
	return out
}

// ── Cases ─────────────────────────────────────────────────────────────────────

// Escenario «Buscar pasajes»: hits carry fuente, version, passage and text,
// by relevance. The denser passage is published last, so a tie-break by id
// alone would put it second.
func buscarPorRelevancia(t *testing.T, e Entorno) {
	mk := marcador()
	a := activa(t, e, nil, mk+" vigilancia de brotes en la comunidad")
	b := activa(t, e, []int64{e.EspecialidadA}, "notificación "+mk+" y "+mk+" con "+mk+" diaria")
	got := buscar(t, e, fuentes.Busqueda{Consulta: mk, EspecialidadID: e.EspecialidadA})
	if fmt.Sprint(ids(got)) != fmt.Sprint([]int64{b.pasajes[0], a.pasajes[0]}) {
		t.Fatalf("order = %v, want [%d %d] (denser first)", ids(got), b.pasajes[0], a.pasajes[0])
	}
	h := got[0]
	if h.FuenteID != b.fuenteID || h.Version != 1 || h.Orden != 1 || h.LineaInicio != 1 || h.LineaFin != 9 ||
		h.Seccion == nil || *h.Seccion != "Sección 1" || !strings.Contains(h.Texto, "notificación") {
		t.Errorf("hit = %+v", h)
	}
}

// Regla 2: at most 20, and the requested limit (0 = 20, > 20 capped). Equal
// passages tie on rank, so the order is by passage id.
func limiteDeResultados(t *testing.T, e Entorno) {
	mk := marcador()
	textos := make([]string, 25)
	for i := range textos {
		textos[i] = fmt.Sprintf("pasaje %s número", mk)
	}
	s := activa(t, e, nil, textos...)
	for _, c := range []struct{ limite, want int }{{0, 20}, {5, 5}, {20, 20}, {25, 20}, {-1, 20}} {
		got := buscar(t, e, fuentes.Busqueda{Consulta: mk, EspecialidadID: e.EspecialidadA, Limite: c.limite})
		if len(got) != c.want || fmt.Sprint(ids(got)) != fmt.Sprint(s.pasajes[:c.want]) {
			t.Errorf("limite %d: got %d hits %v, want the first %d by id", c.limite, len(got), ids(got), c.want)
		}
	}
}

// Regla 2: only the requested profession; a fuente without professions
// applies to all (E2-03 regla 2).
func filtroPorEspecialidad(t *testing.T, e Entorno) {
	mk := marcador()
	deA := activa(t, e, []int64{e.EspecialidadA}, mk+" de A")
	deB := activa(t, e, []int64{e.EspecialidadB}, mk+" de B")
	todas := activa(t, e, nil, mk+" de todas")
	ambas := activa(t, e, []int64{e.EspecialidadA, e.EspecialidadB}, mk+" de ambas")
	for _, c := range []struct {
		esp  int64
		want []int64
	}{
		{e.EspecialidadA, []int64{deA.pasajes[0], todas.pasajes[0], ambas.pasajes[0]}},
		{e.EspecialidadB, []int64{deB.pasajes[0], todas.pasajes[0], ambas.pasajes[0]}},
	} {
		if got := ids(buscar(t, e, fuentes.Busqueda{Consulta: mk, EspecialidadID: c.esp})); fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("especialidad %d: got %v, want %v", c.esp, got, c.want)
		}
	}
}

// Escenario «Fuente inactiva excluida» and regla 2: 'procesando', 'fallida'
// and 'retirada' fuentes and non-current versions never appear.
func soloActivasYVigente(t *testing.T, e Entorno) {
	ctx := context.Background()
	mk := marcador()
	esp := []int64{e.EspecialidadA}

	guardar(t, e, esp) // procesando: nothing published
	fid, fv := guardar(t, e, esp)
	if err := e.Repo.MarcarVersionFallida(ctx, fid, fv, fuentes.MotivoSinContenido, nil); err != nil {
		t.Fatal(err)
	}
	retirada := activa(t, e, esp, mk+" retirada")
	e.Retirar(t, retirada.fuenteID)
	vieja := activa(t, e, esp, mk+" versión vieja")
	v2 := e.NuevaVersion(t, vieja.fuenteID, []byte("v2 "+marcador()))
	nueva := sembrada{fuenteID: vieja.fuenteID, version: v2}.publicar(t, e, mk+" versión nueva")
	viva := activa(t, e, esp, mk+" viva")

	got := ids(buscar(t, e, fuentes.Busqueda{Consulta: mk, EspecialidadID: esp[0]}))
	want := []int64{nueva.pasajes[0], viva.pasajes[0]}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("got %v, want only the current version of active fuentes %v", got, want)
	}
}

// Escenario «Área y tema no filtran en el MVP» (regla 6): same passages in the
// same order with and without área and tema.
func areaYTemaNoFiltran(t *testing.T, e Entorno) {
	mk := marcador()
	activa(t, e, []int64{e.EspecialidadA}, mk+" uno", mk+" "+mk+" dos")
	activa(t, e, nil, mk+" tres")
	area, tema := int64(999999), int64(888888)
	base := buscar(t, e, fuentes.Busqueda{Consulta: mk, EspecialidadID: e.EspecialidadA})
	for _, b := range []fuentes.Busqueda{
		{Consulta: mk, EspecialidadID: e.EspecialidadA, AsignaturaID: &area},
		{Consulta: mk, EspecialidadID: e.EspecialidadA, TemaID: &tema},
		{Consulta: mk, EspecialidadID: e.EspecialidadA, AsignaturaID: &area, TemaID: &tema},
	} {
		if got := buscar(t, e, b); fmt.Sprint(ids(got)) != fmt.Sprint(ids(base)) || len(base) != 3 {
			t.Errorf("with área/tema got %v, want %v (3 hits)", ids(got), ids(base))
		}
	}
}

// Escenario «Consulta fuera de límite» (regla 3): 3–300 runes after trimming,
// and a profession is required.
func consultaFueraDeLimite(t *testing.T, e Entorno) {
	esp := e.EspecialidadA
	cases := []struct {
		name string
		b    fuentes.Busqueda
		ok   bool
	}{
		{"2 caracteres", fuentes.Busqueda{Consulta: "ab", EspecialidadID: esp}, false},
		{"3 caracteres", fuentes.Busqueda{Consulta: "abc", EspecialidadID: esp}, true},
		{"3 runes con ñ", fuentes.Busqueda{Consulta: "año", EspecialidadID: esp}, true},
		{"espacios no cuentan", fuentes.Busqueda{Consulta: "   ab   ", EspecialidadID: esp}, false},
		{"300 caracteres", fuentes.Busqueda{Consulta: strings.Repeat("a", 300), EspecialidadID: esp}, true},
		{"301 caracteres", fuentes.Busqueda{Consulta: strings.Repeat("a", 301), EspecialidadID: esp}, false},
		{"300 runes de 2 bytes", fuentes.Busqueda{Consulta: strings.Repeat("ñ", 300), EspecialidadID: esp}, true},
		{"sin especialidad", fuentes.Busqueda{Consulta: "vigilancia"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := e.Repo.BuscarPasajes(context.Background(), c.b)
			if c.ok && err != nil {
				t.Errorf("err = %v, want nil", err)
			}
			if !c.ok && !errors.Is(err, fuentes.ErrConsultaInvalida) {
				t.Errorf("err = %v, want ErrConsultaInvalida", err)
			}
		})
	}
}

// Escenario «Resolver cita histórica» (regla 1): a citation to version 1
// returns its exact text after versions 2 and 3, and also once the fuente is
// retired; the consumer decides with EstadoFuente.
func resolverCitaHistorica(t *testing.T, e Entorno) {
	ctx := context.Background()
	textoV1 := "Texto exacto de la versión 1 " + marcador()
	v1 := activa(t, e, nil, textoV1)
	for range 2 {
		n := e.NuevaVersion(t, v1.fuenteID, []byte("otra versión "+marcador()))
		sembrada{fuenteID: v1.fuenteID, version: n}.publicar(t, e, "texto de la versión "+fmt.Sprint(n))
	}
	cita := fuentes.Cita{FuenteID: v1.fuenteID, Version: 1, PasajeID: v1.pasajes[0]}
	for _, retirar := range []bool{false, true} {
		if retirar {
			e.Retirar(t, v1.fuenteID)
		}
		got, err := e.Repo.ResolverCita(ctx, cita)
		wantEstado := fuentes.EstadoActiva
		if retirar {
			wantEstado = fuentes.EstadoRetirada
		}
		if err != nil || got.Texto != textoV1 || got.Version != 1 || got.FuenteID != v1.fuenteID ||
			got.EstadoFuente != wantEstado || got.TituloFuente == "" {
			t.Errorf("retirada=%v: got %+v, %v", retirar, got, err)
		}
	}
}

// A citation whose (fuente, versión, pasaje) do not match is cita_invalida.
func citaQueNoCoincide(t *testing.T, e Entorno) {
	ctx := context.Background()
	a := activa(t, e, nil, "pasaje de A "+marcador())
	b := activa(t, e, nil, "pasaje de B "+marcador())
	for _, c := range []fuentes.Cita{
		{FuenteID: a.fuenteID, Version: 2, PasajeID: a.pasajes[0]},
		{FuenteID: b.fuenteID, Version: 1, PasajeID: a.pasajes[0]},
		{FuenteID: a.fuenteID, Version: 1, PasajeID: 999999999},
		{FuenteID: 999999999, Version: 1, PasajeID: a.pasajes[0]},
	} {
		if _, err := e.Repo.ResolverCita(ctx, c); !errors.Is(err, fuentes.ErrCitaInvalida) {
			t.Errorf("%+v: err = %v, want ErrCitaInvalida", c, err)
		}
	}
}

// An invalid passage list publishes nothing: the version stays 'procesando'
// and can still be published; a version not in process cannot be published.
func publicacionAtomica(t *testing.T, e Entorno) {
	ctx := context.Background()
	mk := marcador()
	id, v := guardar(t, e, nil)
	malos := map[string][]fuentes.PasajeNuevo{
		"sin pasajes":     nil,
		"orden repetido":  {nuevos(mk + " a")[0], nuevos(mk + " b")[0]},
		"palabras 0":      {{Orden: 1, LineaInicio: 1, LineaFin: 1, Texto: mk, Palabras: 0}},
		"líneas al revés": {{Orden: 1, LineaInicio: 5, LineaFin: 4, Texto: mk, Palabras: 1}},
		"texto vacío":     {{Orden: 1, LineaInicio: 1, LineaFin: 1, Texto: "  ", Palabras: 1}},
	}
	for name, ps := range malos {
		if err := e.Repo.PublicarVersion(ctx, id, v, ps, nil); !errors.Is(err, fuentes.ErrPublicacionInvalida) {
			t.Errorf("%s: err = %v, want ErrPublicacionInvalida", name, err)
		}
	}
	if got := buscar(t, e, fuentes.Busqueda{Consulta: mk, EspecialidadID: e.EspecialidadA}); len(got) != 0 {
		t.Fatalf("a rejected publication left %d searchable passages", len(got))
	}
	s := sembrada{fuenteID: id, version: v}.publicar(t, e, mk+" válido")
	if got := buscar(t, e, fuentes.Busqueda{Consulta: mk, EspecialidadID: e.EspecialidadA}); fmt.Sprint(ids(got)) != fmt.Sprint(s.pasajes) {
		t.Errorf("after a valid publication got %v, want %v", ids(got), s.pasajes)
	}
	if err := e.Repo.PublicarVersion(ctx, id, v, nuevos(mk+" otra vez"), nil); !errors.Is(err, fuentes.ErrNoEncontrado) {
		t.Errorf("publishing a 'lista' version again: err = %v, want ErrNoEncontrado", err)
	}
}

// A failed new version leaves the fuente active with its current version.
func versionFallida(t *testing.T, e Entorno) {
	ctx := context.Background()
	mk := marcador()
	s := activa(t, e, nil, mk+" vigente")
	n := e.NuevaVersion(t, s.fuenteID, []byte("nueva "+marcador()))
	if err := e.Repo.MarcarVersionFallida(ctx, s.fuenteID, n, "otro_motivo", nil); !errors.Is(err, fuentes.ErrPublicacionInvalida) {
		t.Errorf("unknown motivo: err = %v", err)
	}
	if err := e.Repo.MarcarVersionFallida(ctx, s.fuenteID, n, fuentes.MotivoLimitePasajes, nil); err != nil {
		t.Fatal(err)
	}
	if got := buscar(t, e, fuentes.Busqueda{Consulta: mk, EspecialidadID: e.EspecialidadA}); fmt.Sprint(ids(got)) != fmt.Sprint(s.pasajes) {
		t.Errorf("after a failed new version got %v, want the current %v", ids(got), s.pasajes)
	}
	if err := e.Repo.MarcarVersionFallida(ctx, s.fuenteID, n, fuentes.MotivoErrorInterno, nil); !errors.Is(err, fuentes.ErrNoEncontrado) {
		t.Errorf("failing a failed version again: err = %v, want ErrNoEncontrado", err)
	}
}

func obtenerPasajes(t *testing.T, e Entorno) {
	ctx := context.Background()
	textos := make([]string, 5)
	for i := range textos {
		textos[i] = fmt.Sprintf("pasaje %d %s", i+1, marcador())
	}
	s := activa(t, e, nil, textos...)
	pag, err := e.Repo.ObtenerPasajes(ctx, s.fuenteID, s.version, 2, 2)
	if err != nil || pag.Total != 5 || pag.Page != 2 || pag.PerPage != 2 || len(pag.Data) != 2 ||
		pag.Data[0].Orden != 3 || pag.Data[1].Texto != textos[3] {
		t.Errorf("page 2 = %+v, %v", pag, err)
	}
	if pag, _ := e.Repo.ObtenerPasajes(ctx, s.fuenteID, s.version, 9, 2); len(pag.Data) != 0 || pag.Total != 5 {
		t.Errorf("page past the end = %+v", pag)
	}
	if _, err := e.Repo.ObtenerPasajes(ctx, s.fuenteID, 7, 1, 20); !errors.Is(err, fuentes.ErrNoEncontrado) {
		t.Errorf("unknown version: err = %v", err)
	}
}

// E2-03 regla 3: the same content can never be stored twice, in any fuente.
func hashDuplicado(t *testing.T, e Entorno) {
	ctx := context.Background()
	contenido := []byte("contenido único " + marcador())
	nf := fuentes.NuevaFuente{Titulo: "Duplicada " + marcador(), Tipo: "norma", NombreArchivo: "a.md", Contenido: contenido}
	if _, _, err := e.Repo.GuardarFuente(ctx, nf, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.Repo.GuardarFuente(ctx, nf, nil); !errors.Is(err, fuentes.ErrFuenteDuplicada) {
		t.Errorf("same content again: err = %v, want ErrFuenteDuplicada", err)
	}
}

// Publishing a new version of a retired fuente keeps it retired: only S5
// restores it (contract E2 § Estados). Its passages stay out of the search,
// but the new version's citations resolve.
func publicarEnRetirada(t *testing.T, e Entorno) {
	ctx := context.Background()
	mk := marcador()
	s := activa(t, e, nil, mk+" antes de retirar")
	e.Retirar(t, s.fuenteID)
	n := e.NuevaVersion(t, s.fuenteID, []byte("tras retirar "+marcador()))
	nueva := sembrada{fuenteID: s.fuenteID, version: n}.publicar(t, e, mk+" después de retirar")
	if got := buscar(t, e, fuentes.Busqueda{Consulta: mk, EspecialidadID: e.EspecialidadA}); len(got) != 0 {
		t.Errorf("a retired fuente is searchable after publishing: %v", ids(got))
	}
	c, err := e.Repo.ResolverCita(ctx, fuentes.Cita{FuenteID: s.fuenteID, Version: n, PasajeID: nueva.pasajes[0]})
	if err != nil || c.EstadoFuente != fuentes.EstadoRetirada {
		t.Errorf("cita of the new version = %+v, %v; want estado retirada", c, err)
	}
}

// ListarFuentes: newest change first (UpdatedAt DESC, ID DESC), filters with
// zero = no filter, ids of especialidades ascending and the current version
// number, and pagination normalised without error.
func listarFuentes(t *testing.T, e Entorno) {
	ctx := context.Background()
	anio := 2025
	guardarCon := func(tipo string, esp []int64) int64 {
		t.Helper()
		id, _, err := e.Repo.GuardarFuente(ctx, fuentes.NuevaFuente{Titulo: "Listado " + marcador(), Tipo: tipo, Anio: &anio,
			EspecialidadIDs: esp, NombreArchivo: "l.md", Contenido: []byte("listado " + marcador())}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	deB := guardarCon("norma", []int64{e.EspecialidadB})
	ambas := guardarCon("guia", []int64{e.EspecialidadB, e.EspecialidadA})
	todas := guardarCon("balotario", nil)
	// Publishing the first one makes it the most recently updated.
	if err := e.Repo.PublicarVersion(ctx, deB, 1, nuevos("publicada "+marcador()), nil); err != nil {
		t.Fatal(err)
	}

	listar := func(f fuentes.FiltroFuentes) fuentes.Pagina[fuentes.FuenteResumen] {
		t.Helper()
		p, err := e.Repo.ListarFuentes(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	idsDe := func(p fuentes.Pagina[fuentes.FuenteResumen]) []int64 {
		var out []int64
		for _, r := range p.Data {
			out = append(out, r.ID)
		}
		return out
	}
	cases := []struct {
		name string
		f    fuentes.FiltroFuentes
		want []int64
	}{
		{"sin filtro, la más reciente primero", fuentes.FiltroFuentes{EspecialidadID: e.EspecialidadB}, []int64{deB, todas, ambas}},
		{"especialidad A: las suyas y las de todas", fuentes.FiltroFuentes{EspecialidadID: e.EspecialidadA}, []int64{todas, ambas}},
		{"estado activa", fuentes.FiltroFuentes{EspecialidadID: e.EspecialidadB, Estado: fuentes.EstadoActiva}, []int64{deB}},
		{"tipo guia", fuentes.FiltroFuentes{EspecialidadID: e.EspecialidadB, Tipo: "guia"}, []int64{ambas}},
		{"página 2 de 2 en 2", fuentes.FiltroFuentes{EspecialidadID: e.EspecialidadB, Page: 2, PerPage: 2}, []int64{ambas}},
	}
	for _, c := range cases {
		if got := idsDe(listar(c.f)); fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}

	p := listar(fuentes.FiltroFuentes{EspecialidadID: e.EspecialidadB, Page: 0, PerPage: 500})
	if p.Page != 1 || p.PerPage != 100 || p.Total != 3 {
		t.Errorf("normalised page = %d/%d total %d, want 1/100 total 3", p.Page, p.PerPage, p.Total)
	}
	for _, r := range p.Data {
		switch r.ID {
		case deB:
			if r.Estado != fuentes.EstadoActiva || r.VersionVigente == nil || *r.VersionVigente != 1 || fmt.Sprint(r.EspecialidadIDs) != fmt.Sprint([]int64{e.EspecialidadB}) {
				t.Errorf("published = %+v", r)
			}
		case ambas:
			want := []int64{min(e.EspecialidadA, e.EspecialidadB), max(e.EspecialidadA, e.EspecialidadB)}
			if r.VersionVigente != nil || r.Estado != fuentes.EstadoProcesando || fmt.Sprint(r.EspecialidadIDs) != fmt.Sprint(want) ||
				r.Tipo != "guia" || r.Anio == nil || *r.Anio != 2025 || r.Titulo == "" || r.CreatedAt.IsZero() {
				t.Errorf("in process = %+v", r)
			}
		case todas:
			if len(r.EspecialidadIDs) != 0 || r.EspecialidadIDs == nil {
				t.Errorf("for all = %+v, want an empty (non-nil) list", r.EspecialidadIDs)
			}
		}
	}
}
