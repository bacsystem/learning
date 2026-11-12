package ia

import (
	"errors"
	"fmt"
	"strconv"
)

// Input and output of each operation (regla 1). E6-01 fixes the minimum the
// double and the tests need; each consuming story extends them without
// breaking the port. Inputs reference questions, passages, temas and areas
// by opaque id only; ids travel in the block labels, never in the texts.

// ItemTexto is one item to classify.
type ItemTexto struct {
	ID    int64
	Texto string
}

// OpcionTaxonomia is one entry of the taxonomy offered to the classifier.
type OpcionTaxonomia struct {
	AsignaturaID int64
	TemaID       *int64
	Nombre       string
}

// Pasaje is a citable passage given as data.
type Pasaje struct {
	ID    int64
	Texto string
}

type EntradaClasificar struct {
	Fuentes   []int64 // fuente_id whose content goes in the data (cache validity)
	Items     []ItemTexto
	Taxonomia []OpcionTaxonomia
}

type SalidaClasificar struct {
	Resultados []Clasificacion `json:"resultados" validate:"min=1,dive"`
}

type Clasificacion struct {
	ID           int64   `json:"id" validate:"gt=0"`
	AsignaturaID int64   `json:"asignatura_id" validate:"gt=0"`
	TemaID       *int64  `json:"tema_id" validate:"omitempty,gt=0"`
	Confianza    float64 `json:"confianza" validate:"gte=0,lte=1"`
}

type EntradaGenerarPreguntas struct {
	Fuentes  []int64 // fuente_id whose content goes in the data (cache validity)
	TemaID   int64
	Cantidad int
	Pasajes  []Pasaje
}

type SalidaGenerarPreguntas struct {
	Preguntas []PreguntaGenerada `json:"preguntas" validate:"min=1,dive"`
}

type PreguntaGenerada struct {
	Enunciado    string   `json:"enunciado" validate:"required"`
	Alternativas []string `json:"alternativas" validate:"min=2,max=6,dive,required"`
	Correcta     int      `json:"correcta" validate:"gte=0"`
	PasajeIDs    []int64  `json:"pasaje_ids" validate:"min=1,dive,gt=0"`
}

type EntradaGenerarMaterial struct {
	Fuentes []int64 // fuente_id whose content goes in the data (cache validity)
	TemaID  int64
	Pasajes []Pasaje
}

type SalidaGenerarMaterial struct {
	Titulo    string  `json:"titulo" validate:"required,max=300"`
	Contenido string  `json:"contenido" validate:"required"`
	PasajeIDs []int64 `json:"pasaje_ids" validate:"min=1,dive,gt=0"`
}

type EntradaGenerarExplicacion struct {
	Fuentes      []int64 // fuente_id whose content goes in the data (cache validity)
	PreguntaID   int64
	Enunciado    string
	Alternativas []string
	Correcta     int
	Pasajes      []Pasaje
}

type SalidaGenerarExplicacion struct {
	Explicacion string  `json:"explicacion" validate:"required"`
	PasajeIDs   []int64 `json:"pasaje_ids" validate:"min=1,dive,gt=0"`
}

// Output schema names (Solicitud.EsquemaSalida).
const (
	esquemaClasificar         = "clasificar.v1"
	esquemaGenerarPreguntas   = "generar_preguntas.v1"
	esquemaGenerarMaterial    = "generar_material.v1"
	esquemaGenerarExplicacion = "generar_explicacion.v1"
)

// Fixed instructions of each operation. The prompts are SIA-E6-10's; these
// placeholders only fix that the instructions are code, not data.
const (
	instruccionesClasificar         = "Clasifica cada item en una opción de la taxonomía. Responde solo con el JSON del esquema."
	instruccionesGenerarPreguntas   = "Genera preguntas de opción múltiple citando los pasajes. Responde solo con el JSON del esquema."
	instruccionesGenerarMaterial    = "Redacta material de estudio citando los pasajes. Responde solo con el JSON del esquema."
	instruccionesGenerarExplicacion = "Explica la respuesta correcta citando los pasajes. Responde solo con el JSON del esquema."
)

// Block labels: kind and ids, so the double (and an adapter) can rebuild the
// structure without reading numbers out of the texts.
const (
	etiquetaItem        = "item"
	etiquetaOpcion      = "opcion"
	etiquetaPasaje      = "pasaje"
	etiquetaEnunciado   = "enunciado"
	etiquetaAlternativa = "alternativa"
	etiquetaParametro   = "parametro"
)

var errSalida = errors.New("salida fuera del esquema")

func etiqueta(tipo string, ids ...int64) string {
	e := tipo
	for _, id := range ids {
		e += ":" + strconv.FormatInt(id, 10)
	}
	return e
}

func bloquesPasajes(ps []Pasaje) []Bloque {
	out := make([]Bloque, len(ps))
	for i, p := range ps {
		out[i] = Bloque{Etiqueta: etiqueta(etiquetaPasaje, p.ID), Texto: p.Texto}
	}
	return out
}

func (in EntradaClasificar) solicitud() Solicitud {
	var datos []Bloque
	for _, it := range in.Items {
		datos = append(datos, Bloque{Etiqueta: etiqueta(etiquetaItem, it.ID), Texto: it.Texto})
	}
	for _, o := range in.Taxonomia {
		ids := []int64{o.AsignaturaID}
		if o.TemaID != nil {
			ids = append(ids, *o.TemaID)
		}
		datos = append(datos, Bloque{Etiqueta: etiqueta(etiquetaOpcion, ids...), Texto: o.Nombre})
	}
	return Solicitud{Operacion: OpClasificar, Instrucciones: instruccionesClasificar, Datos: datos, EsquemaSalida: esquemaClasificar, Fuentes: in.Fuentes}
}

// validar: one result per input item, each id of the input once.
type taxonomiaOfrecida map[[2]int64]bool

// sinTema marks "no topic" in a taxonomiaOfrecida key; ids are positive.
const sinTema int64 = 0

func opcionesOfrecidas(opciones []OpcionTaxonomia) taxonomiaOfrecida {
	t := make(taxonomiaOfrecida, len(opciones))
	for _, o := range opciones {
		t[[2]int64{o.AsignaturaID, temaOCero(o.TemaID)}] = true
	}
	return t
}

func temaOCero(id *int64) int64 {
	if id == nil {
		return sinTema
	}
	return *id
}

// contiene reports whether the model chose an option it was given: the model
// output is untrusted, so an invented subject or topic is rejected.
func (t taxonomiaOfrecida) contiene(asignaturaID int64, temaID *int64) bool {
	return t[[2]int64{asignaturaID, temaOCero(temaID)}]
}

func (s SalidaClasificar) validar(in EntradaClasificar) error {
	items := make(map[int64]bool, len(in.Items))
	for _, it := range in.Items {
		items[it.ID] = false
	}
	ofrecidas := opcionesOfrecidas(in.Taxonomia)
	for _, r := range s.Resultados {
		visto, existe := items[r.ID]
		if !existe || visto {
			return fmt.Errorf("%w: resultado de un item ajeno o repetido", errSalida)
		}
		if !ofrecidas.contiene(r.AsignaturaID, r.TemaID) {
			return fmt.Errorf("%w: clasificación fuera de la taxonomía ofrecida", errSalida)
		}
		items[r.ID] = true
	}
	if len(s.Resultados) != len(in.Items) {
		return fmt.Errorf("%w: faltan resultados", errSalida)
	}
	return nil
}

func (in EntradaGenerarPreguntas) solicitud() Solicitud {
	datos := append([]Bloque{{Etiqueta: etiqueta(etiquetaParametro+":tema", in.TemaID)},
		{Etiqueta: etiqueta(etiquetaParametro+":cantidad", int64(in.Cantidad))}}, bloquesPasajes(in.Pasajes)...)
	return Solicitud{Operacion: OpGenerarPreguntas, Instrucciones: instruccionesGenerarPreguntas, Datos: datos, EsquemaSalida: esquemaGenerarPreguntas, Fuentes: in.Fuentes}
}

// validar: at most Cantidad questions, a correct index inside the
// alternatives and citations to given passages only.
func (s SalidaGenerarPreguntas) validar(in EntradaGenerarPreguntas) error {
	if len(s.Preguntas) > in.Cantidad {
		return fmt.Errorf("%w: más preguntas de las pedidas", errSalida)
	}
	for _, p := range s.Preguntas {
		if p.Correcta >= len(p.Alternativas) {
			return fmt.Errorf("%w: alternativa correcta fuera de rango", errSalida)
		}
		if err := citasValidas(p.PasajeIDs, in.Pasajes); err != nil {
			return err
		}
	}
	return nil
}

func (in EntradaGenerarMaterial) solicitud() Solicitud {
	datos := append([]Bloque{{Etiqueta: etiqueta(etiquetaParametro+":tema", in.TemaID)}}, bloquesPasajes(in.Pasajes)...)
	return Solicitud{Operacion: OpGenerarMaterial, Instrucciones: instruccionesGenerarMaterial, Datos: datos, EsquemaSalida: esquemaGenerarMaterial, Fuentes: in.Fuentes}
}

func (s SalidaGenerarMaterial) validar(in EntradaGenerarMaterial) error {
	return citasValidas(s.PasajeIDs, in.Pasajes)
}

func (in EntradaGenerarExplicacion) solicitud() Solicitud {
	datos := []Bloque{
		{Etiqueta: etiqueta(etiquetaParametro+":pregunta", in.PreguntaID)},
		{Etiqueta: etiqueta(etiquetaParametro+":correcta", int64(in.Correcta))},
		{Etiqueta: etiquetaEnunciado, Texto: in.Enunciado},
	}
	for i, a := range in.Alternativas {
		datos = append(datos, Bloque{Etiqueta: etiqueta(etiquetaAlternativa, int64(i)), Texto: a})
	}
	datos = append(datos, bloquesPasajes(in.Pasajes)...)
	return Solicitud{Operacion: OpGenerarExplicacion, Instrucciones: instruccionesGenerarExplicacion, Datos: datos, EsquemaSalida: esquemaGenerarExplicacion, Fuentes: in.Fuentes}
}

func (s SalidaGenerarExplicacion) validar(in EntradaGenerarExplicacion) error {
	return citasValidas(s.PasajeIDs, in.Pasajes)
}

// citasValidas: every cited passage was given as data.
func citasValidas(citas []int64, pasajes []Pasaje) error {
	dados := make(map[int64]bool, len(pasajes))
	for _, p := range pasajes {
		dados[p.ID] = true
	}
	for _, id := range citas {
		if !dados[id] {
			return fmt.Errorf("%w: cita a un pasaje no dado", errSalida)
		}
	}
	return nil
}

// validarSalida makes every input an Entrada: it checks the output against it.
func (in EntradaClasificar) validarSalida(o SalidaClasificar) error { return o.validar(in) }

func (in EntradaGenerarPreguntas) validarSalida(o SalidaGenerarPreguntas) error { return o.validar(in) }

func (in EntradaGenerarMaterial) validarSalida(o SalidaGenerarMaterial) error { return o.validar(in) }

func (in EntradaGenerarExplicacion) validarSalida(o SalidaGenerarExplicacion) error {
	return o.validar(in)
}
