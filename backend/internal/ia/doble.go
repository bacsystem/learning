package ia

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

// NombreDoble is IA_PROVEEDOR of the deterministic double; it is also its
// Nombre() and its model.
const NombreDoble = "doble"

// confianzaDoble is the fixed confidence of the double's classification.
const confianzaDoble = 0.5

// alternativasDoble are the alternatives of every generated question.
var alternativasDoble = []string{"Alternativa A", "Alternativa B", "Alternativa C", "Alternativa D"}

// ProveedorDesdeNombre returns the provider of IA_PROVEEDOR: nil (disabled)
// for "", the double for "doble". The real adapters come with SIA-E6-10.
func ProveedorDesdeNombre(nombre string) (Proveedor, error) {
	switch nombre {
	case "":
		return nil, nil
	case NombreDoble:
		return NewDoble(), nil
	default:
		return nil, fmt.Errorf("IA_PROVEEDOR desconocido: %q", nombre)
	}
}

// Doble is the deterministic adapter (regla 7): no network, output derived
// only from the input, tokens = runes / 4, cost 0. Its failure modes are set
// by constructor options, never by the input.
type Doble struct {
	mu             sync.Mutex
	lento          bool
	transitorios   int // calls left that fail as transient
	salidaInvalida int // calls left that answer outside the schema
}

// OpcionDoble sets a failure mode of the double.
type OpcionDoble func(*Doble)

// Lento blocks every call until its ctx ends.
func Lento() OpcionDoble { return func(d *Doble) { d.lento = true } }

// Transitorio makes the first n calls fail with *ErrorTransitorio.
func Transitorio(n int) OpcionDoble { return func(d *Doble) { d.transitorios = n } }

// SalidaInvalida makes the first n answers JSON outside the schema.
func SalidaInvalida(n int) OpcionDoble { return func(d *Doble) { d.salidaInvalida = n } }

// NewDoble creates the double with its failure modes.
func NewDoble(opciones ...OpcionDoble) *Doble {
	d := &Doble{}
	for _, o := range opciones {
		o(d)
	}
	return d
}

func (d *Doble) Nombre() string { return NombreDoble }

func (d *Doble) CostoUSD(Respuesta) float64 { return 0 }

var errTransitorioDoble = errors.New("fallo transitorio simulado")

// Completar answers s deterministically, or fails as its mode says.
func (d *Doble) Completar(ctx context.Context, s Solicitud) (Respuesta, error) {
	if d.lento {
		<-ctx.Done()
		return Respuesta{}, ctx.Err()
	}
	if d.consumir(&d.transitorios) {
		return Respuesta{}, &ErrorTransitorio{Causa: errTransitorioDoble}
	}
	salida := []byte(`{"campo_fuera_del_esquema":true}`)
	if !d.consumir(&d.salidaInvalida) {
		var err error
		if salida, err = salidaDoble(s); err != nil {
			return Respuesta{}, err
		}
	}
	return Respuesta{JSON: salida, TokensEntrada: tokensDe(s), TokensSalida: utf8.RuneCount(salida) / 4, Modelo: NombreDoble}, nil
}

// consumir takes one unit of a failure counter, safely across goroutines.
func (d *Doble) consumir(n *int) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if *n == 0 {
		return false
	}
	*n--
	return true
}

func tokensDe(s Solicitud) int {
	n := utf8.RuneCountInString(s.Instrucciones)
	for _, b := range s.Datos {
		n += utf8.RuneCountInString(b.Texto)
	}
	return n / 4
}

// salidaDoble builds the answer of each operation from the block labels.
func salidaDoble(s Solicitud) ([]byte, error) {
	l := leerEtiquetas(s.Datos)
	var v any
	switch s.Operacion {
	case OpClasificar:
		v = clasificarDoble(l)
	case OpGenerarPreguntas:
		v = preguntasDoble(l)
	case OpGenerarMaterial:
		v = SalidaGenerarMaterial{Titulo: "Material de estudio", Contenido: "Resumen de los pasajes dados.", PasajeIDs: l.pasajes}
	case OpGenerarExplicacion:
		v = SalidaGenerarExplicacion{Explicacion: "La alternativa marcada es la correcta según los pasajes.", PasajeIDs: l.pasajes}
	default:
		return nil, fmt.Errorf("operación desconocida del doble: %q", s.Operacion)
	}
	return json.Marshal(v)
}

// etiquetas is the structure rebuilt from the block labels.
type etiquetas struct {
	items      []int64
	opciones   [][]int64 // asignatura_id[, tema_id]
	pasajes    []int64
	parametros map[string]int64
}

func leerEtiquetas(datos []Bloque) etiquetas {
	l := etiquetas{parametros: map[string]int64{}}
	for _, b := range datos {
		partes := strings.Split(b.Etiqueta, ":")
		ids := numeros(partes[1:])
		switch partes[0] {
		case etiquetaItem:
			l.items = append(l.items, ids...)
		case etiquetaOpcion:
			l.opciones = append(l.opciones, ids)
		case etiquetaPasaje:
			l.pasajes = append(l.pasajes, ids...)
		case etiquetaParametro:
			if len(partes) == 3 {
				l.parametros[partes[1]] = ids[len(ids)-1]
			}
		}
	}
	return l
}

func numeros(partes []string) []int64 {
	var out []int64
	for _, p := range partes {
		if n, err := strconv.ParseInt(p, 10, 64); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// clasificarDoble assigns every item to the first option of the taxonomy.
func clasificarDoble(l etiquetas) SalidaClasificar {
	out := SalidaClasificar{Resultados: []Clasificacion{}}
	if len(l.opciones) == 0 {
		return out
	}
	primera := l.opciones[0]
	for _, id := range l.items {
		c := Clasificacion{ID: id, AsignaturaID: primera[0], Confianza: confianzaDoble}
		if len(primera) > 1 {
			tema := primera[1]
			c.TemaID = &tema
		}
		out.Resultados = append(out.Resultados, c)
	}
	return out
}

// preguntasDoble generates the asked number of questions, each citing the
// first passage.
func preguntasDoble(l etiquetas) SalidaGenerarPreguntas {
	out := SalidaGenerarPreguntas{Preguntas: []PreguntaGenerada{}}
	if len(l.pasajes) == 0 {
		return out
	}
	for i := range l.parametros["cantidad"] {
		out.Preguntas = append(out.Preguntas, PreguntaGenerada{
			Enunciado:    fmt.Sprintf("Pregunta %d sobre el tema", i+1),
			Alternativas: alternativasDoble,
			Correcta:     0,
			PasajeIDs:    []int64{l.pasajes[0]},
		})
	}
	return out
}
