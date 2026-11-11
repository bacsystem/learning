package fuentestest

import "testing"

// The in-memory adapter passes the same contract as the Postgres one.
func TestContrato_Memoria(t *testing.T) {
	RunContract(t, func(t *testing.T) Entorno {
		m := NuevaMemoria()
		return Entorno{Repo: m, Retirar: m.Retirar, NuevaVersion: m.NuevaVersion, Especialidades: [2]int64{2, 5}}
	})
}
