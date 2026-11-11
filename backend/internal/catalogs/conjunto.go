package catalogs

import "fmt"

// ConjuntoDePesos is the SQL predicate that keeps, among the rows of
// proceso_asignaturas aliased as alias, the set that applies to proceso
// $procesoParam and especialidad $especialidadParam (0 = none): the rows of
// that especialidad if the proceso has at least one, otherwise only the
// compatibility rows. Sets are never mixed row by row (ADR 0001 § Nivel de
// resolución), so the result has at most one row per asignatura.
func ConjuntoDePesos(alias string, procesoParam, especialidadParam int) string {
	return fmt.Sprintf(`%[1]s.proceso_id = $%[2]d AND %[1]s.especialidad_id IS NOT DISTINCT FROM (
		SELECT NULLIF($%[3]d::bigint, 0)
		WHERE EXISTS (SELECT 1 FROM proceso_asignaturas conj
			WHERE conj.proceso_id = $%[2]d AND conj.especialidad_id = NULLIF($%[3]d::bigint, 0)))`,
		alias, procesoParam, especialidadParam)
}

// filtroCompatibilidad keeps only the compatibility rows (no especialidad).
func filtroCompatibilidad(alias string, procesoParam int) string {
	return fmt.Sprintf(`%[1]s.proceso_id = $%[2]d AND %[1]s.especialidad_id IS NULL`, alias, procesoParam)
}
