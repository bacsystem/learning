package inscripciones

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cbaciliod/learning/internal/db/dbtest"
)

// POST /inscripciones answers the whole inscripción: the proceso data too, for
// a new one and for a re-application after a rejection.
func TestInscribir_RespuestaTraeElProceso(t *testing.T) {
	db, h, _ := setup(t)
	setConfigPago(t, db, testAnio, "10")
	cat := dbtest.SeedCatalog(t, db)
	admin := dbtest.SeedUser(t, db, cat, "admin", "activo")
	user := dbtest.SeedUser(t, db, cat, "usuario", "activo")
	proc := dbtest.SeedProceso(t, db, cat, testAnio, "activo")
	var nombre string
	if err := db.QueryRow(`SELECT nombre FROM procesos WHERE id=$1`, proc).Scan(&nombre); err != nil {
		t.Fatal(err)
	}

	check := func(label string) InscripcionDTO {
		rr := inscribir(t, h, user.ID, proc)
		if rr.Code != http.StatusCreated {
			t.Fatalf("%s: want 201, got %d %s", label, rr.Code, rr.Body)
		}
		var got InscripcionDTO
		if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.ProcesoID != proc || got.ProcesoNombre != nombre || got.ProcesoTipo != "ascenso" || got.ProcesoAnio != testAnio || got.Estado != "pendiente" {
			t.Fatalf("%s: response = %+v", label, got)
		}
		return got
	}
	first := check("new")

	if rr := patchEstado(t, h, admin.ID, first.ID, "rechazado"); rr.Code != http.StatusOK {
		t.Fatalf("reject: %d %s", rr.Code, rr.Body)
	}
	again := check("re-application")
	if again.ID != first.ID || again.Observacion != nil {
		t.Fatalf("re-application = %+v, want the same id and no observación", again)
	}
}
