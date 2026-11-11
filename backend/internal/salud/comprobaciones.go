package salud

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
)

// Service ids, in the contract order.
const (
	IDAPI       = "api"
	IDBaseDatos = "base_datos"
)

// umbralLatenciaBD: above it the database is degradado (PLT-01 regla 3).
const umbralLatenciaBD = 500 * time.Millisecond

// ComprobarAPI: if the answer arrives the API works (PLT-01 regla 2). version
// is main.version, the same as /health; arranque is when the process started.
func ComprobarAPI(version string, arranque time.Time, reloj func() time.Time) Comprobacion {
	return Comprobacion{
		ID: IDAPI,
		Comprobar: func(context.Context) Servicio {
			enMarcha := int64(reloj().Sub(arranque) / time.Second) // truncated
			return Servicio{ID: IDAPI, Estado: EstadoOperativo,
				Datos: map[string]any{"version": version, "en_marcha_s": enMarcha}}
		},
		SinResultado: map[string]any{"version": nil, "en_marcha_s": nil},
	}
}

// ComprobarBD runs a trivial read-only query on the normal pool and measures
// its latency (PLT-01 regla 3). Pool waits count against the deadline.
func ComprobarBD(db *sqlx.DB) Comprobacion {
	return comprobarBD(func(ctx context.Context) error {
		var uno int
		return db.QueryRowContext(ctx, `SELECT 1`).Scan(&uno)
	}, time.Now)
}

// comprobarBD is ComprobarBD over any query function and clock (tests).
func comprobarBD(consultar func(ctx context.Context) error, ahora func() time.Time) Comprobacion {
	sinResultado := map[string]any{"latencia_ms": nil}
	return Comprobacion{
		ID: IDBaseDatos,
		Comprobar: func(ctx context.Context) Servicio {
			inicio := ahora()
			if err := consultar(ctx); err != nil {
				return Servicio{ID: IDBaseDatos, Estado: EstadoCaido, Categoria: categoriaDeError(ctx),
					Datos: copiar(sinResultado)}
			}
			latencia := ahora().Sub(inicio)
			estado := EstadoOperativo
			if latencia > umbralLatenciaBD {
				estado = EstadoDegradado
			}
			return Servicio{ID: IDBaseDatos, Estado: estado,
				Datos: map[string]any{"latencia_ms": latencia.Milliseconds()}} // truncated
		},
		SinResultado: sinResultado,
	}
}

// categoriaDeError: an expired ctx is tiempo_agotado whatever the driver says
// (lib/pq may answer 57014 «canceling statement» instead of
// DeadlineExceeded); any other error is sin_conexion.
func categoriaDeError(ctx context.Context) *string {
	c := CategoriaSinConexion
	if ctx.Err() != nil {
		c = CategoriaTiempoAgotado
	}
	return &c
}
