package questions

import (
	"slices"
)

// ReemplazarCitasRequest is the body of PUT /preguntas/{id}/citas (E3-07 §4).
// PasajeIDs is a pointer so that an absent or null list differs from [].
type ReemplazarCitasRequest struct {
	PasajeIDs *[]int64 `json:"pasaje_ids"`
}

// pasajeInfo is what the rules need to know about a requested pasaje.
type pasajeInfo struct {
	FuenteID     int64
	FuenteActiva bool
}

// planCitas is the validated change of the citations of a pregunta.
// Antes and Despues are sorted, for the audit row.
type planCitas struct {
	Agregar, Quitar, Antes, Despues []int64
}

// sinCambios reports that the list equals the current one: nothing is
// written or audited.
func (p planCitas) sinCambios() bool { return len(p.Agregar) == 0 && len(p.Quitar) == 0 }

// validarPasajeIDs runs steps 2–3 of E3-07 §4.2, which need no database. It
// returns the ids without duplicates, ascending.
func validarPasajeIDs(req ReemplazarCitasRequest) ([]int64, error) {
	if req.PasajeIDs == nil {
		return nil, &ValidacionPreguntaError{Campos: map[string]string{campoPasajes: "obligatorio"}}
	}
	ids := *req.PasajeIDs
	vistos := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, &ValidacionPreguntaError{Campos: map[string]string{campoPasajes: "ids enteros positivos"}}
		}
		if vistos[id] {
			return nil, &ValidacionPreguntaError{Campos: map[string]string{campoPasajes: "sin repetidos"}}
		}
		vistos[id] = true
	}
	if len(ids) > MaxCitas {
		return nil, regla(CodigoLimiteCitas, map[string]any{"maximo": MaxCitas})
	}
	ordenados := append([]int64{}, ids...)
	slices.Sort(ordenados)
	return ordenados, nil
}

// decidirCitas applies steps 6–8 of E3-07 §4.2 to the state read under the
// lock. pedidos are the ids of the request (sorted, no duplicates); info has
// an entry for every pasaje that exists.
func decidirCitas(b preguntaBloqueada, pedidos []int64, info map[int64]pasajeInfo) (planCitas, error) {
	var faltantes []int64
	for _, id := range pedidos {
		if _, ok := info[id]; !ok {
			faltantes = append(faltantes, id)
		}
	}
	if len(faltantes) > 0 {
		return planCitas{}, regla(CodigoReferenciaInvalida, map[string]any{"pasaje_ids": faltantes})
	}

	plan := planCitas{Despues: pedidos}
	actuales := make(map[int64]bool, len(b.Citas))
	for _, c := range b.Citas {
		actuales[c.PasajeID] = true
		plan.Antes = append(plan.Antes, c.PasajeID)
	}
	slices.Sort(plan.Antes)
	pedidosSet := make(map[int64]bool, len(pedidos))
	for _, id := range pedidos {
		pedidosSet[id] = true
		if !actuales[id] {
			plan.Agregar = append(plan.Agregar, id)
		}
	}
	for _, id := range plan.Antes {
		if !pedidosSet[id] {
			plan.Quitar = append(plan.Quitar, id)
		}
	}

	var inactivas []int64
	for _, id := range plan.Agregar {
		if !info[id].FuenteActiva {
			inactivas = append(inactivas, id)
		}
	}
	if len(inactivas) > 0 {
		return planCitas{}, regla(CodigoFuenteNoActiva, map[string]any{"pasaje_ids": inactivas})
	}

	// activo (the inherited bank) has no such guard: it is served without citations.
	if b.Estado == EstadoAprobada && len(plan.Quitar) > 0 && !algunaActiva(pedidos, info) {
		return planCitas{}, regla(CodigoUltimaCitaActiva, nil)
	}
	return plan, nil
}

func algunaActiva(ids []int64, info map[int64]pasajeInfo) bool {
	for _, id := range ids {
		if info[id].FuenteActiva {
			return true
		}
	}
	return false
}
