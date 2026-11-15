package questions

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	qt "github.com/cbaciliod/learning/internal/questions/questionstest"
)

func etiquetaFixture(t *testing.T) (f *qt.Fixture, mux http.Handler, admin string) {
	t.Helper()
	f, m := newMux(t, qt.Options{Asignaturas: []qt.AsignaturaSpec{{NumeroPreguntas: 1, Peso: 100, Unicas: 1}}})
	t.Cleanup(func() {
		if _, err := f.DB.Exec(`DELETE FROM etiquetas_pregunta WHERE asignatura_id=$1`, f.Asignaturas[0]); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	return f, m, f.Token(t, f.UsuarioID, "admin")
}

func postEtiqueta(mux http.Handler, admin string, asignaturaID int64, nombre string) (int, EtiquetaResponse) {
	w := qt.Do(mux, "POST", "/etiquetas", admin, fmt.Sprintf(`{"asignatura_id":%d,"nombre":%q}`, asignaturaID, nombre))
	var et EtiquetaResponse
	_ = json.Unmarshal(w.Body.Bytes(), &et)
	return w.Code, et
}

func TestCreateEtiqueta_NombreSeGuardaRecortado(t *testing.T) {
	f, mux, admin := etiquetaFixture(t)
	code, et := postEtiqueta(mux, admin, f.Asignaturas[0], "  algebra \t")
	if code != http.StatusCreated {
		t.Fatalf("status %d", code)
	}
	if et.Nombre != "algebra" {
		t.Errorf("nombre = %q, want %q", et.Nombre, "algebra")
	}
}

func TestCreateEtiqueta_NombreEnBlancoSeRechaza(t *testing.T) {
	f, mux, admin := etiquetaFixture(t)
	for _, nombre := range []string{"   ", "\t\n", " "} {
		t.Run(fmt.Sprintf("%q", nombre), func(t *testing.T) {
			if code, _ := postEtiqueta(mux, admin, f.Asignaturas[0], nombre); code != http.StatusBadRequest {
				t.Errorf("status %d, want 400", code)
			}
		})
	}
}

func TestCreateEtiqueta_DuplicadoIgnoraMayusculasYEspacios(t *testing.T) {
	f, mux, admin := etiquetaFixture(t)
	if code, _ := postEtiqueta(mux, admin, f.Asignaturas[0], "zz"); code != http.StatusCreated {
		t.Fatalf("first create: %d", code)
	}
	for _, nombre := range []string{"zz", "ZZ", " zz ", "Zz\t"} {
		t.Run(fmt.Sprintf("%q", nombre), func(t *testing.T) {
			if code, _ := postEtiqueta(mux, admin, f.Asignaturas[0], nombre); code != http.StatusConflict {
				t.Errorf("status %d, want 409", code)
			}
		})
	}
}

func TestUpdateEtiqueta_NombreRecortadoYSinDuplicados(t *testing.T) {
	f, mux, admin := etiquetaFixture(t)
	_, a := postEtiqueta(mux, admin, f.Asignaturas[0], "uno")
	_, b := postEtiqueta(mux, admin, f.Asignaturas[0], "dos")
	put := func(id int64, nombre string) (int, EtiquetaResponse) {
		w := qt.Do(mux, "PUT", fmt.Sprintf("/etiquetas/%d", id), admin, fmt.Sprintf(`{"nombre":%q}`, nombre))
		var et EtiquetaResponse
		_ = json.Unmarshal(w.Body.Bytes(), &et)
		return w.Code, et
	}
	if code, _ := put(b.ID, "   "); code != http.StatusBadRequest {
		t.Errorf("blank: status %d, want 400", code)
	}
	if code, _ := put(b.ID, " UNO "); code != http.StatusConflict {
		t.Errorf("duplicate: status %d, want 409", code)
	}
	if code, et := put(a.ID, "  tres "); code != http.StatusOK || et.Nombre != "tres" {
		t.Errorf("trim: status %d nombre %q", code, et.Nombre)
	}
}

func TestCreateEtiqueta_AsignaturaInexistenteNoEs500(t *testing.T) {
	_, mux, admin := etiquetaFixture(t)
	code, _ := postEtiqueta(mux, admin, 999999999, "huerfana")
	if code != http.StatusBadRequest {
		t.Errorf("status %d, want 400", code)
	}
}
