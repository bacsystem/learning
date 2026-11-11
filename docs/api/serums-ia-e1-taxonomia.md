# Contrato API — SERUMS IA, épica E1 Taxonomía (MVP)

- **Historias:** SIA-E1-01 (#11), SIA-E1-02 (#12), SIA-E1-03 (#13), SIA-E1-04 (#14)
- **Estado:** vigente (2026-11-11). Revisión del 2026-11-11 (seguimiento de los PRs #72–#75): T8 documenta lo implementado en el PR #74 y T4 acepta las especialidades inactivas ya asociadas (cambio de contrato: tarea de backend-dev y frontend-dev).
- **Convenciones transversales:** [README.md](README.md) (errores, auth, paginación, límites, auditoría e i18n). Aquí solo se documenta lo propio de E1.
- **ADR relacionado:** [0001](../adr/0001-unicidad-proceso-asignaturas-por-especialidad.md) (unicidad y resolución de `proceso_asignaturas`).

## Resumen de endpoints

| # | Método y ruta | Rol | Historia |
|---|---|---|---|
| T1 | `GET /admin/temas` | admin | E1-02 (y E1-01 lectura) |
| T2 | `GET /admin/temas/{id}` | admin | E1-02 |
| T3 | `POST /admin/temas` | admin | E1-01, E1-02 |
| T4 | `PUT /admin/temas/{id}` | admin | E1-02 |
| T5 | `PATCH /admin/temas/{id}/estado` | admin | E1-01, E1-02 |
| T6 | `PUT /admin/asignaturas/{id}/temas/orden` | admin | E1-01 regla 7, E1-02 reglas 6–8 |
| T7 | `GET /asignaturas/{id}/temas` | autenticado | E1-01 (lectura) |
| T8 | `POST /preguntas`, `PUT /preguntas/{id}` (existentes): campo nuevo `tema_id` | admin | E1-01 regla 5 |
| PE1 | `GET /admin/procesos/{id}/pesos` | admin | E1-03 |
| PE2 | `GET /admin/procesos/{id}/pesos/{especialidadId}` | admin | E1-03 |
| PE3 | `PUT /admin/procesos/{id}/pesos/{especialidadId}` | admin | E1-03 |
| PE5 | `POST /admin/procesos/{id}/asignaturas` (existente): 409 con código `peso_duplicado` | admin | E1-03 (fila de compatibilidad) |
| PE6 | Servicio Go `catalogs.Pesos` (sin ruta HTTP propia) | interno | E1-03 reglas 8–9 |

Los identificadores T*, PE* y F* son de este documento. No confundir con las decisiones de producto P1–P8 del issue #65.
| F1 | `GET /serums-ia/profesion/opciones` | usuario | E1-04 |
| F2 | `GET /serums-ia/profesion` | usuario | E1-04 |
| F3 | `PUT /serums-ia/profesion` | usuario | E1-04 |

Paquetes backend:

- **temas:** paquete nuevo `internal/temas`, con `Mount(mux, svc, audLog, jwtSecret, checkSession)` y la misma firma que `inscripciones.Mount` (`cmd/server/main.go:124`).
- **pesos:** dentro de `internal/catalogs`, que es dueño de `proceso_asignaturas` y de `lockProceso` (`catalogs/repository.go:714`).
- **profesión:** paquete nuevo `internal/perfiles`.
- **T8:** campo nuevo en `internal/questions`.

---

## 1. Temas (SIA-E1-01, SIA-E1-02)

### Objeto `Tema`

```json
{
  "id": 41,
  "asignatura_id": 7,
  "nombre": "Vigilancia epidemiológica",
  "descripcion": "Notificación, investigación de brotes y sistemas de vigilancia.",
  "estado": "activo",
  "orden": 3,
  "aplica_a_todas": false,
  "especialidades": [{ "id": 2, "nombre": "Medicina" }, { "id": 5, "nombre": "Enfermería" }],
  "preguntas_count": 10,
  "created_at": "2026-11-11T15:04:05Z",
  "updated_at": "2026-11-11T15:04:05Z"
}
```

- `aplica_a_todas = true` ⇔ `especialidades` está vacío (SIA-E1-01 regla 3).
- `preguntas_count` = preguntas con `tema_id = id` y `estado <> 'eliminada'`. Es el número que muestra el diálogo de archivar (E1-02 regla 4).

### Reglas de validación (comunes a T3 y T4)

| Campo | Regla | Error |
|---|---|---|
| `nombre` | Se aplica `TrimSpace` y se colapsan los espacios internos; 3–120 caracteres (runes) | 422 `validacion` (`detalle.campos.nombre`) |
| `descripcion` | Opcional; ≤ 500 caracteres; `""` se guarda como `null` | 422 `validacion` |
| `especialidad_ids` | Opcional; 0–50 ids, sin repetidos | 422 `validacion` (tamaño o repetidos) |
| `especialidad_ids` en T3 | Cada id debe ser una especialidad existente y `activo` | 422 `referencia_invalida` (inexistente o inactiva) |
| `especialidad_ids` en T4 | Cada id debe ser una especialidad existente y `activo`, **o** una que el tema ya tenía asociada antes de la edición (aunque hoy esté inactiva). Ver T4 | 422 `referencia_invalida` (inexistente, o inactiva y nueva para el tema) |
| `asignatura_id` (solo T3) | Debe existir y estar `activo` | 422 `referencia_invalida` |
| Unicidad del nombre | `plegar(nombre)` único dentro de la asignatura, sin distinguir mayúsculas ni tildes (á é í ó ú ü à è ì ò ù; la ñ se distingue), con la tilde compuesta (NFC) o descompuesta (NFD, p. ej. pegada desde un PDF) (ADR 0003 § Plegado). Incluye temas archivados | 422 `tema_duplicado` |
| Límite | Máximo 60 temas `activo` por asignatura. Los archivados no cuentan (ver nota) | 422 `limite_temas` |

- `plegar` es `texto.Plegar` (paquete nuevo `internal/texto`, lo crea esta historia; ver README §1 y la ADR 0003).
- **Límite y unicidad (E1-01 reglas 6 y 6a):** el límite cuenta solo los temas activos, así que archivar libera cupo. La unicidad del nombre **sí** incluye los archivados: crear un nombre igual al de un tema archivado da `422 tema_duplicado`, y para volver a usarlo se reactiva ese tema (T5).
- **Concurrencia:** T3 y la reactivación (T5) bloquean la fila de la asignatura (`SELECT … FROM asignaturas WHERE id=$1 FOR UPDATE`) antes de contar. Sin ese bloqueo, dos altas simultáneas podrían dejar 61 temas.
- Las violaciones de `uq_temas_asignatura_nombre_clave` (23505) se mapean a `tema_duplicado`, como red de seguridad ante una carrera.

### T1 `GET /admin/temas`

Rol admin. Listado paginado (E1-02 reglas 1–2).

| Query | Obligatorio | Valores |
|---|---|---|
| `asignatura_id` | sí | id > 0 |
| `especialidad_id` | no | id > 0; devuelve los temas asociados a esa especialidad **y** los que aplican a todas |
| `estado` | no | `activo` \| `archivado`; sin valor = ambos |
| `page`, `per_page` | no | ver README (20 por defecto, máx. 100) |

Orden: `orden ASC, id ASC`.

```json
{ "data": [ { "…": "Tema" } ], "total": 12, "page": 1, "per_page": 20 }
```

| Status | `error` | Cuándo |
|---|---|---|
| 400 | `parametro_invalido` | Falta `asignatura_id`, o un id, `estado`, `page` o `per_page` es inválido |
| 401 / 403 | (middleware) | Sin sesión / rol distinto de admin |

Una asignatura inexistente o sin temas da `200` con `data: []`. La UI muestra el estado vacío «Aún no hay temas en esta área».

### T2 `GET /admin/temas/{id}`

Rol admin. `200` con el objeto `Tema`. `404 no_encontrado` si no existe. `400 parametro_invalido` si `{id}` no es un entero > 0.

### T3 `POST /admin/temas`

Rol admin. Request:

```json
{ "asignatura_id": 7, "nombre": "Vigilancia epidemiológica", "descripcion": null, "especialidad_ids": [2, 5] }
```

`201` con el `Tema` creado (`estado: "activo"`, `orden` = máximo actual + 1 en la asignatura).

| Status | `error` |
|---|---|
| 400 | `cuerpo_invalido` (JSON mal formado o campos desconocidos) |
| 422 | `validacion`, `referencia_invalida`, `tema_duplicado`, `limite_temas` |
| 401 / 403 | middleware (escenario «Permisos»: rol usuario → 403) |

Auditoría, en la misma transacción: `accion = "tema.crear"`, `entidad = "tema"`, `entidad_id = id`, `detalle = {"asignatura_id", "nombre", "especialidad_ids"}`.

### T4 `PUT /admin/temas/{id}`

Rol admin. Reemplazo completo de los campos editables. La asignatura **no** se puede cambiar: mover un tema rompería la clasificación de sus preguntas (la FK compuesta de §4 lo impide).

```json
{ "nombre": "Vigilancia epidemiológica", "descripcion": "…", "especialidad_ids": [] }
```

- `200` con el `Tema` actualizado. Un tema archivado también se puede editar.
- Errores: los de T3 más `404 no_encontrado`. Si se envía `asignatura_id`, se rechaza con `400 cuerpo_invalido` (campo desconocido).
- **Especialidades inactivas ya asociadas** (decisión del architect, 2026-11-11; cambio sobre la versión publicada):
  - Se aceptan los ids que el tema **ya tenía** antes de la edición aunque la especialidad se haya desactivado después. Así se puede corregir el nombre o la descripción sin tocar las especialidades.
  - Se pueden quitar siempre.
  - **No** se puede agregar una especialidad inactiva que el tema no tenía, ni volver a agregar una inactiva que se quitó en una edición anterior: `422 referencia_invalida`.
  - Motivo: obligar a desmarcarla para guardar cambiaba los datos como efecto lateral. Si era la única asociada, el tema pasaba a `aplica_a_todas` solo por corregir un nombre. Es el mismo criterio que T8 sigue con el tema archivado que la pregunta ya tiene.
  - Implementación: `Editar` ya lee `especialidadesAntes` con la fila bloqueada (`temas/repository.go:220`), antes de `validarEspecialidades` (`:224`, consulta en `:396-411`). La validación de T4 acepta `estado = 'activo' OR id = ANY(antes)`. T3 no cambia.
  - UI (PR #73): el formulario sigue mostrando la especialidad inactiva asociada, marcada y con la etiqueta «(inactiva)». Se puede guardar con ella marcada o desmarcarla. Al desmarcarla se puede volver a marcar mientras no se guarde, porque sigue en `antes`. Las inactivas no asociadas no se ofrecen.
- Auditoría: `tema.editar`, `detalle = {"antes": {nombre, descripcion, especialidad_ids}, "despues": {…}}`.

### T5 `PATCH /admin/temas/{id}/estado`

Rol admin. Request `{"estado": "archivado"}` (archivar) o `{"estado": "activo"}` (reactivar, E1-01 regla 6a; E1-02 regla 9). Mismo patrón que `PATCH /…/estado` de catálogos (`catalogs/routes.go:27`).

- `200` con el `Tema`.
- **Archivar** no toca `preguntas.tema_id`: las preguntas conservan su tema. Un tema archivado no aparece en T7 ni en las listas de clasificación (escenario «Archivar tema con preguntas») y sale del orden.
- **Reactivar** (archivado → activo):
  - bajo el bloqueo de la asignatura, revalida el límite (422 `limite_temas` si ya hay 60 activos; el tema sigue archivado);
  - el tema va al final del orden (`orden` = máximo de los activos + 1);
  - conserva sus preguntas y especialidades.

| Status | `error` | Cuándo |
|---|---|---|
| 404 | `no_encontrado` | |
| 409 | `estado_invalido` | El tema ya está en ese estado (archivar uno archivado o reactivar uno activo) |
| 422 | `validacion` | `estado` fuera de `activo` \| `archivado` |
| 422 | `limite_temas` | Reactivar con 60 activos |

Auditoría: `tema.archivar` o `tema.reactivar`, `detalle = {"estado_anterior", "estado_nuevo", "preguntas_count"}`; al reactivar se añade `"orden"`.

UI (E1-02 regla 9): en una fila archivada, «Reactivar» con confirmación. Si el servidor responde `limite_temas`, se muestra el mensaje y el tema sigue archivado.

### T6 `PUT /admin/asignaturas/{id}/temas/orden`

Rol admin. Reordena los temas activos de la asignatura (SIA-E1-01 regla 7; SIA-E1-02 reglas 6–8).

```json
{ "tema_ids": [41, 38, 40, 39] }
```

- `tema_ids` es la lista completa de los temas `activo` de la asignatura, en el orden nuevo. Se guarda de forma atómica: `orden = 1..n` según la posición, bajo el mismo bloqueo de la asignatura que T3 y T5, así que no compite con un alta o un archivado simultáneos.
- `200` con `{"data": [{"id": 41, "orden": 1}, …]}`.

| Status | `error` | Cuándo |
|---|---|---|
| 400 | `cuerpo_invalido` | JSON mal formado o `tema_ids` que no es un array |
| 404 | `no_encontrado` | La asignatura no existe |
| 422 | `orden_invalido` | Ids repetidos, ids ≤ 0 o más de 60 elementos (escenario «Lista de orden inválida») |
| 409 | `orden_desactualizado` | El conjunto no coincide exactamente con los temas activos actuales: falta o sobra alguno, hay un archivado o uno de otra asignatura. No se guarda nada y la UI pide recargar (escenario «Orden desactualizado») |

- Auditoría: `tema.reordenar`, `entidad = "asignatura"`, `entidad_id = asignatura_id`, `detalle = {"antes": [ids], "despues": [ids]}`.
- **Posición de los temas que entran o salen del orden:**
  - un tema nuevo (T3) y uno reactivado (T5 → `activo`) se colocan al final, con `orden` = máximo de los activos + 1;
  - un tema archivado sale del orden: conserva su valor de `orden`, pero ese valor ya no se usa.
- **UI:** el modo «Reordenar» muestra todos los activos de la asignatura sin paginación ni filtro, con botones «Subir» y «Bajar» por fila. khipu no tiene una lista ordenable con arrastre y no se crea una propia (regla del repo).

### T7 `GET /asignaturas/{id}/temas`

Cualquier usuario autenticado (`RequireAuth`), sin rol admin (E1-01, sección Seguridad: «lectura para usuario autenticado»). Devuelve **solo** los temas `activo`, sin paginación (≤ 60).

| Query | Valores |
|---|---|
| `especialidad_id` | opcional; temas de esa especialidad más los que aplican a todas |

```json
{ "data": [ { "id": 41, "nombre": "Vigilancia epidemiológica", "descripcion": null, "orden": 1, "aplica_a_todas": false, "especialidad_ids": [2, 5] } ] }
```

Errores: `404 no_encontrado` (la asignatura no existe) y `400 parametro_invalido`. No incluye `preguntas_count`, que es un dato de administración.

### T8 Campo `tema_id` en preguntas (módulo existente `internal/questions`)

Va en SIA-E1-01: el tech-lead lo confirmó (TL3).

- Request de `POST /preguntas` y `PUT /preguntas/{id}` (`questions/routes.go:37,39`): campo opcional `"tema_id": 41 | null`.
- Response de pregunta: añade `"tema_id"` y `"tema"` (nombre), o `null` si no tiene.
- `GET /preguntas` acepta el filtro opcional `tema_id`.
- **`POST`:** ausente o `null` = sin tema.
- **`PUT`** (decisión del tech-lead en el PR #74, igual que `etiqueta_ids` en H100). El campo distingue ausente de `null` (`OptionalID`, `questions/dto.go:48`), y el tema resultante lo calcula `temaTrasEditar` (`questions/repository.go:98-111`):

  | `tema_id` en el `PUT` | `asignatura_id` | Tema guardado |
  |---|---|---|
  | ausente | igual a la actual | se conserva el tema actual |
  | ausente | cambia | `null` (el tema actual es de otra área) |
  | `null` explícito | cualquiera | `null` (se quita el tema) |
  | un id | cualquiera | ese id, si pasa la validación |

- **Validación** (`validarTema`, `questions/repository.go:73-96`), dentro de la transacción y con `FOR SHARE` sobre el tema: el tema debe existir, pertenecer a la `asignatura_id` resultante de la pregunta y estar `activo`. Excepción: el tema que la pregunta ya tiene se acepta aunque esté archivado, tanto si se reenvía como si se conserva por ausencia. Si no se cumple, `422 tema_invalido` (formato de error del README). Una violación de la FK compuesta (§4) por una carrera también se mapea a `tema_invalido` (`questions/service.go:37-38,77-78`).
- La FK compuesta (§4) impide que el tema y el área queden inconsistentes.
- La UI no la exige ninguna historia del MVP. Esto da a E3-04 y E3-05 el punto de escritura. El frontend puede posponer el selector de tema en el formulario de pregunta; si el formulario no envía `tema_id`, el `PUT` conserva el tema.

---

## 2. Pesos por profesión y proceso (SIA-E1-03)

Resolución, unicidad y estado: ver la [ADR 0001](../adr/0001-unicidad-proceso-asignaturas-por-especialidad.md). En resumen:

- La configuración de (proceso, especialidad) es un **conjunto** que se guarda de forma atómica.
- `completa` ⇔ el conjunto existe (siempre suma 100); `incompleta` ⇔ no existe.
- Nunca se mezclan filas de especialidad con filas de compatibilidad.

### Objeto `ConfiguracionPesos`

```json
{
  "proceso_id": 12,
  "especialidad_id": 2,
  "especialidad": "Medicina",
  "estado": "completa",
  "areas": [
    { "asignatura_id": 7, "nombre": "Salud pública", "codigo": "SP", "peso_porcentual": 20.00, "numero_preguntas": 20, "coeficiente": 1.00 }
  ],
  "suma_pesos": 100.00,
  "suma_preguntas": 100,
  "plantilla": []
}
```

- Si `estado = "incompleta"`, `areas` va vacío y `plantilla` trae las filas de compatibilidad del proceso con el mismo formato, para prellenar el formulario. Si `estado = "completa"`, `plantilla` va vacío.

### PE1 `GET /admin/procesos/{id}/pesos`

Rol admin. Resumen por especialidad para la pantalla del proceso.

```json
{
  "proceso_id": 12,
  "compatibilidad": { "areas": 6, "suma_pesos": 100.00 },
  "especialidades": [
    { "especialidad_id": 2, "nombre": "Medicina", "estado": "completa", "areas": 6, "suma_pesos": 100.00, "suma_preguntas": 100 },
    { "especialidad_id": 5, "nombre": "Enfermería", "estado": "incompleta", "areas": 0, "suma_pesos": 0, "suma_preguntas": 0 }
  ]
}
```

`especialidades` = especialidades `activo` de la institución del proceso (`especialidades.institucion_id = procesos.institucion_id`), ordenadas por nombre. Errores: `404 no_encontrado` y `400 parametro_invalido`.

### PE2 `GET /admin/procesos/{id}/pesos/{especialidadId}`

Rol admin. `200` con `ConfiguracionPesos`. Errores: `404 no_encontrado` (proceso inexistente) y `422 referencia_invalida` (especialidad inexistente, inactiva o de otra institución).

### PE3 `PUT /admin/procesos/{id}/pesos/{especialidadId}`

Rol admin. Guarda o reemplaza **todo** el conjunto en una transacción, con `lockProceso` (`catalogs/repository.go:714`).

```json
{
  "areas": [
    { "asignatura_id": 7, "peso_porcentual": 20.00, "numero_preguntas": 20, "coeficiente": 1.00 },
    { "asignatura_id": 8, "peso_porcentual": 16.67, "numero_preguntas": 17, "coeficiente": 1.00 }
  ]
}
```

Validación, en este orden (se devuelve el primer error):

| # | Regla | Error |
|---|---|---|
| 1 | `areas`: 1–50 elementos. Por fila, las mismas reglas que las filas existentes (`catalogs/dto.go:135-140`): `numero_preguntas` 1–200, `peso_porcentual` 0,01–100 con ≤ 2 decimales, `coeficiente` 0,01–999,99 con ≤ 2 decimales | 422 `validacion` |
| 2 | Especialidad existente, `activo` y de la institución del proceso; cada `asignatura_id` existente | 422 `referencia_invalida` |
| 3 | Un `asignatura_id` repetido en `areas` (regla 7a) | 409 `peso_duplicado` (`detalle.asignatura_id`) |
| 4 | Σ `numero_preguntas` ≤ 200 | 422 `limite_preguntas` (`detalle.suma_preguntas`) |
| 5 | Σ `peso_porcentual` = 100 ± 0,01 (`pesoValido`, `catalogs/repository.go:711`) | 422 `pesos_no_suman_100` (`detalle.suma_pesos`) |

- **Reemplazo (regla 4):** el cuerpo es la configuración entera. En la transacción se borran las filas (P, E) que ya no vienen y se insertan o actualizan las que vienen; un área omitida sale de la configuración (escenario «Reemplazo de la configuración»).
- Con cualquier error **no se guarda nada**. Si no existía configuración, sigue `incompleta`; si existía una `completa`, queda intacta (escenarios «Suma inválida sin configuración previa» y «Suma inválida con configuración completa previa»).
- Las filas de compatibilidad (sin especialidad) no se tocan desde aquí (regla 2).
- `200` con `ConfiguracionPesos` (`estado: "completa"`).
- Una violación 23505 de `uq_proceso_asignaturas_proceso_esp_asig` se mapea a `409 peso_duplicado` (regla 7c, p. ej. dos guardados simultáneos).
- Auditoría: `pesos.guardar`, `entidad = "proceso"`, `entidad_id = proceso_id`, `detalle = {"especialidad_id", "antes": [{asignatura_id, peso_porcentual, numero_preguntas, coeficiente}], "despues": [...]}` (regla 10).

### PE4 (retirado)

No hay borrado de configuraciones. Según la regla 6 de E1-03, `incompleta` significa que «nunca se guardó ninguna», y la historia no pide volver atrás. El identificador PE4 queda libre para no renumerar.

### PE5 Fila de compatibilidad duplicada (endpoint existente)

`POST /admin/procesos/{id}/asignaturas` (`catalogs/routes.go:60`) ya devuelve 409 cuando la asignatura ya está asignada (`catalogs/repository.go:795-797`). Es el caso de la regla 7b. Cambios:

- El `ON CONFLICT` pasa al target de la ADR 0001.
- El cuerpo del 409 pasa a `{"error": "peso_duplicado", "message": "Esta asignatura ya está asignada al proceso."}`. Así se cumple el escenario «Unicidad de la fila de compatibilidad».
- El frontend (`frontend/src/lib/catalogs/api.ts:15`) debe leer `body.message ?? body.error` como texto y conservar `body.error` como código; ver README §3.

### PE6 Servicio de pesos para consumidores (Go, sin ruta HTTP)

Los consumidores (diagnóstico E4-01/E4-02, práctica y simulacro dirigidos E5-04/E5-05, dashboard E5-01, priorización E5-03, plan E7-02 y simulacro oficial E9-05) llaman a una sola función, sin ruta HTTP propia:

```go
type ModoPesos int
const (
    ModoEstimable ModoPesos = iota // diagnóstico, prácticas, dashboard, plan…
    ModoOficial                     // solo SIA-E9-05
)

type PesoArea struct {
    AsignaturaID    int64
    PesoPorcentual  float64
    NumeroPreguntas *int     // nil en pesos estimados
    Coeficiente     float64  // 1.00 en pesos estimados
}

type Pesos struct {
    Areas          []PesoArea
    PesosEstimados bool
}

// ErrFormatoNoConfigurado → los endpoints de E9-05 lo devuelven como 409 formato_no_configurado.
func (s *Service) Pesos(ctx context.Context, procesoID, especialidadID int64, modo ModoPesos) (Pesos, error)
```

- **Con conjunto `completa`:** devuelve sus filas y `PesosEstimados = false`, en los dos modos.
- **Sin conjunto, `ModoEstimable`:**
  - áreas = `DISTINCT asignatura_id` de las preguntas **servibles** del proceso para la especialidad: `p.proceso_id = P AND (p.especialidad_id = E OR p.especialidad_id IS NULL)` y el predicado de pregunta servible. Hoy ese predicado es `estado = 'activo'` (`questions/repository.go:659`); con SIA-E3-02 será `estado IN ('aprobada','activo')`. Debe existir como **un solo** helper compartido;
  - pesos iguales: `floor(10000 / n) / 100` por área, y los centésimos sobrantes se suman de uno en uno a las primeras áreas por `asignatura_id`, para que la suma sea exactamente 100,00;
  - `PesosEstimados = true` (escenario «Sin configuración»);
  - sin preguntas servibles, devuelve `Areas: []` y `PesosEstimados = true`. Cada consumidor decide su error.
- **Sin conjunto, `ModoOficial`:** devuelve `ErrFormatoNoConfigurado`, nunca pesos estimados (escenario «Simulacro oficial sin configuración»).
- Los consumidores informan `pesos_estimados` en sus propias respuestas (regla 8). El perfil del postulante (F2) lo expone desde ya.

### Cambios en código existente por la nueva columna

Ver la tabla «Cambios obligatorios» de la ADR 0001 (L1–L7). Son parte de SIA-E1-03 y los cubre el escenario «Compatibilidad».

---

## 3. Profesión del postulante (SIA-E1-04)

Ningún endpoint lleva `usuario_id`. Todo se resuelve con la sesión (`claims.UserID`). Un usuario no puede nombrar el perfil de otro, y el escenario «Aislamiento» se verifica comprobando que B recibe `404 profesion_no_definida` aunque A tenga perfil.

### F1 `GET /serums-ia/profesion/opciones`

Rol usuario (cualquier autenticado). Alimenta el selector.

```json
{
  "procesos": [
    {
      "id": 12, "nombre": "SERUMS 2026-I", "anio": 2026,
      "especialidades": [
        { "id": 2, "nombre": "Medicina", "codigo": "MED", "pesos": "completa" },
        { "id": 5, "nombre": "Enfermería", "codigo": "ENF", "pesos": "estimada" }
      ]
    }
  ],
  "preseleccion": { "proceso_id": 12, "especialidad_id": 5 }
}
```

- `procesos` = solo procesos donde el usuario tiene inscripción `aprobado` (`internal/examen/examen.go:141-149`). Sin ninguna, `procesos: []` y la UI explica que hace falta una inscripción aprobada.
- `especialidades` = especialidades `activo` de la institución del proceso. `pesos` es `completa` si existe el conjunto (ADR 0001) y `estimada` en otro caso (regla 2).
- `preseleccion`:
  - si hay perfil activo, es ese perfil;
  - si no hay perfil activo, `especialidad_id` es la especialidad de los datos de persona del usuario (regla 5);
  - en otro caso, `null`.
- Cómo se obtiene la especialidad de persona:
  - Se lee de la **base de datos** en cada request: `SELECT p.especialidad_id FROM usuarios u JOIN personas p ON p.id = u.persona_id WHERE u.id = $1` (FK `usuarios.persona_id`, ver `usuarios_persona_id_key` en `auth/repository.go:380`). No se usa `claims.EspecialidadID`, porque es el valor que se copió al emitir el JWT y puede estar desactualizado.
  - Solo se usa si cumple la regla 2: existe, está `activo` y figura entre las `especialidades` de algún proceso de la respuesta. Si no la cumple, `especialidad_id` es `null` (escenario «Sin preselección válida»).
  - `proceso_id` de la preselección es el único proceso disponible si hay uno solo; si hay varios, `null`.
- F1 es de solo lectura: no guarda nada y no modifica `personas` (reglas 5–6).

### F2 `GET /serums-ia/profesion`

Rol usuario. Devuelve el perfil activo.

```json
{
  "id": 301,
  "proceso": { "id": 12, "nombre": "SERUMS 2026-I" },
  "especialidad": { "id": 5, "nombre": "Enfermería" },
  "estado": "activo",
  "created_at": "2026-11-11T15:04:05Z",
  "pesos": {
    "pesos_estimados": true,
    "areas": [ { "asignatura_id": 7, "nombre": "Salud pública", "peso_porcentual": 25.00 } ]
  }
}
```

- `pesos` sale de PE6 con `ModoEstimable`. Es lo que muestra «las áreas de Enfermería» (escenario «Elegir profesión»).
- Errores:
  - `404 profesion_no_definida` si no hay perfil activo;
  - `403 inscripcion_requerida` si la inscripción del perfil dejó de estar `aprobado`. En ese caso el perfil se conserva, pero el módulo no se usa.

### F3 `PUT /serums-ia/profesion`

Rol usuario. Elige o cambia la profesión.

```json
{ "proceso_id": 12, "especialidad_id": 5, "confirmar_reinicio": false }
```

Lógica, verificada en el servidor, en este orden:

| # | Regla | Resultado |
|---|---|---|
| 1 | Cuerpo válido (ids > 0) | si no, 400 `cuerpo_invalido` / 422 `validacion` |
| 2 | Inscripción `aprobado` del usuario en `proceso_id`. Un proceso inexistente cuenta igual, para no revelar qué existe | si no, **403 `inscripcion_requerida`** (escenario «Sin inscripción aprobada») |
| 3 | Especialidad existente, `activo` y de la institución del proceso | si no, **422 `especialidad_invalida`** (escenario «Profesión inválida») |
| 4 | El perfil activo ya es el mismo par | 200 sin cambios (idempotente) |
| 5 | Hay un perfil activo distinto y `confirmar_reinicio != true` | **409 `confirmacion_requerida`**, `detalle = {"actual": {"proceso_id", "especialidad_id"}}`. La UI muestra el diálogo de confirmación y reenvía con `true` |
| 6 | En otro caso | En una transacción: archiva el perfil activo (`estado = 'archivado'`, `archivado_at = NOW()`) y crea el nuevo `activo`. Responde `200` con el objeto de F2 |

- Una violación 23505 de `uq_postulante_profesiones_usuario_activo` (carrera entre dos PUT) se mapea a `409 estado_invalido`.
- **No toca `personas`** (regla 6; escenario «Elegir otra profesión no cambia los datos de persona»). La elección vive solo en `postulante_profesiones`.
- **Reinicio del dominio (regla 3, escenario «Cambio de profesión»):** el modelo de dominio de SIA-E4-03 colgará de `postulante_profesiones.id`. Al archivar el perfil, su dominio queda archivado con él y el perfil nuevo empieza vacío. Los intentos históricos (`practicas`, `simulacros`) no se tocan.
- **Auditoría:** no aplica (no es una acción de admin, README §Auditoría). La profesión no se escribe en logs junto a datos personales (E1-04, sección Seguridad). En los logs de error solo van ids.

---

## 4. Modelo de datos y migraciones

Los números son una propuesta. Hoy la última migración es `058_add_simulacro_asignaturas_asignatura_index.sql`. El número definitivo lo fija el tech-lead al mergear, porque goose falla ante versiones faltantes.

### 059 — temas (SIA-E1-01)

```sql
-- +goose Up
CREATE TABLE temas (
    id            BIGSERIAL    PRIMARY KEY,
    asignatura_id BIGINT       NOT NULL REFERENCES asignaturas(id),
    nombre        VARCHAR(120) NOT NULL CHECK (char_length(nombre) BETWEEN 3 AND 120),
    nombre_clave  VARCHAR(120) NOT NULL,          -- plegar(nombre), calculado en Go
    descripcion   VARCHAR(500),
    estado        VARCHAR(20)  NOT NULL DEFAULT 'activo' CHECK (estado IN ('activo', 'archivado')),
    orden         INT          NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_temas_asignatura_nombre_clave UNIQUE (asignatura_id, nombre_clave),
    CONSTRAINT uq_temas_id_asignatura UNIQUE (id, asignatura_id)   -- destino de la FK compuesta
);
CREATE INDEX idx_temas_asignatura_estado_orden ON temas (asignatura_id, estado, orden);

CREATE TABLE tema_especialidades (
    tema_id         BIGINT NOT NULL REFERENCES temas(id) ON DELETE CASCADE,
    especialidad_id BIGINT NOT NULL REFERENCES especialidades(id),
    PRIMARY KEY (tema_id, especialidad_id)
);
CREATE INDEX idx_tema_especialidades_especialidad ON tema_especialidades (especialidad_id);

ALTER TABLE preguntas ADD COLUMN tema_id BIGINT;
-- MATCH SIMPLE: con tema_id NULL no se comprueba (banco existente).
ALTER TABLE preguntas ADD CONSTRAINT fk_preguntas_tema
    FOREIGN KEY (tema_id, asignatura_id) REFERENCES temas (id, asignatura_id);
CREATE INDEX idx_preguntas_tema ON preguntas (tema_id) WHERE tema_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_preguntas_tema;
ALTER TABLE preguntas DROP CONSTRAINT IF EXISTS fk_preguntas_tema;
ALTER TABLE preguntas DROP COLUMN IF EXISTS tema_id;
DROP TABLE IF EXISTS tema_especialidades;
DROP TABLE IF EXISTS temas;
```

- **Compatibilidad:** `preguntas.tema_id` nace `NULL` para todo el banco existente (regla 5) y ninguna fila se reescribe. El Down quita solo lo nuevo: las preguntas existentes conservan todas sus columnas originales (escenario «Migración reversible»).
- La FK compuesta impide que una pregunta tenga un tema de otra asignatura.
- La FK simple (sin `ON DELETE`) impide borrar un tema con preguntas (regla 4). No hay endpoint de borrado: solo se archiva.

### 060 — `proceso_asignaturas.especialidad_id` (SIA-E1-03)

Ver el SQL completo, el Down y la compatibilidad en la [ADR 0001 § Migración](../adr/0001-unicidad-proceso-asignaturas-por-especialidad.md#migración-goose-número-propuesto-060).

### 061 — perfil del postulante (SIA-E1-04)

```sql
-- +goose Up
CREATE TABLE postulante_profesiones (
    id              BIGSERIAL   PRIMARY KEY,
    usuario_id      BIGINT      NOT NULL REFERENCES usuarios(id),
    proceso_id      BIGINT      NOT NULL REFERENCES procesos(id),
    especialidad_id BIGINT      NOT NULL REFERENCES especialidades(id),
    estado          VARCHAR(20) NOT NULL DEFAULT 'activo' CHECK (estado IN ('activo', 'archivado')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archivado_at    TIMESTAMPTZ,
    CONSTRAINT ck_postulante_profesiones_archivado
        CHECK ((estado = 'archivado') = (archivado_at IS NOT NULL))
);
-- Regla 4: máximo un par (proceso, profesión) activo por usuario (patrón de 049:29-33).
CREATE UNIQUE INDEX uq_postulante_profesiones_usuario_activo
    ON postulante_profesiones (usuario_id) WHERE estado = 'activo';
CREATE INDEX idx_postulante_profesiones_usuario      ON postulante_profesiones (usuario_id, created_at DESC);
CREATE INDEX idx_postulante_profesiones_proceso      ON postulante_profesiones (proceso_id);
CREATE INDEX idx_postulante_profesiones_especialidad ON postulante_profesiones (especialidad_id);

-- +goose Down
DROP TABLE IF EXISTS postulante_profesiones;
```

- Tabla nueva: no afecta datos existentes.
- Es dato de aprendizaje: SIA-E9-07 la incluirá en la exportación y el borrado.

---

## 5. Trazabilidad

| Historia | Escenario / regla | Cubierto por |
|---|---|---|
| E1-01 | Crear tema (201, activo, audit `tema.crear`) | T3, tabla `temas` |
| E1-01 | Nombre duplicado (422 `tema_duplicado`) | T3/T4, `uq_temas_asignatura_nombre_clave` + `plegar` |
| E1-01 | Límite de temas (422 `limite_temas`) | T3, T5 (reactivar), bloqueo de la asignatura |
| E1-01 | Archivar tema con preguntas | T5, T7 (solo activos), T8 (no acepta archivados nuevos; conserva el actual aunque esté archivado) |
| E1-01 | Archivados no cuentan para el límite | T3: conteo solo de `activo` |
| E1-01 | Nombre de un tema archivado → 422 `tema_duplicado` | `uq_temas_asignatura_nombre_clave` (incluye los archivados) |
| E1-01 | Reactivar tema (al final, conserva preguntas y especialidades, audit) | T5 `estado = activo` |
| E1-01 | Reactivar por encima del límite → 422, sigue archivado | T5 |
| E1-01 | Reactivar un tema activo → 409 `estado_invalido` | T5 |
| E1-01 | Permisos (403) | `RequireAdmin` en T1–T6 |
| E1-01 | Migración reversible | 059 Up/Down |
| E1-01 | Regla 2 (3–120 / ≤ 500) | Validación T3/T4 + `CHECK` |
| E1-01 | Regla 3 (0..N especialidades) | `tema_especialidades`, `aplica_a_todas` |
| E1-01 | Regla 5 (`tema_id` nulo) | 059, T8 (`POST` sin tema; `PUT`: ausente conserva, `null` quita, cambio de área sin `tema_id` → `null`) |
| E1-01 | Tema de otra área, inexistente o archivado nuevo → 422 | T8 `tema_invalido` (`validarTema` + FK compuesta) |
| E1-02 | Listar por área (nombre, especialidades, nº preguntas, estado) | T1 (`preguntas_count`) |
| E1-02 | Estado vacío | T1 `data: []` (UI) |
| E1-02 | Editar con error de validación (sin petición) | UI con yup usando las mismas reglas de §1; T4 lo revalida |
| E1-02 | Editar un tema con una especialidad asociada que se desactivó | T4 acepta las ya asociadas; rechaza agregar inactivas nuevas (`referencia_invalida`) |
| E1-02 | Archivar con confirmación (nº de preguntas, audit) | T1/T2 `preguntas_count`, T5 audit |
| E1-02 | Acceso de usuario (redirige) | guard admin de `(dashboard)/layout.tsx` + 403 de la API |
| E1-02 | Falla de red (conserva datos) | UI (formik conserva valores ante un 500) |
| E1-01 | Regla 7 (orden; un tema nuevo va al final) | `temas.orden`, T3 |
| E1-02 | Reordenar temas (+ audit `tema.reordenar`) | T6 |
| E1-02 | Orden desactualizado → 409 | T6 `orden_desactualizado` |
| E1-02 | Lista de orden inválida → 422 | T6 `orden_invalido` |
| E1-02 | Reactivar desde la pantalla (confirmación; `limite_temas` visible) | T5 + UI |
| E1-03 | Configuración completa (audit) | PE3, `pesos.guardar` |
| E1-03 | Suma inválida sin configuración previa (nada guardado, `incompleta`) | PE3 validación 5, transacción todo o nada |
| E1-03 | Suma inválida con configuración completa previa (intacta) | PE3 validación 5, transacción todo o nada |
| E1-03 | Reemplazo de la configuración (el área omitida sale) | PE3 reemplazo |
| E1-03 | Exceso de preguntas (422 `limite_preguntas`, nada guardado) | PE3 validación 4 |
| E1-03 | Pesos distintos por especialidad | PE2/PE3 por `especialidadId`, resolución por conjunto (ADR 0001) |
| E1-03 | Unicidad (área repetida en el cuerpo) → 409 | PE3 validación 3 (regla 7a) + constraint `uq_proceso_asignaturas_proceso_esp_asig` (regla 7c) |
| E1-03 | Unicidad de la fila de compatibilidad → 409 | PE5 + la misma constraint (`NULLS NOT DISTINCT`) |
| E1-03 | Compatibilidad (proceso no SERUMS, simulacro igual) | ADR 0001: L1–L7, resolución sin filas de especialidad = comportamiento actual |
| E1-03 | Sin configuración → pesos iguales, `pesos_estimados` | PE6 `ModoEstimable` |
| E1-03 | Simulacro oficial → 409 `formato_no_configurado` | PE6 `ModoOficial` (lo expone E9-05) |
| E1-03 | Migración reversible (filas sin especialidad intactas) | 060 Down |
| E1-03 | Permisos (403) | `RequireAdmin` en PE1–PE3 |
| E1-03 | Regla 10 (antes/después) | audit `pesos.guardar` |
| E1-03 | Pantalla sin configuración («se usan pesos estimados») | PE2 `estado = "incompleta"`, `areas: []`, `plantilla` + UI |
| E1-03 | Suma en vivo y guardado bloqueado | UI con yup usando las reglas de PE3 (validaciones 3–5) |
| E1-03 | Guardar desde la pantalla | PE3 → `200` con `estado = "completa"` |
| E1-03 | Error del servidor (sin perder lo escrito) | Códigos distinguibles en PE3 (`pesos_no_suman_100`, `limite_preguntas`, `peso_duplicado`, `referencia_invalida`, `validacion`) + formik conserva los valores |
| E1-03 | Acceso a la pantalla | guard admin del layout + `RequireAdmin` en PE1–PE3 |
| E1-04 | Elegir profesión (muestra áreas) | F3 + F2 `pesos` |
| E1-04 | Preselección desde los datos de persona (sin guardar) | F1 `preseleccion` leída de la BD |
| E1-04 | Sin preselección válida | F1 `preseleccion.especialidad_id = null` |
| E1-04 | Elegir otra profesión no cambia los datos de persona | F3 no toca `personas` |
| E1-04 | Sin inscripción aprobada → 403 | F3 regla 2; F1 `procesos: []` para la explicación |
| E1-04 | Cambio de profesión (archiva dominio, el nuevo vacío) | F3 reglas 5–6, `postulante_profesiones.estado` (el dominio llega con E4-03) |
| E1-04 | Profesión inválida → 422 | F3 regla 3 |
| E1-04 | Aislamiento → 404 | Sin `usuario_id` en rutas; F2 `404 profesion_no_definida` |
| E1-04 | Regla 4 (un par activo) | `uq_postulante_profesiones_usuario_activo` |

## 6. UI implicada (khipu)

Componentes ya copiados en `frontend/src/components`:

| Pantalla | Componentes |
|---|---|
| Admin de temas (E1-02) | `TablaListado` (`components/app/` envolviendo `datos/tabla-listado.tsx`), `PillEstado` (`patrones/pill-estado.tsx`), `EstadoVacio` (`feedback/estado-vacio.tsx`), `Dialog` y `CabeceraDialogo`, `Campo`, `SelectCampo`, un grupo de `Casilla` para las especialidades, `PieTabla` y `Pagination` |
| Selector de profesión (E1-04) | `SelectCampo`, `Dialog` (confirmación de reinicio), `Alerta` o `Banner` (sin inscripción, pesos estimados), `Skeleton` (carga) |
| Pesos (E1-03 reglas 11–13) | `SelectCampo` (proceso y especialidad), `PillEstado` (`completa`/`incompleta`), `TablaDatos` con `EntradaMonto` o `StepperNumerico` por área, `Combobox` para añadir un área (desde `GET /asignaturas`, `catalogs/routes.go:44`), `Alerta` para la suma y el total en vivo, `EstadoVacio` («Sin configuración: se usan pesos estimados»), `Skeleton`, `BotonAsync` |

Si alguno no alcanza (por ejemplo, para la selección múltiple de especialidades), primero se abre un PR en khipu (regla del repo).

**Pantalla de pesos (Q9, resuelta):** lee con PE1 y PE2 (estado y `plantilla` para prellenar) y guarda con PE3. No necesita endpoints nuevos. El formulario aplica en el cliente las validaciones 3–5 de PE3: sin áreas repetidas, Σ preguntas ≤ 200 y Σ pesos = 100 ± 0,01. Los errores del servidor se distinguen por código (`pesos_no_suman_100`, `limite_preguntas`, `peso_duplicado`, `referencia_invalida`, `validacion`).
