# Plan de trabajo: spec OpenAPI de la API

- **Decisión:** [ADR 0012](../adr/0012-documentacion-openapi-spec-first.md). Spec primero, OpenAPI 3.1, un archivo por paquete Go y un test de rutas en `backend/internal/apispec`.
- **Estado:** completado (2026-11-15).
  - Los 16 paquetes están documentados en `docs/api/openapi/`.
  - El test `backend/internal/apispec` corre en modo estricto (#311, paso 3a): falla con cualquier ruta sin documentar.
  - La fuente de verdad quedó fijada en el README de `docs/api/` (paso 3b).
  - Desde aquí, el plan es histórico. Las reglas vigentes están en el ADR 0012 y en el README.
- **Aprobaciones (2026-11-15):** el usuario aprobó `@redocly/cli` (fijada en 2.72.1, invocada con `npx`). El tech-lead aprobó el paso de `go.yaml.in/yaml/v3` a dependencia directa y autorizó a backend-dev y backend-dev-2 a escribir en `docs/api/openapi/paquetes/`, cada uno solo en sus paquetes.
- **Sin migraciones ni cambios de comportamiento.** Ningún PR de este plan toca `handler.go`, `service.go`, `repository.go` ni `dto.go`.

## 1. Inventario: 153 operaciones (152 rutas en 15 paquetes, más `GET /health`)

Conteo de las líneas `mux.Handle`/`HandleFunc` de cada `routes.go`, montadas en `cmd/server/main.go:117-149`. Las columnas de contrato se explican en el §4.

| Paquete | Archivo de rutas | Ops | Completo | Delta | Sin contrato | Dueño |
|---|---|---:|---:|---:|---:|---|
| `catalogs` | `catalogs/routes.go:23-67` | 33 | 3 | 1 | 29 | backend-dev |
| `temas` | `temas/routes.go:18-25` | 7 | 7 | 0 | 0 | backend-dev |
| `perfiles` | `perfiles/routes.go:17-19` | 3 | 3 | 0 | 0 | backend-dev |
| `questions` | `questions/routes.go:34-51` | 15 | 2 | 6 | 7 | backend-dev |
| `fuentes` | `fuentes/routes.go:13-46` (constantes) | 8 | 8 | 0 | 0 | backend-dev |
| `balotario` | `balotario/routes.go:12-23` (constantes) | 2 | 2 | 0 | 0 | backend-dev |
| `ia` | `ia/routes.go:18-21` | 4 | 4 | 0 | 0 | backend-dev |
| `practices` | `practices/routes.go:27-40` | 10 | 0 | 1 | 9 | backend-dev |
| `simulations` | `simulations/routes.go:27-41` | 13 | 0 | 1 | 12 | backend-dev |
| `auth` | `auth/routes.go:24-62` | 29 | 1 | 6 | 22 | backend-dev-2 |
| `admin` | `admin/routes.go:21-34` | 8 | 0 | 0 | 8 | backend-dev-2 |
| `inscripciones` | `inscripciones/routes.go:17-29` | 7 | 0 | 3 | 4 | backend-dev-2 |
| `testimonios` | `testimonios/routes.go:15-19` | 5 | 0 | 0 | 5 | backend-dev-2 |
| `notifications` | `notifications/routes.go:20-28` | 7 | 0 | 3 | 4 | backend-dev-2 |
| `salud` | `salud/routes.go:13` | 1 | 1 | 0 | 0 | backend-dev-2 |
| `server` (`GET /health`) | `cmd/server/main.go:118` | 1 | 1 | 0 | 0 | architect (esqueleto) |
| **Total** | | **153** | **32** | **21** | **100** | |

El pedido hablaba de 142 rutas en 13 dominios porque no contaba `fuentes` ni `balotario`.

## 2. Reparto

- **El reparto va por paquete Go, no por prefijo de ruta** (ADR 0012, decisión 1). Por ejemplo, `/admin/usuarios/{id}/estado` es de `auth` y `/admin/usuarios/{id}/rol` es de `admin`, y `/procesos` es de `inscripciones` mientras que `/procesos/{id}/asignaturas` es de `catalogs`.
- Los paquetes acoplados van juntos:
  - `auth` con `admin`;
  - `questions` con `fuentes` y `balotario` (citas y pasajes);
  - `practices` con `simulations` (los dos tienen `tiempo_agotado` con `resultado_id`).
- **backend-dev:** SERUMS IA, catálogos, práctica y simulacro (93 operaciones, 57 sin contrato).
- **backend-dev-2:** cuenta, administración, inscripciones y avisos (57 operaciones, 43 sin contrato), más el test `apispec` y su paso a modo estricto.
- Cada uno escribe **solo** sus `docs/api/openapi/paquetes/<paquete>.yaml` y su bloque reservado de `paths` en `openapi.yaml`. Si un esquema hace falta en los dos lados (por ejemplo, un `ProcesoResumen`), se sube a `components` de la raíz: se pide al architect, que lo añade, y no se duplica.

## 3. Orden de los PR

Las ramas siguen el GitFlow de CLAUDE.md (`feature/*` desde `develop`). La ADR 0012 está **aceptada** (2026-11-15) y entra a `develop` con el PR 0, antes del paso 1.

| # | PR | Autor | Contenido | Depende de |
|---|---|---|---|---|
| 0 | `feature/openapi-adr` | tech-lead | Esta ADR (ya aceptada) y este plan | — |
| 1a | `feature/openapi-skeleton` | architect | `docs/api/openapi/openapi.yaml`:<br>• `info` y `servers` (solo `http://localhost:8080`);<br>• `security` global y `securitySchemes.bearerAuth`;<br>• los 16 `tags`;<br>• `components`: `ErrorCodigo`, `ErrorTexto`, `ErrorTiempoAgotado`, `PaginaMeta`, `Page`, `PerPage`, `Limit` y las `responses` comunes (ADR 0012, decisión 2);<br>• `paths` con un bloque comentado y vacío por paquete, en el orden de la tabla del §1, con una línea en blanco entre bloques para que los merges en paralelo no choquen.<br>También `paquetes/server.yaml` (`GET /health`) y `redocly.yaml`, y la corrección del 413 en el README §3 (el middleware ya responde con código, `bodylimit.go:47`) | 0 |
| 1b | `feature/apispec-route-test` | backend-dev-2 | `backend/internal/apispec/` en **modo pendiente**, con `paquetesDocumentados = []string{"server"}` (ADR 0012, decisión 3), y `go.yaml.in/yaml/v3` pasada a directa | 1a |
| 1c | `feature/openapi-lint-docs` | devops | Lo que se describe en el §5 | 1a |
| 2 | un PR por grupo, en paralelo:<br>• A1 `catalogs` + `temas` + `perfiles`;<br>• A2 `questions` + `fuentes` + `balotario` + `ia`;<br>• A3 `practices` + `simulations`;<br>• B1 `auth` + `admin`;<br>• B2 `inscripciones` + `testimonios` + `notifications` + `salud` | backend-dev (A*), backend-dev-2 (B*) | Los archivos de paquete, los `$ref` en su bloque de la raíz y sus paquetes en `paquetesDocumentados` (una línea por paquete) | 1b y 1c |
| 3a | `feature/apispec-strict` | backend-dev-2 | Quita `paquetesDocumentados`: cualquier ruta sin documentar falla | todos los del paso 2 |
| 3b | `feature/openapi-fuente-de-verdad` | architect | README de `docs/api/`: «`openapi/` es la fuente de verdad de la forma; los `.md`, del diseño» (ADR 0012, decisión 4) | 3a |

- **`paquetesDocumentados`** vive en el test de backend-dev-2. backend-dev añade sus líneas en sus PR del paso 2. Es la única línea fuera de sus archivos.
- Cada PR corre en local `make lint` y `make test` y reporta los resultados en el PR, como manda CLAUDE.md. El tech-lead lo revisa con `/pr-review`.

### Checklist de cada PR de dominio (paso 2)

- [ ] Cada operación tiene `operationId` `<paquete>.<Handler>`, un tag, `x-rol` y `security` (o `security: []` si es pública).
- [ ] El `x-rol` sale de la cadena de middlewares de la línea en `routes.go`, no del prefijo. Ejemplo: `GET /admin/config/recursos` es `autenticado`.
- [ ] Los request y response salen del `dto.go` real: `json` tags, punteros como `[tipo, "null"]`, `omitempty` como no requerido y los `validate` como `minLength`, `maxLength`, `minimum`, `enum`…
- [ ] Cada status de error usa el formato que devuelve **ese** handler (`ErrorCodigo` o `ErrorTexto`) y lista sus códigos.
- [ ] Cada listado declara su modo de paginación real: estricto, tolerante, `limit`/`all` o sin paginar (ADR 0012, contexto 5).
- [ ] Los límites de tamaño se documentan donde aplican: 1 MB para JSON (413), multipart con el techo de su handler (`main.go:156-161`) y los rate limits (429).
- [ ] Los ejemplos son sintéticos (`@example.com`, `00000000`, `Persona Ejemplo`). Nada de `assets/`, de la base ni de logs.
- [ ] `redocly lint` está limpio y `go test ./internal/apispec/...` está en verde con el paquete en `paquetesDocumentados`.
- [ ] Si el código contradice un `.md`, se documenta el código y se avisa al architect con `archivo:línea` (ADR 0012, decisión 4).

## 4. Cobertura de los contratos `.md`

Las columnas de la tabla del §1 significan:

- **Completo:** el `.md` describe la forma de la operación (request, response y errores). Esto no garantiza que el código coincida: el spec se verifica igual contra el código (ADR 0012, decisión 4).
- **Delta:** solo describe un cambio. La forma base se lee del código.
- **Sin contrato:** se escribe **leyendo el código** (`routes.go`, `handler.go` y `dto.go`).

### 4.1 Con contrato completo (32)

| Operaciones | Contrato |
|---|---|
| `temas`: las 7 (T1–T7) | `serums-ia-e1-taxonomia.md` |
| `catalogs`: `GET /admin/procesos/{id}/pesos`, `GET` y `PUT /admin/procesos/{id}/pesos/{especialidadId}` (PE1–PE3) | `serums-ia-e1-taxonomia.md` |
| `perfiles`: las 3 (F1–F3) | `serums-ia-e1-taxonomia.md` |
| `fuentes`: S1–S7 | `serums-ia-e2-fuentes.md` |
| `fuentes`: `GET /admin/pasajes` | `serums-ia-e3-07-revision-basica.md` §2 |
| `balotario`: las 2 | `serums-ia-e3-01-e3-03-balotario.md` §3–4 |
| `ia`: las 4 | `serums-ia-e6-02-control-costos.md` §3 |
| `questions`: `PATCH /preguntas/{id}/estado`, `PUT /preguntas/{id}/citas` | `serums-ia-e3-07-revision-basica.md` §3–4, que es la versión definitiva de E3-02 §4.1–4.2 |
| `salud`: `GET /admin/salud` | `plataforma-salud.md` S1 |
| `server`: `GET /health` | `plataforma-salud.md` |
| `auth`: `PUT /auth/idioma` | `plataforma-avisos-idioma.md` §1.3 |

### 4.2 Solo delta (21): la forma base se lee del código

| Operaciones | Qué cubre el `.md` |
|---|---|
| `auth`: `POST /auth/register`, `/auth/verify-email`, `/auth/forgot-password`, `/auth/reset-password` | Solo `locale` (`auth-idioma-correos.md`) |
| `auth`: `POST /auth/login` | 503 con la BD caída (`plataforma-salud.md`) e `idioma` en `user` (`plataforma-avisos-idioma.md` §1) |
| `auth`: `GET /auth/me` | `idioma` (`plataforma-avisos-idioma.md` §1) |
| `inscripciones`: `GET /procesos`, `POST /inscripciones` | Elegibilidad por institución (`plataforma-multi-institucion.md` §2.2–2.3) |
| `inscripciones`: `PATCH /admin/inscripciones/{id}/estado` | Motivo y aviso (`plataforma-avisos-idioma.md` §2.1) |
| `practices`: `POST /practicas`; `simulations`: `POST /simulacros` | Elegibilidad (`plataforma-multi-institucion.md` §2.4) |
| `questions`: `GET /preguntas`, `POST /preguntas`, `GET`, `PUT` y `DELETE /preguntas/{id}`, `GET /preguntas/stats` | `tema_id` (`serums-ia-e1-taxonomia.md` T8) y estado, origen y citas (`serums-ia-e3-02-ciclo-preguntas.md` §4.3–4.5) |
| `catalogs`: `POST /admin/procesos/{id}/asignaturas` | 409 `peso_duplicado` (README §3) |
| `notifications`: `GET /notificaciones`, `GET /admin/notificaciones`, `DELETE /admin/notificaciones/{id}` | Avisos personales (`plataforma-avisos-idioma.md` §2.3) |

### 4.3 Sin contrato escrito (100): se escriben leyendo el código

- **`auth` (22):**
  - `GET /auth/check-dni` y `GET /auth/check-username`;
  - `GET /admin/personas` y `GET /admin/usuarios` (su filtro `institucion_id` es de PLT-10, que está diferida; §4.4);
  - `POST /auth/logout`;
  - `POST` y `DELETE /auth/tour/completar`;
  - `GET` y `PUT /admin/personas/{id}`;
  - `PATCH /admin/usuarios/{id}/estado` y `PATCH /admin/usuarios/{id}/email`;
  - `DELETE /admin/usuarios/{id}/tour` y `DELETE /admin/registro-limites`;
  - `POST /admin/personas/batch`, `/admin/personas/preview` y `/admin/personas/import` (los dos últimos son multipart);
  - `GET` y `PUT /admin/config/sesiones`;
  - `GET` y `PUT /admin/config/recursos` (el `GET` es `autenticado`);
  - `GET /admin/usuarios/online` y `GET /admin/usuarios/{id}/sesiones`.
- **`admin` (8):**
  - `GET /admin/dashboard` y `GET /admin/dashboard/actividad`;
  - `GET /admin/usuarios/{id}` y `PATCH /admin/usuarios/{id}/rol`;
  - `GET /admin/audit-log`;
  - `GET /admin/ingresos`, `GET /admin/ingresos/config` y `PUT /admin/ingresos/config/{anio}`.
- **`catalogs` (29):**
  - el CRUD de `/instituciones`, `/grados`, `/especialidades` y `/asignaturas`: 5 operaciones cada uno, con `limit`/`all`;
  - las 5 de `/admin/procesos` y `/admin/procesos/{id}`;
  - `GET /procesos/{id}/asignaturas` y `GET /admin/procesos/{id}/asignaturas`;
  - `PUT` y `DELETE /admin/procesos/{id}/asignaturas/{asignaturaId}`.
- **`inscripciones` (4):** `GET /stats/publico` y `GET /procesos/publico` (públicos), `GET /inscripciones/me` y `GET /admin/inscripciones` (PLT-10 diferida; §4.4).
- **`practices` (9):**
  - `DELETE /practicas/batch` y `GET /practicas`;
  - `GET` y `DELETE /practicas/{id}`;
  - `POST /practicas/{id}/responder`, `/practicas/{id}/validar/{preguntaId}` y `/practicas/{id}/finalizar`;
  - `GET /practicas/{id}/resultado` y `/practicas/{id}/revision`.
- **`simulations` (12):**
  - `GET /simulacros/historial`, `/simulacros/en-curso` y `/simulacros/configuracion`;
  - `GET /simulacros/configuraciones`, `POST /simulacros/configuracion` y `PATCH /simulacros/configuracion/{id}` (los tres admin);
  - `GET /simulacros/{id}`;
  - `POST /simulacros/{id}/responder`;
  - `PUT /simulacros/{id}/preguntas/{preguntaId}/respuestas`;
  - `POST /simulacros/{id}/finalizar`;
  - `GET /simulacros/{id}/resultado` y `/simulacros/{id}/revision`.
- **`questions` (7):**
  - `POST /preguntas/import` y `/preguntas/preview` (multipart);
  - `POST /preguntas/generate-set`;
  - `GET`, `POST`, `PUT` y `DELETE` de `/etiquetas`.
- **`notifications` (4):**
  - `GET /notificaciones/no-leidas`;
  - `POST /notificaciones/{id}/leer` y `POST /notificaciones/leer-todo`;
  - `POST /admin/notificaciones`.
- **`testimonios` (5):**
  - `GET /testimonios/publico` (público);
  - `GET /testimonios/me` y `POST /testimonios`;
  - `GET /admin/testimonios` y `PATCH /admin/testimonios/{id}/estado`.

### 4.4 Con contrato, pero sin rutas en el código

En `plataforma-multi-institucion.md`, dos partes están diferidas y **no entran en el spec** hasta que se reactiven sus historias:

- el §3 (PLT-07, catálogos por institución), que define `/admin/instituciones/{id}/{recurso}…`;
- el §5 (PLT-10), con los filtros `institucion_id` de `GET /admin/inscripciones`, `/admin/usuarios` y `/admin/personas`. El spec documenta esas tres rutas como están hoy. Cuando se reactive, el architect añade sus operaciones con `x-estado: planificado` (ADR 0012, decisión 3).

## 5. devops

1. **Makefile:**
   - en `lint` (`Makefile:95-101`), añadir `npx --yes @redocly/cli@2.72.1 lint docs/api/openapi/openapi.yaml --config docs/api/openapi/redocly.yaml`. La versión va en una variable del Makefile (`REDOCLY_VERSION := 2.72.1`) que usan también `docs/api` y el CI;
   - **regla de versión** (ADR 0012, decisión 5): no se fija una versión con menos de dos semanas de publicada. Al cambiarla se actualizan a la vez el Makefile, el CI y el comentario de `redocly.yaml`;
   - un target nuevo, `docs/api` (`redocly build-docs`, que genera `docs/api/openapi/dist/index.html`), con `##` para `make help`. No se usa `redocly preview`, que necesita un paquete de producto de Redocly (ADR 0012, decisión 6).
2. **`.gitignore`:** añadir `docs/api/openapi/dist/`.
3. **CI:** añadir el mismo `redocly lint` a un workflow que tenga Node (`frontend.yml`, o un job nuevo). El test `apispec` ya corre dentro de `go test ./...` (`backend.yml:76-77`) y no necesita ningún cambio, porque el checkout trae todo el repo.
4. **Visor:**
   - **Comprobado por devops (con `@redocly/cli` 2.72.1): sí carga un script de una CDN.**
     - El HTML de `build-docs` carga `https://cdn.redocly.com/redoc/v3.0.1/bundles/redoc.standalone.js` con `modulepreload`, integridad `sha384` (SRI) y `crossorigin="anonymous"`.
     - No usa Google Fonts.
     - **Para abrirlo hace falta red.** Sin conexión, la página no se muestra.
     - Según el ADR 0012, decisión 6, se acepta porque es solo para uso local. El SRI impide que se ejecute un script modificado en la CDN. Si se quisiera usar sin red o publicarlo en preprod, se decide en otro ADR (por ejemplo, servir `redoc.standalone.js` desde el repo).
     - Si cambia la versión de Redocly, cambian la URL y el hash: hay que volver a comprobarlos.
   - no se sirve desde el backend ni se despliega (ADR 0012, decisión 6).
5. Docker no cambia.

## 6. Dudas

- Sobre el contrato y las convenciones del spec: architect.
- Sobre el reparto y el orden: tech-lead.
- Si un handler responde algo que ningún esquema común cubre, se pregunta antes de inventar un esquema compartido.
