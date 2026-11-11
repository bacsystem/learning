// Package fuentestest holds the contract suite of the SourceRepository port
// and an in-memory adapter that passes it (SIA-E2-02 «Intercambiabilidad»).
// It is test support: nothing in production imports it.
package fuentestest

import (
	"context"
	"crypto/sha256"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"

	"github.com/cbaciliod/learning/internal/fuentes"
	"github.com/cbaciliod/learning/internal/texto"
)

// Memoria is an in-memory SourceRepository. Its search has no stemming: a
// passage matches when it contains every folded word of the query, and the
// rank is how many times those words appear. The contract suite only relies
// on orders that both adapters agree on.
type Memoria struct {
	mu        sync.Mutex
	sigFuente int64
	sigPasaje int64
	fuentes   map[int64]*fuenteMem
	hashes    map[[sha256.Size]byte]bool
	ultimo    time.Time
}

type fuenteMem struct {
	id             int64
	titulo         string
	tipo           string
	anio           *int
	procesoID      *int64
	estado         string
	creada         time.Time
	actualizada    time.Time
	especialidades map[int64]bool
	vigente        int // version number; 0 = none
	versiones      map[int]*versionMem
}

type versionMem struct {
	numero  int
	estado  string
	pasajes []fuentes.Pasaje
}

// NuevaMemoria returns an empty in-memory adapter.
func NuevaMemoria() *Memoria {
	return &Memoria{fuentes: map[int64]*fuenteMem{}, hashes: map[[sha256.Size]byte]bool{}}
}

var _ fuentes.SourceRepository = (*Memoria)(nil)

// GuardarFuente never calls aud: the in-memory adapter has no audit log.
func (m *Memoria) GuardarFuente(_ context.Context, f fuentes.NuevaFuente, _ fuentes.Auditor) (int64, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.registrarHash(f.Contenido); err != nil {
		return 0, 0, err
	}
	m.sigFuente++
	ahora := m.ahora()
	fm := &fuenteMem{id: m.sigFuente, titulo: f.Titulo, tipo: f.Tipo, anio: f.Anio, procesoID: f.ProcesoID,
		estado: fuentes.EstadoProcesando, creada: ahora, actualizada: ahora,
		especialidades: map[int64]bool{}, versiones: map[int]*versionMem{1: {numero: 1, estado: fuentes.EstadoProcesando}}}
	for _, e := range f.EspecialidadIDs {
		fm.especialidades[e] = true
	}
	m.fuentes[fm.id] = fm
	return fm.id, 1, nil
}

func (m *Memoria) ListarFuentes(_ context.Context, fl fuentes.FiltroFuentes) (fuentes.Pagina[fuentes.FuenteResumen], error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	page, perPage := fuentes.NormalizarPagina(fl.Page, fl.PerPage)
	var todas []fuentes.FuenteResumen
	for _, f := range m.fuentes {
		if (fl.EspecialidadID > 0 && !f.aplicaA(fl.EspecialidadID)) || (fl.Estado != "" && f.estado != fl.Estado) ||
			(fl.Tipo != "" && f.tipo != fl.Tipo) {
			continue
		}
		todas = append(todas, f.resumen())
	}
	sort.Slice(todas, func(i, j int) bool {
		if !todas[i].UpdatedAt.Equal(todas[j].UpdatedAt) {
			return todas[i].UpdatedAt.After(todas[j].UpdatedAt)
		}
		return todas[i].ID > todas[j].ID
	})
	res := fuentes.Pagina[fuentes.FuenteResumen]{Page: page, PerPage: perPage, Total: len(todas), Data: []fuentes.FuenteResumen{}}
	desde := min((page-1)*perPage, len(todas))
	res.Data = append(res.Data, todas[desde:min(desde+perPage, len(todas))]...)
	return res, nil
}

func (m *Memoria) ObtenerPasajes(_ context.Context, fuenteID int64, version int, page, perPage int) (fuentes.Pagina[fuentes.Pasaje], error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	page, perPage = fuentes.NormalizarPagina(page, perPage)
	v, err := m.version(fuenteID, version)
	if err != nil {
		return fuentes.Pagina[fuentes.Pasaje]{}, err
	}
	res := fuentes.Pagina[fuentes.Pasaje]{Page: page, PerPage: perPage, Total: len(v.pasajes), Data: []fuentes.Pasaje{}}
	desde := min((page-1)*perPage, len(v.pasajes))
	res.Data = append(res.Data, v.pasajes[desde:min(desde+perPage, len(v.pasajes))]...)
	return res, nil
}

func (m *Memoria) BuscarPasajes(_ context.Context, b fuentes.Busqueda) ([]fuentes.PasajeEncontrado, error) {
	consulta, limite, err := fuentes.NormalizarBusqueda(b)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	terminos := palabras(texto.Plegar(consulta))
	type hit struct {
		p    fuentes.Pasaje
		rank int
	}
	var hits []hit
	for _, f := range m.fuentes {
		if f.estado != fuentes.EstadoActiva || !f.aplicaA(b.EspecialidadID) {
			continue
		}
		for _, p := range f.versiones[f.vigente].pasajes {
			if r := rank(terminos, palabras(texto.Plegar(p.Texto))); r > 0 {
				hits = append(hits, hit{p, r})
			}
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].rank != hits[j].rank {
			return hits[i].rank > hits[j].rank
		}
		return hits[i].p.PasajeID < hits[j].p.PasajeID
	})
	out := make([]fuentes.PasajeEncontrado, 0, min(len(hits), limite))
	for _, h := range hits[:min(len(hits), limite)] {
		out = append(out, h.p)
	}
	return out, nil
}

func (m *Memoria) ResolverCita(_ context.Context, c fuentes.Cita) (fuentes.PasajeCitado, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.fuentes[c.FuenteID]
	if !ok {
		return fuentes.PasajeCitado{}, fuentes.ErrCitaInvalida
	}
	v, ok := f.versiones[c.Version]
	if !ok {
		return fuentes.PasajeCitado{}, fuentes.ErrCitaInvalida
	}
	for _, p := range v.pasajes {
		if p.PasajeID == c.PasajeID {
			return fuentes.PasajeCitado{Pasaje: p, TituloFuente: f.titulo, EstadoFuente: f.estado}, nil
		}
	}
	return fuentes.PasajeCitado{}, fuentes.ErrCitaInvalida
}

func (m *Memoria) PublicarVersion(_ context.Context, fuenteID int64, version int, pasajes []fuentes.PasajeNuevo, _ fuentes.Auditor) error {
	if err := fuentes.ValidarPasajes(pasajes); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	v, err := m.enProceso(fuenteID, version)
	if err != nil {
		return err
	}
	ordenados := append([]fuentes.PasajeNuevo(nil), pasajes...)
	sort.Slice(ordenados, func(i, j int) bool { return ordenados[i].Orden < ordenados[j].Orden })
	for _, p := range ordenados {
		m.sigPasaje++
		v.pasajes = append(v.pasajes, fuentes.Pasaje{FuenteID: fuenteID, Version: version, PasajeID: m.sigPasaje,
			Orden: p.Orden, LineaInicio: p.LineaInicio, LineaFin: p.LineaFin, Seccion: p.Seccion, Texto: p.Texto})
	}
	v.estado = fuentes.VersionLista
	f := m.fuentes[fuenteID]
	f.vigente = version
	f.actualizada = m.ahora()
	if f.estado != fuentes.EstadoRetirada {
		f.estado = fuentes.EstadoActiva
	}
	return nil
}

func (m *Memoria) MarcarVersionFallida(_ context.Context, fuenteID int64, version int, motivo string, _ fuentes.Auditor) error {
	if !fuentes.MotivoValido(motivo) {
		return fuentes.ErrPublicacionInvalida
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	v, err := m.enProceso(fuenteID, version)
	if err != nil {
		return err
	}
	v.estado = fuentes.EstadoFallida
	f := m.fuentes[fuenteID]
	f.actualizada = m.ahora()
	if f.vigente == 0 {
		f.estado = fuentes.EstadoFallida
	}
	return nil
}

// ── Test hooks (outside the port) ─────────────────────────────────────────────

// Retirar retires an active fuente, as S5 will (E2-06); the life cycle is
// not in the port. Contract hook (Entorno.Retirar).
func (m *Memoria) Retirar(t *testing.T, fuenteID int64) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.fuentes[fuenteID]
	if !ok || f.vigente == 0 {
		t.Fatalf("Retirar: fuente %d has no current version", fuenteID)
	}
	f.estado = fuentes.EstadoRetirada
	f.actualizada = m.ahora()
}

// NuevaVersion adds version n+1 in 'procesando', as S2 will (E2-06); the
// contract then publishes it through the port. Contract hook.
func (m *Memoria) NuevaVersion(t *testing.T, fuenteID int64, contenido []byte) int {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.fuentes[fuenteID]
	if !ok {
		t.Fatalf("NuevaVersion: fuente %d does not exist", fuenteID)
	}
	if err := m.registrarHash(contenido); err != nil {
		t.Fatalf("NuevaVersion: %v", err)
	}
	n := len(f.versiones) + 1
	f.versiones[n] = &versionMem{numero: n, estado: fuentes.EstadoProcesando}
	return n
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// ahora returns a strictly increasing time, so UpdatedAt orders mutations
// even when the clock does not advance between them.
func (m *Memoria) ahora() time.Time {
	t := time.Now().UTC()
	if !t.After(m.ultimo) {
		t = m.ultimo.Add(time.Microsecond)
	}
	m.ultimo = t
	return t
}

func (f *fuenteMem) resumen() fuentes.FuenteResumen {
	r := fuentes.FuenteResumen{ID: f.id, Titulo: f.titulo, Tipo: f.tipo, Estado: f.estado, Anio: f.anio,
		ProcesoID: f.procesoID, EspecialidadIDs: []int64{}, CreatedAt: f.creada, UpdatedAt: f.actualizada}
	for e := range f.especialidades {
		r.EspecialidadIDs = append(r.EspecialidadIDs, e)
	}
	sort.Slice(r.EspecialidadIDs, func(i, j int) bool { return r.EspecialidadIDs[i] < r.EspecialidadIDs[j] })
	if f.vigente > 0 {
		v := f.vigente
		r.VersionVigente = &v
	}
	return r
}

// aplicaA: a fuente without professions applies to every one (E2-03 regla 2).
func (f *fuenteMem) aplicaA(especialidadID int64) bool {
	return len(f.especialidades) == 0 || f.especialidades[especialidadID]
}

func (m *Memoria) registrarHash(contenido []byte) error {
	h := sha256.Sum256(contenido)
	if m.hashes[h] {
		return fuentes.ErrFuenteDuplicada
	}
	m.hashes[h] = true
	return nil
}

func (m *Memoria) version(fuenteID int64, version int) (*versionMem, error) {
	f, ok := m.fuentes[fuenteID]
	if !ok {
		return nil, fuentes.ErrNoEncontrado
	}
	v, ok := f.versiones[version]
	if !ok {
		return nil, fuentes.ErrNoEncontrado
	}
	return v, nil
}

func (m *Memoria) enProceso(fuenteID int64, version int) (*versionMem, error) {
	v, err := m.version(fuenteID, version)
	if err != nil || v.estado != fuentes.EstadoProcesando {
		return nil, fuentes.ErrNoEncontrado
	}
	return v, nil
}

func palabras(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

func rank(terminos, palabrasTexto []string) int {
	cuenta := map[string]int{}
	for _, p := range palabrasTexto {
		cuenta[p]++
	}
	total := 0
	for _, t := range terminos {
		if cuenta[t] == 0 {
			return 0
		}
		total += cuenta[t]
	}
	return total
}
