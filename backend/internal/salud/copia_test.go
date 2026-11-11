package salud

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cbaciliod/learning/internal/db/dbtest"
)

func hace(d time.Duration) *time.Time { t := inicio.Add(-d); return &t }
func texto(s string) *string          { return &s }
func bytesCopia(n int64) *int64       { return &n }

// The PLT-05 scenarios over the evaluation, in the contract order.
func TestComprobarCopia(t *testing.T) {
	correcta, fallida := texto("correcta"), texto(resultadoFallida)
	cases := []struct {
		name          string
		m             medidasCopia
		err           error
		ctxVencido    bool
		wantEstado    string
		wantCategoria *string
	}{
		{"copia al día (6 h)", medidasCopia{HayFilas: true, UltimaCorrectaEn: hace(6 * time.Hour), TamanoBytes: bytesCopia(52428800), UltimaEjecucion: correcta, UltimaEjecucionEn: hace(6 * time.Hour)}, nil, false, EstadoOperativo, nil},
		{"exactly 26 h is still operativo", medidasCopia{HayFilas: true, UltimaCorrectaEn: hace(26 * time.Hour), UltimaEjecucion: correcta}, nil, false, EstadoOperativo, nil},
		{"última ejecución fallida, correcta hace 25 h", medidasCopia{HayFilas: true, UltimaCorrectaEn: hace(25 * time.Hour), UltimaEjecucion: fallida, UltimaEjecucionEn: hace(time.Hour)}, nil, false, EstadoDegradado, categoria(CategoriaAtrasado)},
		{"sin registro por base caída (correcta hace 30 h)", medidasCopia{HayFilas: true, UltimaCorrectaEn: hace(30 * time.Hour), UltimaEjecucion: correcta}, nil, false, EstadoDegradado, categoria(CategoriaAtrasado)},
		{"exactly 50 h is still degradado", medidasCopia{HayFilas: true, UltimaCorrectaEn: hace(50 * time.Hour), UltimaEjecucion: correcta}, nil, false, EstadoDegradado, categoria(CategoriaAtrasado)},
		{"sin copia reciente (60 h)", medidasCopia{HayFilas: true, UltimaCorrectaEn: hace(60 * time.Hour), UltimaEjecucion: correcta}, nil, false, EstadoCaido, categoria(CategoriaAtrasado)},
		{"ejecuciones sin ninguna correcta", medidasCopia{HayFilas: true, UltimaEjecucion: fallida, UltimaEjecucionEn: hace(time.Hour)}, nil, false, EstadoCaido, categoria(CategoriaAtrasado)},
		{"sin ejecuciones", medidasCopia{}, nil, false, EstadoDesconocido, categoria(CategoriaSinDatos)},
		{"query error", medidasCopia{}, errors.New("dial tcp db.internal:5432"), false, EstadoCaido, categoria(CategoriaSinConexion)},
		{"query error after the deadline", medidasCopia{}, errors.New("pq: canceling statement"), true, EstadoCaido, categoria(CategoriaTiempoAgotado)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			if c.ctxVencido {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			got := comprobarCopia(func(context.Context) (medidasCopia, error) { return c.m, c.err }, func() time.Time { return inicio }).Comprobar(ctx)
			if got.Estado != c.wantEstado || (got.Categoria == nil) != (c.wantCategoria == nil) ||
				(c.wantCategoria != nil && *got.Categoria != *c.wantCategoria) {
				t.Fatalf("got %s/%v, want %s/%v", got.Estado, got.Categoria, c.wantEstado, c.wantCategoria)
			}
			for _, k := range []string{"ultima_correcta_en", "tamano_bytes", "ultima_ejecucion", "ultima_ejecucion_en"} {
				if _, ok := got.Datos[k]; !ok {
					t.Errorf("datos without %q: %v", k, got.Datos)
				}
			}
		})
	}
}

func TestComprobarCopia_Datos(t *testing.T) {
	got := comprobarCopia(func(context.Context) (medidasCopia, error) {
		return medidasCopia{HayFilas: true, UltimaCorrectaEn: hace(6*time.Hour - 400*time.Millisecond), TamanoBytes: bytesCopia(52428800),
			UltimaEjecucion: texto("correcta"), UltimaEjecucionEn: hace(6*time.Hour - 400*time.Millisecond)}, nil
	}, func() time.Time { return inicio }).Comprobar(context.Background())
	b, _ := json.Marshal(got.Datos)
	want := `{"tamano_bytes":52428800,"ultima_correcta_en":"2026-11-11T09:04:05Z","ultima_ejecucion":"correcta","ultima_ejecucion_en":"2026-11-11T09:04:05Z"}`
	if string(b) != want {
		t.Errorf("datos %s, want %s", b, want)
	}
}

// The real query and the 064 constraints, in a rolled-back transaction.
func TestComprobarCopia_Postgres(t *testing.T) {
	db := dbtest.Open(t)
	tx, err := db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM copias_bd`); err != nil {
		t.Fatal(err)
	}
	comprobar := func() Servicio { return ComprobarCopia(tx, time.Now).Comprobar(context.Background()) }
	if got := comprobar(); got.Estado != EstadoDesconocido {
		t.Fatalf("empty table: %s, want desconocido", got.Estado)
	}
	insertar := func(resultado string, tamano any, hace string) {
		t.Helper()
		if _, err := tx.Exec(`INSERT INTO copias_bd (iniciada_en, terminada_en, resultado, tamano_bytes)
			VALUES (NOW() - $3::interval - INTERVAL '40 seconds', NOW() - $3::interval, $1, $2)`, resultado, tamano, hace); err != nil {
			t.Fatal(err)
		}
	}
	insertar("fallida", nil, "2 hours")
	if got := comprobar(); got.Estado != EstadoCaido || got.Datos["ultima_ejecucion"] != "fallida" || got.Datos["ultima_correcta_en"] != nil {
		t.Errorf("only failed runs: %+v, want caido with no correct backup", got)
	}
	insertar("correcta", 52428800, "25 hours")
	got := comprobar()
	if got.Estado != EstadoDegradado || got.Datos["tamano_bytes"] != int64(52428800) || got.Datos["ultima_ejecucion"] != "fallida" {
		t.Errorf("failed today after a correct one 25 h ago: %+v, want degradado", got)
	}
	insertar("correcta", 1024, "1 hour")
	if got := comprobar(); got.Estado != EstadoOperativo || got.Datos["tamano_bytes"] != int64(1024) || got.Datos["ultima_ejecucion"] != "correcta" {
		t.Errorf("correct 1 h ago: %+v, want operativo with its size", got)
	}
	if b, _ := json.Marshal(got); strings.Contains(string(b), "r2") || strings.Contains(string(b), "bucket") {
		t.Errorf("answer mentions R2 internals: %s", b)
	}

	// 064 constraints: size only on a correct run, end not before start.
	for _, q := range []string{
		`INSERT INTO copias_bd (iniciada_en, terminada_en, resultado, tamano_bytes) VALUES (NOW(), NOW(), 'correcta', NULL)`,
		`INSERT INTO copias_bd (iniciada_en, terminada_en, resultado, tamano_bytes) VALUES (NOW(), NOW(), 'fallida', 10)`,
		`INSERT INTO copias_bd (iniciada_en, terminada_en, resultado, tamano_bytes) VALUES (NOW(), NOW() - INTERVAL '1 second', 'fallida', NULL)`,
		`INSERT INTO copias_bd (iniciada_en, terminada_en, resultado, tamano_bytes) VALUES (NOW(), NOW(), 'parcial', NULL)`,
	} {
		if _, err := tx.Exec(`SAVEPOINT s`); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(q); err == nil {
			t.Errorf("accepted: %s", q)
		}
		if _, err := tx.Exec(`ROLLBACK TO SAVEPOINT s`); err != nil {
			t.Fatal(err)
		}
	}
}
