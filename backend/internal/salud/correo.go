package salud

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
)

// IDCorreo is the e-mail queue service (PLT-03).
const IDCorreo = "correo"

// Thresholds of the correo state (PLT-03 regla 2).
const (
	antiguedadAtrasoMin = 10 // minutes of the oldest unsent e-mail
	fallidosAtraso24h   = 3  // final failures in 24 h
	fallidosCaida30m    = 3  // final failures in 30 min with nothing sent
)

// medidasCorreo are the aggregates of email_queue. Only counts and times: no
// address, subject, body or last_error is ever read.
type medidasCorreo struct {
	SinEnviar        int64      `db:"sin_enviar"`
	AntiguedadMaxMin *int64     `db:"antiguedad_max_min"`
	Fallidos24h      int64      `db:"fallidos_24h"`
	Fallidos30m      int64      `db:"fallidos_30m"`
	Enviados30m      int64      `db:"enviados_30m"`
	UltimoEnviadoEn  *time.Time `db:"ultimo_enviado_en"`
}

// consultaCorreo: sin enviar = pending, sending or failed with retries left;
// fallido definitivo = failed with no retries left, timed by updated_at.
// $1 is notifier.MaxRetries.
const consultaCorreo = `
SELECT
  COUNT(*) FILTER (WHERE status IN ('pending','sending') OR (status = 'failed' AND retry_count < $1)) AS sin_enviar,
  FLOOR(EXTRACT(EPOCH FROM NOW() - MIN(created_at) FILTER (
        WHERE status IN ('pending','sending') OR (status = 'failed' AND retry_count < $1))) / 60)::bigint AS antiguedad_max_min,
  COUNT(*) FILTER (WHERE status = 'failed' AND retry_count >= $1 AND updated_at > NOW() - INTERVAL '24 hours') AS fallidos_24h,
  COUNT(*) FILTER (WHERE status = 'failed' AND retry_count >= $1 AND updated_at > NOW() - INTERVAL '30 minutes') AS fallidos_30m,
  (SELECT COUNT(*) FROM email_queue WHERE status = 'sent' AND updated_at > NOW() - INTERVAL '30 minutes') AS enviados_30m,
  (SELECT MAX(updated_at) FROM email_queue WHERE status = 'sent') AS ultimo_enviado_en
FROM email_queue
WHERE status IN ('pending','sending','failed')`

// ComprobarCorreo reads the queue state without connecting to SMTP (PLT-03).
// smtpConfigurado is false only in development/test (config.Load).
func ComprobarCorreo(db sqlx.QueryerContext, maxReintentos int, smtpConfigurado bool) Comprobacion {
	return comprobarCorreo(func(ctx context.Context) (medidasCorreo, error) {
		var m medidasCorreo
		err := sqlx.GetContext(ctx, db, &m, consultaCorreo, maxReintentos)
		return m, err
	}, smtpConfigurado)
}

func comprobarCorreo(medir func(ctx context.Context) (medidasCorreo, error), smtpConfigurado bool) Comprobacion {
	sinResultado := map[string]any{"sin_enviar": nil, "antiguedad_max_min": nil, "fallidos_24h": nil, "ultimo_enviado_en": nil}
	return Comprobacion{
		ID:           IDCorreo,
		SinResultado: sinResultado,
		Comprobar: func(ctx context.Context) Servicio {
			m, err := medir(ctx)
			datos := copiar(sinResultado)
			if err == nil {
				datos = datosCorreo(m)
			}
			switch {
			case !smtpConfigurado: // evaluated first and wins (regla 2)
				return Servicio{Estado: EstadoNoConfigurado, Datos: datos}
			case err != nil:
				return Servicio{Estado: EstadoCaido, Categoria: categoriaDeError(ctx), Datos: datos}
			}
			estado, cat := estadoCorreo(m)
			return Servicio{Estado: estado, Categoria: cat, Datos: datos}
		},
	}
}

// estadoCorreo applies the rules in the contract order, after no_configurado
// and the query error.
func estadoCorreo(m medidasCorreo) (string, *string) {
	switch {
	case m.Fallidos30m >= fallidosCaida30m && m.Enviados30m == 0:
		return EstadoCaido, ptrCategoria(CategoriaSinConexion)
	case (m.AntiguedadMaxMin != nil && *m.AntiguedadMaxMin > antiguedadAtrasoMin) || m.Fallidos24h >= fallidosAtraso24h:
		return EstadoDegradado, ptrCategoria(CategoriaAtrasado)
	default:
		return EstadoOperativo, nil
	}
}

func datosCorreo(m medidasCorreo) map[string]any {
	var ultimo any
	if m.UltimoEnviadoEn != nil {
		ultimo = m.UltimoEnviadoEn.UTC().Truncate(time.Second)
	}
	var antiguedad any
	if m.AntiguedadMaxMin != nil {
		antiguedad = *m.AntiguedadMaxMin
	}
	return map[string]any{"sin_enviar": m.SinEnviar, "antiguedad_max_min": antiguedad,
		"fallidos_24h": m.Fallidos24h, "ultimo_enviado_en": ultimo}
}

func ptrCategoria(c string) *string { return &c }
