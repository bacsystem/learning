package questions

// PreguntaServible is the SQL predicate of a question that can be served in a
// practice, simulacro or diagnostic, for the preguntas table aliased as alias.
// It is the one shared definition (contract PE6): an approved question or one
// of the inherited bank ('activo'); a borrador, en_revision, rechazada or
// eliminada one is never served (SIA-E3-02 regla 1).
func PreguntaServible(alias string) string {
	return alias + ".estado IN ('aprobada', 'activo')"
}

// Question life cycle (SIA-E3-02, contract §3).
const (
	EstadoBorrador   = "borrador"
	EstadoEnRevision = "en_revision"
	EstadoAprobada   = "aprobada"
	EstadoRechazada  = "rechazada"
	EstadoActivo     = "activo" // the inherited bank, served like aprobada
	EstadoEliminada  = "eliminada"
)

// Origins of a question (preguntas.origen).
const (
	OrigenManual    = "manual"
	OrigenBalotario = "balotario"
	OrigenIA        = "ia"
)

// EstadosListables are the values of the estado filter of GET /preguntas:
// every state but eliminada, which is never listed.
var EstadosListables = map[string]bool{EstadoBorrador: true, EstadoEnRevision: true, EstadoAprobada: true,
	EstadoRechazada: true, EstadoActivo: true}

// Origenes are the values of preguntas.origen.
var Origenes = map[string]bool{OrigenManual: true, OrigenBalotario: true, OrigenIA: true}
