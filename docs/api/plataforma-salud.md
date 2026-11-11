# Contrato API — Plataforma, épica PLT-E1 Salud de servicios

- **Historias:** [PLT-01](../stories/plataforma/PLT-01-estado-salud-api-bd.md), [PLT-02](../stories/plataforma/PLT-02-pantalla-salud-servicios.md), [PLT-03](../stories/plataforma/PLT-03-salud-correo-version-frontend.md), [PLT-04](../stories/plataforma/PLT-04-latido-worker-ingesta.md), [PLT-05](../stories/plataforma/PLT-05-copia-diaria-r2.md). Decisiones del PO en el [README de la épica](../stories/plataforma/README.md) (PR #108).
- **Estado:** vigente (2026-11-11).
- **ADR:** [0005](../adr/0005-estado-operativo-panel-salud.md): 503 de la autenticación con la BD caída, registro de copias y caché. La copia de R2 está en la [ADR 0004 § Decisión 11](../adr/0004-alojamiento-base-de-datos-produccion.md).

## Convenciones

Estas historias no son del módulo SERUMS IA, pero el contrato adopta del [README de contratos](README.md):

- **§2:** autenticación y rol con `RequireAuth` + `RequireAdmin`;
- **§3:** la regla de 500 y la de logs sin datos personales;
- **§5:** fechas en RFC 3339 UTC.

**Excepciones:**

- Este endpoint **no** tiene errores propios: si la autenticación pasa, siempre responde 200 con el estado. Los fallos de los servicios van dentro del cuerpo como categorías, nunca como status HTTP.
- **Sin `audit_log`** (decisión 9 del PO): es una lectura sin efectos.

Paquete backend nuevo, `internal/salud`, con `handler.go`, `service.go`, `dto.go` y `routes.go`. Se monta con `Mount(mux, svc, jwtSecret, checkSession)`, la misma firma que `catalogs.Mount` (`cmd/server/main.go:128`). No tiene `repository.go` propio: cada comprobación es una función con su consulta (abajo).

## `GET /health` (existente, no cambia)

`cmd/server/main.go:113-121`, público:

- Hace `PingContext` a la BD.
- Si falla, responde `503 {"status":"unhealthy"}`.
- Si no, responde `200 {"status":"ok","version":"<versión>"}`.

No gana ningún detalle (decisión 7 del PO).

## S1 `GET /admin/salud`

Rol admin. Sin query ni cuerpo.

### Respuesta `200`

```json
{
  "estado": "degradado",
  "comprobado_en": "2026-11-11T15:04:05Z",
  "servicios": [
    { "id": "api", "estado": "operativo", "categoria": null,
      "datos": { "version": "1.4.0", "en_marcha_s": 86400 } },
    { "id": "base_datos", "estado": "operativo", "categoria": null,
      "datos": { "latencia_ms": 20 } },
    { "id": "correo", "estado": "degradado", "categoria": "atrasado",
      "datos": { "sin_enviar": 2, "antiguedad_max_min": 12, "fallidos_24h": 0, "ultimo_enviado_en": "2026-11-11T14:50:00Z" } },
    { "id": "copia_r2", "estado": "operativo", "categoria": null,
      "datos": { "ultima_correcta_en": "2026-11-11T05:00:40Z", "tamano_bytes": 52428800,
                 "ultima_ejecucion": "correcta", "ultima_ejecucion_en": "2026-11-11T05:00:40Z" } }
  ]
}
```

| Campo | Regla |
|---|---|
| `estado` | Global: el peor de los individuales, con el orden `caido` > `degradado` = `desconocido` > `operativo`. `no_configurado` no cuenta. Si todos son `no_configurado`, el global es `operativo` |
| `comprobado_en` | Instante UTC en que se ejecutaron las comprobaciones. Con la caché devuelve el de la ejecución cacheada |
| `servicios` | Lista **abierta** en orden fijo: `api`, `base_datos`, `correo`, `worker_ingesta` (v2) y `copia_r2` (v2). La UI ignora los `id` que no conoce (PLT-01 regla 7). Un servicio de v2 sale en la lista solo cuando su historia está implementada |
| `servicios[].estado` | `operativo` \| `degradado` \| `caido` \| `no_configurado` \| `desconocido` |
| `servicios[].categoria` | `null` o una categoría **cerrada**: `sin_conexion` \| `tiempo_agotado` \| `error_interno` \| `atrasado` \| `sin_datos` (decisión 8). Nunca el mensaje de error |
| `servicios[].datos` | Objeto propio de cada servicio, solo con números, versiones y fechas UTC. Sin host, IP, puerto, URL, usuario, credenciales, bucket, ruta, direcciones de correo ni texto de error. Un dato que no se pudo obtener va como `null` |

### Errores (solo de middleware)

| Status | Cuándo | Cuerpo |
|---|---|---|
| 401 | Sin token, token inválido, o sesión inexistente, expirada o revocada | `{"error": "No autenticado." \| "Sesión revocada."}` (heredado, `auth/middleware.go:38,44,52`) |
| 403 | Rol distinto de admin | `{"error": "Acceso denegado."}` |
| **503** | **La comprobación de la sesión falla por un error de la BD** (cambio nuevo, ver «Autenticación con la BD caída») | `{"error": "Servicio no disponible."}` |

### Ejecución de las comprobaciones (PLT-01 reglas 4–6; decisiones 4–5)

- Todas corren **en paralelo**, cada una con su `context.WithTimeout` de **2 s**.
  - Si no termina, el servicio se informa como `caido` / `tiempo_agotado`, sin esperar más.
  - La respuesta tarda como máximo unos 2 s más la serialización, por debajo de los 3 s.
- **Caché en proceso de 15 s:** la primera petición ejecuta las comprobaciones y las demás, dentro de los 15 s, reciben el mismo resultado y el mismo `comprobado_en`.
  - Con varias réplicas, cada una tiene su caché. Se acepta: el peor caso es una ejecución por réplica cada 15 s.
  - Peticiones simultáneas con la caché vencida comparten **una** ejecución: un `sync.Mutex` con doble comprobación. No se agrega ninguna dependencia: `golang.org/x/sync` no está en `go.mod`.
  - «Actualizar» no salta la caché (decisión 6).
- Un pánico o error inesperado dentro de una comprobación da `caido` / `error_interno` para ese servicio. El endpoint **no** responde 500 por eso.
- Logs: solo `op`, el `id` del servicio y la categoría. El error técnico puede ir al log, pero sin credenciales ni datos personales: los errores de driver pueden traer el DSN y no se registran tal cual (README §9).

### Servicio `api` (PLT-01 regla 2)

- Siempre `operativo`: si hay respuesta, la API funciona.
- `datos.version` es la misma variable que usa `/health`: `main.version`, inyectada con `-ldflags -X main.version=X.Y.Z` y `0.0.0-dev` fuera de un release (hallazgo H-6, PR #100).
- `datos.en_marcha_s` son los segundos desde que arrancó el proceso, guardados en `main` al iniciar.

### Servicio `base_datos` (PLT-01 regla 3)

- Consulta `SELECT 1` con timeout de 2 s; se mide el tiempo en ms (`datos.latencia_ms`).

| Resultado | Estado | Categoría |
|---|---|---|
| ≤ 500 ms | `operativo` | `null` |
| > 500 ms | `degradado` | `null` |
| Error de conexión | `caido` | `sin_conexion` |
| Sin respuesta en 2 s | `caido` | `tiempo_agotado` |

- Usa el pool normal (`*sqlx.DB`). Si el pool está agotado, la espera cuenta para el timeout y el resultado es `tiempo_agotado`. Es una señal real.
- **Límite conocido:** con la BD totalmente caída, la autenticación falla antes y la respuesta es `503` del middleware, no `200` con `base_datos: caido`. La UI lo trata como «No se pudo consultar» (PLT-02 regla 5).
  - Los escenarios de PLT-01 «Base de datos caída» y «Comprobación que no termina» se prueban con la **sonda aislada**: una función de comprobación inyectable que en el test devuelve un error o se bloquea, con la autenticación sobre un doble.
  - El PO reformula esos escenarios en PLT-01 (PR #108).

### Servicio `correo` (PLT-03)

Lee `email_queue` (`backend/migrations/007_create_email_queue.sql:2-13`) sin conectarse por SMTP. Todos los tiempos usan `updated_at`, que el worker actualiza en cada transición (`internal/notifier/email_worker.go:150,159,239,266`). El tope de reintentos es `notifier.maxRetries = 5` (`email_worker.go:16`): se **exporta** como `notifier.MaxRetries` para no duplicar la constante.

| Medida | Definición SQL |
|---|---|
| Sin enviar | `status IN ('pending','sending') OR (status = 'failed' AND retry_count < MaxRetries)` |
| `datos.sin_enviar` | `COUNT(*)` de lo anterior |
| `datos.antiguedad_max_min` | `floor((NOW() - MIN(created_at)) / 1 min)` de lo anterior; `null` si no hay ninguno |
| Fallido definitivo | `status = 'failed' AND retry_count >= MaxRetries`, con la hora en `updated_at` |
| `datos.fallidos_24h` | Fallidos definitivos con `updated_at > NOW() - 24 h` |
| `datos.ultimo_enviado_en` | `MAX(updated_at) WHERE status = 'sent'`; `null` si nunca se envió ninguno |

| Condición, evaluada en este orden | Estado | Categoría |
|---|---|---|
| SMTP no configurado (`cfg.SMTPHost == ""`; solo se permite con `ENV` development/test, `config/config.go:89-101`) | `no_configurado` | `null` |
| Error o timeout de la consulta | `caido` | `sin_conexion` / `tiempo_agotado` |
| En los últimos 30 min, ≥ 3 fallidos definitivos y ningún `sent` | `caido` | `sin_conexion` |
| `antiguedad_max_min > 10`, o `fallidos_24h ≥ 3` | `degradado` | `atrasado` |
| En otro caso | `operativo` | `null` |

- Con `no_configurado`, `datos` puede seguir trayendo los conteos.
- **Nunca** se leen ni devuelven `to_address`, `subject`, `body_html` ni `last_error`. Las consultas solo seleccionan agregados.
- **Índice nuevo**, porque `sent` es la mayoría de las filas y hoy no tiene índice: `CREATE INDEX idx_email_queue_sent_updated ON email_queue (updated_at) WHERE status = 'sent'`. Va en la migración **063** (PLT-03).

### Servicio `worker_ingesta` (PLT-04, v2)

`id` reservado. Dónde se guarda el latido se define cuando se reparta PLT-04: no hay worker de ingesta en producción todavía. Las reglas de estado están en la historia.

### Servicio `copia_r2` (PLT-05, v2)

Lee la tabla `copias_bd` (abajo), que escribe el cron de la ADR 0004 § Decisión 11. La API **no** usa credenciales de R2 ni la API de Railway.

| Medida | Definición |
|---|---|
| `datos.ultima_correcta_en` | `MAX(terminada_en) WHERE resultado = 'correcta'`; `null` si no hay ninguna |
| `datos.tamano_bytes` | El de esa misma fila. La UI lo muestra en MB |
| `datos.ultima_ejecucion`, `datos.ultima_ejecucion_en` | `resultado` y `terminada_en` de la fila con mayor `terminada_en` |

| Condición, evaluada en este orden | Estado | Categoría |
|---|---|---|
| Error o timeout de la consulta | `caido` | `sin_conexion` / `tiempo_agotado` |
| No hay ninguna fila | `desconocido` | `sin_datos` |
| No hay ninguna correcta, o la última correcta tiene > 50 h | `caido` | `atrasado` |
| La última correcta tiene > 26 h | `degradado` | `atrasado` |
| La última ejecución es `fallida` (con una correcta en las últimas 26 h) | `degradado` | `atrasado` |
| En otro caso | `operativo` | `null` |

Nunca se devuelve bucket, ruta, clave del objeto, endpoint ni credenciales: la tabla no los guarda.

### Versión del frontend (PLT-03 regla 4)

No pasa por la API. El frontend muestra el `version` de `frontend/package.json`, que ya usa «Acerca de» (`frontend/src/components/app/acerca-de.tsx:25`).

- Mismo formato que el backend: sin «v», p. ej. `1.4.0`.
- El release verifica que coincida con el tag sin la «v» (tarea de devops, ADR 0004 § Decisión 9).
- La UI la compara con `servicios[id=api].datos.version`. Si difieren, muestra la nota informativa, que no cambia el estado global.
- Fuera de un release, el backend dice `0.0.0-dev` y el frontend el `version` del repo, así que en desarrollo la nota puede aparecer. Se acepta: es informativa.

## Autenticación con la BD caída (cambio transversal; ADR 0005)

Hoy `RequireAuth` responde **401** «Sesión revocada.» tanto si la sesión no existe como si `checkSession` devuelve un **error** de BD (`auth/middleware.go:50-54`). El frontend trata todo 401 como sesión expirada y dispara `auth:expired` (p. ej. `frontend/src/lib/admin/api.ts:13`), así que una caída de la BD **saca a todos los usuarios**.

Contrato nuevo, que aplica a **todos** los endpoints autenticados:

| `checkSession` devuelve | Respuesta |
|---|---|
| `(nil, nil)`: no existe, expiró o fue revocada; o `UserID` distinto; o `estado ≠ activo` | **401** `{"error": "Sesión revocada."}` (sin cambios) |
| `(_, err)` con `err ≠ nil`: la BD falla o da timeout | **503** `{"error": "Servicio no disponible."}`, formato heredado; se registra `op` sin el error crudo |

- **Falla cerrado:** con un error nunca se deja pasar la petición, y el 401 solo sale cuando la sesión no existe, expiró o fue revocada.
- **Frontend:** un `503`, igual que un `403`, **no** dispara `auth:expired`. Se muestra como error del servidor y la sesión se conserva. `auth-context.tsx:44-47` ya distingue 401 de 5xx al restaurar la sesión; hay que verificar todos los clientes `src/lib/*/api.ts`.
- **Tareas:**
  - backend-dev: middleware y tests de los tres casos (sesión inexistente → 401, error de BD → 503, sesión válida → pasa);
  - frontend-dev: verificar que 403 y 503 no disparan `auth:expired`.

### `POST /auth/login` con la BD caída (extensión del mismo criterio; issue #133)

Hoy el handler mapea a 401 «Credenciales inválidas.» todo error que no sea de cuenta (`backend/internal/auth/handler.go:440-441`), incluidos los de la BD. Con la BD caída, el usuario ve «credenciales inválidas» y cree que su contraseña está mal. Contrato:

| Caso | Respuesta (formato heredado, `{"error": "…"}`) |
|---|---|
| Usuario inexistente o contraseña incorrecta | **401** «Credenciales inválidas.» (`ErrInvalidCredentials`, `auth/errors.go:10`). Sin cambios, y sin revelar si el usuario existe |
| Cuenta bloqueada por intentos fallidos | **429** «Demasiados intentos fallidos. Cuenta bloqueada temporalmente. Intente en 15 minutos.» (`ErrAccountLocked`, `errors.go:11`; `handler.go:434-435`). Sin cambios |
| Cuenta pendiente de activación | **403** «Tu cuenta está pendiente de activación por el administrador.» (`ErrAccountPending`, `errors.go:12`; `handler.go:436-437`). Sin cambios |
| Cuenta rechazada o desactivada | **403** «Tu cuenta ha sido rechazada. Contacta al administrador.» o «Tu cuenta está desactivada.» (`ErrAccountRejected` / `ErrAccountInactive`, `errors.go:13-14`; `handler.go:438-439`). Sin cambios |
| **Cualquier error de la BD dentro del login**: lectura del usuario, registro o reinicio de intentos, perfil, sesiones | **503** «Servicio no disponible.» (centinela nuevo `ErrServicioNoDisponible`) |
| Cualquier otro error inesperado (p. ej. al firmar el JWT) | **500** genérico con `httpx.InternalError` (regla de 500 del README §3). Deja de ser 401 |

- **Falla cerrado:** si algo de la BD falló, **nunca** se devuelve token ni queda una sesión creada. Si `CreateSesion` falla después de firmar el JWT, el token no se entrega.
- **Logs:** solo `op=auth.login`, sin el usuario, la contraseña ni el error crudo.
- **Frontend:** sin cambios. Un 5xx ya se muestra como error del servidor (`frontend/src/lib/auth/errores-auth.ts:19`), y el login no lleva `Authorization`, así que no dispara `auth:expired`.
- **Tests mínimos** (backend-dev-2):
  - usuario inexistente → 401;
  - contraseña incorrecta → 401;
  - error de BD al leer el usuario → 503;
  - error de BD en `CreateSesion` → 503 sin token;
  - cuenta bloqueada → 429.

## Modelo de datos

### Migración 064 `copias_bd` (PLT-05)

Número asignado por el tech-lead: **063** es el índice de correo (PLT-03) y **064** es `copias_bd` (PLT-05), en el orden de las historias.

```sql
-- +goose Up
CREATE TABLE copias_bd (
    id           BIGSERIAL   PRIMARY KEY,
    iniciada_en  TIMESTAMPTZ NOT NULL,
    terminada_en TIMESTAMPTZ NOT NULL,
    resultado    VARCHAR(10) NOT NULL CHECK (resultado IN ('correcta', 'fallida')),
    tamano_bytes BIGINT      CHECK (tamano_bytes >= 0),
    CONSTRAINT ck_copias_bd_tamano CHECK ((resultado = 'correcta') = (tamano_bytes IS NOT NULL)),
    CONSTRAINT ck_copias_bd_tiempos CHECK (terminada_en >= iniciada_en)
);
CREATE INDEX idx_copias_bd_terminada ON copias_bd (terminada_en DESC);
CREATE INDEX idx_copias_bd_correcta  ON copias_bd (terminada_en DESC) WHERE resultado = 'correcta';

-- +goose Down
DROP TABLE IF EXISTS copias_bd;
```

- Sin datos personales, bucket, ruta ni mensaje de error.
- **Una fila por ejecución**, que el cron inserta **al terminar**, correcta o fallida, con `trap` y su timeout propio. Sin UPDATE ni DELETE.
- **Permisos (aprovisionamiento, no goose):** el rol `backup_ro` no existe en CI ni en compose, así que el GRANT lo hace devops al aprovisionar y lo documenta en `docs/ops`:

  ```sql
  GRANT INSERT ON copias_bd TO backup_ro;
  GRANT USAGE ON SEQUENCE copias_bd_id_seq TO backup_ro;
  ```

- **Orden de despliegue:** la migración llega a producción **antes** de la primera ejecución del cron.
- **Si el INSERT falla después de una subida correcta**, el cron sale con error y alerta (ADR 0004 C13). La copia en R2 es válida, pero el panel la ve atrasada hasta la siguiente. Se acepta.
- **Si la BD no responde** no queda fila, y el estado sale de la antigüedad de la última correcta.
- **Crecimiento:** unas 365 filas al año. No hay purga en el MVP (YAGNI).
- Es la tabla que lee `copia_r2`.

### Índice de correo (PLT-03)

`idx_email_queue_sent_updated` (ver «Servicio `correo`»), en la migración **063** (PLT-03).

## Seguridad

- Solo `admin`. No hay IDOR posible: no hay parámetros.
- Respuesta sin secretos ni datos personales (PLT-01 «Sin secretos»; PLT-03 «Sin datos personales del correo»; PLT-05 «Sin datos de R2»).
- El endpoint no revela la topología: ni nombres de host ni de base.

## Trazabilidad

| Historia | Escenario | Cubierto por |
|---|---|---|
| PLT-01 | Todo operativo / BD lenta | S1, `base_datos` |
| PLT-01 | BD caída / comprobación que no termina | `base_datos` con la sonda aislada (límite conocido) |
| PLT-01 | Caché de 15 s | «Ejecución de las comprobaciones» |
| PLT-01 | Sin secretos | categorías cerradas, `datos` sin campos sensibles, logs |
| PLT-01 | `/health` público sin cambios | `GET /health` |
| PLT-01 | 403 / 401 | errores de middleware |
| PLT-02 | Backend inalcanzable (también con la BD caída) | 503 de `RequireAuth` sin `auth:expired` |
| PLT-03 | Correo: operativo, atrasado, SMTP caído, fallidos repetidos, fallo aislado, no configurado, sin datos personales | `correo` |
| PLT-03 | Versión del frontend / versiones distintas | «Versión del frontend» |
| PLT-04 | Worker | `worker_ingesta` (se diseña al repartir PLT-04) |
| PLT-05 | Copia al día, ejecución fallida, sin copia reciente, sin ejecuciones, sin datos de R2 | `copia_r2`, tabla `copias_bd` |

## UI implicada (khipu)

Pantalla de PLT-02 en `(dashboard)/admin/salud`:

- `PillEstado` (`components/patrones/pill-estado.tsx`) o `Badge` con icono Phosphor y texto para los 5 estados (nunca solo color);
- tarjetas con los componentes de khipu ya copiados;
- `Alerta` / `Banner` para «No se pudo consultar» y «desactualizado»;
- `Skeleton` para la carga;
- `BotonAsync` para «Actualizar».

Si `PillEstado` no cubre los 5 estados con icono, primero se abre un PR en khipu (regla del repo).
