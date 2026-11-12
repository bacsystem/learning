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

**Ojo con el seed y `TAG=`:** las imágenes salen del registro, pero
`make local/seed` corre `cmd/seed/demo` desde el **código de tu checkout**.
Para validar un release, haz checkout de ese tag (`git switch --detach vX.Y.Z`),
así el seed coincide con el esquema de la imagen. `cmd/seed/demo` necesita las
migraciones hasta la 062.

Las imágenes publicadas son `linux/amd64`. En un Mac Apple Silicon, Docker las
ejecuta emuladas: funcionan, pero más lento. `BUILD=1` las construye nativas.

## Una sola preproducción por máquina

El proyecto de compose se llama siempre `capacitate-preprod` y usa puertos
fijos, así que en cada máquina hay **una** preproducción. La levanta un
checkout. Si ya corre desde **otro** checkout (de qa o de otro agente),
`local/up`, `local/down`, `local/reset` y `local/seed` **abortan sin tocar
nada** y dicen desde dónde corre. Para usarla desde aquí, quien la levantó hace
antes `make local/down` en su checkout.

`local/reset` además pide escribir `borrar` antes de borrar los datos. Sin
terminal: `make local/reset BUILD=1 CONFIRM=borrar`.

## Puertos (todos en 127.0.0.1)

| Uso | Base | Backend | Frontend | Otros |
|---|---|---|---|---|
| Desarrollo (`docker-compose.yml`) | 5433 | 8080 | 3000 | — |
| Tests (`TEST_DATABASE_URL`, base `*_test_<n>`) | 5434 | — | — | — |
| **Preproducción (`deploy/local`)** | **5441** | **8180** | **3110** | Mailpit 8125 |
| Pruebas ad hoc de qa | 5442 | 8190 | 3120 | — |

No uses los puertos de otra fila: así nada choca.

## Comandos

| Comando | Qué hace |
|---|---|
| `make local/up TAG=…` o `BUILD=1` | Levanta todo y espera a que esté sano |
| `make local/seed` | Carga los datos sintéticos (`cmd/seed/demo`), idempotente. Usuarios `demo_admin`, `demo_enfermeria`, `demo_medicina`, `demo_pendiente` y `demo_esp_inactiva`, todos activos y con el tour completado; contraseñas `PREPROD_DEMO_*` de `preprod.vars` (valores falsos, solo de preproducción). Requiere `cmd/seed/demo` (#114) |
| `make local/reset TAG=…` o `BUILD=1` | **Borra** la base (pide escribir `borrar`, o `CONFIRM=borrar`), vuelve a levantar (migra) y siembra |
| `make local/logs` (`s=backend`) | Logs en vivo: sigue sin terminar (Ctrl-C para salir). Para una captura puntual, por ejemplo para adjuntar a un reporte: `docker logs --tail 200 capacitate-preprod-backend-1` (o `-db-1`, `-frontend-1`, `-mailpit-1`) |
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
make local/test    # = make test con TEST_DATABASE_URL hacia capacitate_preprod_test (TLS)
```

La URL se arma con `PREPROD_PG_USER` y `PREPROD_PG_PASSWORD` de `preprod.vars`.

## Variables

Están en [`preprod.vars`](preprod.vars). Son **falsas** (`NOT-FOR-PRODUCTION`)
y se versionan a propósito. **Ningún valor de ahí se usa en Railway**: lo
comprueba el checklist de release.
