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
	"unicode"

	"github.com/cbaciliod/learning/internal/audit"
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
}

type fuenteMem struct {
	id             int64
	titulo         string
	estado         string
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

func (m *Memoria) GuardarFuente(_ context.Context, f fuentes.NuevaFuente) (int64, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.registrarHash(f.Contenido); err != nil {
		return 0, 0, err
	}
	m.sigFuente++
	fm := &fuenteMem{id: m.sigFuente, titulo: f.Titulo, estado: fuentes.EstadoProcesando,
		especialidades: map[int64]bool{}, versiones: map[int]*versionMem{1: {numero: 1, estado: fuentes.EstadoProcesando}}}
	for _, e := range f.EspecialidadIDs {
		fm.especialidades[e] = true
	}
	m.fuentes[fm.id] = fm
	return fm.id, 1, nil
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
		if f.estado != fuentes.EstadoActiva || !(len(f.especialidades) == 0 || f.especialidades[b.EspecialidadID]) {
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

func (m *Memoria) PublicarVersion(_ context.Context, fuenteID int64, version int, pasajes []fuentes.PasajeNuevo, _ audit.TxFunc) error {
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
	if f.estado != fuentes.EstadoRetirada {
		f.estado = fuentes.EstadoActiva
	}
	return nil
}

func (m *Memoria) MarcarVersionFallida(_ context.Context, fuenteID int64, version int, motivo string, _ audit.TxFunc) error {
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
	if f := m.fuentes[fuenteID]; f.vigente == 0 {
		f.estado = fuentes.EstadoFallida
	}
	return nil
}

// ── Test hooks (outside the port) ─────────────────────────────────────────────

// Retirar retires a fuente, as S5 will (E2-06); the life cycle is not in the port.
func (m *Memoria) Retirar(_ context.Context, fuenteID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.fuentes[fuenteID]
	if !ok || f.vigente == 0 {
		return fuentes.ErrNoEncontrado
	}
	f.estado = fuentes.EstadoRetirada
	return nil
}

// NuevaVersion adds the next version in 'procesando', as S2 will (E2-06).
func (m *Memoria) NuevaVersion(_ context.Context, fuenteID int64, contenido []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.fuentes[fuenteID]
	if !ok {
		return 0, fuentes.ErrNoEncontrado
	}
	if err := m.registrarHash(contenido); err != nil {
		return 0, err
	}
	n := len(f.versiones) + 1
	f.versiones[n] = &versionMem{numero: n, estado: fuentes.EstadoProcesando}
	return n, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

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
