package apispec

import (
	"fmt"
	"slices"
	"sort"
)

// comparar checks the spec against the routes of the code and returns every
// mismatch (strict mode, ADR 0012 decision 3): a route of the code missing
// from the spec, an operation of the spec missing from the code without
// x-estado: planificado, a different x-rol, and the shape rules of each
// operation.
func comparar(codigo map[string][]rutaCodigo, spec infoSpec) []string {
	var errores []string
	enCodigo := map[string]rutaCodigo{}
	paqueteDe := map[string]string{}
	for paquete, rutas := range codigo {
		for _, r := range rutas {
			enCodigo[r.Clave] = r
			paqueteDe[r.Clave] = paquete
		}
	}

	for _, clave := range clavesDeOps(spec.Ops) {
		errores = append(errores, compararOperacion(spec.Ops[clave], enCodigo)...)
	}
	for _, clave := range clavesDeRutas(enCodigo) {
		if _, ok := spec.Ops[clave]; ok {
			continue
		}
		errores = append(errores, fmt.Sprintf(
			"%s (%s) no está en el spec: documéntala en docs/api/openapi/paquetes/%s.yaml y referénciala desde openapi.yaml",
			clave, enCodigo[clave].Pos, paqueteDe[clave]))
	}
	for _, h := range spec.Huerfanas {
		errores = append(errores, fmt.Sprintf("%s no está referenciada desde openapi.yaml", h))
	}
	return errores
}

func compararOperacion(op opSpec, enCodigo map[string]rutaCodigo) []string {
	var errores []string
	if !rolValido(op.Rol) {
		errores = append(errores, fmt.Sprintf("%s: x-rol %q inválido o ausente (publico|autenticado|admin)", op.Clave, op.Rol))
	}
	if len(op.Tags) != 1 || op.Tags[0] == "" {
		errores = append(errores, fmt.Sprintf("%s: debe tener exactamente un tag (tiene %v)", op.Clave, op.Tags))
	}
	if op.Estado != "" && op.Estado != estadoPlanificado {
		errores = append(errores, fmt.Sprintf("%s: x-estado %q no permitido (solo %q)", op.Clave, op.Estado, estadoPlanificado))
	}
	r, existe := enCodigo[op.Clave]
	switch {
	case !existe && op.Estado != estadoPlanificado:
		errores = append(errores, fmt.Sprintf("%s está en el spec pero no en el código; si aún no se implementa, márquela con x-estado: planificado", op.Clave))
	case existe && op.Estado == estadoPlanificado:
		errores = append(errores, fmt.Sprintf("%s ya existe en el código (%s); quite x-estado: planificado", op.Clave, r.Pos))
	case existe && op.Rol != string(r.Rol) && rolValido(op.Rol):
		errores = append(errores, fmt.Sprintf("%s: x-rol es %q en el spec y %q en el código (%s)", op.Clave, op.Rol, r.Rol, r.Pos))
	}
	return errores
}

func clavesDeOps(m map[string]opSpec) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func clavesDeRutas(m map[string]rutaCodigo) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// rolValido reports whether s is one of the three x-rol values.
func rolValido(s string) bool {
	return slices.Contains([]string{string(rolPublico), string(rolAutenticado), string(rolAdmin)}, s)
}
