# Contratos de API — módulo SERUMS IA

Convenciones transversales para todos los endpoints del módulo «Aprendizaje asistido por IA para SERUMS» (`docs/stories/serums-ia/`). Cada épica tiene su archivo con endpoints, modelo de datos, migraciones y trazabilidad. Este documento fija lo que comparten y lo ancla al código existente.

| Documento | Alcance | Estado |
|---|---|---|
| [serums-ia-e1-taxonomia.md](serums-ia-e1-taxonomia.md) | E1-01, E1-02, E1-03, E1-04 (MVP) | vigente (2026-11-11) |
| [serums-ia-e2-fuentes.md](serums-ia-e2-fuentes.md) | E2-02, E2-03, E2-05, E2-06, E2-07 (MVP) | aprobado, pendiente de implementación (2026-11-11) |

ADRs: [0001](../adr/0001-unicidad-proceso-asignaturas-por-especialidad.md) unicidad de `proceso_asignaturas`, [0002](../adr/0002-secreto-hmac-contadores-antifraude.md) secreto HMAC antifraude, [0003](../adr/0003-puerto-source-repository-y-adaptador-postgres.md) `SourceRepository`.

Un cambio a un contrato ya publicado se avisa directamente a backend-dev, frontend-dev y al tech-lead.

---

## 1. Estructura del backend

- Un paquete por módulo en `backend/internal/<modulo>` con `handler.go`, `service.go`, `repository.go`, `dto.go`, `errors.go` y `routes.go`. Patrón de referencia: `internal/catalogs`.
- **Montaje:** función `Mount(mux, svc, audLog *audit.Logger, jwtSecret, checkSession)`, con la misma firma que `inscripciones.Mount` (`cmd/server/main.go:124`). Se llama desde `cmd/server/main.go:123-131`. Los módulos nuevos reciben el `audLog` de `main` en vez de construir otro, que es la deuda anotada en `catalogs/routes.go:13-17` (H86).
- **Rutas** con el `ServeMux` de Go 1.22+ (`"METHOD /ruta/{param}"`). Las rutas admin van bajo `/admin/…`. Las del postulante en el módulo van bajo `/serums-ia/…`. Las lecturas de catálogo para cualquier autenticado se quedan junto al recurso (p. ej. `GET /asignaturas/{id}/temas`, como `GET /asignaturas`, `catalogs/routes.go:44`).
- **IDs de ruta:** enteros > 0. Si no, `400 parametro_invalido` (equivale a `pathID` de `catalogs/handler.go:85-89`).
- **JSON:** `snake_case`. Las fechas y horas van en RFC 3339 UTC (`time.Time` serializado), p. ej. `"2026-11-11T15:04:05Z"`. Los decimales de peso y coeficiente van como número con ≤ 2 decimales (validación `dec2`, `catalogs/handler.go:21-36`). En `PUT`, los campos opcionales ausentes y `null` significan «sin valor», salvo que el endpoint diga otra cosa. Excepción vigente: `tema_id` en `PUT /preguntas/{id}`, donde ausente conserva el valor actual y `null` lo quita (E1 T8, como `etiqueta_ids`).
- Los handlers decodifican con `DisallowUnknownFields`: un campo desconocido da `400 cuerpo_invalido`.
- **Plegado de texto:** las comparaciones sin mayúsculas ni tildes (nombre de tema, búsqueda de pasajes) usan una única función, `texto.Plegar` (`internal/texto/texto.go`). La creó SIA-E1-01 y la reutiliza SIA-E2-02. Pliega igual una tilde compuesta (NFC) que una descompuesta (NFD) y conserva la `ñ`. Las reglas están en la ADR 0003 § Plegado.

## 2. Autenticación y roles

| Capa | Dónde | Comportamiento |
|---|---|---|
| Sesión | `auth.RequireAuth` (`internal/auth/middleware.go:33-65`) | Bearer JWT y sesión viva en la BD. El rol y el estado se leen de la BD en **cada** request (`:59-61`). Sin sesión o con sesión revocada responde 401 |
| Rol admin | `auth.RequireAdmin` (`internal/auth/middleware.go:69-78`) | `claims.Rol != "admin"` responde 403. Se encadena después de `RequireAuth`: `authn(admin(h))` (`catalogs/routes.go:19-25`) |
| Identidad | `auth.ClaimsFromContext` (`middleware.go:81-84`) | `UserID`, `Rol`, `GradoID`, `EspecialidadID` e `InstitucionID` (`internal/jwt/jwt.go:17`) |
| Inscripción | `examen.InscripcionAprobada` (`internal/examen/examen.go:141-149`) | Los endpoints del postulante que dependen de un proceso la verifican en el servidor. Si falta, `403 inscripcion_requerida` (supuesto 3 del README de historias; mismo criterio que `practices/errors.go:28` y `simulations/errors.go:27`) |

- Roles: solo `admin` y `usuario` (el «postulante»). No se crean roles nuevos (supuesto 2).
- **Autorización por recurso:** ningún endpoint del postulante recibe un `usuario_id`. El usuario sale siempre de `claims.UserID`. Un recurso ajeno o inexistente responde `404` con el mismo código, para no revelar si existe.

## 3. Formato de error

El repo tiene dos formatos:

1. `{"error": "<texto>"}`: `httpx.JSONError` (`internal/httpx/httpx.go:17-22`) y los `jsonError` locales (`catalogs/handler.go:73-77`).
2. `{"error": "<codigo>", "message": "<texto>", …}`: el único precedente con código, `tiempo_agotado` y `simulacro_en_curso` en `simulations/handler.go:58-70,85-88` y `practices/handler.go:60,82`. El frontend ya lo consume como código (`frontend/src/lib/practices/api.ts:25-38`, `frontend/src/lib/simulations/api.ts:26-32`).

**Todo endpoint del módulo SERUMS IA usa el formato 2:**

```json
{ "error": "tema_duplicado", "message": "Ya existe un tema con ese nombre en esta área.", "detalle": { "campo": "nombre" } }
```

| Campo | Regla |
|---|---|
| `error` | Código estable en `snake_case`, que es el contrato. Nunca cambia por un cambio de texto |
| `message` | Texto en español para logs del cliente y como respaldo. La UI no lo muestra si existe la traducción del código (§8) |
| `detalle` | Opcional, objeto sin datos personales (p. ej. `{"suma_pesos": 95.0}`, `{"campos": {"nombre": "min"}}`) |

- **Helper:** backend-dev añade a `internal/httpx` una función `JSONErrorCode(w, status, code, msg string, detalle map[string]any)`, junto a `JSONError`, para no duplicar `jsonError` por paquete.
- **500:** siempre `{"error": "error_interno", "message": "Error interno del servidor."}`. La causa se registra con `op` y sin datos personales, como `httpx.InternalError` (`httpx.go:24-34`).
- **Excepciones (respuestas que no salen del handler):**
  - `RequireAuth` y `RequireAdmin` responden `{"error": "No autenticado." | "Sesión revocada." | "Acceso denegado."}` (`auth/middleware.go:38,44,52,57,73`);
  - `MaxBodySize` responde `413 {"error": "El cuerpo de la solicitud es demasiado grande."}` (`middleware/bodylimit.go:42-44`) cuando el `Content-Length` declarado ya supera el límite.

  El frontend trata esos tres status (401, 403 y 413 de middleware) **por status** y no por código. Si hay `message`, el código está en `error`; si no, `error` es texto.
- **Cuerpo demasiado grande en el handler (convención):** sin `Content-Length` (p. ej. `Transfer-Encoding: chunked`), el middleware no rechaza la petición: envuelve el cuerpo en `http.MaxBytesReader` (`bodylimit.go:46`). Pasado el límite, la lectura falla dentro del handler con `*http.MaxBytesError`.
  - **Regla:** todo handler del módulo que decodifica un cuerpo JSON comprueba `var mbe *http.MaxBytesError; errors.As(err, &mbe)` **antes** de responder `400 cuerpo_invalido`. Si es ese error, responde `413` con el formato 2: `{"error": "cuerpo_demasiado_grande", "message": "El cuerpo de la solicitud es demasiado grande."}`.
  - Precedente: hoy solo `auth` hace este mapeo (`internal/auth/handler.go:58-65`), con el formato heredado. Los handlers de temas responden 400 (`internal/temas/handler.go:255-264`, `decodificar`) y deben corregirse (tarea de backend-dev).
  - Las subidas multipart siguen usando su propio código, `413 archivo_demasiado_grande` (E2 S1 paso 1).
  - Alcance: los endpoints nuevos del módulo con cuerpo. Los endpoints existentes fuera del módulo no cambian.
  - El frontend trata `cuerpo_demasiado_grande` por código, con su clave i18n (§8). El 413 del middleware sigue tratándose por status.
- **Frontend (tarea de frontend-dev):** los clientes usan **`body.message ?? body.error`** como texto y guardan el código aparte:

  ```ts
  Object.assign(new Error(body.message ?? body.error), {
    status: res.status,
    code: body.message ? body.error : undefined,
    detalle: body.detalle,
  })
  ```

  - Así funcionan los dos formatos. Con el heredado (`{"error": "<texto>"}`), `message` no existe y se muestra `error`. Con el nuevo, se muestra `message` (o la traducción de `code`).
  - Los clientes nuevos (`src/lib/serums-ia/*/api.ts`) lo traen desde el inicio.
  - Hay que cambiar dos clientes existentes, porque hoy usan `body.error` como mensaje y mostrarían el código: `frontend/src/lib/catalogs/api.ts:14-15` y `frontend/src/lib/questions/api.ts:15,31`.
  - **No** se tocan `practices/api.ts` ni `simulations/api.ts`. Ya interpretan `body.error` como código en sus 409 (`errorCode`, `tiempoAgotado`) y sus endpoints no cambian.
- **Endpoints existentes que cambian de formato** (solo en las respuestas indicadas; el resto de sus errores siguen igual):

  | Endpoint | Respuesta nueva | Historia |
  |---|---|---|
  | `POST /admin/procesos/{id}/asignaturas` (`catalogs/routes.go:60`) | 409 `{"error": "peso_duplicado", "message": …}` (hoy, texto en `catalogs/repository.go:796`) | E1-03 (PE5) |
  | `POST /preguntas`, `PUT /preguntas/{id}` (`questions/routes.go:37,39`) | 422 `{"error": "tema_invalido", "message": …}` cuando falla el `tema_id` nuevo | E1-01 (T8) |

  Ningún otro endpoint existente cambia. Los endpoints nuevos del módulo nacen con el formato nuevo.

## 4. Códigos HTTP y códigos de error comunes

| HTTP | Uso en el módulo | Códigos comunes |
|---|---|---|
| 200 | Lectura o mutación con cuerpo | — |
| 201 | Creación síncrona | — |
| 202 | Aceptado para proceso asíncrono (ingesta de fuentes) | — |
| 204 | Borrado sin cuerpo | — |
| 400 | Sintaxis: JSON mal formado, campo desconocido, id o query no numérico, multipart inválido | `cuerpo_invalido`, `parametro_invalido` |
| 401 | Sin sesión (middleware) | texto del middleware |
| 403 | Sin rol (middleware) o sin inscripción aprobada | `inscripcion_requerida` |
| 404 | Recurso inexistente o ajeno | `no_encontrado` o uno específico (`profesion_no_definida`) |
| 409 | Estado del recurso incompatible con la acción, duplicados de identidad, datos desactualizados | `estado_invalido`, `orden_desactualizado`, `peso_duplicado`, `fuente_duplicada`, `confirmacion_requerida`, `formato_no_configurado`, `tiempo_agotado` |
| 413 | Archivo o cuerpo demasiado grande | `archivo_demasiado_grande` (multipart), `cuerpo_demasiado_grande` (JSON leído en el handler, §3), texto del middleware |
| 422 | Validación semántica y límites de negocio | `validacion`, `referencia_invalida`, `limite_*`, más los específicos de cada épica |
| 429 | Límite de uso (cuotas de IA, exportaciones; épicas posteriores) | `limite_uso` |
| 500 | Error inesperado | `error_interno` |

- Los handlers existentes mezclan 400 y 422 para la validación (`catalogs/handler.go:100,190`). En el módulo, **toda** validación de reglas da 422 y 400 queda solo para errores de sintaxis.
- Las validaciones se evalúan en el orden que fija cada endpoint y se devuelve el primer error.

Catálogo de códigos usados por E1 y E2:

| Código | HTTP | Épica |
|---|---|---|
| `cuerpo_invalido`, `parametro_invalido` | 400 | todas |
| `inscripcion_requerida` | 403 | E1-04 |
| `no_encontrado` | 404 | todas |
| `profesion_no_definida` | 404 | E1-04 |
| `estado_invalido` | 409 | E1-01, E1-04, E2-06, E2-07 |
| `orden_desactualizado` | 409 | E1-02 |
| `peso_duplicado` | 409 | E1-03 |
| `confirmacion_requerida` | 409 | E1-04 |
| `fuente_duplicada` | 409 | E2-03, E2-06 (`detalle`: `fuente_id`, `titulo`, `estado`, `version`) |
| `formato_no_configurado` | 409 | E1-03 (lo expone E9-05) |
| `archivo_demasiado_grande` | 413 | E2-03 |
| `cuerpo_demasiado_grande` | 413 | todas (endpoints con cuerpo JSON) |
| `validacion`, `referencia_invalida` | 422 | todas |
| `tema_duplicado`, `limite_temas`, `orden_invalido`, `tema_invalido` | 422 | E1-01, E1-02 |
| `pesos_no_suman_100`, `limite_preguntas` | 422 | E1-03 |
| `especialidad_invalida` | 422 | E1-04 |
| `archivo_invalido`, `limite_fuentes`, `limite_versiones` | 422 | E2-03, E2-06, E2-07 |
| `consulta_invalida`, `cita_invalida` | — (errores del puerto Go, sin HTTP en MVP) | E2-02 |
| `error_interno` | 500 | todas |

## 5. Paginación

- Query: `page` (base 1) y `per_page` (**20 por defecto, máx. 100**; README de historias:175). Se parsean con `httpx.ParsePage` y `httpx.ParsePerPage` (`internal/httpx/httpx.go:56-65`), igual que `admin/handler.go:142-143` y `testimonios/handler.go:105-106`.
- Modo **estricto**, como catálogos (`catalogs/handler.go:131-146`): un número mal formado o ≤ 0 da `400 parametro_invalido`. Un `per_page` > 100 se recorta a 100 y una página más allá del final devuelve `data: []`.
- Respuesta: `{"data": [...], "total": n, "page": p, "per_page": pp}`, el mismo sobre que `questions/dto.go:129-132`, `practices/dto.go:163-166` y `admin/dto.go:75-78`.
- Orden siempre total: cada listado fija su orden con `id` como desempate, para que la paginación sea estable.
- Las listas acotadas por regla (temas activos de un área, ≤ 60) se devuelven sin paginar dentro de `{"data": [...]}`.

## 6. Límites de entrada

| Límite | Valor | Dónde se aplica |
|---|---|---|
| Cuerpo JSON | 1 MB: 413 del middleware si el `Content-Length` lo supera; `413 cuerpo_demasiado_grande` del handler si se supera al leer (§3) | `middleware/bodylimit.go:10,42-46`, global en `cmd/server/main.go:138-144` |
| Subida multipart | techo global de 20 MB solo para rutas registradas; cada handler aplica su propio `http.MaxBytesReader` | `bodylimit.go:11,26-49`. Una ruta de subida nueva **debe** añadirse a la lista de `main.go:138-143`, que compara `r.URL.Path` exacto: no puede llevar `{id}` |
| Archivo `.md` de fuente | 2 MiB, 20 000 líneas, UTF-8, `.md`, `text/markdown` o `text/plain` | E2 S1/S2 |
| Texto libre (notas, motivos) | ≤ 500 caracteres; el mínimo lo fija cada regla | README de historias:174 |
| Strings | Se miden en caracteres (runes) después de `TrimSpace` | todas |
| Arrays de ids en el request | máx. 50 elementos, sin repetidos, salvo que el endpoint diga otra cosa | todas |
| Búsqueda por texto en listados | ≤ 100 caracteres, `ILIKE` con `%` y `_` escapados | E2 S3 |

## 7. Auditoría (`audit_log`)

- Tabla `audit_log` (`backend/migrations/006_create_audit_log.sql:2-12`): `usuario_id` (actor; `NULL` = sistema), `accion` (≤ 50), `entidad` (≤ 50), `entidad_id`, `detalle JSONB`, `ip` y `user_agent`.
- **Toda acción de admin** del módulo se audita (README de historias:182). Las mutaciones en transacción usan `audit.Logger.LogTx` dentro de esa transacción (`internal/audit/audit.go:52-71`), con el patrón `auditTx` y `audit.RunTx` de `catalogs/handler.go:59-69` y `catalogs/repository.go:690-693`. Si la auditoría falla, la mutación se revierte.
- **Nombres:** las historias fijan el formato `<entidad>.<verbo>` (`tema.crear`, `fuente.crear`). Es distinto del formato heredado `crear_institucion` (`catalogs/handler.go:198`). Las acciones nuevas usan el formato de las historias porque está en sus criterios de aceptación.

| Acción | Entidad / `entidad_id` | Historia |
|---|---|---|
| `tema.crear`, `tema.editar`, `tema.archivar`, `tema.reactivar` | `tema` / id | E1-01, E1-02 |
| `tema.reordenar` | `asignatura` / id | E1-02 |
| `pesos.guardar` | `proceso` / id | E1-03 |
| `fuente.crear`, `fuente.version_crear`, `fuente.editar`, `fuente.retirar`, `fuente.restaurar`, `fuente.reintentar`, `fuente.marcar_oficial`, `fuente.desmarcar_oficial` | `fuente` / id | E2-03, E2-06 |
| `fuente.procesar`, `fuente.fallar` (actor `NULL`, worker) | `fuente` / id | E2-03, E2-06 |

- **`detalle`:** antes y después resumido, estados anterior y nuevo, ids y hashes. **Nunca** lleva datos personales, el contenido de fuentes ni textos de preguntas. Los motivos y notas que escribe el admin sí se guardan, porque la regla 9 de E2-06 lo exige.
- Las acciones del postulante (p. ej. elegir profesión) no se auditan en `audit_log`.

## 8. i18n

- La API responde solo en español (`message`). La UI traduce por código: clave `dashboard.serumsIa.errores.<codigo>` en `frontend/src/i18n/messages/es.json` y `en.json`. Toda clave existe en ambos archivos (CLAUDE.md y supuesto 8). Si un código no tiene clave, la UI muestra `message`.
- Los estados y enums (`activo`/`archivado`, `procesando`/`activa`/`fallida`/`retirada`, `completa`/`incompleta`, `tipo` de fuente y `motivo_fallo`) viajan como códigos y la UI los traduce con `dashboard.serumsIa.<dominio>.<enum>.<valor>`.
- El contenido (nombres de temas, títulos y pasajes de fuentes) es solo en español y no se traduce.

## 9. Seguridad desde el diseño

- **Sin IDOR:** ver §2. Los endpoints admin también validan que los ids relacionados existan y sean coherentes (p. ej. especialidad de la institución del proceso).
- **Estado en el servidor:** inscripción, estado de recursos, límites y transiciones se verifican en el servidor, dentro de la transacción que muta y con bloqueo de fila o *advisory lock* cuando hay que contar para un límite.
- **Logs:** solo ids y códigos. Nunca nombre, DNI, email, teléfono, contenido de fuentes ni consultas de búsqueda (README de historias:181; SIA-E2-02 y E2-03, sección Seguridad).
- **SQL** siempre parametrizado (`sqlx`, como en todo el repo).
- **Prompts de IA** (épicas posteriores): solo identificadores opacos y contenido académico. El contenido de las fuentes va delimitado como datos (SIA-E2-03 regla 5).
- **Secretos:** ver la ADR 0002 para el HMAC de los contadores antifraude.

## 10. Estados de intentos (regla para historias posteriores)

Los intentos del módulo que **extienden** las tablas existentes usan **exactamente** sus estados actuales y no inventan otros:

| Tabla | Estados permitidos | Evidencia |
|---|---|---|
| `simulacros` | `en_curso`, `completado` | `CHECK` en `backend/migrations/049_unico_en_curso_y_estados.sql:49-51` |
| `practicas` | `en_curso`, `completada`, `abandonada`, `eliminada` (en femenino) | `CHECK` en `backend/migrations/018_create_practicas.sql:15-16`; constantes en `internal/practices/repository.go:19,529,565` |

- Como máximo **un** intento `en_curso` por usuario y tabla: índices únicos parciales `uq_simulacros_usuario_en_curso` y `uq_practicas_usuario_en_curso` (`049:29-33`).
- **`tiempo_agotado` no es un estado:** es un código de error **409** (`{"error": "tiempo_agotado", "message": …, "resultado_id": n}`, `simulations/handler.go:58-63`, `practices/handler.go:60`). El intento vencido se finaliza de forma perezosa y queda en su estado terminal (`simulations/service.go:161-173`).
- **Diagnóstico (SIA-E4-01):** tiene tabla propia (no extiende `simulacros` ni `practicas`) con su propio `CHECK` de estados y su índice único parcial de un solo intento `en_curso` por usuario, con el mismo patrón de 049. Lo diseña E4-01.
- Las prácticas y simulacros dirigidos (E5-04, E5-05) y el simulacro oficial (E9-05) extienden `practicas` / `simulacros`. Si necesitan distinguirse, lo hacen con columnas nuevas (p. ej. `origen`), nunca con estados nuevos.

## 11. Migraciones

- goose v3 en `backend/migrations/NNN_descripcion.sql` con `-- +goose Up` y `-- +goose Down`. El Down debe funcionar: el CI corre `up → down-to 0 → up` (`.github/workflows/backend.yml:71-73`).
- Hoy la última es `058_add_simulacro_asignaturas_asignatura_index.sql`. Números propuestos para el MVP E1–E2: **059** temas, **060** `proceso_asignaturas.especialidad_id`, **061** `postulante_profesiones` y **062** fuentes. El tech-lead fija el número definitivo al mergear, porque goose falla ante versiones faltantes si se mergea fuera de orden.
- Toda migración declara su efecto sobre los datos existentes y qué pierde el Down, si pierde algo.
- Las FK nuevas llevan índice, por la convención de `054_add_fk_indexes.sql`. Las restricciones llevan nombre explícito cuando el código las mapea a un código de error.

## 12. Preguntas abiertas

**No queda ninguna pregunta abierta.** Todas se resolvieron el 2026-11-11 y los contratos ya aplican las decisiones.

Q1–Q10, resueltas por el PO (supuesto 23 de `docs/stories/serums-ia/README.md`):

| # | Historia | Decisión del PO | Dónde se aplica |
|---|---|---|---|
| Q1 | E1-04 | Preselección desde `personas.especialidad_id`, leída de la BD y solo si es válida; no modifica `personas` | E1 F1, F3 |
| Q2 | E1-01, E1-02 | Orden por asignatura solo entre activos; lista completa; 422 `orden_invalido` y 409 `orden_desactualizado` | E1 T6 |
| Q3 | E1-03 | Guardado atómico por reemplazo, sin borradores; estado derivado; tres casos de 409 `peso_duplicado` | E1 PE3, PE5; ADR 0001 |
| Q4 | E2-03, E2-06 | Duplicado global contra todas las versiones de todas las fuentes, en cualquier estado; el 409 trae id, título y estado | E2 S1, S2; `uq_fuente_versiones_sha256` |
| Q5 | E2-03, E2-06, E2-07 | Cuentan `activa` + `procesando`; una fuente sin profesiones cuenta para cada una; se valida también al reintentar, restaurar y editar profesiones. Endpoint nuevo de edición de metadatos | E2 S1, S2, S5, S7 |
| Q6 | E2-02 | Área y tema no filtran; van en el texto de la consulta | ADR 0003 |
| Q7 | E2-05 | Un fragmento de < 150 palabras se une al pasaje anterior (al siguiente si es el primero) y se divide si pasa de 800; un documento de 1–149 palabras es un único pasaje; los balotarios no se unen | ADR 0003 § Normalización |
| Q8 | E2-06 | Retirar emite el aviso «fuente retirada» una vez y dentro de la misma operación; si un suscriptor falla, el retiro no se aplica | E2 S5, `SuscriptorRetiro` |
| Q9 | E1-03 | Hay pantalla de admin para los pesos en el MVP (reglas 11–13). La pantalla valida en el cliente lo mismo que el servidor y los errores llevan un código distinguible | E1 PE1–PE3 (sin endpoints nuevos) |
| Q10 | E1-01, E1-02 | El límite de 60 cuenta solo los activos. La unicidad del nombre incluye los archivados. Reactivar revalida el límite, pone el tema al final y, si ya está activo, da 409 `estado_invalido` (E1-01 reglas 6 y 6a; E1-02 regla 9) | E1 T3, T5, T6 |

TL1–TL3, resueltas por el tech-lead:

| # | Decisión | Dónde se aplica |
|---|---|---|
| TL1 | Se mantiene `UNIQUE NULLS NOT DISTINCT`. Actualizado el 2026-11-11: el usuario confirmó que aún no existe base de datos de producción, así que producción usará Postgres 16 (como compose y CI); el mínimo soportado es 15 | ADR 0001 § Versión de Postgres en producción |
| TL2 | No se aprueba `unaccent` por ahora; el plegado se hace en Go. Si hiciera falta, se escribe un ADR nuevo y se pide aprobación al usuario | ADR 0003 |
| TL3 | `tema_id` en `POST /preguntas` y `PUT /preguntas/{id}` va en E1-01, opcional y validado (tema activo de la misma asignatura). Precisado en el PR #74: en `PUT`, ausente conserva el tema (salvo cambio de área) y `null` lo quita | E1 T8 |

Seguimiento de los reviews de E1-01/E1-02 (PRs #72–#75), 2026-11-11:

| # | Decisión | Dónde se aplica | Tarea |
|---|---|---|---|
| R1 | T4 acepta las especialidades inactivas que el tema ya tenía; no deja agregar inactivas nuevas | E1 T4 | backend-dev (`temas.Editar`), frontend-dev (formulario de tema) |
| R2 | `texto.Plegar` pliega también las formas NFD (vocal + marca combinante; `n` + U+0303 → `ñ`), sin dependencias nuevas | ADR 0003 § Plegado | backend-dev (`internal/texto`) |
| R3 | Un `*http.MaxBytesError` en el handler da `413 cuerpo_demasiado_grande` con el formato 2 | §3, §4, §6 | backend-dev (`temas.decodificar`), frontend-dev (clave i18n) |
