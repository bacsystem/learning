package fuentestest

import "testing"

// The in-memory adapter passes the same contract as the Postgres one.
func TestContrato_Memoria(t *testing.T) {
	RunContract(t, func(t *testing.T) Entorno {
		m := NuevaMemoria()
		return Entorno{Repo: m, EspecialidadA: 5, EspecialidadB: 2, Retirar: m.Retirar, NuevaVersion: m.NuevaVersion}
	})
}
