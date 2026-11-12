package ia

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/rs/zerolog/log"
)

// Cache limits (contract E6-02 §4).
const (
	vigenciaCache    = 30 * 24 * time.Hour
	maxSalidaCache   = 256 << 10 // 256 KiB
	purgaCache       = 100
	tiempoCache      = 5 * time.Second
	separadorClave   = "\x00"
	proveedorDeCache = "" // an ia_usos row served from the cache has no provider
)

// Entrada is the typed input of an operation, for the code that only needs to
// look at the cache (BuscarEnCache). Only this package implements it, so a
// consumer cannot build a cache key by itself.
type Entrada[S any] interface {
	solicitud() Solicitud
	validarSalida(S) error
}

// Cache stores validated outputs by the hash of the request (PgCache in
// production). It is shared among actors: the input already went through the
// personal data check, and the entry holds no actor.
type Cache interface {
	// Buscar returns the stored output if the entry is still valid; a stale
	// one (expired, a source retired or with a new current version) is deleted
	// and reported as a miss.
	Buscar(ctx context.Context, clave string) (salida []byte, ok bool, err error)
	// Guardar stores salida for the sources it used; it keeps an existing entry.
	Guardar(ctx context.Context, clave, operacion string, salida []byte, fuentes []int64) error
}

// claveCache is hex(sha256(operacion ‖ esquema ‖ instrucciones ‖ datos)) with
// \x00 between the parts (contract E6-02 §4). Each data block is its label,
// \x00 and its text with the whitespace collapsed. The sources are not part
// of the key: a new version changes the passages in the data, hence the key,
// and Solicitud.Fuentes only decides how long an entry stays valid.
func claveCache(sol Solicitud) string {
	h := sha256.New()
	partes := []string{limpiarNUL(sol.Operacion), limpiarNUL(sol.EsquemaSalida), limpiarNUL(sol.Instrucciones)}
	for _, b := range sol.Datos {
		partes = append(partes, limpiarNUL(b.Etiqueta)+separadorClave+normalizarEspacios(b.Texto))
	}
	_, _ = io.WriteString(h, strings.Join(partes, separadorClave))
	return hex.EncodeToString(h.Sum(nil))
}

func normalizarEspacios(s string) string { return strings.Join(strings.Fields(limpiarNUL(s)), " ") }

// limpiarNUL keeps the separator unambiguous: it never appears in a part.
func limpiarNUL(s string) string { return strings.ReplaceAll(s, separadorClave, " ") }

// decodificarCache reads a stored output strictly and checks it against the
// input, like a fresh answer. The struct tags are not checked again: the
// schema version is part of the key.
func decodificarCache[S any](raw []byte, in Entrada[S]) (S, bool) {
	var out S
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return out, false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return out, false
	}
	return out, in.validarSalida(out) == nil
}

// BuscarEnCache is the synchronous check of a consumer, before Reservar and
// without calling the provider (contract E6-02 §2.2): with a valid entry it
// returns the output, writes the ia_usos row of a cache hit for a and costs
// no action. A consumer calls it in the request so a cached answer is served
// even with the budget or the quota spent (D16).
func BuscarEnCache[S any](ctx context.Context, s Servicio, a Actor, in Entrada[S]) (S, bool, error) {
	var cero S
	var salida S
	_, ok, err := s.BuscarCache(ctx, a, in.solicitud(), func(raw json.RawMessage) bool {
		var aceptada bool
		salida, aceptada = decodificarCache(raw, in)
		return aceptada
	})
	if err != nil || !ok {
		return cero, false, err
	}
	return salida, true, nil
}

func (s *servicio) BuscarCache(ctx context.Context, a Actor, sol Solicitud, acepta func(json.RawMessage) bool) (json.RawMessage, bool, error) {
	if !actorValido(a) {
		return nil, false, ErrActorInvalido
	}
	inicio := s.ahora()
	raw, ok := s.leerCache(ctx, sol, acepta)
	if !ok {
		return nil, false, nil
	}
	s.registrar(ctx, usoDeCache(sol, a, s.ahora().Sub(inicio)))
	return raw, true, nil
}

// leerCache looks the entry up; a cache failure is a miss, logged by op only.
func (s *servicio) leerCache(ctx context.Context, sol Solicitud, acepta func(json.RawMessage) bool) (json.RawMessage, bool) {
	if s.cache == nil {
		return nil, false
	}
	raw, ok, err := s.cache.Buscar(ctx, claveCache(sol))
	if err != nil {
		log.Error().Str("op", "ia.cache_buscar").Str("operacion", sol.Operacion).Msg("ia: cache lookup failed")
		return nil, false
	}
	if !ok || !acepta(raw) {
		return nil, false
	}
	return raw, true
}

// usoDeCache is the ia_usos row of a cache hit: no provider, no model, no
// tokens, no cost and no attempts (contract E6-02 §2.3 ★1).
func usoDeCache(sol Solicitud, a Actor, d time.Duration) Uso {
	return Uso{Operacion: sol.Operacion, Actor: a, Proveedor: proveedorDeCache, Duracion: d, Resultado: ResultadoOK, DesdeCache: true}
}

// guardarCache stores a good output; a failure is logged and never replaces it.
func (s *servicio) guardarCache(ctx context.Context, sol Solicitud, salida any) {
	if s.cache == nil {
		return
	}
	raw, err := json.Marshal(salida)
	if err != nil || len(raw) > maxSalidaCache {
		return
	}
	if err := s.cache.Guardar(ctx, claveCache(sol), sol.Operacion, raw, sol.Fuentes); err != nil {
		log.Error().Str("op", "ia.cache_guardar").Str("operacion", sol.Operacion).Msg("ia: cache write failed")
	}
}

// PgCache implements Cache over ia_cache (migration 068).
type PgCache struct {
	db    *sqlx.DB
	ahora func() time.Time
}

// OpcionCache tunes PgCache; tests inject the clock.
type OpcionCache func(*PgCache)

// ConRelojCache sets the clock that decides the expiry of the entries.
func ConRelojCache(ahora func() time.Time) OpcionCache { return func(c *PgCache) { c.ahora = ahora } }

// NewPgCache creates the cache over db.
func NewPgCache(db *sqlx.DB, opciones ...OpcionCache) *PgCache {
	c := &PgCache{db: db, ahora: time.Now}
	for _, o := range opciones {
		o(c)
	}
	return c
}

// Buscar returns the entry if it has not expired and every source it used is
// still `activa` with the same current version as when it was written.
func (c *PgCache) Buscar(ctx context.Context, clave string) ([]byte, bool, error) {
	var e struct {
		Salida     []byte        `db:"salida"`
		FuenteIDs  pq.Int64Array `db:"fuente_ids"`
		VigenteIDs pq.Int64Array `db:"vigente_ids"`
		ExpiraEn   time.Time     `db:"expira_en"`
		CreadaEn   time.Time     `db:"created_at"`
	}
	err := c.db.GetContext(ctx, &e, `SELECT salida, fuente_ids, vigente_ids, expira_en, created_at FROM ia_cache WHERE clave = $1`, clave)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("buscar en la caché de ia: %w", err)
	}
	vigente := e.ExpiraEn.After(c.ahora())
	if vigente {
		vigente, err = c.fuentesVigentes(ctx, e.FuenteIDs, e.VigenteIDs)
		if err != nil {
			return nil, false, err
		}
	}
	if !vigente {
		if err := c.borrarEntradaLeida(ctx, clave, e.CreadaEn); err != nil {
			return nil, false, err
		}
		return nil, false, nil
	}
	return e.Salida, true, nil
}

// borrarEntradaLeida deletes the stale entry that was read, not whatever is
// under the key now: another request may have written a fresh one between the
// read and the delete, and that one must stay.
func (c *PgCache) borrarEntradaLeida(ctx context.Context, clave string, creadaEn time.Time) error {
	if _, err := c.db.ExecContext(ctx, `DELETE FROM ia_cache WHERE clave = $1 AND created_at = $2`, clave, creadaEn); err != nil {
		return fmt.Errorf("borrar entrada vencida de la caché de ia: %w", err)
	}
	return nil
}

// fuentesVigentes: every (source, version) pair still matches an `activa`
// source whose current version is that one. No sources: always true.
func (c *PgCache) fuentesVigentes(ctx context.Context, fuentes, versiones pq.Int64Array) (bool, error) {
	if len(fuentes) == 0 {
		return true, nil
	}
	var coinciden int
	err := c.db.GetContext(ctx, &coinciden, `
		SELECT COUNT(*) FROM unnest($1::bigint[], $2::bigint[]) AS p(fuente_id, version_id)
		JOIN fuentes f ON f.id = p.fuente_id
		WHERE f.estado = 'activa' AND f.version_vigente_id = p.version_id`, fuentes, versiones)
	if err != nil {
		return false, fmt.Errorf("validar fuentes de la caché de ia: %w", err)
	}
	return coinciden == len(fuentes), nil
}

// Guardar writes the entry with the current version of each source (not the
// version of the content, so an explanation of an older cited version is not
// born stale). A source that is not `activa` now cannot be pinned: nothing is
// stored. Up to purgaCache expired entries are deleted each time.
func (c *PgCache) Guardar(ctx context.Context, clave, operacion string, salida []byte, fuentes []int64) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), tiempoCache)
	defer cancel()
	fuentes = append([]int64{}, fuentes...) // never NULL: the columns are NOT NULL
	versiones := make([]int64, 0, len(fuentes))
	for _, f := range fuentes {
		var v int64
		err := c.db.GetContext(ctx, &v, `SELECT version_vigente_id FROM fuentes WHERE id = $1 AND estado = 'activa'`, f)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("leer la versión vigente de una fuente: %w", err)
		}
		versiones = append(versiones, v)
	}
	ahora := c.ahora()
	if _, err := c.db.ExecContext(ctx, `
		INSERT INTO ia_cache (clave, operacion, salida, fuente_ids, vigente_ids, expira_en, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (clave) DO NOTHING`,
		clave, operacion, salida, pq.Int64Array(fuentes), pq.Int64Array(versiones), ahora.Add(vigenciaCache), ahora); err != nil {
		return fmt.Errorf("guardar en la caché de ia: %w", err)
	}
	if _, err := c.db.ExecContext(ctx, `
		DELETE FROM ia_cache WHERE clave IN
		  (SELECT clave FROM ia_cache WHERE expira_en < $1 LIMIT $2)`, ahora, purgaCache); err != nil {
		return fmt.Errorf("purgar la caché de ia: %w", err)
	}
	return nil
}
