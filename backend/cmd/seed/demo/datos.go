package main

// The synthetic dataset. Every row hangs from the two demo institutions or
// carries the DEMO- prefix, so the seed never adopts rows it did not create.

const (
	institucionSalud      = "Institución Demo Salud"
	institucionCompat     = "Institución Demo Compatibilidad"
	gradoDemo             = "Grado Demo"
	nivelDemo             = "suboficial"
	procesoSERUMS         = "SERUMS 2026-I"
	procesoSERUMS2        = "SERUMS 2026-II" // second proceso of the same institution
	procesoCompatibilidad = "Proceso de compatibilidad demo"
	montoInscripcion      = 0.00
	prefijoUsuario        = "demo_"         // every demo username; production ones never carry it
	dominioEmail          = "@demo.invalid" // RFC 6761: never delivered
	tiempoSimulacroSeg    = 3600
	preguntasPorArea      = 10 // servable questions seeded per area and proceso
	preguntasExamen       = 2  // numero_preguntas per area (≤ preguntasPorArea)
)

var institucionesDemo = []string{institucionSalud, institucionCompat}

type especialidadDemo struct {
	Institucion    string
	Codigo, Nombre string
	Activa         bool
}

// The compatibility institution has its own Enfermería: it is what
// demo_enfermeria picks in that proceso, and it is not valid in SERUMS.
var especialidadesDemo = []especialidadDemo{
	{institucionSalud, "DEMO-MED", "Medicina", true},
	{institucionSalud, "DEMO-ENF", "Enfermería", true},
	{institucionSalud, "DEMO-OBS", "Obstetricia", true},
	{institucionSalud, "DEMO-ODO", "Odontología", false},
	{institucionCompat, "DEMO-ENF", "Enfermería", true},
}

type asignaturaDemo struct{ Codigo, Nombre string }

var asignaturasDemo = []asignaturaDemo{
	{"DEMO-SP", "Salud pública"},
	{"DEMO-EPI", "Epidemiología"},
	{"DEMO-GES", "Gestión sanitaria"},
	{"DEMO-ETI", "Ética y deontología"},
	{"DEMO-MI", "Medicina interna"},
	{"DEMO-CE", "Cuidados de enfermería"},
}

// areaDemo is one area of a set of weights: asignatura code and percentage.
type areaDemo struct {
	Codigo string
	Peso   float64
}

type procesoDemo struct {
	Institucion string
	Nombre      string
	Tipo        string
	// Compatibilidad: the general rows (no especialidad), summing 100.
	Compatibilidad []areaDemo
	// Pesos per especialidad code (E1-03), each set summing 100.
	Pesos map[string][]areaDemo
}

var procesosDemo = []procesoDemo{
	{
		Institucion: institucionSalud, Nombre: procesoSERUMS, Tipo: "admision",
		Compatibilidad: []areaDemo{
			{"DEMO-SP", 20}, {"DEMO-EPI", 20}, {"DEMO-GES", 20}, {"DEMO-ETI", 20}, {"DEMO-MI", 10}, {"DEMO-CE", 10},
		},
		// Medicina is complete; Enfermería and Obstetricia stay estimated.
		Pesos: map[string][]areaDemo{
			"DEMO-MED": {{"DEMO-SP", 20}, {"DEMO-EPI", 15}, {"DEMO-GES", 15}, {"DEMO-ETI", 10}, {"DEMO-MI", 30}, {"DEMO-CE", 10}},
		},
	},
	{
		Institucion: institucionSalud, Nombre: procesoSERUMS2, Tipo: "admision",
		Compatibilidad: []areaDemo{{"DEMO-SP", 40}, {"DEMO-EPI", 30}, {"DEMO-CE", 30}},
	},
	{
		Institucion: institucionCompat, Nombre: procesoCompatibilidad, Tipo: "ascenso",
		Compatibilidad: []areaDemo{{"DEMO-SP", 50}, {"DEMO-EPI", 50}},
	},
}

// temasDemo are general temas (every especialidad) of one asignatura.
var temasDemo = []struct{ Asignatura, Nombre string }{
	{"DEMO-SP", "Atención primaria de salud"},
	{"DEMO-SP", "Promoción de la salud"},
}

// usuarioDemo is a synthetic, verified and active account with the tour done,
// whose persona belongs to the health institution.
type usuarioDemo struct {
	Username      string
	Rol           string // admin | usuario
	Especialidad  string // code of the persona's especialidad
	Nombres       string
	Inscripciones map[string]string // proceso name → estado (aprobado | pendiente)
}

var usuariosDemo = []usuarioDemo{
	{"demo_admin", "admin", "DEMO-MED", "Admin", nil},
	{"demo_enfermeria", "usuario", "DEMO-ENF", "Enfermera", map[string]string{procesoSERUMS: "aprobado", procesoSERUMS2: "aprobado", procesoCompatibilidad: "aprobado"}},
	{"demo_medicina", "usuario", "DEMO-MED", "Médico", map[string]string{procesoSERUMS: "aprobado"}},
	{"demo_pendiente", "usuario", "DEMO-OBS", "Obstetra", map[string]string{procesoSERUMS: "pendiente"}},
	{"demo_esp_inactiva", "usuario", "DEMO-ODO", "Odontólogo", map[string]string{procesoSERUMS: "aprobado"}},
}
