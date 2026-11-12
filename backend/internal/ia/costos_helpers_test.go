package ia_test

import (
	"bytes"
	"testing"

	"github.com/jmoiron/sqlx"

	"github.com/cbaciliod/learning/internal/antifraude"
	"github.com/cbaciliod/learning/internal/ia"
)

// reserva is a reservation of one action for a, as a consumer's job holds it.
func reserva(a ia.Actor) *ia.Reserva { return &ia.Reserva{Actor: a, Acciones: 1} }

// clavesDePrueba are the antifraud keys of the tests; the previous one is
// only set by the rotation tests.
func clavesDePrueba(conAnterior bool) *antifraude.Claves {
	var anterior []byte
	if conAnterior {
		anterior = bytes.Repeat([]byte{2}, 32)
	}
	return antifraude.New(bytes.Repeat([]byte{1}, 32), anterior)
}

func nuevoPgCostos(t *testing.T, db *sqlx.DB, opciones ...ia.OpcionCostos) *ia.PgCostos {
	t.Helper()
	c, err := ia.NewPgCostos(db, clavesDePrueba(false), opciones...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
