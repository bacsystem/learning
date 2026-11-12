// Package antifraude derives the pseudonymous keys of the antifraud counters
// (ADR 0002): HMAC-SHA256 of the user id with a server secret, one prefix per
// counter so the same user has unrelated keys in each one. It is the only
// place that builds them. It never logs a key nor an HMAC.
package antifraude

import (
	"strconv"

	"github.com/cbaciliod/learning/internal/crypto"
)

// Prefijos of the counters; the prefix separates their domains.
const (
	PrefijoCuotaIA = "ia_cuota"
)

// Claves derives the keys of a counter from ANTIFRAUD_HMAC_KEY (the current
// one, which writes and reads) and ANTIFRAUD_HMAC_KEY_PREVIOUS (optional,
// only reads during a rotation).
type Claves struct {
	vigente  []byte
	anterior []byte
}

// New creates the derivation over the decoded keys; anterior may be empty.
func New(vigente, anterior []byte) *Claves {
	return &Claves{vigente: vigente, anterior: anterior}
}

// Clave returns prefijo ":" hex(HMAC(key, prefijo ":" usuarioID)) for the
// current key and, during a rotation, for the previous one ("" otherwise).
func (c *Claves) Clave(prefijo string, usuarioID int64) (vigente, anterior string) {
	dato := prefijo + ":" + strconv.FormatInt(usuarioID, 10)
	vigente = prefijo + ":" + crypto.HMAC(c.vigente, dato)
	if len(c.anterior) > 0 {
		anterior = prefijo + ":" + crypto.HMAC(c.anterior, dato)
	}
	return vigente, anterior
}
