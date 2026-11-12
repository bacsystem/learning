package questions

import (
	"context"
	"testing"

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
