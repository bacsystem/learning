package questions

import (
	"context"
	"errors"
	"fmt"
	"github.com/lib/pq"
	"reflect"
	"strings"
	"testing"
	"time"

	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

// maxTiempoLote is the objective of contract E3-01/E3-03 §4.3: 1 000 questions
// with 6 alternatives in less than 5 s (the WriteTimeout is 15 s).
const maxTiempoLote = 5 * time.Second

func respuestasDePrueba(n int, correcta int) []CreateRespuestaItem {
	out := make([]CreateRespuestaItem, n)
	for i := range out {
		out[i] = CreateRespuestaItem{Texto: fmt.Sprintf("Alternativa %d", i+1), EsCorrecta: i+1 == correcta, Orden: i + 1}
	}
	return out
}

func loteDePrueba(f *qt.Fixture, n, alternativas int) []NuevaPregunta {
	lote := make([]NuevaPregunta, n)
	for i := range lote {
		lote[i] = NuevaPregunta{InstitucionID: f.InstitucionID, ProcesoID: f.ProcesoID, AsignaturaID: f.Asignaturas[0],
			EspecialidadID: &f.EspecialidadID, Enunciado: fmt.Sprintf("Enunciado %d", i+1), Tipo: "unica",
			Dificultad: "medio", Origen: OrigenBalotario, Respuestas: respuestasDePrueba(alternativas, 1+i%alternativas)}
	}
	return lote
}

func TestInsertarLote_GuardaBorradoresConSusRespuestas(t *testing.T) {
	f, _ := newMux(t, qt.Options{Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 1, Peso: 100, Unicas: 1}}})
	lote := loteDePrueba(f, 3, 4)
	lote[1].EspecialidadID = nil
	lote[2].Tipo, lote[2].Respuestas = "multiple", []CreateRespuestaItem{
		{Texto: "Uno", EsCorrecta: true, Orden: 1}, {Texto: "Dos", EsCorrecta: true, Orden: 2}, {Texto: "Tres", Orden: 3}}

	tx := f.DB.MustBeginTx(context.Background(), nil)
	ids, err := InsertarLote(context.Background(), tx, lote)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 || ids[0] >= ids[1] || ids[1] >= ids[2] {
		t.Fatalf("ids = %v, want 3 ascending", ids)
	}
	for i, id := range ids {
		var got struct {
			Enunciado, Tipo, Estado, Origen string
			Especialidad                    *int64 `db:"especialidad_id"`
			Respuestas                      int
			Correctas                       int
		}
		err := f.DB.Get(&got, `SELECT p.enunciado, p.tipo, p.estado, p.origen, p.especialidad_id,
			(SELECT COUNT(*) FROM respuestas r WHERE r.pregunta_id = p.id) AS respuestas,
			(SELECT COUNT(*) FROM respuestas r WHERE r.pregunta_id = p.id AND r.es_correcta) AS correctas
			FROM preguntas p WHERE p.id = $1`, id)
		if err != nil {
			t.Fatal(err)
		}
		if got.Enunciado != lote[i].Enunciado || got.Tipo != lote[i].Tipo || got.Estado != EstadoBorrador || got.Origen != OrigenBalotario ||
			got.Respuestas != len(lote[i].Respuestas) {
			t.Errorf("pregunta %d = %+v", i, got)
		}
		if (got.Especialidad == nil) != (lote[i].EspecialidadID == nil) {
			t.Errorf("pregunta %d: especialidad_id = %v", i, got.Especialidad)
		}
	}
	// Every answer keeps its text, its key and its order, pregunta by pregunta.
	for i, id := range ids {
		var filas []struct {
			Texto      string
			EsCorrecta bool `db:"es_correcta"`
			Orden      int
		}
		if err := f.DB.Select(&filas, `SELECT texto, es_correcta, orden FROM respuestas WHERE pregunta_id = $1 ORDER BY orden`, id); err != nil {
			t.Fatal(err)
		}
		got := make([]CreateRespuestaItem, len(filas))
		for j, fila := range filas {
			got[j] = CreateRespuestaItem{Texto: fila.Texto, EsCorrecta: fila.EsCorrecta, Orden: fila.Orden}
		}
		if !reflect.DeepEqual(got, lote[i].Respuestas) {
			t.Errorf("respuestas de la pregunta %d = %+v, want %+v", i, got, lote[i].Respuestas)
		}
	}
}

func TestInsertarLote_Vacio(t *testing.T) {
	f, _ := newMux(t, qt.Options{Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 1, Peso: 100, Unicas: 1}}})
	tx := f.DB.MustBeginTx(context.Background(), nil)
	defer tx.Rollback()
	if ids, err := InsertarLote(context.Background(), tx, nil); err != nil || len(ids) != 0 {
		t.Errorf("ids = %v, err = %v", ids, err)
	}
}

func TestInsertarLote_Rechazos(t *testing.T) {
	f, _ := newMux(t, qt.Options{Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 1, Peso: 100, Unicas: 1}}})
	tests := []struct {
		name       string
		mutar      func(*NuevaPregunta)
		validacion bool
	}{
		{"una sola respuesta", func(p *NuevaPregunta) { p.Respuestas = p.Respuestas[:1] }, true},
		{"sin clave", func(p *NuevaPregunta) {
			for i := range p.Respuestas {
				p.Respuestas[i].EsCorrecta = false
			}
		}, true},
		{"tipo invalido", func(p *NuevaPregunta) { p.Tipo = "otro" }, true},
		{"origen invalido", func(p *NuevaPregunta) { p.Origen = "otro" }, false},
		{"enunciado vacio", func(p *NuevaPregunta) { p.Enunciado = "  " }, true},
		{"respuesta sin texto", func(p *NuevaPregunta) { p.Respuestas[1].Texto = "" }, true},
		{"respuesta de 501", func(p *NuevaPregunta) { p.Respuestas[1].Texto = strings.Repeat("a", 501) }, true},
		{"orden cero", func(p *NuevaPregunta) { p.Respuestas[0].Orden = 0 }, true},
		{"orden repetido", func(p *NuevaPregunta) { p.Respuestas[1].Orden = 1 }, true},
		{"orden fuera de rango", func(p *NuevaPregunta) { p.Respuestas[1].Orden = 9 }, true},
		{"proceso inexistente", func(p *NuevaPregunta) { p.ProcesoID = -1 }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lote := loteDePrueba(f, 2, 3)
			tt.mutar(&lote[1])
			tx := f.DB.MustBeginTx(context.Background(), nil)
			defer tx.Rollback()
			_, err := InsertarLote(context.Background(), tx, lote)
			if err == nil {
				t.Fatal("want an error")
			}
			if tt.validacion != errors.Is(err, ErrValidation) {
				t.Errorf("err = %v, ErrValidation = %v", err, tt.validacion)
			}
		})
	}
}

// The lote lives in the caller's transaction: rolling it back leaves nothing.
func TestInsertarLote_RollbackNoDejaNada(t *testing.T) {
	f, _ := newMux(t, qt.Options{Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 1, Peso: 100, Unicas: 1}}})
	var antes int
	if err := f.DB.Get(&antes, `SELECT COUNT(*) FROM preguntas`); err != nil {
		t.Fatal(err)
	}
	tx := f.DB.MustBeginTx(context.Background(), nil)
	if _, err := InsertarLote(context.Background(), tx, loteDePrueba(f, 20, 4)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var despues, respuestasHuerfanas int
	if err := f.DB.Get(&despues, `SELECT COUNT(*) FROM preguntas`); err != nil || despues != antes {
		t.Errorf("preguntas %d -> %d (%v)", antes, despues, err)
	}
	if err := f.DB.Get(&respuestasHuerfanas, `SELECT COUNT(*) FROM respuestas WHERE pregunta_id NOT IN (SELECT id FROM preguntas)`); err != nil || respuestasHuerfanas != 0 {
		t.Errorf("respuestas huérfanas = %d (%v)", respuestasHuerfanas, err)
	}
}

func TestInsertarLote_MilPreguntasEnMenosDeCincoSegundos(t *testing.T) {
	f, _ := newMux(t, qt.Options{Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 1, Peso: 100, Unicas: 1}}})
	lote := loteDePrueba(f, 1000, 6)
	tx := f.DB.MustBeginTx(context.Background(), nil)
	defer tx.Rollback()

	inicio := time.Now()
	ids, err := InsertarLote(context.Background(), tx, lote)
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Since(inicio); d > maxTiempoLote {
		t.Errorf("1 000 preguntas tardaron %v, máximo %v", d, maxTiempoLote)
	}
	var respuestas int
	if err := tx.Get(&respuestas, `SELECT COUNT(*) FROM respuestas WHERE pregunta_id = ANY($1)`, pq.Array(ids)); err != nil || respuestas != 6000 {
		t.Errorf("respuestas = %d (%v), want 6000", respuestas, err)
	}
}
