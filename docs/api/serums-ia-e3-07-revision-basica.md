# Contrato API — SERUMS IA, SIA-E3-07 Revisión básica de preguntas

- **Historia:** [SIA-E3-07](../stories/serums-ia/E3-07-revision-basica-preguntas.md). Issue #155. Las pantallas son SIA-E3-08 (listado y detalle con citas, #156), SIA-E3-09 (acciones, #157) y SIA-E3-10 (avisos, #158). Las historias llegan con el PR #153 (rama `docs/sia-e3-02-decisiones`) y el próximo PR de historias del PO; hasta su merge, los enlaces no resuelven en esta rama.
- **Estado:** borrador para revisión del tech-lead (2026-11-11). Reglas acordadas entre el PO y el tech-lead el 2026-11-11.
- **Extiende:** el contrato de [SIA-E3-02](serums-ia-e3-02-ciclo-preguntas.md). Donde los dos difieran, **manda este**. E3-02 §4.1, §4.2, §4.4 y §7 ya remiten aquí.
- **ADR:** [0007](../adr/0007-busqueda-pasajes-admin-fuera-del-puerto.md), búsqueda de pasajes para el admin.
- **Convenciones:** [README.md](README.md). Todos los errores usan el **formato 2**.
- **Migración:** **ninguna**. Todo cabe en la 065 de E3-02 (`backend/migrations/065_add_ciclo_preguntas.sql`).
- **Anclas de código:** worktree `learning-backend`, rama `feature/serums-ia-e3-02-ciclo-preguntas` (develop con E2 S3–S7, #132 y #138, más B1 y B2 de E3-02).
- **Release:** E3-02 no sale sin E3-07 y E3-08 (decisión del PO).

## 1. Qué añade y qué cambia

| # | Lugar | Hoy | Cambio |
|---|---|---|---|
| R1 | Búsqueda de pasajes | Solo existe el puerto `BuscarPasajes` (`fuentes/source.go:188-190`; `pgsource.go:165-191`). Exige `EspecialidadID > 0` (`source.go:207`) y no devuelve el título de la fuente. No hay ruta HTTP (`fuentes/routes.go:21-43`) | Ruta admin nueva `GET /admin/pasajes` (§2), con una consulta admin fuera del puerto que comparte el SQL del adaptador ([ADR 0007](../adr/0007-busqueda-pasajes-admin-fuera-del-puerto.md)) |
| R2 | `PATCH /preguntas/{id}/estado` (E3-02 §4.1) | Sin guardas por origen ni confirmación | Cuerpo definitivo con `verificado` y `nota`. Guardas `revision_especializada`, `verificacion_requerida` y `nota_obligatoria`, con un orden total de validación (§3) |
| R3 | `PUT /preguntas/{id}/citas` (E3-02 §4.2) | `409 estado_invalido` en `aprobada`; se puede citar una fuente retirada; más de 20 da `422 validacion` | Se permite en `aprobada`, con la guarda `ultima_cita_activa`. Las citas **nuevas** exigen una fuente `activa` (`fuente_no_activa`). Más de 20 da `limite_citas` (§4) |
| R4 | Objeto pregunta, `citas[]` (E3-02 §4.4; `questions/dto.go:112-118`, `CitaResponse`) | Sin texto del pasaje | Suma `seccion`, `linea_inicio`, `linea_fin`, `extracto` y `texto` (el pasaje completo), para que E3-08 muestre y el admin verifique las citas sin otra llamada (§5) |
| R5 | `internal/texto` (`texto.go:35`, `Plegar`) | — | Función nueva `texto.Extracto(s string) string`, única regla del extracto para R1 y R4 (§5) |

- **Sin cambios:** la máquina de estados (E3-02 §3), el `DELETE` (E3-02 P2), el `PUT /preguntas/{id}` (E3-02 §4.5) y el suscriptor de retiro (E3-02 §5).
- **Importación Excel (confirmado, 2026-11-11):** `POST /preguntas/import` crea cada fila con `svc.CreatePregunta` (`questions/importer.go:238-259`), igual que el alta manual. Las preguntas importadas nacen con **`estado = 'borrador'` y `origen = 'manual'`** (E3-02 regla 2 y P1; E3-02 I8). El Excel no es un balotario: el origen `balotario` lo fija solo la importación de SIA-E3-01. Así, una pregunta del Excel se aprueba por esta vía básica (§3.2, paso 7).
- **Listado (necesidad de E3-08):** `GET /preguntas` ya trae, por E3-02 §4.3, los filtros exactos `estado` y `origen` y `citas_count` en cada item. E3-07 no lo cambia.
- **`conflicto_clave` (regla 4 de la historia): diferido a SIA-E3-01/E3-03** (confirmado por el PO, 2026-11-11). El código `409 conflicto_clave` queda **reservado**, con su lugar en el orden de §3 (paso 8), pero **no se implementa** en E3-07. El dato que lo dispara (una clave en conflicto sin resolver, de SIA-E3-06) lo crean E3-01/E3-03, y la regla se activa con ellas, sin columna nueva ahora. El escenario «Conflicto de clave» y su test de la DoD se validan con esas historias; qa cierra E3-07 sin ese escenario.
- **No hay `falta_area` ni `falta_tema`:** `asignatura_id` es obligatoria (`014_create_preguntas.sql`) y el tema es opcional en el MVP.

## 2. `GET /admin/pasajes` (nuevo; admin)

Búsqueda por texto en los pasajes de la versión vigente de las fuentes `activa`, por relevancia (regla 1).

| Query | Regla | Error |
|---|---|---|
| `q` | Obligatorio. **3–300 runes** tras `TrimSpace`: los límites del puerto (`MinConsulta`, `MaxConsulta`, `source.go:34-35`), no los ≤ 100 de las búsquedas de listados (README §6), porque es la misma búsqueda del puerto | **`422 consulta_invalida`** (`detalle = {"min": 3, "max": 300}`) |
| otro parámetro | Se ignora, como en el resto de los listados | — |

- **Sin filtro de profesión, área ni tema** (YAGNI): la historia pide solo texto. La consulta admin busca en **todas** las fuentes `activa` (ADR 0007).
- **Sin paginación:** devuelve como máximo **20** resultados (`MaxResultados`, `source.go:36`) dentro de `{"data": [...]}`, como las listas acotadas (README §5). No hay parámetro `limite`.
- **Orden:** `ts_rank_cd` descendente y `pasaje_id` ascendente, igual que el puerto (`pgsource.go:182`).
- Una consulta que, plegada, queda sin términos buscables (solo palabras vacías) devuelve `200 {"data": []}`, como el puerto (`pgsource.go:161-164`). La UI muestra «Sin resultados».

```json
{
  "data": [
    {
      "pasaje_id": 9012,
      "fuente_id": 9,
      "fuente_titulo": "NTS de vigilancia del dengue",
      "version": 2,
      "seccion": "3.2 Definición de caso",
      "linea_inicio": 120,
      "linea_fin": 148,
      "extracto": "Caso probable de dengue sin signos de alarma: toda persona con fiebre de hasta siete días…"
    }
  ]
}
```

- **Nunca** trae `nombre_archivo`, `sha256`, el contenido de la versión ni ninguna ruta. Solo los campos de arriba.
- `seccion` puede ser `null`.
- **Logs:** nunca se registra `q` (README §9). El backend no tiene logger de peticiones que escriba la URL: el único middleware que mira la URL usa `r.URL.Path` (`middleware/bodylimit.go:39`). Si un día se agrega uno, tiene que omitir la query de esta ruta. Hay precedente de `q` en `GET` en `GET /admin/fuentes` (`fuentes/listado.go:40`).
- **Errores:** `401`/`403` del middleware; `422 consulta_invalida`; `500 error_interno`.
- Sin auditoría: es una lectura.

## 3. `PATCH /preguntas/{id}/estado`: versión definitiva

Sustituye el cuerpo y la tabla de validaciones de E3-02 §4.1. Lo demás de §4.1 se mantiene: bloqueo, `200` con el objeto pregunta y auditoría.

### 3.1 Cuerpo

```json
{ "estado": "aprobada", "verificado": true, "nota": "Revisada contra la NTS 2024, pasaje 3.2." }
```

| Campo | Tipo | Uso |
|---|---|---|
| `estado` | string, obligatorio | `aprobada` \| `rechazada` \| `borrador` \| `en_revision` |
| `motivo` | string \| null | Rechazar (obligatorio) y pasar a `en_revision` (opcional). Igual que E3-02 |
| `verificado` | boolean \| null | Solo para `aprobada`: la casilla «He verificado la pregunta y su clave contra la cita» |
| `nota` | string \| null | Solo para `aprobada` |

- El cuerpo trae los **cuatro campos desde el inicio**. Los handlers decodifican con `DisallowUnknownFields`, así que agregarlos después rompería a los clientes, incluido el seed B8 de E3-02.
- Un tipo incorrecto (p. ej. `"verificado": "si"`) da `400 cuerpo_invalido`.
- Con un `estado` distinto de `aprobada`, `verificado` y `nota` **se ignoran**: no se validan ni se guardan.

### 3.2 Validaciones, en orden (se devuelve el primer error)

| # | Validación | Error |
|---|---|---|
| 1 | JSON válido, sin campos desconocidos y dentro del límite de cuerpo; `{id}` entero > 0 | `400 cuerpo_invalido` / `400 parametro_invalido` / `413 cuerpo_demasiado_grande` |
| 2 | `estado` ∈ `aprobada`, `rechazada`, `borrador`, `en_revision` | `422 validacion` (`detalle.campos.estado`) |
| 3 | `motivo`: obligatorio para `rechazada`, 5–500 runes tras `TrimSpace`; opcional para `en_revision`, ≤ 500 | `422 validacion` (`detalle.campos.motivo`) |
| 4 | La pregunta existe y no está `eliminada` | `404 no_encontrado` |
| 5 | Bloqueo (E3-02 §4.1): fuentes de las citas `FOR SHARE` `ORDER BY id`, después la pregunta `FOR UPDATE`, y relectura de las citas. Si cambiaron | `409 estado_invalido` |
| 6 | La transición está permitida (E3-02 §3) | `409 estado_invalido` (`detalle = {"estado_actual"}`) |
| | **Los pasos 7–11 solo corren si `estado = "aprobada"`** | |
| 7 | **Aprobar** (`borrador` → `aprobada`) solo si `origen = 'manual'`. Revalidar (`en_revision` → `aprobada`) se permite con cualquier origen | **`409 revision_especializada`** (`detalle = {"origen"}`) |
| 8 | *Reservado:* sin `conflicto_clave` sin resolver. No se implementa en E3-07 (§1) | *`409 conflicto_clave`* |
| 9 | ≥ 1 cita a un pasaje de una fuente `activa`, en **todos los procesos** (decisión del usuario; E3-02 §4.1, paso 6) | `422 falta_cita` |
| 10 | **Nota.** Revalidar con origen `balotario` o `ia`: obligatoria, **10–500 runes** tras `TrimSpace`. Cualquier otro caso (aprobar `manual` o revalidar `manual`): opcional, ≤ 500 | **`422 nota_obligatoria`** (`detalle = {"min": 10, "max": 500}`) / `422 validacion` (`detalle.campos.nota`) |
| 11 | `verificado === true`. Ausente, `null` o `false` no valen | **`422 verificacion_requerida`** |

- **Por qué este orden:** cada escenario de la historia da su código sin depender de los campos que no menciona.
  - «Concurrencia» (otro admin la rechazó): el paso 6 responde `estado_invalido` antes que cualquier guarda.
  - «Origen especializado»: el paso 7 responde `revision_especializada` con o sin cita, nota o casilla.
  - «Aprobar sin cita»: el paso 9 responde `falta_cita` aunque falte la casilla.
  - «Revalidar una de IA sin nota»: el paso 10 responde `nota_obligatoria` aunque falte la casilla.
  - «Aprobar sin confirmación» (manual con cita): la nota es opcional, así que llega al paso 11.
- **Una sola validación de la nota:** para revalidar `balotario`/`ia`, todo lo que esté fuera de 10–500 (ausente, corta o larga) es `nota_obligatoria`, como la tabla acordada. En los demás casos, solo puede fallar por larga, y es `validacion`.
- Las guardas viven en el **service**, no en el handler: así las cumple también el seed de demo (B8 de E3-02), que aprueba con `Verificado: true`, sin nota y solo con preguntas `manual`.

### 3.3 Auditoría

En la misma transacción, como en E3-02 §4.1: `accion = "pregunta.estado"`, `entidad = "pregunta"`.

| Transición | `detalle` |
|---|---|
| A `aprobada` (aprobar o revalidar) | `{"estado_anterior", "estado_nuevo", "verificado": true, "nota"}`. `nota` es `null` si no se envió |
| A `rechazada` o `en_revision` | `{"estado_anterior", "estado_nuevo", "motivo"}` (sin cambios) |
| A `borrador` (reabrir) | `{"estado_anterior", "estado_nuevo", "motivo": null}` |

La nota y el motivo van **solo** en `audit_log` (regla 8 de la historia; README §7). Nunca en los logs de aplicación, que llevan solo ids y códigos.

## 4. `PUT /preguntas/{id}/citas`: versión definitiva

Sustituye las reglas de E3-02 §4.2. Se mantienen: el cuerpo `{"pasaje_ids": [...]}`, que la lista **reemplaza** el conjunto de citas, el orden de bloqueo, `200` con el objeto pregunta y la acción de auditoría.

### 4.1 Estados

Se permite en `borrador`, `en_revision`, `rechazada`, **`aprobada`** y `activo`. En `eliminada` responde `404 no_encontrado`, como el resto de `/preguntas/{id}`. Cambiar las citas **nunca** cambia el estado.

### 4.2 Validaciones, en orden

Sean `actuales` las citas de la pregunta tras el bloqueo, `nuevas = pasaje_ids − actuales` y `quitadas = actuales − pasaje_ids`.

**El estado que cuenta es el leído con `FOR UPDATE` en el paso 5**, no el del paso 4: el paso 4 solo evita bloquear si la pregunta no existe. Si tras el bloqueo la pregunta está `eliminada`, responde `404 no_encontrado`. El paso 8 usa ese estado bloqueado: así una aprobación concurrente (que bloquea la misma fila) y un `PUT` que quita la última cita no pueden dejar una `aprobada` sin cita. Lo mismo vale en §3.2: los pasos 6–11 usan el estado y el origen leídos tras el paso 5.

| # | Validación | Error |
|---|---|---|
| 1 | JSON válido, sin campos desconocidos y dentro del límite; `{id}` entero > 0 | `400 cuerpo_invalido` / `400 parametro_invalido` / `413 cuerpo_demasiado_grande` |
| 2 | `pasaje_ids` presente y no `null` (`[]` vale: quita todas); cada id entero > 0; sin repetidos | `422 validacion` (`detalle.campos.pasaje_ids`) |
| 3 | Como máximo **20** ids | **`422 limite_citas`** (`detalle = {"maximo": 20}`) |
| 4 | La pregunta existe y no está `eliminada` | `404 no_encontrado` |
| 5 | Bloqueo de E3-02 §4.2 (sin cambios): se resuelven los `fuente_id` de `pasaje_ids` y se unen con los de las citas actuales; esas fuentes `FOR SHARE` `ORDER BY id`; la pregunta `FOR UPDATE`; relectura de las citas actuales. Si sus fuentes ya no están dentro de la unión | `409 estado_invalido` |
| 6 | Todos los `pasaje_ids` existen. Se comprueba **después** del bloqueo, porque una versión se puede archivar y borrar sus pasajes (`fuentes/repository.go:326`, `liberarVersion`) | `422 referencia_invalida` (`detalle = {"pasaje_ids": [faltantes]}`) |
| 7 | Cada pasaje de `nuevas` es de una fuente en estado `activa` (`fuentes.estado`, por `pasajes.fuente_version_id` → `fuente_versiones.fuente_id`), de cualquier versión de esa fuente | **`422 fuente_no_activa`** (`detalle = {"pasaje_ids": [los de fuentes no activas]}`) |
| 8 | Si la pregunta está `aprobada` y `quitadas` no está vacío: al menos un pasaje de `pasaje_ids` es de una fuente `activa` | **`409 ultima_cita_activa`** |

- **`fuente_no_activa` solo mira las citas nuevas.** Una cita que ya estaba se puede conservar aunque su fuente se haya retirado: el cliente reenvía la lista completa. Es lo que pide «Revalidar tras fuente retirada»: el admin envía `[cita_retirada, cita_nueva_activa]` y solo se valida la nueva. La cita retirada no cuenta para aprobar (§3.2, paso 9).
- **`ultima_cita_activa` solo mira las quitas.** En una `aprobada`, agregar siempre se permite, y una sustitución en un solo `PUT` (quitar A y poner B activa) también. «Quitar la última cita de una aprobada»: `[A] → []` da `409`; `[A] → [A, B]` y después `[A, B] → [B]` dan `200`.
- **`activo` heredado no tiene la guarda** (confirmado por el PO, 2026-11-11): el banco heredado se sirve sin citas (E3-02 §2, Compatibilidad), así que puede quedarse sin ellas, como hoy. Si se edita su contenido, pasa a `en_revision` (E3-02 §4.5) y entonces necesita una cita activa para revalidarse.
- **El origen `balotario`** sigue la excepción de E3-02 (regla 3, P4): su cita es a la propia fuente del balotario y cuenta solo si está `activa`. No hay rama especial en el código.
- **Lista idéntica a la actual:** `200` sin escribir nada y **sin** fila de auditoría («cada cambio se audita»; no cambió nada).

### 4.3 Auditoría

`pregunta.citas`, `entidad = "pregunta"`, `detalle = {"antes": [ids], "despues": [ids]}`, ids ordenados de forma ascendente (sin cambios respecto de E3-02).

## 5. Extracto y objeto pregunta

- **`texto.Extracto(s string) string`** en `internal/texto` (junto a `Plegar`, `texto.go:35`). Es la única regla, porque `internal/questions` no importa `internal/fuentes` (E3-02 §5):
  1. reemplaza cada secuencia de espacios en blanco (saltos de línea incluidos) por un espacio y aplica `TrimSpace`;
  2. si quedan más de **300 runes**, corta en 300 y agrega `…` (U+2026).

  Es texto plano: la UI lo muestra como texto, sin interpretar Markdown.
- **`citas[]` del objeto pregunta** (E3-02 §4.4) suma cinco campos. Valen para `GET /preguntas/{id}` y para las respuestas de `PATCH …/estado`, `PUT …/citas` y `PUT /preguntas/{id}`:

  ```json
  { "pasaje_id": 9012, "fuente_id": 9, "version": 2, "fuente_titulo": "NTS de vigilancia del dengue",
    "fuente_estado": "activa", "seccion": "3.2 Definición de caso", "linea_inicio": 120, "linea_fin": 148,
    "extracto": "Caso probable de dengue sin signos de alarma: …",
    "texto": "Caso probable de dengue sin signos de alarma: toda persona con fiebre de hasta siete días…(pasaje completo)" }
  ```

  - **`texto` es el pasaje completo** (decisión del PO, 2026-11-11): el extracto sirve para elegir un pasaje, pero no para verificar la clave contra la cita. Lo acota la regla de pasajes (≤ 800 palabras, ADR 0003 § Normalización): con 20 citas, la respuesta queda en el orden de 100–150 KB en el peor caso. E3-08 lo muestra como texto plano expandible; el visor con resaltado sigue fuera de alcance.
  - `texto` va solo en `citas[]` del objeto pregunta. La búsqueda (§2) **no** lo trae: solo `extracto`.
  - Se leen en la misma consulta que ya resuelve `fuente_titulo` y `fuente_estado`, con un `JOIN` a `pasajes`. No se llama a `ResolverCita` por cada cita. El texto es el de la versión citada (inmutable, ADR 0003), aunque la fuente tenga versiones nuevas o esté retirada.
  - El listado `GET /preguntas` sigue con `citas_count` y sin `citas` (E3-02 §4.3).

## 6. Seguridad

- Las tres rutas son **solo admin**: `RequireAuth` + `RequireAdmin`. El rol `usuario` recibe `403` (escenario «Acceso»).
- La búsqueda solo ve fuentes `activa` y no expone rutas ni metadatos de archivo (§2). Una cita a una fuente retirada se ve en el detalle de la pregunta con `fuente_estado = "retirada"`, para que el admin sepa por qué ya no cuenta.
- Estado, origen, citas y confirmación se validan en el servidor, dentro de la transacción y con el orden de bloqueo fuente → pregunta, el mismo que usa el retiro (`fuentes/gestion.go:31-58`: *advisory* → `bloquearParaCambio`, `repository.go:298-303` → suscriptores).
- Logs: solo ids y códigos. Nunca `q`, el motivo, la nota ni el texto de la pregunta o del pasaje.

## 7. Errores nuevos

| Código | HTTP | Dónde |
|---|---|---|
| `consulta_invalida` | 422 | §2. Ya existía como error del puerto, sin HTTP (README §4); aquí se expone |
| `revision_especializada` | 409 | §3.2, paso 7 |
| `conflicto_clave` | 409 | §3.2, paso 8: **reservado**, se activa con E3-01/E3-03 |
| `nota_obligatoria` | 422 | §3.2, paso 10 |
| `verificacion_requerida` | 422 | §3.2, paso 11 |
| `limite_citas` | 422 | §4.2, paso 3 |
| `fuente_no_activa` | 422 | §4.2, paso 7 |
| `ultima_cita_activa` | 409 | §4.2, paso 8 |

Se suman a los de E3-02 §7 (`estado_invalido`, `falta_cita`, `validacion`, `referencia_invalida`, `no_encontrado`…). E3-08 y E3-09 los traducen con `dashboard.serumsIa.errores.<codigo>` (README §8; reparto de claves en §9). Ante cualquier `409`, la pantalla recarga la pregunta.

## 8. Trazabilidad con los criterios de aceptación

| Escenario | Cómo se cumple |
|---|---|
| Aprobar una pregunta manual con cita | §3.2 pasa los pasos 7–11; `PreguntaServible` incluye `aprobada` (E3-02 I1); auditoría §3.3 |
| Aprobar sin cita | §3.2, paso 9: `422 falta_cita` |
| Aprobar sin confirmación | §3.2, paso 11: `422 verificacion_requerida` |
| Origen especializado | §3.2, paso 7: `409 revision_especializada`; la pregunta no cambia |
| Revalidar tras fuente retirada | §4.2, la regla de citas nuevas, y después §3.2 |
| Revalidar una de IA sin nota | §3.2, paso 10: `422 nota_obligatoria` |
| Conflicto de clave | **Diferido** a E3-01/E3-03 (§1). Código reservado |
| Cita a fuente retirada | §4.2, paso 7: `422 fuente_no_activa` |
| Quitar la última cita de una aprobada | §4.2, paso 8: `409 ultima_cita_activa`; con otra cita activa antes, `200` |
| Quitar la única cita de una `activo` heredada | §4.2: el paso 8 solo aplica a `aprobada`, así que da `200`, la pregunta queda sin citas y sigue `activo` (§4.1: las citas nunca cambian el estado) |
| Límite de citas | §4.2, paso 3: `422 limite_citas` |
| Búsqueda solo en fuentes activas | §2: solo `activa` y versión vigente, sin rutas |
| Rechazar y reabrir | E3-02 §3 y §4.1, sin cambios; auditoría §3.3 |
| Concurrencia | §3.2, paso 6 (y paso 5): `409 estado_invalido` |
| Acceso | §6: `403` del middleware |

## 9. Partición en tareas (1–3 h cada una)

### backend-dev (sustituye a B4 y B5 de E3-02 §9; B8 depende de B4a y B4b)

| # | Tarea | Depende de | h |
|---|---|---|---|
| B3b | §5: los cinco campos nuevos de `CitaResponse` (incluido `texto`, el pasaje completo) (`questions/dto.go:112-118`) en la consulta de citas, con `texto.Extracto` | B3, `texto.Extracto` de B9 | 1 |
| B4a | E3-02 §4.1 con el cuerpo definitivo de §3.1: DTO con los cuatro campos, pasos 1–6, transiciones de E3-02 §3, motivo, orden de bloqueo y auditoría (§3.3). Un test por fila de E3-02 §3, más «Rechazar y reabrir» y «Concurrencia» | B3 | 3 |
| B4b | §3.2, pasos 7–11, en el service: `revision_especializada`, `falta_cita`, `nota_obligatoria` y `verificacion_requerida`. Un test por escenario y uno que fije el orden (p. ej. una `ia` en `en_revision`, sin nota ni casilla, da `nota_obligatoria`) | B4a | 2 |
| B5 | §4: `PUT …/citas` con `limite_citas`, `referencia_invalida` tras el bloqueo, `fuente_no_activa` solo en las nuevas, `ultima_cita_activa` solo en `aprobada`, lista idéntica sin auditoría, y la implementación de `Citantes` (E3-02 §5). Tests de «Cita a fuente retirada», «Quitar la última cita» (con la sustitución), «Límite» y «Revalidar tras fuente retirada» | B3 | 3 |
| B9 | §2, §5 y ADR 0007: `texto.Extracto` con sus tests (espacios, corte en 300 runes, multibyte), y `GET /admin/pasajes` en `internal/fuentes` (`RutaBuscarPasajes`, handler, DTO), con el SQL compartido con `PgSource.BuscarPasajes`. Tests: solo fuentes `activa` y versión vigente, ≤ 20, orden, sin campos de archivo, `consulta_invalida` (2 y 301 runes), `403`, y que el puerto siga pasando su suite de contrato (`fuentestest/contract.go`) | — | 3 |
| B8 | Sin cambios de alcance (E3-02 §9). Aprueba por el service de B4 con `Verificado: true`, sin nota y solo preguntas `manual`, citando pasajes de la «Fuente demo (no oficial)» | B4a, B4b, B5 | 2 |

- B9 vive en `internal/fuentes` e `internal/texto` y no toca `internal/questions`: el tech-lead puede dársela a **backend-dev-2**, en paralelo y sin dependencias. Si `texto.Extracto` todavía no está en develop cuando backend-dev llegue a B3b, B3b lo toma de la rama de B9 o lo escribe con la regla de §5 y B9 lo reutiliza: una sola función.
- Orden sugerido para backend-dev: B3 → B4a → B4b → B5 → B3b → B8.

### frontend-dev (contra este contrato, sin esperar al backend)

El PO dividió la pantalla en tres historias (review del PR #153, H4): **SIA-E3-08** (listado y detalle con citas, issue #156), **SIA-E3-09** (acciones de revisión, #157) y **SIA-E3-10** (avisos en las pantallas existentes, #158).

**F1 de E3-02, alcance actualizado.** La rama `feature/sia-e3-02-estados-preguntas` entrega:
- los tipos de `questions/api.ts` (estados, `origen`, `citas`, `citas_count`);
- en el listado, la insignia de estado, el origen y el nº de citas;
- las claves i18n es/en de los estados (`dashboard.serumsIa.estadosPregunta.*`) y del origen;
- y adelanta además **eliminar según el estado** y el **aviso al editar contenido**.

Lo que F1 adelanta **no se rehace**. Se acepta contra los criterios de la historia que es su dueña: el listado contra E3-08 y eliminar y el aviso de edición contra E3-10. F1 se revisa contra E3-02 solo en lo que es suyo (tipos, estado visible e i18n de los estados).

| # | Historia | Tarea | Depende de | h |
|---|---|---|---|---|
| F2 | E3-08 | Clientes: `cambiarEstado` y `reemplazarCitas` en `src/lib/questions/api.ts`, `buscarPasajes` en `src/lib/serums-ia/pasajes/api.ts`, con el manejo de errores del README §3 (`code`, `detalle`). Tipos de §2, §3.1 y §5 (incluido `texto`). Claves i18n es/en `dashboard.serumsIa.errores.{fuente_no_activa,limite_citas,ultima_cita_activa}` y el genérico `dashboard.serumsIa.errores.generico`, que se usa para todo código sin clave (README §8) | F1 | 2 |
| F3 | E3-08 | Listado: filtros por estado y por origen (`GET /preguntas?estado=&origen=`, E3-02 §4.3), sobre las insignias, el origen y el nº de citas de F1. Estados de carga, vacío («No hay preguntas con ese filtro») y error | F1 | 2 |
| F4 | E3-08 | Detalle, citas: citas actuales con fuente, versión, `fuente_estado`, extracto y el **texto completo como texto plano expandible**, con «quitar»; buscador de pasajes con carga, «Sin resultados» y error. Añadir y quitar envían la lista completa (§4) y, tras un error, la lista se recarga. Deja visible la zona de acciones para E3-09 | F2 | 3 |
| F5 | E3-09 | Acciones según estado y origen: Aprobar (solo `borrador` + `manual`, con casilla), Revalidar (casilla, más nota de 10–500 si el origen es `balotario`/`ia`), Rechazar (motivo de 5–500), Reabrir y «Se revisa en su cola». Ante un 409, recarga y muestra «Esta pregunta cambió de estado…». Claves i18n es/en de `falta_cita`, `verificacion_requerida`, `nota_obligatoria`, `conflicto_clave`, `revision_especializada` y `estado_invalido` | F4 | 3 |
| F6 | E3-10 | Avisos: alta y Excel en borrador; completar lo que F1 no haya cubierto de la confirmación al editar contenido de una `aprobada`/`activo`, de «Eliminar» deshabilitado en `borrador`/`en_revision` y del 409 al eliminar; `422 clave_oficial` traducido. Solo consume el contrato de E3-02, así que puede ir en paralelo con F2–F5 | F1 | 2 |

- Orden sugerido: F1 → F2 → F3 → F4 → F5, con F6 en cualquier hueco después de F1.
- **Claves de error** (alineadas con las historias, 2026-11-11):
  - Van en `dashboard.serumsIa.errores.<codigo>`, una por código; si la clave ya existe, se reutiliza (README §8). **Excepción:** los cuatro códigos comunes (`estado_invalido`, `no_encontrado`, `validacion` y `referencia_invalida`) van en `dashboard.serumsIa.preguntas.errores.<codigo>`, porque su sentido depende del recurso (README §8, dos niveles).
  - Reparto: E3-08 tiene `fuente_no_activa`, `limite_citas` y `ultima_cita_activa`; E3-09, `falta_cita`, `verificacion_requerida`, `nota_obligatoria`, `conflicto_clave`, `revision_especializada` y `estado_invalido`; E3-10, `clave_oficial`.
  - Un código sin clave muestra `dashboard.serumsIa.errores.generico`, la clave única del módulo (README §8), nunca `message`.
  - Los textos que no son códigos van en `revision.acciones.*` y `avisosPregunta.*`: el aviso de recarga por un 409 y la ayuda de eliminar.
- Todo con componentes y tokens de khipu. Si falta un componente (p. ej. la insignia de estado con icono), va primero el PR a khipu (CLAUDE.md).
- La validación del cliente (casilla, nota, motivo, ≤ 20 citas, 3–300 caracteres en la búsqueda) replica la del servidor; el servidor sigue siendo la fuente de verdad.
