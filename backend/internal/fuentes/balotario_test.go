package fuentes_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/cbaciliod/learning/internal/fuentes"
	"github.com/cbaciliod/learning/internal/fuentes/fuentestest"
)

// publicarBalotario stores and publishes a balotario with the given content
// and one passage per range of lines.
func publicarBalotario(t *testing.T, e fuentestest.Entorno, contenido string, especialidades []int64, rangos [][2]int) (int64, int) {
	t.Helper()
	ctx := context.Background()
	id, v, err := e.Repo.GuardarFuente(ctx, fuentes.NuevaFuente{Titulo: "Balotario " + dbtest.Uniq(), Tipo: "balotario",
		EspecialidadIDs: especialidades, NombreArchivo: "b.md", Contenido: []byte(contenido)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ps := make([]fuentes.PasajeNuevo, len(rangos))
	for i, r := range rangos {
		ps[i] = fuentes.PasajeNuevo{Orden: i + 1, LineaInicio: r[0], LineaFin: r[1], Texto: fmt.Sprintf("pasaje %d", i+1), Palabras: 2}
	}
	if err := e.Repo.PublicarVersion(ctx, id, v, ps, nil); err != nil {
		t.Fatal(err)
	}
	return id, v
}

func TestVersionVigenteBalotario(t *testing.T) {
	e, db := entornoPg(t)
	ctx := context.Background()
	contenido := "1. Pregunta\nA) Uno\n<mark>B) Dos</mark>\n"
	id, v := publicarBalotario(t, e, contenido, []int64{e.EspecialidadA}, [][2]int{{1, 3}})

	t.Run("fuente activa", func(t *testing.T) {
		got, err := fuentes.VersionVigenteBalotario(ctx, db, id)
		if err != nil {
			t.Fatal(err)
		}
		if got.FuenteID != id || got.Tipo != "balotario" || got.Estado != "activa" || got.Numero != v || got.VersionID == 0 ||
			got.Contenido != contenido || got.OficialVerificada || got.OficialEntidad != nil ||
			fmt.Sprint(got.EspecialidadIDs) != fmt.Sprint([]int64{e.EspecialidadA}) {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("marca de oficial verificada", func(t *testing.T) {
		if _, err := db.Exec(`UPDATE fuente_versiones SET oficial_verificada = true, oficial_entidad = 'MINSA',
			oficial_nota = 'ok', oficial_at = NOW() WHERE fuente_id = $1`, id); err != nil {
			t.Fatal(err)
		}
		got, err := fuentes.VersionVigenteBalotario(ctx, db, id)
		if err != nil || !got.OficialVerificada || got.OficialEntidad == nil || *got.OficialEntidad != "MINSA" {
			t.Errorf("got %+v, %v", got, err)
		}
	})

	t.Run("sin especialidades aplica a todas", func(t *testing.T) {
		libre, _ := publicarBalotario(t, e, "x "+dbtest.Uniq(), nil, [][2]int{{1, 1}})
		got, err := fuentes.VersionVigenteBalotario(ctx, db, libre)
		if err != nil || got.EspecialidadIDs == nil || len(got.EspecialidadIDs) != 0 {
			t.Errorf("got %+v, %v", got, err)
		}
	})

	t.Run("fuente aun procesando: sin version vigente", func(t *testing.T) {
		pid, _, err := e.Repo.GuardarFuente(ctx, fuentes.NuevaFuente{Titulo: "Procesando " + dbtest.Uniq(), Tipo: "balotario",
			NombreArchivo: "p.md", Contenido: []byte("p " + dbtest.Uniq())}, nil)
		if err != nil {
			t.Fatal(err)
		}
		got, err := fuentes.VersionVigenteBalotario(ctx, db, pid)
		if err != nil || got.Estado != "procesando" || got.VersionID != 0 || got.Numero != 0 || got.Contenido != "" {
			t.Errorf("got %+v, %v", got, err)
		}
	})

	t.Run("fuente inexistente", func(t *testing.T) {
		if _, err := fuentes.VersionVigenteBalotario(ctx, db, -1); !errors.Is(err, fuentes.ErrNoEncontrado) {
			t.Errorf("err = %v, want ErrNoEncontrado", err)
		}
	})

	t.Run("una version nueva sin publicar no cambia la vigente", func(t *testing.T) {
		otra, vOtra := publicarBalotario(t, e, "primera "+dbtest.Uniq(), nil, [][2]int{{1, 1}})
		e.NuevaVersion(t, otra, []byte("segunda en proceso "+dbtest.Uniq()))
		got, err := fuentes.VersionVigenteBalotario(ctx, db, otra)
		if err != nil || got.Numero != vOtra || got.Estado != "activa" || !strings.HasPrefix(got.Contenido, "primera ") {
			t.Errorf("got %+v, %v; want la versión %d", got, err, vOtra)
		}
	})

	t.Run("no filtra por tipo ni por estado", func(t *testing.T) {
		guia, _, err := e.Repo.GuardarFuente(ctx, fuentes.NuevaFuente{Titulo: "Guía " + dbtest.Uniq(), Tipo: "guia",
			NombreArchivo: "g.md", Contenido: []byte("g " + dbtest.Uniq())}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := fuentes.VersionVigenteBalotario(ctx, db, guia); err != nil || got.Tipo != "guia" {
			t.Errorf("guía: %+v, %v", got, err)
		}
		e.Retirar(t, id)
		if got, err := fuentes.VersionVigenteBalotario(ctx, db, id); err != nil || got.Estado != "retirada" || got.VersionID == 0 {
			t.Errorf("retirada: %+v, %v", got, err)
		}
	})

	t.Run("la vigente es la ultima publicada", func(t *testing.T) {
		nuevo := e.NuevaVersion(t, id, []byte("segunda "+dbtest.Uniq()))
		if err := e.Repo.PublicarVersion(ctx, id, nuevo, []fuentes.PasajeNuevo{{Orden: 1, LineaInicio: 1, LineaFin: 1, Texto: "n", Palabras: 1}}, nil); err != nil {
			t.Fatal(err)
		}
		got, err := fuentes.VersionVigenteBalotario(ctx, db, id)
		if err != nil || got.Numero != nuevo || got.Contenido == contenido {
			t.Errorf("got %+v, %v", got, err)
		}
	})
}

func TestRangosPasajes(t *testing.T) {
	e, db := entornoPg(t)
	ctx := context.Background()
	id, _ := publicarBalotario(t, e, "x "+dbtest.Uniq(), nil, [][2]int{{1, 4}, {5, 9}, {10, 10}})
	vig, err := fuentes.VersionVigenteBalotario(ctx, db, id)
	if err != nil {
		t.Fatal(err)
	}

	got, err := fuentes.RangosPasajes(ctx, db, vig.VersionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].LineaInicio != 1 || got[0].LineaFin != 4 || got[1].LineaInicio != 5 || got[2].LineaFin != 10 ||
		got[0].PasajeID == 0 || got[0].PasajeID >= got[1].PasajeID {
		t.Errorf("rangos = %+v", got)
	}
	if vacio, err := fuentes.RangosPasajes(ctx, db, -1); err != nil || vacio == nil || len(vacio) != 0 {
		t.Errorf("version inexistente: %v, %v", vacio, err)
	}
}
