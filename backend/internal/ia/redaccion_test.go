package ia

import "testing"

// Regla 3 and contract §4: what is rejected as personal data and what is not.
func TestTipoDatoPersonal(t *testing.T) {
	cases := []struct {
		texto string
		tipo  string // "" = accepted
	}{
		{"El DNI 45678912 del paciente", TipoDNI},
		{"45678912", TipoDNI},
		{"(45678912)", TipoDNI},
		{"Fecha 20261111", TipoDNI}, // accepted false positive (contract §4)
		{"123456789 tiene nueve dígitos", ""},
		{"1234567 tiene siete", ""},
		{"Código A12345678", TipoDNI},
		{"DNI12345678", TipoDNI},
		{"Código 12345678B", TipoDNI},
		{"Ref 12345678901 987654321", TipoTelefono},
		{"987654321 987654322", TipoTelefono},
		{"1987654321", ""},
		{"9876543210", ""},
		{"+51987654321", TipoTelefono},
		{"51987654321", TipoTelefono},
		{"987-654-321", TipoTelefono},
		{"fecha 202611110", ""},
		{"Año 2026 y dosis de 0,5 ml", ""},
		{"Escribe a persona@example.com hoy", TipoEmail},
		{"nombre.apellido+x@minsa.gob.pe", TipoEmail},
		{"arroba suelta @ sin dominio", ""},
		{"Llamar al 987654321", TipoTelefono},
		{"Llamar al 987 654 321", TipoTelefono},
		{"Llamar al +51 987-654-321", TipoTelefono},
		{"Central (01) 3156600", TipoTelefono},
		{"Central (044) 123456", TipoTelefono},
		{"Tabla 9: 87 casos", ""},
		{"La respuesta correcta es la alternativa B", ""},
	}
	for _, c := range cases {
		t.Run(c.texto, func(t *testing.T) {
			tipo, ok := tipoDatoPersonal(c.texto)
			if ok != (c.tipo != "") || tipo != c.tipo {
				t.Errorf("got %q (%v), want %q", tipo, ok, c.tipo)
			}
		})
	}
}

// Enmascarar shares the detection: both numbers of "987654321 987654322" go,
// the overlapping candidate does not hide the real one, and it is idempotent.
func TestEnmascarar(t *testing.T) {
	cases := []struct{ in, want string }{
		{"987654321 987654322", "[telefono] [telefono]"},
		{"Ref 12345678901 987654321", "Ref 12345678901 [telefono]"},
		{"DNI12345678 y a@b.pe", "DNI[dni] y [email]"},
		{"12345678@minsa.gob.pe", "[email]"},
		{"+51 987 654 321", "[telefono]"},
		{"sin datos 123456789", "sin datos 123456789"},
		{"ñandú 45678912 ñ", "ñandú [dni] ñ"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := Enmascarar(c.in)
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
			if again := Enmascarar(got); again != got {
				t.Errorf("not idempotent: %q -> %q", got, again)
			}
			if _, ok := tipoDatoPersonal(got); ok {
				t.Errorf("masked text still detected: %q", got)
			}
		})
	}
}

// The instructions are checked too, and reported as BloqueInstrucciones.
func TestBuscarDatosPersonales_Bloques(t *testing.T) {
	cases := []struct {
		name       string
		s          Solicitud
		wantBloque int
		wantNada   bool
	}{
		{"clean", Solicitud{Instrucciones: "Clasifica", Datos: []Bloque{{Texto: "Vacunas"}}}, 0, true},
		{"in the instructions", Solicitud{Instrucciones: "Avisa a x@y.pe", Datos: []Bloque{{Texto: "ok"}}}, BloqueInstrucciones, false},
		{"in the third block", Solicitud{Datos: []Bloque{{Texto: "a"}, {Texto: "b"}, {Texto: "DNI 45678912"}}}, 2, false},
		{"labels are not checked", Solicitud{Datos: []Bloque{{Etiqueta: "item:45678912", Texto: "ok"}}}, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := buscarDatosPersonales(c.s)
			if c.wantNada {
				if err != nil {
					t.Fatalf("got %v, want nothing", err)
				}
				return
			}
			dp, ok := err.(*ErrorDatosPersonales)
			if !ok || dp.Bloque != c.wantBloque {
				t.Errorf("got %v, want block %d", err, c.wantBloque)
			}
		})
	}
}

func TestRunesDeDatos(t *testing.T) {
	s := Solicitud{Instrucciones: "no cuenta", Datos: []Bloque{{Etiqueta: "no:cuenta", Texto: "ñandú"}, {Texto: "ab"}}}
	if n := runesDeDatos(s); n != 7 {
		t.Errorf("runes %d, want 7 (only the data texts, in runes)", n)
	}
}

// Output validation edges (regla 1): zero is a valid index and confidence;
// out-of-range alternatives and foreign citations are invalid.
func TestSalidas_Validar(t *testing.T) {
	pasajes := []Pasaje{{ID: 11}}
	cases := []struct {
		name string
		err  error
	}{
		{"correcta 0 is valid", SalidaGenerarPreguntas{Preguntas: []PreguntaGenerada{{Enunciado: "x", Alternativas: []string{"a", "b"}, Correcta: 0, PasajeIDs: []int64{11}}}}.validar(EntradaGenerarPreguntas{Cantidad: 1, Pasajes: pasajes})},
		{"correcta out of range", invertir(SalidaGenerarPreguntas{Preguntas: []PreguntaGenerada{{Enunciado: "x", Alternativas: []string{"a", "b"}, Correcta: 2, PasajeIDs: []int64{11}}}}.validar(EntradaGenerarPreguntas{Cantidad: 1, Pasajes: pasajes}))},
		{"more questions than asked", invertir(SalidaGenerarPreguntas{Preguntas: make([]PreguntaGenerada, 2)}.validar(EntradaGenerarPreguntas{Cantidad: 1}))},
		{"foreign citation", invertir(SalidaGenerarMaterial{PasajeIDs: []int64{99}}.validar(EntradaGenerarMaterial{Pasajes: pasajes}))},
		{"citation of a given passage", SalidaGenerarExplicacion{PasajeIDs: []int64{11}}.validar(EntradaGenerarExplicacion{Pasajes: pasajes})},
		{"repeated item", invertir(SalidaClasificar{Resultados: []Clasificacion{{ID: 1}, {ID: 1}}}.validar(EntradaClasificar{Items: []ItemTexto{{ID: 1}, {ID: 2}}}))},
	}
	for _, c := range cases {
		if c.err != nil {
			t.Errorf("%s: %v", c.name, c.err)
		}
	}
}

// invertir turns an expected validation error into nil, and its absence into
// an error, so every row of TestSalidas_Validar reads «want nil».
func invertir(err error) error {
	if err == nil {
		return errSalida
	}
	return nil
}
