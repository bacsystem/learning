package salud

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/cbaciliod/learning/internal/db/dbtest"
	"github.com/cbaciliod/learning/internal/notifier"
)

func minutos(n int64) *int64 { return &n }

// The PLT-03 scenarios over the evaluation, in the contract order.
func TestComprobarCorreo(t *testing.T) {
	enviado := time.Date(2026, 11, 11, 14, 50, 0, 123, time.UTC)
	errBD := errors.New(`dial tcp smtp.internal:587 user=no-reply@example.com`)
	cases := []struct {
		name          string
		m             medidasCorreo
		err           error
		ctxVencido    bool
		smtp          bool
		wantEstado    string
		wantCategoria *string
	}{
		{"correo al día", medidasCorreo{SinEnviar: 2, AntiguedadMaxMin: minutos(1), UltimoEnviadoEn: &enviado, Enviados30m: 4}, nil, false, true, EstadoOperativo, nil},
		{"atrasado: oldest unsent over 10 min", medidasCorreo{SinEnviar: 1, AntiguedadMaxMin: minutos(12)}, nil, false, true, EstadoDegradado, categoria(CategoriaAtrasado)},
		{"exactly 10 min is still operativo", medidasCorreo{SinEnviar: 1, AntiguedadMaxMin: minutos(10)}, nil, false, true, EstadoOperativo, nil},
		{"SMTP caído: 3 final failures in 30 min, none sent", medidasCorreo{Fallidos24h: 3, Fallidos30m: 3}, nil, false, true, EstadoCaido, categoria(CategoriaSinConexion)},
		{"3 failures in 30 min but something sent: degradado", medidasCorreo{Fallidos24h: 3, Fallidos30m: 3, Enviados30m: 1}, nil, false, true, EstadoDegradado, categoria(CategoriaAtrasado)},
		{"fallos definitivos repetidos in 24 h", medidasCorreo{Fallidos24h: 3, Enviados30m: 2, UltimoEnviadoEn: &enviado}, nil, false, true, EstadoDegradado, categoria(CategoriaAtrasado)},
		{"fallo aislado", medidasCorreo{Fallidos24h: 1}, nil, false, true, EstadoOperativo, nil},
		{"no SMTP in development", medidasCorreo{}, nil, false, false, EstadoNoConfigurado, nil},
		{"no SMTP and a 2 h queue: still no_configurado", medidasCorreo{SinEnviar: 5, AntiguedadMaxMin: minutos(120), Fallidos30m: 5}, nil, false, false, EstadoNoConfigurado, nil},
		{"no SMTP and a query error: no_configurado wins", medidasCorreo{}, errBD, false, false, EstadoNoConfigurado, nil},
		{"query error", medidasCorreo{}, errBD, false, true, EstadoCaido, categoria(CategoriaSinConexion)},
		{"query error after the deadline", medidasCorreo{}, errBD, true, true, EstadoCaido, categoria(CategoriaTiempoAgotado)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			if c.ctxVencido {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			got := comprobarCorreo(func(context.Context) (medidasCorreo, error) { return c.m, c.err }, c.smtp).Comprobar(ctx)
			if got.Estado != c.wantEstado || (got.Categoria == nil) != (c.wantCategoria == nil) ||
				(c.wantCategoria != nil && *got.Categoria != *c.wantCategoria) {
				t.Fatalf("got %s/%v, want %s/%v", got.Estado, got.Categoria, c.wantEstado, c.wantCategoria)
			}
			for _, k := range []string{"sin_enviar", "antiguedad_max_min", "fallidos_24h", "ultimo_enviado_en"} {
				if _, ok := got.Datos[k]; !ok {
					t.Errorf("datos without %q: %v", k, got.Datos)
				}
			}
		})
	}
}

func TestComprobarCorreo_Datos(t *testing.T) {
	enviado := time.Date(2026, 11, 11, 14, 50, 0, 999, time.FixedZone("Lima", -5*3600))
	got := comprobarCorreo(func(context.Context) (medidasCorreo, error) {
		return medidasCorreo{SinEnviar: 2, AntiguedadMaxMin: minutos(1), Fallidos24h: 1, UltimoEnviadoEn: &enviado}, nil
	}, true).Comprobar(context.Background())
	b, err := json.Marshal(got.Datos)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"antiguedad_max_min":1,"fallidos_24h":1,"sin_enviar":2,"ultimo_enviado_en":"2026-11-11T19:50:00Z"}`; string(b) != want {
		t.Errorf("datos %s, want %s", b, want)
	}
	vacio := comprobarCorreo(func(context.Context) (medidasCorreo, error) { return medidasCorreo{}, nil }, true).Comprobar(context.Background())
	if vacio.Datos["antiguedad_max_min"] != nil || vacio.Datos["ultimo_enviado_en"] != nil {
		t.Errorf("an empty queue: %v, want antiguedad and ultimo_enviado_en null", vacio.Datos)
	}
}

// The real query over Postgres, isolated in a transaction that is rolled
// back: definitions of unsent / final failure / sent, and no personal data.
func TestComprobarCorreo_Postgres(t *testing.T) {
	db := dbtest.Open(t)
	tx, err := db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM email_queue`); err != nil {
		t.Fatal(err)
	}
	insertar := func(status string, retry int, creado, actualizado string) {
		t.Helper()
		if _, err := tx.Exec(`INSERT INTO email_queue (to_address, subject, body_html, status, retry_count, last_error, created_at, updated_at)
			VALUES ('persona@example.com', 'Verifica tu cuenta', '<p>secreto</p>', $1, $2, 'smtp.internal: 550 mailbox', NOW() - $3::interval, NOW() - $4::interval)`,
			status, retry, creado, actualizado); err != nil {
			t.Fatal(err)
		}
	}
	max := notifier.MaxRetries
	insertar("pending", 0, "12 minutes", "12 minutes") // unsent, the oldest
	insertar("failed", max-1, "3 minutes", "1 minute") // unsent: retries left
	insertar("sending", 1, "1 minute", "1 minute")     // unsent
	insertar("failed", max, "5 hours", "3 hours")      // final failure in 24 h, not in 30 min
	insertar("failed", max, "2 days", "2 days")        // final failure, older than 24 h
	insertar("sent", 0, "40 minutes", "35 minutes")    // the last sent, outside 30 min

	got := ComprobarCorreo(tx, max, true).Comprobar(context.Background())
	if got.Estado != EstadoDegradado || got.Categoria == nil || *got.Categoria != CategoriaAtrasado {
		t.Errorf("got %s/%v, want degradado/atrasado", got.Estado, got.Categoria)
	}
	if got.Datos["sin_enviar"] != int64(3) || got.Datos["antiguedad_max_min"] != int64(12) || got.Datos["fallidos_24h"] != int64(1) {
		t.Errorf("datos %v, want 3 unsent, 12 min, 1 failure in 24 h", got.Datos)
	}
	if _, ok := got.Datos["ultimo_enviado_en"].(time.Time); !ok {
		t.Errorf("ultimo_enviado_en %v", got.Datos["ultimo_enviado_en"])
	}
	b, _ := json.Marshal(got)
	for _, dato := range []string{"persona@example.com", "Verifica", "secreto", "smtp.internal", "550"} {
		if strings.Contains(string(b), dato) {
			t.Errorf("answer leaks %q: %s", dato, b)
		}
	}

	// SMTP caído: 3 final failures in the last 30 min and nothing sent then.
	for range 3 {
		insertar("failed", max, "20 minutes", "5 minutes")
	}
	if got := ComprobarCorreo(tx, max, true).Comprobar(context.Background()); got.Estado != EstadoCaido {
		t.Errorf("after 3 failures in 30 min: %s, want caido", got.Estado)
	}
}
