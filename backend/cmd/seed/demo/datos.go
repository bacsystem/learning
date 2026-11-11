package main

// The synthetic dataset. Every row hangs from the two demo institutions or
// carries the DEMO- prefix, so the seed never adopts rows it did not create.

const (
	institucionSalud      = "Institución Demo Salud"
	institucionCompat     = "Institución Demo Compatibilidad"
	gradoDemo             = "Grado Demo"
	nivelDemo             = "suboficial"
	procesoSERUMS         = "SERUMS 2026-I"
	procesoCompatibilidad = "Proceso de compatibilidad demo"
	montoInscripcion      = 0.00
	dominioEmail          = "@demo.invalid" // RFC 6761: never delivered
	tiempoSimulacroSeg    = 3600
	preguntasPorArea      = 3 // servable questions seeded per area and proceso
	preguntasExamen       = 2 // numero_preguntas per area (≤ preguntasPorArea)
)

type especialidadDemo struct {
	Codigo, Nombre string
	Activa         bool
}

var especialidadesDemo = []especialidadDemo{
	{"DEMO-MED", "Medicina", true},
	{"DEMO-ENF", "Enfermería", true},
	{"DEMO-OBS", "Obstetricia", true},
	{"DEMO-ODO", "Odontología", false},
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
		Pesos: map[string][]areaDemo{
			"DEMO-MED": {{"DEMO-SP", 30}, {"DEMO-EPI", 30}, {"DEMO-MI", 40}},
		},
	},
	{
		Institucion: institucionCompat, Nombre: procesoCompatibilidad, Tipo: "ascenso",
		Compatibilidad: []areaDemo{{"DEMO-SP", 50}, {"DEMO-EPI", 50}},
	},
}

// usuarioDemo is a synthetic, verified and active account with the tour done.
type usuarioDemo struct {
	Username     string
	Rol          string // admin | usuario
	Especialidad string // code of the persona's especialidad
	Nombres      string
	Inscripcion  string // estado in SERUMS 2026-I: "" (none), aprobado, pendiente
}

var usuariosDemo = []usuarioDemo{
	{"demo_admin", "admin", "DEMO-MED", "Admin", ""},
	{"demo_enfermeria", "usuario", "DEMO-ENF", "Enfermera", "aprobado"},
	{"demo_medicina", "usuario", "DEMO-MED", "Médico", "aprobado"},
	{"demo_pendiente", "usuario", "DEMO-OBS", "Obstetra", "pendiente"},
	{"demo_esp_inactiva", "usuario", "DEMO-ODO", "Odontólogo", "aprobado"},
}
