# Contrato de API — Multi-institución (PLT-06, PLT-07, PLT-08, PLT-10, PLT-14)

Estado: borrador para revisión (2026-11-15).

> **Alcance de esta ronda (decisión del usuario, 2026-11-15):** solo está vigente **PLT-06** (§1, §2). PLT-07, PLT-08, PLT-09, PLT-10 y PLT-14 están **diferidas**. Sus secciones (§3–§6 y §8) se conservan como diseño para cuando se retomen, pero no se implementan ahora. La migración de PLT-07 no tiene número reservado: tomará el siguiente libre.

ADR: [0010](../adr/0010-multi-institucion-elegibilidad-y-catalogos.md). Historias:

- [PLT-06](../stories/plataforma/PLT-06-elegibilidad-institucion.md) (vigente).
- Diferidas: [PLT-07](../stories/plataforma/PLT-07-catalogo-niveles-tipos-institucion.md), [PLT-08](../stories/plataforma/PLT-08-regla-grado-exigido-generica.md), [PLT-09](../stories/plataforma/PLT-09-pantallas-catalogo-admin.md), [PLT-10](../stories/plataforma/PLT-10-institucion-filtrable-admin.md) y [PLT-14](../stories/plataforma/PLT-14-institucion-visible-postulante-landing.md).
- Las decisiones de la épica están en el [README de plataforma](../stories/plataforma/README.md), sección PLT-E2.
- El PO confirmó P1–P11 con las decisiones por defecto (2026-11-15).

Convenciones del [README](README.md): formato de error 2 (§3), paginación (§5), límites (§6) y auditoría (§7). Este contrato está fuera del módulo SERUMS IA, así que **solo** pasan al formato 2 las respuestas que se indican aquí. Los demás errores de cada endpoint siguen igual. El frontend ya lee los dos formatos con `apiErrorFrom` (`frontend/src/lib/http/api-error.ts:38-51`) y traduce por código con `claveErrorApi` (`frontend/src/lib/errores-api.ts`). Las claves van en `dashboard`, en es y en.

## 0. Decisiones (confirmadas por el PO el 2026-11-15 con estos valores por defecto)

| # | Pregunta | Por defecto (aplicado en este contrato) |
|---|---|---|
| P1 | ¿Los simulacros entran en PLT-06? Tienen el mismo hueco (§1, fila 4). **Contradice la historia:** la decisión 9 y el «Fuera de alcance» de PLT-06 dejan los simulacros fuera hasta que qa los verifique, y la verificación ya está hecha aquí | **Sí**: mismo 403 en `POST /simulacros`, con el mismo helper. Si el PO dice que no, se quita la fila 4 de §2.4 y va en su propia historia |
| P2 | ¿Qué pasa con las inscripciones cruzadas ya aprobadas? | Siguen en `/inscripciones/me`. Una práctica o un simulacro **nuevo** da 403. Los intentos en curso no se cortan |
| P3 | *(Diferida, PLT-10)* ¿La institución de `/admin/inscripciones` es la del proceso o la de la persona? | **La del proceso** |
| P4 | ¿`GET /procesos` aplica la regla a cualquier rol? Solo lo consumen pantallas del postulante (`procesos/page.tsx:106`, `practicas/nueva/page.tsx:71`, `simulacros/nueva/page.tsx:115`). Las de admin usan `/admin/procesos` (`frontend/src/lib/catalogs/api.ts:236`) | **Sí**. Las vistas `/admin/*` no cambian |
| P5 | *(Diferida, PLT-07/09)* `procesos.nivel` no se escribe por la API (ADR 0010, contexto 4). PLT-06 no depende de esto: con el nivel en NULL, el filtro de nivel deja pasar todos, como hoy | PLT-07 añade `nivel` opcional a `POST` y `PUT /admin/procesos`, y PLT-09 añade el selector |
| P8 | ¿Se cierran las lecturas de configuración por `proceso_id` (§1, filas 6 y 7)? | Fuera de PLT-06. Van a un issue de seguimiento de prioridad baja, que abre el tech-lead |
| P9 | *(Diferida, PLT-10)* Decisión del architect que cambia el comportamiento: `PUT /admin/personas/{id}` exige que el grado y la especialidad sean de la misma institución (§5). Una persona que ya sea incoherente debe corregirse al editarla | Sí |
| P10 | Un admin todavía puede **aprobar** una inscripción cruzada que quedó `pendiente` antes del fix. Así la fila 5 de §1 sigue abierta para esas inscripciones | No se bloquea la aprobación (decisión 7: el PO decide caso por caso). El PO decide antes de aprobar usando la lista de §2.5 |

(P6 y P7 están en [plataforma-avisos-idioma.md](plataforma-avisos-idioma.md).)

## 1. Huecos encontrados (alcance real de PLT-06)

| # | Endpoint (rol) | Evidencia | Qué permite hoy | Historia |
|---|---|---|---|---|
| 1 | `GET /procesos` (autenticado) | `inscripciones/routes.go:21`; `repository.go:46-57` sin filtro de institución; `service.go:86-107` solo compara el nivel | Ver los procesos activos de otras instituciones | **PLT-06** |
| 2 | `POST /inscripciones` (autenticado) | `routes.go:24`; `service.go:123-141`; `repository.go:113-135` | Inscribirse a un proceso de otra institución (lo reprodujo qa) | **PLT-06** |
| 3 | `POST /practicas` (autenticado) | `practices/routes.go:29`; `service.go:76-89` (nivel + inscripción aprobada) | Practicar sobre un proceso de otra institución si tiene una inscripción aprobada (por la fila 2 o una cruzada existente) | **PLT-06** |
| 4 | `POST /simulacros` (autenticado) | `simulations/routes.go:34`; `service.go:74-89` | Lo mismo que la fila 3 | **PLT-06** (P1) |
| 5 | `GET /serums-ia/profesion/opciones`, `PUT /serums-ia/profesion` | `perfiles/routes.go:17-19`; `repository.go:65-94,153-174` (la única barrera es la inscripción aprobada) | Elegir una profesión en un proceso de otra institución con una inscripción cruzada ya aprobada | Se cierra de forma transitiva con la fila 2 para las inscripciones nuevas. Las existentes las decide el PO (decisión 7) |
| 6 | `GET /simulacros/configuracion?proceso_id=` | `simulations/handler.go:315-337`; `service.go:285-287` sin control | Leer la configuración (tiempo y preguntas) de cualquier proceso. No hay datos personales | Fuera (P8) |
| 7 | `GET /procesos/{id}/asignaturas` | `catalogs/routes.go:58`; `handler.go:652-677`; `service.go:261-263` sin control | Leer las asignaturas, los pesos y las preguntas disponibles de cualquier proceso. No hay datos personales | Fuera (P8) |

**Sin hueco:**

- **`GET /procesos/publico`:** público por diseño (PLT-14).
- **Intentos existentes** (`/practicas/{id}…`, `/simulacros/{id}…`): se filtran por `usuario_id` (IDOR cubierto en `practices/idor_test.go` y `simulations/idor_test.go`).
- **Preguntas:** todos sus endpoints son admin (`questions/routes.go:34-51`). La generación del set filtra por `proceso_id`, y `preguntas.institucion_id` se copia del proceso (`questions/repository.go:30-34,132`).
- **Catálogos:** `GET /grados`, `/especialidades` e `/instituciones` (`catalogs/routes.go:23-41`) son nombres de catálogo, sin datos personales ni acciones. Las `/asignaturas` son globales.
- **Otros:** notificaciones, testimonios y `GET /stats/publico` (conteos globales).

## 2. PLT-06 Elegibilidad por institución

### 2.1 Regla única (`internal/examen`)

```go
// AccesoProceso: una consulta; sin filas => el proceso no existe.
type Acceso struct{ Activo, MismaInstitucion, NivelOK bool }
func AccesoProceso(ctx context.Context, q Querier, usuarioID, procesoID int64) (Acceso, error) // ErrProcesoNoEncontrado
```

```sql
SELECT p.estado = 'activo'                               AS activo,
       p.institucion_id = pe.institucion_id              AS misma_institucion,
       (COALESCE(p.nivel, '') = '' OR p.nivel = g.nivel) AS nivel_ok
FROM procesos p
CROSS JOIN usuarios u
JOIN personas pe ON pe.id = u.persona_id
JOIN grados   g  ON g.id  = pe.grado_id
WHERE p.id = $2 AND u.id = $1
```

- La institución y el nivel **se leen de la BD**, nunca de `claims.InstitucionID` ni de `claims.Nivel` (ADR 0010, decisión 1).
- **Por qué no hay filas solo si falta el proceso:** el usuario autenticado existe, y `usuarios.persona_id`, `personas.grado_id` y `personas.institucion_id` son NOT NULL con FK (`003_create_usuarios.sql:4`, `002_create_personas.sql:8-10`).
- **Nivel vacío** (NULL o `''`) significa «todos los niveles», como hoy (`inscripciones/service.go:93,132`).
- **Orden fijo; gana el primer error:**
  1. el proceso no existe;
  2. no está activo (solo al inscribirse);
  3. es de otra institución;
  4. el nivel no es compatible;
  5. falta la inscripción aprobada (solo en prácticas y simulacros).

### 2.2 `GET /procesos` (autenticado, cualquier rol, P4)

- **Filtro en SQL:** `p.estado='activo' AND p.institucion_id = <institución de la persona> AND (COALESCE(p.nivel,'')='' OR p.nivel = <nivel de la persona>)`, ordenado por `anio DESC, nombre, id`.
- **Respuesta:** sin cambios en PLT-06. Es un array de `ProcesoDTO` (`inscripciones/service.go:56-64`).
- **En esta ronda (PLT-06), `GET /procesos` no suma campos.** Comparte `ProcesoDTO` con `GET /procesos/publico`, que debe quedar igual (escenario «Landing sin cambios»).
- `institucion_id` e `institucion` llegan a los **dos** endpoints a la vez con PLT-14, que está diferida (§6). Se añaden en el DTO compartido y no se crea un DTO aparte.
- Si no hay procesos, responde `200 []` y la UI muestra el estado vacío traducido.

### 2.3 `POST /inscripciones` (autenticado)

Request sin cambios: `{"proceso_id": 12}`. Respuesta `201` sin cambios (`InscripcionDTO`). **Todos sus errores pasan al formato 2:**

| HTTP | `error` | Cuándo |
|---|---|---|
| 400 | `cuerpo_invalido` | JSON mal formado o `proceso_id` ausente o ≤ 0 |
| 401 | (middleware) | Sin sesión |
| 403 | `institucion_no_permitida` | El proceso es de otra institución. No se crea nada |
| 403 | `nivel_no_permitido` | Misma institución, nivel incompatible |
| 404 | `no_encontrado` | El proceso no existe o no está activo |
| 409 | `ya_inscrito` | Ya hay una inscripción `pendiente` o `aprobado` (`ErrYaInscrito`) |
| 409 | `sin_monto_configurado` | El año del proceso no tiene `configuracion_pago` (`ErrSinConfigPago`). Separado de `ya_inscrito` (#269) |
| 500 | `error_interno` | — |

```json
{ "error": "institucion_no_permitida", "message": "Este proceso pertenece a otra institución." }
```

- **Firma:** `Service.Inscribir(ctx, usuarioID, procesoID int64, onReapply)` deja de recibir `userNivel`. Hay que ajustar `cmd/seed/demo/sembrar.go:581`.
- La comprobación usa `AccesoProceso`. Ya no recorre la lista completa de procesos (`service.go:125-141`).

### 2.4 `POST /practicas` y `POST /simulacros` (autenticado)

Request y respuesta `201` sin cambios. **Solo** pasan al formato 2 sus 403. Sus demás errores (`practices/handler.go:60-82`, `simulations/handler.go:58-88`) no cambian.

| HTTP | `error` | Cuándo |
|---|---|---|
| 403 | `institucion_no_permitida` | El proceso es de otra institución, aunque exista una inscripción cruzada aprobada (P2) |
| 403 | `nivel_no_permitido` | Nivel incompatible |
| 403 | `inscripcion_requerida` | Sin inscripción aprobada (código ya definido en el README §4) |

- Un proceso inexistente da el 404 actual (`ErrNotFound`).
- `CreatePractica` y `CreateSimulacro` dejan de recibir `userNivel` (`practices/service.go:71`, `simulations/service.go:68`).
- Los intentos ya creados no cambian (P2).

### 2.5 Inscripciones cruzadas existentes (PR de PLT-06)

backend-dev corre esta consulta contra la copia de la base y pega en el PR **solo** el conteo y los ids (nada de datos personales):

```sql
SELECT i.id, i.proceso_id, i.estado
FROM inscripciones i
JOIN usuarios u  ON u.id = i.usuario_id
JOIN personas pe ON pe.id = u.persona_id
JOIN procesos p  ON p.id = i.proceso_id
WHERE p.institucion_id <> pe.institucion_id
ORDER BY i.id;
```

### 2.6 Pruebas mínimas

- Listado: A ve solo los procesos de A que cumplen su nivel, más los de nivel vacío.
- Inscripción a un proceso de B: 403 `institucion_no_permitida` y ninguna fila nueva.
- Misma institución con otro nivel: `nivel_no_permitido`.
- Institución y nivel distintos a la vez: `institucion_no_permitida` (por el orden).
- Práctica y simulacro sobre B con una inscripción cruzada aprobada (sembrada por SQL): 403 `institucion_no_permitida`.
- Regresión con el caso de qa.
- `ya_inscrito` y `sin_monto_configurado`.
- `GET /procesos/publico` byte a byte igual que antes.

## 3. PLT-07 Catálogo de niveles y tipos de proceso por institución — **DIFERIDA**

> Diferida (2026-11-15): no se implementa en esta ronda. Esta sección y §3.5 son diseño para cuando se retome.

### 3.1 Modelo (migración de PLT-07, sin número reservado, §8)

| Tabla | Columnas |
|---|---|
| `institucion_niveles` | `id BIGSERIAL PK`, `institucion_id BIGINT NOT NULL FK instituciones`, `codigo VARCHAR(30) NOT NULL`, `nombre VARCHAR(100) NOT NULL`, `orden INT NOT NULL DEFAULT 0`, `estado VARCHAR(20) NOT NULL DEFAULT 'activo' CHECK (estado IN ('activo','inactivo'))`, `created_at`, `updated_at`; `UNIQUE (institucion_id, codigo)` con nombre `uq_institucion_niveles_codigo` |
| `institucion_tipos_proceso` | Las mismas, pero con `codigo VARCHAR(50)` y `exige_grado BOOLEAN NOT NULL DEFAULT false`; `UNIQUE (institucion_id, codigo)` con nombre `uq_institucion_tipos_proceso_codigo` |

La `UNIQUE` empieza por `institucion_id`, así que cubre el índice de la FK (convención de `054`).

### 3.2 Endpoints (admin; `authn(admin(…))`; paquete `catalogs`)

`{recurso}` es `niveles` o `tipos-proceso`. Los dos recursos tienen la misma forma; los tipos añaden `exige_grado`.

| Método y ruta | Éxito |
|---|---|
| `GET /admin/instituciones/{id}/{recurso}?estado=activo\|inactivo` | `200 {"data": [ … ]}` sin paginar (máx. 50 por institución), ordenado por `orden, id` |
| `POST /admin/instituciones/{id}/{recurso}` | `201` con el objeto |
| `PUT /admin/instituciones/{id}/{recurso}/{valorId}` | `200` con el objeto |
| `PATCH /admin/instituciones/{id}/{recurso}/{valorId}/estado` | `200` con el objeto |
| `DELETE /admin/instituciones/{id}/{recurso}/{valorId}` | `204` |

Objeto:

```json
{ "id": 7, "institucion_id": 2, "codigo": "cadete", "nombre": "Cadete", "orden": 3,
  "estado": "activo", "exige_grado": false, "en_uso": true, "created_at": "2026-11-15T15:04:05Z" }
```

- `exige_grado` solo aparece en `tipos-proceso`.
- `en_uso` dice si algún grado, especialidad o proceso de esa institución usa el código. La UI lo usa para deshabilitar «Eliminar».

**Requests** (decodificación estricta, `httpx.DecodeJSONStrict`):

- **`POST`:** `{"codigo": "cadete", "nombre": "Cadete", "orden": 3}`. `orden` es opcional (por defecto, máximo + 1). En tipos, `"exige_grado": true` es opcional (por defecto `false`).
- **`PUT`:** `{"nombre": "…", "orden": 3}`, y en tipos además `"exige_grado"`. **`codigo` no se acepta:** un campo desconocido da `400 cuerpo_invalido`. El código no cambia nunca.
- **`PATCH …/estado`:** `{"estado": "inactivo"}`. Repetir el mismo estado responde 200 y no se audita.

**Reglas** (después de `TrimSpace`; longitudes en runas):

| Campo | Regla |
|---|---|
| `codigo` | `^[a-z][a-z0-9_]{1,29}$` (2 a 30, minúsculas). Único por institución |
| `nombre` | De 2 a 100 caracteres. Texto libre en un idioma; no se traduce |
| `orden` | Entero de 0 a 9999. No es único, porque el orden real es `orden, id` |
| Cantidad | Máximo 50 valores por institución y recurso, contando activos e inactivos |

**Errores** (formato 2):

| HTTP | `error` | Cuándo |
|---|---|---|
| 400 | `cuerpo_invalido`, `parametro_invalido` | Sintaxis, campo desconocido (incluido `codigo` en `PUT`), id o `estado` de la query inválidos |
| 403 | (middleware) | No es admin |
| 404 | `no_encontrado` | La institución no existe, o `valorId` no existe **o es de otra institución** |
| 409 | `codigo_duplicado` | El código ya existe en esa institución, sea cual sea su estado (`uq_…_codigo`). En otra institución se permite |
| 409 | `valor_en_uso` | `DELETE` de un valor usado. `detalle`: `{"grados": n, "especialidades": n, "procesos": n}` |
| 422 | `validacion` | Una regla de la tabla. `detalle`: `{"campos": {"codigo": "formato"}}` |
| 422 | `limite_catalogo` | Ya hay 50 valores |
| 500 | `error_interno` | — |

- **`DELETE`:** en una transacción con `SELECT … FOR UPDATE` sobre la fila; cuenta el uso y borra (ADR 0010, decisión 8).
- **«En uso»:** se cuenta en la misma institución.
  - Niveles: `grados.nivel`, `especialidades.nivel` y `procesos.nivel`.
  - Tipos: `procesos.tipo`.
- **Cambiar `exige_grado`** se permite aunque el tipo esté en uso. Los procesos que ya existen sin grado siguen igual, pero su próxima edición exige un grado (§4, regla 4).

**Auditoría** (`LogTx` en la misma transacción):

- Acciones de nivel: `catalogo_nivel.crear`, `catalogo_nivel.editar`, `catalogo_nivel.activar`, `catalogo_nivel.inactivar` y `catalogo_nivel.eliminar`.
- Acciones de tipo: `catalogo_tipo_proceso.*`, con los mismos verbos.
- Entidad: `institucion_nivel` o `institucion_tipo_proceso`, con su id.
- `detalle`: `{institucion_id, codigo}` más el antes y el después de los campos cambiados.

### 3.3 Validación de grados, especialidades y procesos contra el catálogo

Aplica a `POST /grados`, `PUT /grados/{id}`, `POST /especialidades`, `PUT /especialidades/{id}`, `POST /admin/procesos` y `PUT /admin/procesos/{id}` (`catalogs/routes.go:32-54`).

1. Se quitan los `oneof` de `catalogs/dto.go:42,64,98`.
   - Grado: `nivel` sigue `required` y pasa a `max=30`.
   - Proceso: `tipo` sigue `required` y pasa a `max=50`.
2. **El valor debe existir en el catálogo de la institución del registro** (`req.InstitucionID`, que en un `PUT` puede ser la nueva). Si no: `422 nivel_invalido` o `422 tipo_invalido`.
3. **Si el valor está inactivo**, solo se acepta en un `PUT` donde el registro ya tenía ese código y esa institución. En un alta, o al cambiar a ese valor, da el mismo 422.
4. Un `nivel` vacío o ausente sigue siendo válido en especialidades y procesos («todos los niveles»). En grados es obligatorio.
5. **Proceso con `nivel`** (nuevo, P5): `"nivel": "suboficial" | null` en `CreateProcesoRequest` (`dto.go:95-102`); ausente o `null` se guarda como NULL. `ProcesoResponse` (`dto.go:104-116`) añade `"nivel": ""` cuando no hay nivel.

Estos códigos son las **únicas** respuestas de esos endpoints que pasan al formato 2, igual que el precedente `peso_duplicado` (`catalogs/handler.go:95-97`):

```json
{ "error": "nivel_invalido", "message": "El nivel no existe o está inactivo en la institución.", "detalle": { "campo": "nivel" } }
```

### 3.4 Pruebas mínimas

- Migración: subida, bajada y subida (§8).
- B con «cadete»: un grado de B con `cadete` → 201; un grado de A con `cadete` → 422 `nivel_invalido`.
- Borrar `oficial` en uso → 409 `valor_en_uso`; inactivarlo → 200, y los grados que lo tienen se editan sin error.
- `ascenso` duplicado en A → 409 `codigo_duplicado`; en B → 201.
- Usuario sin rol admin → 403.
- `valorId` de otra institución → 404.
- `PUT` con `codigo` → 400.

### 3.5 PLT-09 (solo UI), sobre este contrato — **DIFERIDA**

PLT-09 no necesita endpoints propios:

- **Pantalla de catálogo:** los endpoints de §3.2.
- **Selectores de nivel y tipo de los formularios:** `GET /admin/instituciones/{id}/{recurso}?estado=activo`. Al editar, se añade el valor actual si está inactivo, para mostrarlo.
- **Selector de grado del proceso:** `GET /grados?institucion_id=` (`catalogs/handler.go:117-126`).
- **Campo `nivel` del proceso:** depende de P5.
- **Errores:** se traducen `codigo_duplicado`, `valor_en_uso`, `limite_catalogo`, `nivel_invalido`, `tipo_invalido`, `grado_requerido`, `grado_otra_institucion` y `referencia_invalida` (§3.2, §3.3, §4).
- **UI:** solo con componentes y tokens de khipu.

## 4. PLT-08 Regla de grado exigido genérica — **DIFERIDA**

> Diferida (2026-11-15). Hasta que se retome, `ValidateSO3Grado` sigue vigente.

Se elimina `ValidateSO3Grado` (`catalogs/validation.go:8-25`) y sus llamadas (`service.go:218,235`). En `POST` y `PUT /admin/procesos`, después de §3.3:

1. Se lee el tipo de `(req.InstitucionID, req.Tipo)` (ya validado en §3.3).
2. Si tiene `exige_grado` y `grado_id` es `null` o ausente: `422 grado_requerido`.
3. Si viene un `grado_id`:
   - si no existe: `422 referencia_invalida`;
   - si su `institucion_id` no es la del proceso: `422 grado_otra_institucion`.

   Esto aplica también a los tipos sin `exige_grado` (regla 2 de la historia).
4. Cambiar a un tipo que exige grado sin enviar uno da `422 grado_requerido`. Editar un proceso que ya tiene su grado, sin tocarlo, se guarda sin error.

Los cuatro códigos van en el formato 2 con `detalle: {"campo": "grado_id"}` (`tipo` en el caso de `tipo_invalido`). Hoy estos casos responden `400` con texto (`ErrValidation`, `handler.go:102-103`), y la UI del formulario de proceso los pasa a traducir por código.

`validation.go` no menciona «SO 3°». No hay migración: `exige_grado` llega con la migración de PLT-07.

## 5. PLT-10 Institución filtrable en el admin — **DIFERIDA**

> Diferida (2026-11-15).

**Parámetro común `institucion_id`** (opcional, entero > 0):

- Ausente: todas las instituciones.
- Inexistente: `200` con lista vacía.
- Mal formado o ≤ 0: `400 {"error": "parametro_invalido", …}` (formato 2).

Se combina con los filtros actuales.

| Endpoint | Filtro | Campos nuevos por ítem |
|---|---|---|
| `GET /admin/inscripciones?estado=&institucion_id=&page=&per_page=` (`inscripciones/routes.go:28`) | La institución **del proceso** (P3): `pr.institucion_id` | `institucion_id`, `institucion` (nombre), de la institución del proceso |
| `GET /admin/usuarios?estado=&institucion_id=` (`auth/routes.go:42`) | La institución de la persona vinculada | `institucion_id`, `institucion` en `UsuarioPendiente` (`auth/dto.go:79-89`) |
| `GET /admin/personas?institucion_id=` (`auth/routes.go:39`) | `personas.institucion_id` | Ninguno: `PersonaListItem` ya los trae (`auth/dto.go:124-125`) |

- **Paginación y búsqueda:** no cambian.
  - `/admin/inscripciones` sigue paginando en el servidor con la forma actual (array). El frontend pide todas las páginas (`frontend/src/lib/inscripciones/api.ts:116-128`).
  - `/admin/usuarios` y `/admin/personas` siguen devolviendo la lista completa (`auth/handler_admin.go:20-31,167-177`).
  - La búsqueda por texto y la paginación de la tabla siguen en el cliente, porque los nombres y el DNI están cifrados. Así la paginación cuenta solo el conjunto filtrado.
- **`PUT /admin/personas/{id}`:** `institucion_id` ya es obligatorio (`dto.go:138`). Ya se audita como campo cambiado (`auth/repository.go:1243-1255`) y ya revoca las sesiones (`:1275-1279`).
  - **Nuevo:** `grado_id` y `especialidad_id` deben ser de esa misma institución. Si no: `422 {"error": "referencia_invalida", "detalle": {"campo": "grado_id" | "especialidad_id"}}`. Así no queda una persona de B con un grado de A, que le daría el nivel equivocado.
  - Las inscripciones previas no cambian.
- **Frontend:** se quita `institucion_id: 1` de `admin/personas/page.tsx:40`. El campo queda obligatorio y sin valor inicial.

## 6. PLT-14 Institución visible al postulante y en la landing — **DIFERIDA**

> Diferida (2026-11-15). `ProcesoDTO` no cambia en esta ronda. Cuando se retome, los dos campos se añaden a la vez a `GET /procesos/publico` y a `GET /procesos`, porque comparten el DTO (§2.2).

- **`GET /procesos/publico`** (público) y **`GET /procesos`** (comparten `ProcesoDTO`): se **añaden** dos campos y no cambia ninguno de los existentes ni el orden.

  ```json
  { "id": 12, "nombre": "…", "tipo": "ascenso", "grado": "", "nivel": "", "anio": 2026,
    "institucion_id": 1, "institucion": "Nombre de la institución" }
  ```

  - `institucion` es `instituciones.nombre`, que no es un dato personal. No se exponen las siglas ni otros campos.
  - Los procesos de una institución inactiva ya están inactivos (`catalogs/repository.go:148`) y no aparecen.
- **Configuración y subtítulo de `/procesos`:** no hay cambio de API. `GET /auth/me` ya devuelve `institucion` (`auth/dto.go:40`, consulta en `auth/repository.go:1385-1418`).
- La landing agrupa por `institucion_id` en el cliente y no muestra ningún botón de inscripción.
- **Prueba:** todos los campos anteriores siguen presentes y sin cambios, más los dos nuevos.

## 7. Reparto sugerido (sin pisarse)

**Esta ronda (tech-lead, 2026-11-15): solo PLT-06, PLT-11, PLT-12 y PLT-13.**

| Dev | Dominio | Historias, en orden |
|---|---|---|
| backend-dev | `inscripciones`, `examen`, `practices`, `simulations`, `notifications`, `internal/correo`, `cmd/seed/demo:581` | **PLT-06** → PLT-12 |
| backend-dev-2 | `auth` | **PLT-11** → partes de backend del #274 (textos de `auth/email_templates.go`: «unidad» y la constante `emailInstitution`) → PLT-13 |
| frontend-dev | `frontend/` | PLT-06 (estado vacío y 403) → PLT-11 → PLT-12 |

- **En paralelo:** PLT-06 y PLT-11 arrancan a la vez. PLT-06 no tiene migración.
- **Orden de merge obligatorio:**
  - **069** (PLT-11) se mergea antes que **070** (PLT-12), porque goose corre sin allow-missing.
  - PLT-12 necesita además que `usuarios.idioma` ya exista.
  - **PLT-13 se rebasea sobre PLT-12**, porque PLT-12 mueve el layout de `auth/email_templates.go:11-89` a `internal/correo`.
  - Los cambios de texto del #274 en `email_templates.go` también se rebasean sobre PLT-12 si tocan el layout (la marca, el pie o `emailInstitution`, `:14-18`).
- **Archivos compartidos:**
  - `auth/email_templates.go`: PLT-12 mueve el layout; el #274 y PLT-13 editan las plantillas de cuenta. Por eso, PLT-12 se mergea primero.
  - `auth/repository.go`: PLT-11 añade `idioma` a `GetUserProfileByID` (`:1377`) y PLT-13 lo usa. El mismo dev las hace en orden.

**Diferido (cuando se retome):**

- backend-dev: PLT-10 (solo `/admin/inscripciones`) y PLT-14.
- backend-dev-2: PLT-07, PLT-08 y PLT-10 (`/admin/usuarios`, `/admin/personas` y `PUT` de persona).
- frontend-dev: PLT-07/09, PLT-10 y PLT-14.

`cmd/seed/demo/sembrar.go`: PLT-06 toca `:581` y PLT-07 tocará `:175,200`.

## 8. Migración de PLT-07 `create_catalogos_institucion` — **DIFERIDA**, sin número reservado (tomará el siguiente libre)

**Up** (en una transacción):

1. `CREATE TABLE` de las dos tablas de §3.1.
2. **Sembrado guiado por los datos e idempotente** (`ON CONFLICT (institucion_id, codigo) DO NOTHING`):
   - **Niveles:** la unión de los `DISTINCT (institucion_id, nivel)` de `grados`, `especialidades` y `procesos`, excluyendo NULL y `''`.
   - **Tipos:** los `DISTINCT (institucion_id, tipo)` de `procesos`.
   - **Nombre:** `suboficial` → «Suboficial», `oficial` → «Oficial», `ascenso` → «Ascenso», `pase_efectividad` → «Pase de efectividad», `admision` → «Admisión»; cualquier otro, `initcap(replace(codigo,'_',' '))`.
   - **Orden:** el de esa lista (1, 2, 3); los demás, a partir de 100, por código.
   - **`exige_grado`:** `codigo = 'pase_efectividad'`.
   - Una institución sin datos queda con el catálogo vacío.
3. `ALTER TABLE grados DROP CONSTRAINT IF EXISTS grados_nivel_check;`

**No modifica** ninguna fila de `grados`, `especialidades`, `procesos` ni `inscripciones`. Los `''` y los NULL se quedan como están.

**Down:**

1. `DROP TABLE IF EXISTS institucion_tipos_proceso, institucion_niveles;`
2. `ALTER TABLE grados ADD CONSTRAINT grados_nivel_check CHECK (nivel IN ('suboficial','oficial')) NOT VALID;`

**Lo que pierde el Down:** nombres, orden, estado y `exige_grado` del catálogo. El CHECK vuelve como `NOT VALID`, así que los grados con un nivel propio (p. ej. `cadete`) se conservan y solo se rechazan las altas nuevas fuera de la lista.

**Prueba:** el CI hace `up → down-to 0 → up`. La prueba de la historia hace up → down → up sobre datos sembrados y comprueba que el catálogo no se duplica y que las filas de los catálogos de grados y procesos son idénticas antes y después.

**Antes del PR:** se prueba contra una copia de la base (PLT-07, Seguridad). Si aparece un código de más de 30 caracteres en `procesos.nivel` o `especialidades.nivel`, la columna ya lo impide (`025`, `026`).
