# Preproducción local (guía para qa)

Un entorno local lo más parecido posible a producción, para validar una
historia **antes** de salir a Railway:

- las **mismas imágenes** que se publican, o construidas con los mismos
  Dockerfiles;
- `ENV=preprod`, así que la validación de configuración es la de producción;
- Postgres 16 con **TLS** (`sslmode=require`);
- migraciones con `cmd/migrate` al arrancar el backend;
- correo capturado por **Mailpit**: nada sale a personas reales;
- **datos 100 % sintéticos**.

No toca las bases de dev (5432/5433) ni la de test (5434) y no lee `.env` ni
`assets/`.

## Requisitos

Docker (con Compose v2), `make` y `openssl`. Para el seed, la imagen
`golang:1.27.4-alpine3.23` se descarga sola.

## Levantar

```bash
# Con las imágenes publicadas del release (lo que va a Railway):
make local/up TAG=v1.0.0        # backend v1.0.0 + frontend v1.0.0-preprod

# O construyéndolas en local con los mismos Dockerfiles (antes de publicar):
make local/up BUILD=1
```

| Servicio | URL (solo 127.0.0.1) |
|---|---|
| Frontend | http://localhost:3110 |
| Backend (`GET /health`) | http://localhost:8180 |
| Mailpit (correos capturados) | http://localhost:8125 |
| Postgres (TLS) | `localhost:5441`, base `capacitate_preprod` |

`CORS_ORIGINS` y `FRONTEND_BASE_URL` apuntan a `http://localhost:3110`, y el
frontend se construye con `NEXT_PUBLIC_API_URL=http://localhost:8180`.

## Comandos

| Comando | Qué hace |
|---|---|
| `make local/up TAG=…` o `BUILD=1` | Levanta todo y espera a que esté sano |
| `make local/seed` | Carga los datos sintéticos (`cmd/seed/demo`). **Pendiente**: lo está haciendo backend-dev. Hasta entonces el comando avisa y sale con error |
| `make local/reset TAG=…` o `BUILD=1` | **Borra** la base, vuelve a levantar (migra) y siembra |
| `make local/logs` (`s=backend`) | Logs en vivo |
| `make local/ps` | Estado de los contenedores |
| `make local/psql` | Consola psql en la base de preproducción |
| `make local/down` | Detiene todo (conserva los datos) |

## Validar una historia

1. `make local/reset BUILD=1` (o `TAG=` del release candidato), para partir de
   un estado conocido.
2. Comprobar `GET http://localhost:8180/health` → `{"status":"ok",…}`.
3. Entrar en http://localhost:3110 con los usuarios del seed y recorrer los
   criterios de aceptación de la historia (`docs/stories/`).
4. Los correos (verificación, recuperación) se leen en Mailpit:
   http://localhost:8125.
5. Si algo falla, adjuntar al reporte `make local/logs s=backend` (los logs no
   llevan datos personales) y la versión de `/health`.

**Límite de login:** el backend admite 10 intentos por minuto. No hay atajo
(cambiarlo sería tocar el código): espera un minuto si lo alcanzas.

## Tests contra esta base

`make local/up` crea también `capacitate_preprod_test`:

```bash
make test TEST_DATABASE_URL='postgres://capacitate:NOT-FOR-PRODUCTION-preprod-pg-password@localhost:5441/capacitate_preprod_test?sslmode=require'
```

## Variables

Están en [`preprod.vars`](preprod.vars). Son **falsas** (`NOT-FOR-PRODUCTION`)
y se versionan a propósito. **Ningún valor de ahí se usa en Railway**: lo
comprueba el checklist de release.
