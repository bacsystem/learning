package examen

import (
	"context"
	"errors"
	"testing"

	"github.com/cbaciliod/learning/internal/db/dbtest"
)

func TestAcceso_Permitido(t *testing.T) {
	cases := []struct {
		name string
		in   Acceso
		want error
	}{
		{"misma institucion y nivel", Acceso{Activo: true, MismaInstitucion: true, NivelOK: true}, nil},
		{"otra institucion", Acceso{Activo: true, MismaInstitucion: false, NivelOK: true}, ErrInstitucionNoPermitida},
		{"nivel distinto", Acceso{Activo: true, MismaInstitucion: true, NivelOK: false}, ErrNivelNoPermitido},
		{"institucion gana al nivel", Acceso{Activo: true, MismaInstitucion: false, NivelOK: false}, ErrInstitucionNoPermitida},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.in.Permitido(); !errors.Is(got, c.want) {
				t.Fatalf("Permitido() = %v, want %v", got, c.want)
			}
		})
	}
}

// The institution and the nivel come from the database rows of the user, not
// from the token (ADR 0010, decision 1).
func TestAccesoProceso(t *testing.T) {
	db := dbtest.Open(t)
	a := dbtest.SeedCatalog(t, db)
	b := dbtest.SeedCatalog(t, db)
	user := dbtest.SeedUser(t, db, a, "usuario", "activo") // grado de A: suboficial
	mk := func(c dbtest.Catalog, estado, nivel string) int64 {
		id := dbtest.SeedProceso(t, db, c, 2096, estado)
		if _, err := db.Exec(`UPDATE procesos SET nivel = NULLIF($2, '') WHERE id = $1`, id, nivel); err != nil {
			t.Fatal(err)
		}
		return id
	}
	cases := []struct {
		name    string
		proceso int64
		want    Acceso
	}{
		{"misma institucion, nivel vacio", mk(a, "activo", ""), Acceso{true, true, true}},
		{"misma institucion, mismo nivel", mk(a, "activo", "suboficial"), Acceso{true, true, true}},
		{"misma institucion, otro nivel", mk(a, "activo", "oficial"), Acceso{true, true, false}},
		{"otra institucion", mk(b, "activo", ""), Acceso{true, false, true}},
		{"otra institucion y otro nivel", mk(b, "activo", "oficial"), Acceso{true, false, false}},
		{"inactivo", mk(a, "inactivo", ""), Acceso{false, true, true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := AccesoProceso(context.Background(), db, user.ID, c.proceso)
			if err != nil || got != c.want {
				t.Fatalf("AccesoProceso = %+v, %v; want %+v", got, err, c.want)
			}
		})
	}
	if _, err := AccesoProceso(context.Background(), db, user.ID, -1); !errors.Is(err, ErrProcesoNoEncontrado) {
		t.Fatalf("unknown proceso: err = %v, want ErrProcesoNoEncontrado", err)
	}
}
