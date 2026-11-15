package questions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"

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

// Without a unique index the duplicate check is serialized by an advisory
// lock. While one transaction holds the lock for a name, a second one for the
// same name in another spelling waits; once the first commits its tag, the
// second sees it and answers ErrConflict.
func TestRechazarEtiquetaDuplicada_SerializaPorNombre(t *testing.T) {
	f, _, _ := etiquetaFixture(t)
	ctx := context.Background()
	asignatura := f.Asignaturas[0]

	tx1, err := f.DB.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx1.Rollback()
	if err := rechazarEtiquetaDuplicada(ctx, tx1, asignatura, "Carrera", 0); err != nil {
		t.Fatalf("first check: %v", err)
	}

	second := make(chan error, 1)
	go func() {
		tx2, err := f.DB.BeginTxx(ctx, nil)
		if err != nil {
			second <- err
			return
		}
		defer tx2.Rollback()
		second <- rechazarEtiquetaDuplicada(ctx, tx2, asignatura, " CARRERA ", 0)
	}()

	select {
	case err := <-second:
		t.Fatalf("second check did not wait for the lock (returned %v)", err)
	case <-time.After(300 * time.Millisecond):
	}

	if _, err := tx1.ExecContext(ctx, `INSERT INTO etiquetas_pregunta (asignatura_id, nombre) VALUES ($1, 'Carrera')`, asignatura); err != nil {
		t.Fatal(err)
	}
	if err := tx1.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-second:
		if !errors.Is(err, ErrConflict) {
			t.Fatalf("second check = %v, want ErrConflict", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("second check never returned after the first committed")
	}
}

// Rows saved before the validation may hold padded or upper-case names; they
// still count as duplicates.
func TestCreateEtiqueta_DuplicadoContraNombreHeredado(t *testing.T) {
	f, mux, admin := etiquetaFixture(t)
	if _, err := f.DB.Exec(`INSERT INTO etiquetas_pregunta (asignatura_id, nombre) VALUES ($1, '  Legado ')`, f.Asignaturas[0]); err != nil {
		t.Fatal(err)
	}
	if code, _ := postEtiqueta(mux, admin, f.Asignaturas[0], "legado"); code != http.StatusConflict {
		t.Errorf("status %d, want 409", code)
	}
}

// The legacy name may carry a tab or a no-break space: SQL btrim alone would
// not trim them, so the comparison passes the same set Go trims.
func TestCreateEtiqueta_DuplicadoContraNombreConTabulador(t *testing.T) {
	f, mux, admin := etiquetaFixture(t)
	for _, legado := range []string{"Legado\t", "\u00a0Legado\u00a0"} {
		if _, err := f.DB.Exec(`INSERT INTO etiquetas_pregunta (asignatura_id, nombre) VALUES ($1, $2)`, f.Asignaturas[0], legado); err != nil {
			t.Fatal(err)
		}
		if code, _ := postEtiqueta(mux, admin, f.Asignaturas[0], "legado"); code != http.StatusConflict {
			t.Errorf("legacy %q: status %d, want 409", legado, code)
		}
		if _, err := f.DB.Exec(`DELETE FROM etiquetas_pregunta WHERE asignatura_id=$1`, f.Asignaturas[0]); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCreateEtiqueta_MensajeDeDuplicadoEsElPropio(t *testing.T) {
	f, mux, admin := etiquetaFixture(t)
	postEtiqueta(mux, admin, f.Asignaturas[0], "zz")
	w := qt.Do(mux, "POST", "/etiquetas", admin, fmt.Sprintf(`{"asignatura_id":%d,"nombre":"ZZ"}`, f.Asignaturas[0]))
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), msgEtiquetaDuplicada) {
		t.Errorf("status %d body %s, want 409 with %q", w.Code, w.Body, msgEtiquetaDuplicada)
	}
}

// The edit form always resends the name. Existing duplicates (saved before the
// check) must not block changing the estado of either of them.
func TestUpdateEtiqueta_SinCambiarNombreNoChocaConDuplicadosExistentes(t *testing.T) {
	f, mux, admin := etiquetaFixture(t)
	var ids []int64
	for _, nombre := range []string{"zz", "ZZ"} {
		var id int64
		if err := f.DB.QueryRow(`INSERT INTO etiquetas_pregunta (asignatura_id, nombre) VALUES ($1, $2) RETURNING id`,
			f.Asignaturas[0], nombre).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	for i, nombre := range []string{"zz", "ZZ"} {
		w := qt.Do(mux, "PUT", fmt.Sprintf("/etiquetas/%d", ids[i]), admin, fmt.Sprintf(`{"nombre":%q,"estado":"inactivo"}`, nombre))
		if w.Code != http.StatusOK {
			t.Errorf("tag %q: status %d body %s, want 200", nombre, w.Code, w.Body)
		}
	}
}

func TestUpdateEtiqueta_NombreEnBlancoConEstadoNoCambiaNada(t *testing.T) {
	f, mux, admin := etiquetaFixture(t)
	_, et := postEtiqueta(mux, admin, f.Asignaturas[0], "intacta")
	w := qt.Do(mux, "PUT", fmt.Sprintf("/etiquetas/%d", et.ID), admin, `{"nombre":"   ","estado":"inactivo"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", w.Code)
	}
	var estado string
	if err := f.DB.QueryRow(`SELECT estado FROM etiquetas_pregunta WHERE id=$1`, et.ID).Scan(&estado); err != nil {
		t.Fatal(err)
	}
	if estado != "activo" {
		t.Errorf("estado = %q, the rejected request must not change it", estado)
	}
}

// Concurrent creates through the repository (spellings of one name) leave
// exactly one tag. The race window is microseconds, so this test cannot fail
// reliably without the lock: it only guards against regressions that make
// concurrent creates error out. The locks themselves are proven by the
// deterministic tests TestRechazarEtiquetaDuplicada_SerializaPorNombre and
// TestUpdateEtiqueta_EsperaALaFilaBloqueada.
func TestRepositoryCreateEtiqueta_ConcurrenteCreaUna(t *testing.T) {
	f, _, _ := etiquetaFixture(t)
	repo := NewRepository(f.DB)
	base := []string{"carrera", "CARRERA", " Carrera ", "caRRera", "carrera\t", "CarRera", "  carrera", "CARRERA  "}
	var variantes []string
	for range 4 {
		variantes = append(variantes, base...)
	}
	errs := make([]error, len(variantes))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, nombre := range variantes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			nombre, _ := normalizarNombreEtiqueta(nombre)
			_, errs[i] = repo.CreateEtiqueta(context.Background(), CreateEtiquetaRequest{AsignaturaID: f.Asignaturas[0], Nombre: nombre})
		}()
	}
	close(start)
	wg.Wait()
	created := 0
	for _, err := range errs {
		switch {
		case err == nil:
			created++
		case !errors.Is(err, ErrConflict):
			t.Errorf("unexpected error: %v", err)
		}
	}
	if created != 1 {
		t.Fatalf("created %d tags, want 1", created)
	}
}

// recorteNombre must trim exactly what strings.TrimSpace trims, or a name
// stored trimmed by Go would not match a legacy row trimmed by SQL.
func TestRecorteNombre_EsElConjuntoDeUnicodeIsSpace(t *testing.T) {
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if unicode.IsSpace(r) != strings.ContainsRune(recorteNombre, r) {
			t.Fatalf("U+%04X: IsSpace=%v but in recorteNombre=%v", r, unicode.IsSpace(r), strings.ContainsRune(recorteNombre, r))
		}
	}
}

// Legacy rows "zz " and "ZZ" are duplicates; resending the stored name (the
// form does) must still work.
func TestUpdateEtiqueta_ReenviarNombreHeredadoConEspacioFinal(t *testing.T) {
	f, mux, admin := etiquetaFixture(t)
	var id int64
	if err := f.DB.QueryRow(`INSERT INTO etiquetas_pregunta (asignatura_id, nombre) VALUES ($1, 'zz ') RETURNING id`, f.Asignaturas[0]).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := f.DB.Exec(`INSERT INTO etiquetas_pregunta (asignatura_id, nombre) VALUES ($1, 'ZZ')`, f.Asignaturas[0]); err != nil {
		t.Fatal(err)
	}
	w := qt.Do(mux, "PUT", fmt.Sprintf("/etiquetas/%d", id), admin, `{"nombre":"zz ","estado":"inactivo"}`)
	if w.Code != http.StatusOK {
		t.Errorf("status %d body %s, want 200", w.Code, w.Body)
	}
}

// UpdateEtiqueta reads the row FOR UPDATE before the duplicate check, so while
// another transaction holds the row it waits at the read: it has not taken the
// advisory lock yet. Without FOR UPDATE it would pass the check holding that
// lock and only then block on its UPDATE (the final UPDATE waits for the row
// either way, which is why the lock is what this test inspects).
func TestUpdateEtiqueta_EsperaALaFilaBloqueada(t *testing.T) {
	f, mux, admin := etiquetaFixture(t)
	_, et := postEtiqueta(mux, admin, f.Asignaturas[0], "foo")
	ctx := context.Background()

	tx, err := f.DB.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT id FROM etiquetas_pregunta WHERE id = $1 FOR UPDATE`, et.ID); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := NewRepository(f.DB).UpdateEtiqueta(ctx, et.ID, UpdateEtiquetaRequest{Nombre: "bar"})
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("UpdateEtiqueta did not wait for the locked row (returned %v)", err)
	case <-time.After(300 * time.Millisecond):
	}
	probe, err := f.DB.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var libre bool
	if err := probe.QueryRowContext(ctx, `
		SELECT pg_try_advisory_xact_lock(hashtextextended($1::text || ':' || 'bar', 0))`, f.Asignaturas[0]).Scan(&libre); err != nil {
		t.Fatal(err)
	}
	_ = probe.Rollback()
	if !libre {
		t.Fatal("UpdateEtiqueta took the name lock before reading the row FOR UPDATE")
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("UpdateEtiqueta after release: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("UpdateEtiqueta never returned after the row was released")
	}
}
