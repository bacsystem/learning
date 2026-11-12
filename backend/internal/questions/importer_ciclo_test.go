package questions

import (
	"context"
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/db/dbtest"
	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

// SIA-E3-02 rule 2: imported questions are born as manual drafts, so they
// are not served until approved.
func TestImportPreguntas_NaceBorradorManual(t *testing.T) {
	db := qt.DB(t)
	f := qt.Seed(t, db, qt.Options{Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 1, Peso: 100, Unicas: 1}}})
	rows := []importRow{{
		Enunciado: "Enunciado importado desde Excel para el ciclo",
		Respuestas: []CreateRespuestaItem{
			{Texto: "Correcta", EsCorrecta: true, Orden: 1},
			{Texto: "Incorrecta", Orden: 2},
		},
	}}

	res := ImportPreguntas(context.Background(), rows, f.ProcesoID, f.Asignaturas[0], NewService(NewRepository(db)))
	if res.Imported != 1 || len(res.Errors) != 0 {
		t.Fatalf("import = %+v, want 1 imported and no errors", res)
	}

	var estado, origen string
	var servible bool
	if err := db.QueryRow(`SELECT estado, origen, `+PreguntaServible("p")+` FROM preguntas p WHERE enunciado = $1`,
		rows[0].Enunciado).Scan(&estado, &origen, &servible); err != nil {
		t.Fatal(err)
	}
	if estado != EstadoBorrador || origen != OrigenManual || servible {
		t.Errorf("estado/origen/servible = %s/%s/%v, want %s/%s/false", estado, origen, servible, EstadoBorrador, OrigenManual)
	}
}

// #181: a row that fails for a reason that is not a rule of the questions
// shows the admin a generic message, never the driver's text; the rules keep
// their own message.
func TestImportPreguntas_ErrorNoClasificadoEsGenerico(t *testing.T) {
	db := qt.DB(t)
	f := qt.Seed(t, db, qt.Options{Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 1, Peso: 100, Unicas: 1}}})
	fila := importRow{
		Enunciado: "Enunciado importado para probar el mensaje",
		Respuestas: []CreateRespuestaItem{
			{Texto: "Correcta", EsCorrecta: true, Orden: 1},
			{Texto: "Incorrecta", Orden: 2},
		},
	}

	t.Run("unclassified error", func(t *testing.T) {
		cerrada := dbtest.Closed(t)
		res := ImportPreguntas(context.Background(), []importRow{fila}, f.ProcesoID, f.Asignaturas[0], NewService(NewRepository(cerrada)))
		if res.Imported != 0 || len(res.Errors) != 1 || res.Errors[0].Row != 2 {
			t.Fatalf("import = %+v", res)
		}
		msg := res.Errors[0].Message
		if msg != mensajeInternoFila {
			t.Errorf("message = %q, want the generic one", msg)
		}
		for _, interno := range []string{"sql:", "closed", "pq:", "database", "driver"} {
			if strings.Contains(msg, interno) {
				t.Errorf("the message leaks %q: %s", interno, msg)
			}
		}
	})

	t.Run("a rule of the questions keeps its message", func(t *testing.T) {
		res := ImportPreguntas(context.Background(), []importRow{fila}, f.ProcesoID, 999999999, NewService(NewRepository(db)))
		if len(res.Errors) != 1 || res.Errors[0].Message == mensajeInternoFila || res.Errors[0].Message == "" {
			t.Errorf("errors = %+v, want the message of the rule", res.Errors)
		}
	})
}
