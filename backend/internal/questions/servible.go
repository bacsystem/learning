package questions

// PreguntaServible is the SQL predicate of a question that can be served in a
// practice, simulacro or diagnostic, for the preguntas table aliased as alias.
// It is the one shared definition (contract PE6); SIA-E3-02 will widen it to
// estado IN ('aprobada', 'activo').
func PreguntaServible(alias string) string {
	return alias + ".estado = 'activo'"
}
