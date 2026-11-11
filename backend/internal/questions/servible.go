package questions

// PreguntaServible is the SQL predicate of a question that can be served in a
// practice, simulacro or diagnostic, for the preguntas table aliased as alias.
// It is the one shared definition (contract PE6): an approved question or one
// of the inherited bank ('activo'); a borrador, en_revision, rechazada or
// eliminada one is never served (SIA-E3-02 regla 1).
func PreguntaServible(alias string) string {
	return alias + ".estado IN ('aprobada', 'activo')"
}
