# ADR 0012: documentación de la API con OpenAPI 3.1, primero el spec

- **Estado:** aceptado (2026-11-15). Revisado por el tech-lead, que también confirmó el paso de `go.yaml.in/yaml/v3` a dependencia directa y los permisos de escritura en `docs/api/openapi/paquetes/`. El usuario aprobó `@redocly/cli`.
- **Fecha:** 2026-11-15
- **Pedido:** el usuario pidió documentar la API con OpenAPI/Swagger y dejó la elección al equipo. Esta ADR valida y ajusta la propuesta del tech-lead.
- **Plan de trabajo:** [docs/api/openapi-plan.md](../api/openapi-plan.md)
- **Relacionado:** convenciones de [docs/api/README.md](../api/README.md) §2 (autenticación), §3 (errores) y §5 (paginación)

## Contexto

1. **Tamaño real.** `cmd/server/main.go:117-149` monta **153 operaciones**: `GET /health` (`main.go:118`) y 152 rutas repartidas en **15 paquetes** de `backend/internal`. El pedido hablaba de 142 rutas en 13 dominios porque no contaba `fuentes` (8) ni `balotario` (2). El detalle por paquete está en el plan §1.
2. **Enrutado.** Las rutas usan el `http.ServeMux` de la biblioteca estándar con patrones `"MÉTODO /ruta/{param}"`. `ServeMux` no permite listar los patrones registrados: solo tiene `Handle`, `HandleFunc`, `Handler` y `ServeHTTP`. `fuentes` y `balotario` registran las rutas con constantes (`fuentes/routes.go:13-31`, `balotario/routes.go:12-13`), no con literales.
3. **El rol no se deduce del prefijo.** Lo decide la cadena de middlewares de cada línea:
   - `GET /admin/config/recursos` es solo autenticado (`auth/routes.go:57`; lo comprueba `auth/routes_test.go:52-60`).
   - Son admin sin `/admin`: `/preguntas*` y `/etiquetas*` (`questions/routes.go:34-51`), las altas y ediciones de catálogos (`catalogs/routes.go:25-48`) y la configuración de simulacros (`simulations/routes.go:30-32`).
   - Hay rutas públicas, con rate limit o sin él (`auth/routes.go:24-30`, `inscripciones/routes.go:17-18`, `testimonios/routes.go:15`).
4. **Hay dos formatos de error** (README §3):
   - el heredado `{"error": "<texto>"}` (`httpx.JSONError`, `internal/httpx/httpx.go:19-23`), que usan los middlewares de sesión y de rol, el 429 del rate limiter (`middleware/ratelimit.go:114-122`) y la mayoría de los handlers anteriores a SERUMS IA;
   - el formato con código `{"error", "message", "detalle"}` (`httpx.JSONErrorCode`, `httpx.go:91-95`). Lo usan los módulos nuevos y el 413 del middleware desde el #172 (`middleware/bodylimit.go:47`).

   Además, los 409 `tiempo_agotado` de prácticas y simulacros llevan `resultado_id` en la raíz y no dentro de `detalle` (`simulations/handler.go:58-63`, `practices/handler.go:60`).
5. **La paginación no es uniforme:**
   - **Estricta**, con `page` y `per_page` y `400` si el valor es malo: temas, fuentes y balotario (`temas/handler.go:276-280`, `fuentes/handler.go:249-252`, `balotario/handler.go:59-64`).
   - **Estricta con `limit`** en vez de `per_page`, que además acepta `limit=all`: catálogos (`catalogs/handler.go:136-148`).
   - **Propia:** `GET /preguntas` parsea `per_page` a mano (`questions/handler.go:167`). Su modo se confirma al escribir el paquete.
   - **Tolerante**, que ignora un valor malo y usa el defecto: admin, testimonios, inscripciones, prácticas y simulacros (`admin/handler.go:142-143`, `testimonios/handler.go:105-106`, `inscripciones/handler.go:74-75`, `practices/handler.go:287-288`, `simulations/handler.go:274-275`).
   - **Sin paginar**: `GET /notificaciones` devuelve un array de hasta 100 elementos (`plataforma-avisos-idioma.md` §3).
6. **Los contratos `.md` cubren poco.** De las 153 operaciones, 32 tienen un contrato completo, 21 solo un delta (el `.md` describe un cambio, no la forma base) y **100 no tienen contrato** (plan §4). Por eso el spec **no se escribe a partir de los `.md`**: se escribe a partir del código (`routes.go`, `handler.go` y `dto.go`), y los `.md` sirven de apoyo donde existen.
7. **Dependencias Go.** `go.yaml.in/yaml/v3 v3.0.4` ya está en el grafo como indirecta, vía viper (`backend/go.mod:41`). `make test` exige que `go mod tidy` quede limpio (`Makefile:87`).
8. **CI.** Solo corre con tags `v*` (`.github/workflows/backend.yml:5-7`). Antes de cada PR se corre en local `make lint` (`Makefile:95-101`) y `make test` (`Makefile:86-93`).

## Opciones consideradas

### 1. Cómo se produce el spec

- **A. Anotaciones en el código con swaggo (`swag`).** Descartada:
  - genera Swagger 2.0 («Swag converts Go annotations to Swagger Documentation 2.0», README de swaggo/swag);
  - no lee los patrones de `ServeMux`, así que cada ruta se repite en un comentario `@Router`, que se desincroniza sin que nada falle;
  - añade unos 150 bloques de comentarios a los handlers y archivos generados que hay que versionar.
- **B. Primero el código, con generación de servidor (ogen u oapi-codegen `std-http-server`).** oapi-codegen soporta 3.1 y genera servidores sobre `net/http`. Descartada ahora:
  - obliga a reescribir 152 handlers que funcionan, con sus middlewares por ruta, a las interfaces generadas, con su enrutado y su decodificación (con ogen, además, a su propio router en lugar de `ServeMux`, según su documentación; no se evaluó a fondo);
  - los dos formatos de error y las respuestas a medida (`resultado_id`) no encajan sin excepciones;
  - es una dependencia nueva de compilación y un riesgo alto sin beneficio para los criterios del pedido (documentar).

  Se podrá reconsiderar para un **módulo nuevo** en otra ADR.
- **C. Primero el spec: OpenAPI 3.1 escrito a mano y versionado, con un test que lo ata a las rutas reales.** **Elegida.** No toca los handlers, describe el comportamiento real, incluidas sus irregularidades, y la desincronización hace fallar `make test`.

### 2. Versión de OpenAPI

- **3.0.3.** Tiene el soporte más amplio, pero `nullable` ya está obsoleto y los esquemas no son JSON Schema.
- **3.1.0.** **Elegida.** Es JSON Schema 2020-12, y `null` se escribe como `type: [string, "null"]`, que es lo que devuelven los punteros de Go. Las herramientas futuras la soportan:
  - kin-openapi declara 3.1 como versión objetivo y `openapi3filter` valida peticiones y respuestas;
  - oapi-codegen soporta 3.0 y 3.1.

### 3. Uno o varios archivos

- **A. Un solo `openapi.yaml`.** Con más de 150 operaciones y dos backends escribiendo en paralelo, los conflictos de merge se concentrarían en `paths:`. Descartada.
- **B. Un archivo por ruta** (el diseño «split» de Redocly). Serían más de 110 archivos. Descartada por exceso.
- **C. Un archivo raíz con `info`, `security`, `tags`, `components` comunes y `paths` que solo tienen `$ref`, más un archivo por paquete Go.** **Elegida.** Cada dev toca sus archivos de paquete y su bloque de `paths` en la raíz, ya reservado por el esqueleto.

### 4. Cómo obtiene el test las rutas registradas

- **A. Interceptar el registro** con un registrador que envuelva al mux. Obliga a cambiar la firma de 15 `Mount`/`RegisterRoutes` (`*http.ServeMux` concreto). Descartada.
- **B. Sondear el mux armado con `ServeMux.Handler(r)`.** Solo sirve en un sentido (que lo del spec exista), no detecta rutas que falten en el spec y exige construir todas las dependencias de `main.go`. Descartada.
- **C. Analizar el código fuente con `go/parser` y `go/ast`**, que son de la biblioteca estándar. **Elegida:**
  - recorre los `.go` (sin `_test.go`) de `backend/internal/**` (sin `internal/db/dbtest`) y `backend/cmd/server/main.go`;
  - toma cada llamada `X.Handle(…)` o `X.HandleFunc(…)` cuyo primer argumento es un literal o una constante de paquete con un literal;
  - **si el primer argumento no se puede resolver, el test falla** con `archivo:línea`, así que ninguna ruta se pierde en silencio;
  - del segundo argumento deduce el rol (decisión 5).

### 5. Lint y visor

- **Spectral** solo hace lint, así que haría falta otra herramienta para el visor y el bundle.
- **Redocly CLI.** **Elegida.** Una sola herramienta para:
  - `lint`, con reglas integradas y reglas configurables (por ejemplo, exigir `x-rol`);
  - `bundle`, que junta los archivos en uno;
  - `build-docs`, que genera un HTML estático con Redoc.
- **Visor:**
  - **Swagger UI** y **Scalar** traen «probar ahora», que invita a lanzar peticiones con un token real contra un entorno. Descartados por ahora.
  - **Redoc**, de solo lectura, generado por la misma herramienta. **Elegido.**
- **Servir el visor desde el backend**, aunque sea con un flag desactivado por defecto. Descartado por YAGNI: añade superficie y archivos estáticos al binario de producción sin que haga falta. El HTML se genera en local con `make` y no se versiona. Si el usuario quiere publicarlo en preprod, se decide en otra ADR.

## Decisión

1. **El spec se escribe primero, en OpenAPI 3.1.0 y a mano:**
   - **Raíz:** `docs/api/openapi/openapi.yaml`, con `info`, `servers` (solo local), `security`, `tags`, `components` comunes y `paths`, donde cada entrada es solo un `$ref`.
   - **Un archivo por paquete Go:** `docs/api/openapi/paquetes/<paquete>.yaml`, con dos claves de primer nivel:
     - `paths:`, con los path items completos;
     - `schemas:`, con los esquemas propios del paquete.
   - `GET /health` va en `paquetes/server.yaml` (paquete `cmd/server`).
   - Una ruta pertenece al paquete cuyo `routes.go` la registra, no a su prefijo. Por ejemplo, `/admin/usuarios/{id}/estado` es de `auth` y `/admin/usuarios/{id}/rol` es de `admin`. Hoy ningún path se comparte entre dos paquetes. Si alguna vez pasa, el path item vive en el paquete del primer método registrado y el test lo admite.
   - **Formato de la raíz:**

     ```yaml
     paths:
       # ── auth ── (backend-dev-2)
       /auth/login:
         $ref: './paquetes/auth.yaml#/paths/~1auth~1login'
     ```

2. **Convenciones del spec:**

   | Tema | Regla |
   |---|---|
   | Seguridad | `components.securitySchemes.bearerAuth: {type: http, scheme: bearer, bearerFormat: JWT}` y `security: [{bearerAuth: []}]` global. Las rutas públicas declaran `security: []` |
   | Rol | Extensión obligatoria `x-rol` en cada operación: `publico`, `autenticado` o `admin`. El test la compara con la cadena de middlewares del código (decisión 3). La `description` de la operación repite el rol en texto para el visor |
   | Respuestas por rol | `autenticado` y `admin` incluyen `401: $ref NoAutenticado` (y `503: $ref ServicioNoDisponible`); `admin` incluye además `403: $ref AccesoDenegado` |
   | Errores | Dos esquemas en `components/schemas`:<br>• `ErrorCodigo`: `error` (requerido, `pattern: '^[a-z0-9_]+$'`), `message` (requerido) y `detalle` (objeto opcional), sin `additionalProperties: false`, para que se pueda extender con `allOf`. Por ejemplo, `ErrorTiempoAgotado` = `ErrorCodigo` + `resultado_id`;<br>• `ErrorTexto`: `error` (requerido, texto), el formato heredado.<br>Cada respuesta de error usa **el formato que devuelve el código**. El spec no normaliza en el papel: si un handler heredado responde texto, el spec dice `ErrorTexto` |
   | Respuestas reutilizables | `components/responses`:<br>• `NoAutenticado` (401) y `AccesoDenegado` (403), con `ErrorTexto`;<br>• `ServicioNoDisponible` (503, con `ErrorTexto`: `RequireAuth` cuando falla la BD, `auth/middleware.go:59`, ADR 0005);<br>• `DemasiadasSolicitudes` (429, con `ErrorTexto`, rate limiter);<br>• `CuerpoDemasiadoGrande` (413, con `ErrorCodigo`: `cuerpo_demasiado_grande` o `archivo_demasiado_grande`);<br>• dos 500: `ErrorInternoCodigo` (con `ErrorCodigo` y `error_interno`, `httpx.InternalErrorCode`) y `ErrorInternoTexto` (con `ErrorTexto`, `httpx.InternalError`). Cada operación usa la que llama su handler |
   | Códigos de error | En las respuestas con `ErrorCodigo`, la `description` de cada status lista los códigos posibles. Por ejemplo, `422: tema_duplicado, limite_temas, validacion`. Es el catálogo del README §4 |
   | Paginación | `components/parameters`: `Page` (`page`, entero ≥ 1, defecto 1), `PerPage` (`per_page`, defecto 20, máx. 100) y `Limit` (`limit`, entero o `all`, solo para catálogos). `components/schemas/PaginaMeta`: `total`, `page` y `per_page`. Cada listado paginado es `allOf: [PaginaMeta, {data: array de <Item>}]`. La `description` del parámetro dice si es **estricto** (400 `parametro_invalido`) o **tolerante** (cae al defecto), según el código (contexto 5). Las listas sin paginar se documentan como son (array o `{data}`) |
   | Tags | Un tag por paquete Go, con el nombre del paquete (`auth`, `catalogs`, `fuentes`…) y una `description` en español con el dominio. Cada operación lleva exactamente un tag |
   | `operationId` | `<paquete>.<NombreDelHandler>` (`temas.Crear`, `auth.Login`), único. Así se puede rastrear hasta `routes.go` |
   | Parámetros de ruta | El mismo nombre que en el patrón Go (`{id}`, `{asignaturaId}`, `{especialidadId}`, `{anio}`, `{preguntaId}`), de tipo `integer`, `minimum: 1`, salvo que el handler diga otra cosa |
   | JSON | `snake_case`, fechas `format: date-time` (RFC 3339 UTC) y los nulos como `type: [<tipo>, "null"]` (README §1) |
   | Ejemplos | Sintéticos y evidentes: personas `Persona Ejemplo`, correos `@example.com` (RFC 2606), DNI `00000000`, teléfonos `900000000` y textos académicos inventados. **Nunca** se copian datos de `assets/`, de la base ni de logs. El repo es público |
   | Idioma | `summary` y `description` en español, como la API (`message`) |
   | Estado de una operación | `x-estado: planificado` marca una operación que tiene contrato pero aún no está implementada (decisión 3). No se usa para nada más |

3. **Guardia contra la desincronización:** un test de Go en un paquete nuevo, `backend/internal/apispec/`, con `doc.go` y `apispec_test.go`.
   - **Lectura del spec:** lee la raíz con `go.yaml.in/yaml/v3`, que pasa de indirecta a directa en `go.mod`. No es un módulo nuevo: ya está en el grafo (contexto 7). Sigue cada `$ref` (archivo y puntero JSON, con `~1` → `/`) y obtiene el conjunto de `MÉTODO /ruta` con su `x-rol`, su `x-estado` y su tag.
   - **Lectura del código:** usa `go/parser` y `go/ast` (opción 4C). El rol se deduce del segundo argumento de `Handle`/`HandleFunc`, mirando el nombre de cada función llamada (el `Sel` en los selectores):
     - **Sin efecto de rol:**
       - `HandlerFunc`;
       - `Limit` (rate limit, `auth/routes.go:24-30`);
       - los valores de método `h.X` (no son llamadas);
       - las fábricas de handler listadas de forma explícita en el test. Hoy solo `healthHandler` (`main.go:118`).
     - **Rol:**
       - `admin`, `requireAdmin` o `RequireAdmin` dan `admin`;
       - si no, `authn`, `auth`, `requireAuth` o `RequireAuth` dan `autenticado`;
       - si no hay ninguna, `publico`.
     - **Cualquier otra llamada** hace fallar el test con `archivo:línea`, para que alguien decida si es un middleware de rol o una fábrica y la añada a la lista que corresponde.
     - Las formas de hoy, revisadas una por una: `authn(admin(http.HandlerFunc(h.X)))`, `requireAuth(requireAdmin(…))`, `auth(admin(…))`, `authn(auth.RequireAdmin(…))` (`salud/routes.go:13`), `checkRL.Limit(…)`/`authRL.Limit(…)` y `mux.HandleFunc(patrón, h.X)` (`inscripciones/routes.go:17-18`, `testimonios/routes.go:15`).
   - **Siempre falla si:**
     - una operación del spec no existe en el código y no tiene `x-estado: planificado`;
     - una operación con `x-estado: planificado` ya existe en el código (hay que quitar la marca);
     - el `x-rol` no coincide con el rol del código;
     - una operación no tiene `x-rol` ni exactamente un tag;
     - una ruta del código no se puede resolver;
     - una clave de `paths` de un archivo de paquete no está referenciada desde la raíz.
   - **Modo pendiente, mientras se escribe:** una variable `paquetesDocumentados` lista los paquetes terminados, uno por línea.
     - En esos paquetes, una ruta del código que falte en el spec hace fallar el test.
     - En los demás, solo se registra con `t.Log`.
     - Cada PR de dominio añade su paquete, y la lista solo crece.
   - **Modo estricto, al final:** se elimina la lista y cualquier ruta que falte falla.
   - **Entorno:** el test no necesita base de datos, así que corre en `make test` sin `TEST_DATABASE_URL` especial. Ubica la raíz del repo subiendo desde el paquete hasta `go.mod` y luego `..`.
4. **Fuente de verdad, desde el PR estricto:**
   - `docs/api/openapi/` es la fuente de verdad de la **forma** de cada endpoint: ruta, método, rol, parámetros, request, response y códigos de error.
   - Los `.md` de `docs/api/` siguen siendo el contrato de **diseño**: reglas, orden de validación, modelo de datos, migraciones, auditoría y trazabilidad.
   - **Quién lo mantiene:**
     - **Contrato nuevo:** el architect escribe el `.md` y el delta del spec con `x-estado: planificado`, en el mismo PR de docs.
     - **Implementación:** el backend que implementa quita la marca y ajusta el spec a lo que implementó.
     - **Revisión:** el tech-lead revisa los dos en `/pr-review`.
   - **Si el código contradice al `.md`** mientras se escribe el spec, el spec documenta el código y el dev avisa al architect con `archivo:línea`. El architect decide si es un bug (se abre un issue, siguiendo la regla de «bugs al final») o si hay que corregir el `.md`. El spec no se escribe con el comportamiento deseado.
5. **Lint:** `@redocly/cli` **2.72.1**, publicada el 2026-10-29, con un `redocly.yaml` en `docs/api/openapi/`.
   - **Regla de versión:** no se fija una versión con menos de dos semanas de publicada. Así una release rota o comprometida tiene tiempo de detectarse y retirarse. La versión se cambia en `redocly.yaml`, en el Makefile y en el CI a la vez.
   - Necesita Node ≥ 20.19 o ≥ 22.12.
   - **Reglas** (ids comprobados en la lista de reglas integradas de Redocly):
     - el preset `recommended`;
     - `operation-operationId`, `operation-operationId-unique`, `operation-tag-defined` y `operation-singular-tag` como error;
     - la regla configurable `rule/operation-x-rol` (`subject: {type: Operation, property: x-rol}`, `assertions: {defined: true, enum: [publico, autenticado, admin]}`);
     - `no-unused-components` como aviso.
   - **Dónde se integra:** en `make lint` (`Makefile:95-101`). Como el CI corre lo mismo, devops lo añade también al workflow.
6. **Visor:** `redocly build-docs` genera `docs/api/openapi/dist/index.html`, que no se versiona (va en `.gitignore`).
   - Se genera con el target `make docs/api` y se abre en el navegador. **No se usa `redocly preview`**, porque necesita un proyecto y un paquete de producto de Redocly (documentación de los comandos de Redocly CLI).
   - **No se sirve desde el backend ni se publica en producción.** Publicarlo en preprod requiere una decisión del usuario y otra ADR.
   - devops comprueba si el HTML carga scripts desde una CDN. Si los carga, se acepta solo para uso local, y la nota va en `docs/api/openapi-plan.md`.
7. **Fuera de alcance (posibles ADR posteriores):**
   - validar las respuestas reales de los tests de integración contra el spec con kin-openapi `openapi3filter`, que es una dependencia nueva;
   - generar tipos TypeScript para `frontend/src/lib/*/api.ts`;
   - generar el servidor para módulos nuevos.

## Dependencias (aprobadas el 2026-11-15)

| Herramienta | Uso | Dónde corre | Estado |
|---|---|---|---|
| `@redocly/cli` 2.72.1 (invocada con `npx`; decisión 5) | lint, bundle y HTML estático | `make lint`, `make docs/api` y el CI | **aprobada por el usuario** (versión fijada, invocada con `npx`) |
| `go.yaml.in/yaml/v3` (de indirecta a directa) | leer el spec en el test | `go test` | **aprobada por el tech-lead**. Ya está en el grafo (`go.mod:41`), así que no entra ningún módulo nuevo |

**Permisos:** backend-dev y backend-dev-2 escriben en `docs/api/openapi/paquetes/`, cada uno solo en los archivos de sus paquetes (plan §2). La raíz `openapi.yaml` y `redocly.yaml` son del architect, salvo el bloque de `paths` que cada dev tiene reservado.

## Consecuencias

- **Positivas:**
  - Documentación en 3.1, legible y con el rol verificado contra el código: es un control de seguridad, no solo documentación.
  - Si alguien añade una ruta y no la documenta, o cambia el rol de una ruta y no el spec, falla `make test`.
  - Los handlers no se tocan. Los dos backends trabajan en paralelo en archivos distintos.
- **Negativas:**
  - El test no verifica los cuerpos ni los status. Un cambio de DTO sin actualizar el spec no falla hasta que se adopte la validación con kin-openapi (decisión 7). Mientras tanto, lo cubre la revisión del tech-lead: todo PR que toque un `dto.go` o un `handler.go` debe tocar el spec o justificar por qué no.
  - El análisis AST depende de cómo se registran las rutas hoy: `X.Handle`/`HandleFunc` con un literal o una constante. Una forma nueva (por ejemplo, un bucle sobre una tabla) hace fallar el test hasta que se adapte. Es intencional.
  - El spec documenta irregularidades reales (dos formatos de error, tres modos de paginación). Unificarlas es trabajo aparte, con issues, y no bloquea esta ADR.
- **Sin migraciones ni cambios de comportamiento de la API.**
