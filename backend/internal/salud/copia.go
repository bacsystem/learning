package salud

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
)

// IDCopiaR2 is the daily backup to R2 (PLT-05).
const IDCopiaR2 = "copia_r2"

// Age of the last correct backup (PLT-05 regla 1).
const (
	copiaAtrasada = 26 * time.Hour // degradado above it
	copiaCaida    = 50 * time.Hour // caido above it
)

const resultadoFallida = "fallida"

// medidasCopia: the last correct run and the last run of copias_bd. The
// table has no bucket, path, endpoint or credentials to leak.
type medidasCopia struct {
	HayFilas          bool       `db:"hay_filas"`
	UltimaCorrectaEn  *time.Time `db:"ultima_correcta_en"`
	TamanoBytes       *int64     `db:"tamano_bytes"`
	UltimaEjecucion   *string    `db:"ultima_ejecucion"`
	UltimaEjecucionEn *time.Time `db:"ultima_ejecucion_en"`
}

const consultaCopia = `
SELECT EXISTS (SELECT 1 FROM copias_bd) AS hay_filas,
       c.terminada_en AS ultima_correcta_en, c.tamano_bytes,
       u.resultado AS ultima_ejecucion, u.terminada_en AS ultima_ejecucion_en
FROM (SELECT 1) AS uno
LEFT JOIN LATERAL (SELECT terminada_en, tamano_bytes FROM copias_bd WHERE resultado = 'correcta'
                   ORDER BY terminada_en DESC, id DESC LIMIT 1) AS c ON true
LEFT JOIN LATERAL (SELECT resultado, terminada_en FROM copias_bd
                   ORDER BY terminada_en DESC, id DESC LIMIT 1) AS u ON true`

// ComprobarCopia reads copias_bd, written by the backup cron; the API never
// uses R2 or Railway credentials.
func ComprobarCopia(db sqlx.QueryerContext, reloj func() time.Time) Comprobacion {
	return comprobarCopia(func(ctx context.Context) (medidasCopia, error) {
		var m medidasCopia
		err := sqlx.GetContext(ctx, db, &m, consultaCopia)
		return m, err
	}, reloj)
}

func comprobarCopia(medir func(ctx context.Context) (medidasCopia, error), reloj func() time.Time) Comprobacion {
	sinResultado := map[string]any{"ultima_correcta_en": nil, "tamano_bytes": nil, "ultima_ejecucion": nil, "ultima_ejecucion_en": nil}
	return Comprobacion{
		ID:           IDCopiaR2,
		SinResultado: sinResultado,
		Comprobar: func(ctx context.Context) Servicio {
			m, err := medir(ctx)
			if err != nil {
				return Servicio{Estado: EstadoCaido, Categoria: categoriaDeError(ctx), Datos: copiar(sinResultado)}
			}
			estado, cat := estadoCopia(m, reloj())
			return Servicio{Estado: estado, Categoria: cat, Datos: datosCopia(m)}
		},
	}
}

// estadoCopia applies the rules in the contract order, after the query error.
func estadoCopia(m medidasCopia, ahora time.Time) (string, *string) {
	switch {
	case !m.HayFilas:
		return EstadoDesconocido, ptrCategoria(CategoriaSinDatos)
	case m.UltimaCorrectaEn == nil || ahora.Sub(*m.UltimaCorrectaEn) > copiaCaida:
		return EstadoCaido, ptrCategoria(CategoriaAtrasado)
	case ahora.Sub(*m.UltimaCorrectaEn) > copiaAtrasada:
		return EstadoDegradado, ptrCategoria(CategoriaAtrasado)
	case m.UltimaEjecucion != nil && *m.UltimaEjecucion == resultadoFallida:
		return EstadoDegradado, ptrCategoria(CategoriaAtrasado)
	default:
		return EstadoOperativo, nil
	}
}

func datosCopia(m medidasCopia) map[string]any {
	return map[string]any{
		"ultima_correcta_en":  fechaUTC(m.UltimaCorrectaEn),
		"tamano_bytes":        valor(m.TamanoBytes),
		"ultima_ejecucion":    valor(m.UltimaEjecucion),
		"ultima_ejecucion_en": fechaUTC(m.UltimaEjecucionEn),
	}
}
