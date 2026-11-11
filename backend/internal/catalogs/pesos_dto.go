package catalogs

// Estados derivados de una configuración de pesos (ADR 0001; E1-03 regla 6).
const (
	EstadoPesosCompleta   = "completa"
	EstadoPesosIncompleta = "incompleta"
)

// ── Requests ──────────────────────────────────────────────────────────────────

// AreaPesoRequest is one area of PUT /admin/procesos/{id}/pesos/{especialidadId}.
type AreaPesoRequest struct {
	AsignaturaID    int64   `json:"asignatura_id"`
	PesoPorcentual  float64 `json:"peso_porcentual"`
	NumeroPreguntas int     `json:"numero_preguntas"`
	Coeficiente     float64 `json:"coeficiente"`
}

// GuardarPesosRequest is the whole configuration of (proceso, especialidad).
type GuardarPesosRequest struct {
	Areas []AreaPesoRequest `json:"areas"`
}

// ── Responses ─────────────────────────────────────────────────────────────────

// AreaPeso is one area of a ConfiguracionPesos.
type AreaPeso struct {
	AsignaturaID    int64   `json:"asignatura_id" db:"asignatura_id"`
	Nombre          string  `json:"nombre" db:"nombre"`
	Codigo          string  `json:"codigo" db:"codigo"`
	PesoPorcentual  float64 `json:"peso_porcentual" db:"peso_porcentual"`
	NumeroPreguntas int     `json:"numero_preguntas" db:"numero_preguntas"`
	Coeficiente     float64 `json:"coeficiente" db:"coeficiente"`
}

// ConfiguracionPesos is the weights of (proceso, especialidad) (PE2, PE3).
// Incompleta: Areas is empty and Plantilla holds the compatibility rows to
// prefill the form; completa: Plantilla is empty.
type ConfiguracionPesos struct {
	ProcesoID      int64      `json:"proceso_id"`
	EspecialidadID int64      `json:"especialidad_id"`
	Especialidad   string     `json:"especialidad"`
	Estado         string     `json:"estado"`
	Areas          []AreaPeso `json:"areas"`
	SumaPesos      float64    `json:"suma_pesos"`
	SumaPreguntas  int        `json:"suma_preguntas"`
	Plantilla      []AreaPeso `json:"plantilla"`
}

// ResumenCompatibilidad summarises the compatibility rows of a proceso.
type ResumenCompatibilidad struct {
	Areas     int     `json:"areas" db:"areas"`
	SumaPesos float64 `json:"suma_pesos" db:"suma_pesos"`
}

// ResumenEspecialidad is one especialidad of PE1.
type ResumenEspecialidad struct {
	EspecialidadID int64   `json:"especialidad_id" db:"especialidad_id"`
	Nombre         string  `json:"nombre" db:"nombre"`
	Estado         string  `json:"estado" db:"-"`
	Areas          int     `json:"areas" db:"areas"`
	SumaPesos      float64 `json:"suma_pesos" db:"suma_pesos"`
	SumaPreguntas  int     `json:"suma_preguntas" db:"suma_preguntas"`
}

// ResumenPesos is the PE1 response.
type ResumenPesos struct {
	ProcesoID      int64                 `json:"proceso_id"`
	Compatibilidad ResumenCompatibilidad `json:"compatibilidad"`
	Especialidades []ResumenEspecialidad `json:"especialidades"`
}

// ── PE6 (Go service, no HTTP route) ───────────────────────────────────────────

// ModoPesos tells consumers that accept estimated weights from the official
// simulacro, which never does (E1-03 reglas 8–9).
type ModoPesos int

const (
	ModoEstimable ModoPesos = iota // diagnóstico, prácticas, dashboard, plan…
	ModoOficial                    // solo SIA-E9-05
)

// PesoArea is the weight of one area for a consumer.
type PesoArea struct {
	AsignaturaID    int64
	PesoPorcentual  float64
	NumeroPreguntas *int    // nil in estimated weights
	Coeficiente     float64 // 1.00 in estimated weights
}

// Pesos is what Service.Pesos returns.
type Pesos struct {
	Areas          []PesoArea
	PesosEstimados bool
}
