# Contrato API — SERUMS IA, SIA-E3-01 Parser del balotario y SIA-E3-03 Importación con vista previa

- **Historias:**
  - [SIA-E3-01](../stories/serums-ia/E3-01-parser-balotario.md) (issue #23).
  - [SIA-E3-03](../stories/serums-ia/E3-03-importacion-balotario.md) (issue #25; 5 pts).
  - Van juntas porque el parser no tiene ruta propia: lo expone la vista previa de E3-03.
- **Estado:** borrador para revisión del tech-lead (2026-11-12).
  - Aplica las decisiones del PO D1–D13 y sus ajustes (supuesto 25 del README de historias).
  - Las decisiones P-B1–P-B7 están **todas cerradas** (§9; PR #174 y #180).
- **ADR:** [0008](../adr/0008-balotario-parser-bajo-demanda-y-cita-por-rango.md): el parser corre bajo demanda y la cita se resuelve por rango de líneas.
- **Convenciones:** [README.md](README.md). Todos los errores usan el **formato 2**.
- **Depende de:**
  - E3-02 (migración **065**, PR #171): `origen`, `pregunta_citas` y el estado `borrador`;
  - E6-01 (migración 066, PR #166), solo por el orden de las migraciones;
  - E3-07: `revision_especializada` y `clave_oficial`;
  - E2: fuentes, versiones y pasajes (migración 062).
- **Migraciones:** E3-01 no lleva ninguna. E3-03 lleva la **067**, reservada por el tech-lead. Orden de merge: 065 → 066 → 067 → 068.

## 1. Impacto en el código existente

| # | Lugar | Hoy | Cambio |
|---|---|---|---|
| B1 | — | No hay parser. `fuentes/normalizar.go:42` solo quita `<mark>` para contar palabras | Paquete nuevo **`internal/balotario`**: `parser.go` es puro (§2); `handler`, `service`, `repository`, `dto`, `errors` y `routes` son la importación (§3–§4) |
| B2 | Numeración de líneas | `fuentes.DividirLineas` (`fuentes/source.go:290`) numera las líneas de los pasajes (`normalizar.go:106,132`) | El parser usa la misma función sobre el `contenido` original de la versión vigente (D1). Así, los rangos de las preguntas y de los pasajes son comparables |
| B3 | Segmentación de balotarios | La ADR 0003 (§ Normalización, pasos 6, 8 y 9) dejaba a E3-01 un normalizador por pregunta. El código no lo aplica: `Normalizar` no recibe el tipo (`normalizar.go:80`) | **No se crea.** Los pasajes siguen en v1 y cada pregunta cita los pasajes que se solapan con su rango (D8; ADR 0008) |
| B4 | Alta de preguntas | `Repository.CreatePregunta` abre su propia transacción (`questions/repository.go:114-119`) e inserta respuesta por respuesta (`:142-151`) | `internal/questions` exporta un insert **por conjunto** dentro de la transacción del llamador (§4.3) |
| B5 | Datos de la fuente | El puerto (`fuentes/source.go:184-207`) no expone el contenido, la marca de oficial ni los rangos de los pasajes | `internal/fuentes` exporta dos consultas admin **fuera del puerto**, como en la ADR 0007 (§4.3) |
| B6 | Importación Excel (`questions/importer.go:259-264`) | La rama «pregunta duplicada» nunca se ejecuta (#165) | **Decidido** (PO y tech-lead): no hay índice único; backend-dev quita la rama; la detección queda en E3-06 (§8) |
| B7 | `conflicto_clave` (E3-07 §3.2, paso 8) | Reservado «para E3-01/E3-03» | **Nace en E3-06** (D6). E3-01 y E3-03 no lo producen |

- **Sin cambios:**
  - La máquina de estados (E3-02 §3).
  - `PATCH …/estado`: su guarda `revision_especializada` (E3-07 §3.2, paso 7) impide aprobar un `balotario` por la vía básica. Esas preguntas se aprueban en la cola de SIA-E3-05, que exige el área explícita (`falta_area`).
  - `PUT /preguntas/{id}`, con `clave_oficial` (E3-02 §4.5).
  - El suscriptor de retiro (E3-02 §5), que no toca los borradores.
- **La versión queda protegida:** las preguntas importadas citan pasajes de su versión. `Citantes.VersionCitada` impide que `liberarVersion` (`fuentes/repository.go:326`) borre esos pasajes al archivar la versión.

## 2. Parser (SIA-E3-01) — `balotario.Parsear`

```go
// Pura y determinista (D5): la misma entrada da la misma salida. Su único error es ErrLimitePreguntas (D3).
func Parsear(contenido []byte) (Resultado, error)

type Resultado struct {
    Validas   []Pregunta // por orden de aparición
    Problemas []Problema // por orden de aparición
}
type Pregunta struct {
    Numero                int
    LineaInicio, LineaFin int           // 1-based, numeradas con fuentes.DividirLineas sobre el contenido original
    Enunciado             string
    Tipo                  string        // "unica" | "multiple"
    Alternativas          []Alternativa // 2–6, con letras A, B, C… por posición
    Alertas               []string      // alertas_estructura; vacía (no nil) si el parser no intervino
}
type Alternativa struct{ Letra, Texto string; Correcta bool }
type Problema struct {
    Numero                *int     // nil si no se pudo leer
    LineaInicio, LineaFin int
    Motivos               []string // ≥ 1, todos los que aplican (D2), en el orden de §2.4
}
```

### 2.1 Preparación

1. Se quita el BOM y el UTF-8 inválido se reemplaza (`normalizar.go:105-106`). Después se divide con `fuentes.DividirLineas`. El número de línea es el índice + 1, igual que en `pasajes.linea_*` (D1).
2. En cada línea, los espacios en blanco se colapsan a uno y se aplica `TrimSpace`. **Es la única reescritura del texto** (escenario «Conservación del texto»). El Markdown interno (`**`, `_`) se conserva.

### 2.2 Gramática (determinista)

| Elemento | Regla | Ejemplos |
|---|---|---|
| **Inicio de pregunta** | Línea que empieza con un número N de 1–4 dígitos seguido de `.`, `)`, `-` o `:`. Puede llevar un prefijo `Pregunta`/`Nº` y el número envuelto en `**`/`__`. Abre una pregunta en tres casos: (a) no hay ninguna abierta; (b) la abierta ya tiene ≥ 1 alternativa; (c) N es el **siguiente esperado** (la anterior + 1, o 1 tras un encabezado), aunque la abierta no tenga alternativas, que entonces queda con `alternativas_insuficientes`. En otro caso, la línea se suma al enunciado (p. ej. una lista «1. … 3. …» dentro de la pregunta 1). La regla (c) evita que una pregunta sin alternativas absorba en silencio a la siguiente | `12. ¿Cuál…?`, `**12.** …`, `Pregunta 12: …`, `12) …` |
| **Etiqueta de alternativa** | Al inicio de la línea: una letra `A`–`F`, mayúscula o minúscula, seguida de `)`, `.` o `-`, o entre paréntesis (`(a)`); o una viñeta sin letra (`-`, `*`, `•`, `+`, `▪`, `◦`). También vale envuelta en `**` o en `<mark>` | `A) …`, `a. …`, `(b) …`, `- …`, `<mark>C) …</mark>` |
| **Alternativas pegadas** | En una línea de alternativa con etiqueta `X`, el texto se corta ante la **siguiente letra esperada con el mismo estilo**, precedida de un espacio (`␠B)␠`), y se sigue con C, D… El enunciado puede terminar en mitad de la línea si aparece `A)` y después `B)` del mismo estilo en esa misma línea. Así no se parten textos como «vitamina b)» | `A) Uno B) Dos C) Tres D) Cuatro` → 4 |
| **Continuación** | Una línea sin etiqueta, pegada a la anterior (sin línea en blanco), se suma con un espacio al último elemento: el enunciado o la última alternativa | — |
| **Fin de pregunta** | El siguiente inicio de pregunta; un encabezado `#`; o, **después de la primera alternativa**, una línea en blanco seguida de texto sin etiqueta. Ese texto y lo que sigue hasta el próximo inicio de pregunta quedan **fuera** de las preguntas (regla 7). Si tras la línea en blanco viene otra etiqueta, la pregunta continúa | — |
| **Fuera de las preguntas** | Lo anterior a la primera pregunta, los encabezados y el texto de cierre. No se reporta ni se importa | portadas, pies de página |
| **Rango de líneas** | Va de la línea del número a la última línea con texto de la última alternativa o continuación | — |

### 2.3 Marcas, HTML y etiquetas

- **Clave (D4):** una alternativa es correcta si cualquier parte de su línea, **incluida la etiqueta**, está entre `<mark>` y `</mark>`. Las etiquetas `<mark>` no quedan en el texto guardado. Una marca sin cerrar, o un cierre sin apertura, no forma una marca: se elimina como HTML. Si no queda ninguna clave, la pregunta tiene el motivo `sin_clave`.
- **Marca en el enunciado:** se quita, no indica clave y añade la alerta `html_eliminado`.
- **Otro HTML** (D4; ajuste de la regla 16): se eliminan **solo las etiquetas reconocibles** (`</?[A-Za-z][A-Za-z0-9-]*(\s[^<>]*)?/?>`). Si hay alguna, se pone la alerta `html_eliminado`.
  - `<br>` pasa a ser un espacio.
  - `<script>…</script>` y `<style>…</style>` se eliminan **con su contenido**.
  - Un `<` que no forma etiqueta («PA < 90») se conserva y no genera alerta.
  - Las entidades se decodifican con `html.UnescapeString` **después** de detectar las marcas: un `&lt;mark&gt;` queda como texto y no cuenta como clave.
- **El texto guardado es texto plano.** La UI lo muestra siempre como texto.
- **Letras:** las alternativas se guardan con las letras A, B, C… por posición.
- **Alerta `vinetas_normalizadas`:** se pone si alguna alternativa llegó **sin letra** (viñeta), si sus alternativas **mezclan estilos** de etiqueta o si sus letras no estaban en orden (A, B, D se guardan como A, B, C). Un balotario que usa siempre `a.` no genera alerta.
- **Alerta `alternativas_separadas`:** se pone si se partió alguna línea.

### 2.4 Motivos de «con problemas» (D2; regla 4)

Lista cerrada. Se informan **todos** los que aplican, en este orden. Ninguna pregunta se descarta en silencio.

| Motivo | Condición |
|---|---|
| `numeracion_rota` | El número está repetido o no es el de la pregunta anterior + 1. Siempre se compara solo con la anterior, cuente esta como válida o con problemas, y se sigue desde ese número. La primera pregunta, y **la primera tras un encabezado de sección que vuelve a 1**, no lo tienen |
| `enunciado_vacio` | El enunciado está vacío |
| `enunciado_largo` | El enunciado tiene más de **2 000** runes |
| `alternativas_insuficientes` | Tiene menos de 2 alternativas |
| `alternativas_excedidas` | Tiene más de 6 alternativas |
| `alternativa_vacia` | Una etiqueta sin texto, por ejemplo «B)» sola (P-B7; E3-01 regla 10) |
| `alternativa_larga` | Una alternativa tiene más de **500** runes (`respuestas.texto VARCHAR(500)`, `015_create_respuestas.sql:5`) |
| `alternativas_duplicadas` | Dos alternativas tienen el mismo texto plegado (`texto.Plegar`) |
| `sin_clave` | Ninguna alternativa tiene marca |

- **Tipo** (regla 3): 1 marca da `unica` y 2 o más dan `multiple`. Cumple `ValidateRespuestas` (`questions/validation.go:12-38`).
- **Límite (D3):** si el total de bloques detectados (válidos + con problemas) supera **1 000**, se devuelve `ErrLimitePreguntas` y ningún resultado.
- **`sin_pasaje`** no es un motivo del parser: lo añade E3-03 (§3).
- **Totales:** `validas`, `con_problemas`, `unica`, `multiple` y `con_alertas` (válidas con al menos una alerta).

### 2.5 Pruebas (DoD de E3-01)

Fixtures **sintéticos** en `internal/balotario/testdata/`, nunca de `assets/`: al menos uno por regla y por escenario. Además:

- alternativas pegadas en una línea, también con el enunciado en esa misma línea; «vitamina b)» sin partir;
- alternativas separadas por líneas en blanco;
- una lista numerada en el enunciado, con números no esperados (continuación) y con «2.» dentro de la pregunta 1 (la regla c la parte, y se fija el resultado esperado);
- una pregunta sin alternativas seguida de la siguiente (`alternativas_insuficientes` en la primera, la segunda válida);
- numeración que vuelve a 1 tras un encabezado (sin problema) y que vuelve a 1 sin encabezado (`numeracion_rota`);
- un número repetido;
- marcas: envolviendo la etiqueta, sin cerrar, y `&lt;mark&gt;`;
- `<script>` con contenido; `PA < 90`;
- 1 000 y 1 001 bloques;
- portada y pie de página excluidos;
- determinismo.

## 3. `GET /admin/fuentes/{id}/balotario` — vista previa (SIA-E3-03; admin)

`RequireAuth` + `RequireAdmin`. Parsea en cada petición la **versión vigente**, sin guardar nada y sin auditar. Query: `page` y `per_page` (D11). Paginan **solo** la lista `problemas`, con las reglas del README §5: 20 por defecto, máximo 100, en modo estricto.

| # | Validación, en orden | Error |
|---|---|---|
| 1 | `{id}` entero > 0; `page` y `per_page` válidos | `400 parametro_invalido` |
| 2 | La fuente existe | `404 no_encontrado` |
| 3 | `tipo = 'balotario'` (D7) | **`422 tipo_fuente_invalido`** |
| 4 | `estado = 'activa'` (D7) | `422 fuente_no_activa` (`detalle = {"estado"}`) |
| 5 | `Parsear` no supera los 1 000 bloques (D3) | **`422 limite_preguntas`** (`detalle = {"maximo": 1000, "detectadas": n}`) |

- **`sin_pasaje`** (regla 9): una válida cuyo rango no se solapa con ningún pasaje de la versión (`RangosPasajes`, §4.3) pasa a `problemas` con el motivo `sin_pasaje` y no se importa.
  - Los pasajes cubren toda línea que tenga alguna palabra (`normalizar.go:80-98`). Solo quedan fuera las líneas en blanco o sin palabras, así que una pregunta válida siempre tiene pasaje. El motivo es una defensa.
  - Los totales se calculan **después** de aplicarlo.

```json
{
  "fuente": { "id": 9, "titulo": "Balotario SERUMS 2025 — Medicina", "version": 2,
              "oficial_verificada": true, "oficial_entidad": "MINSA" },
  "ruta_revision": "estructural",
  "totales": { "validas": 580, "con_problemas": 12, "unica": 570, "multiple": 10, "con_alertas": 34 },
  "preguntas": [
    { "numero": 1, "linea_inicio": 14, "linea_fin": 19, "tipo": "unica",
      "enunciado": "¿Cuál es el agente etiológico…?",
      "alternativas": [ { "letra": "A", "texto": "…", "correcta": false },
                        { "letra": "B", "texto": "…", "correcta": true } ],
      "alertas_estructura": ["alternativas_separadas"] }
  ],
  "problemas": {
    "data": [ { "numero": 17, "linea_inicio": 230, "linea_fin": 236, "motivos": ["sin_clave"] } ],
    "total": 12, "page": 1, "per_page": 20
  },
  "importacion": null,
  "otras_importaciones": [ { "version": 1, "creadas": 575, "created_at": "2026-11-10T15:04:05Z" } ],
  "puede_confirmar": true,
  "motivo_no_confirmable": null
}
```

| Campo | Regla |
|---|---|
| `fuente.version` | Número de la versión vigente. El cliente lo reenvía al confirmar (D9) |
| `fuente.oficial_*` | Marca de la versión vigente (`fuente_versiones.oficial_*`, `062:55-56`). La UI muestra la insignia «Oficial verificada · MINSA» (regla 6) |
| `ruta_revision` | `estructural` si la versión es oficial verificada; si no, `verificacion_clave` (los nombres de E3-05). Es **informativa**: E3-05 la vuelve a calcular en el servidor al aprobar (regla 7) |
| `preguntas` | Las **primeras 20 válidas** por orden de aparición, sin paginar (regla 1) |
| `problemas` | Todas las «con problemas», **paginadas** (D11), con línea y motivos y **sin texto**. El admin las ubica por número de línea |
| `importacion` | `{"id", "creadas", "created_at"}` si esta versión ya se importó; si no, `null` |
| `otras_importaciones` | Importaciones de otras versiones de la misma fuente, de la más reciente a la más antigua. Sirven para avisar que puede haber preguntas repetidas, que resuelve E3-06 (D10) |
| `puede_confirmar` / `motivo_no_confirmable` | `false` con `sin_validas` o `ya_importado`; si no, `true` y `null`. La UI deshabilita «Confirmar» y explica el motivo |

- **Nunca** trae `nombre_archivo`, `sha256`, el contenido completo ni rutas.
- Logs: solo ids, totales y códigos.

## 4. `POST /admin/fuentes/{id}/balotario/importaciones` — confirmar (SIA-E3-03; admin)

```json
{ "version": 2, "proceso_id": 3, "especialidad_id": 2, "asignatura_id": 7 }
```

`asignatura_id` es el **área inicial y provisional** de todas las preguntas (opción A del PO; reglas 2 y 13).

- Igual que en el Excel (`questions/importer.go:222`), cumple el `NOT NULL` de `014_create_preguntas.sql:6`.
- La IA de E3-04 nunca cambia `asignatura_id`: solo guarda su propuesta.
- E3-05 exige que la aprobación traiga el área explícita (`422 falta_area`), y la UI nunca la rellena con el área inicial.

### 4.1 Validaciones, en orden (se devuelve el primer error)

| # | Validación | Error |
|---|---|---|
| 1 | JSON válido, sin campos desconocidos y dentro del límite; `{id}` entero > 0 | `400 cuerpo_invalido` / `400 parametro_invalido` / `413 cuerpo_demasiado_grande` |
| 2 | Los cuatro campos presentes y enteros > 0 | `422 validacion` (`detalle.campos`) |
| 3 | La fuente existe | `404 no_encontrado` |
| 4 | **Bloqueo**, ya en la transacción: `SELECT … FROM fuentes WHERE id = $1 FOR SHARE`. Bloquea el retiro y la publicación de una versión nueva, y sigue el orden fuente → pregunta (E3-02 §4.1) | — |
| 5 | `tipo = 'balotario'` y `estado = 'activa'` (D7) | `422 tipo_fuente_invalido` / `422 fuente_no_activa` |
| 6 | `version` es la versión vigente (D9) | **`409 version_desactualizada`** (`detalle = {"version_vigente"}`). La UI recarga la vista previa |
| 7 | Esa versión no se importó antes (regla 3) | **`409 ya_importado`** (`detalle = {"importacion_id", "created_at"}`) |
| 8a | El proceso existe | `422 referencia_invalida` (`detalle.campos.proceso_id`) |
| 8b | La especialidad existe, está activa y es de la institución del proceso (D13). Es el mismo chequeo y el mismo código que E1-04 (`serums-ia-e1-taxonomia.md:436`) | **`422 especialidad_invalida`** |
| 9 | Si la fuente tiene profesiones (`fuente_especialidades`), la especialidad está entre ellas (D13, P-B6). Lleva un código distinto del paso 8b para que la UI explique cuál de los dos casos falló | **`422 especialidad_no_corresponde`** (`detalle = {"especialidad_ids": [las de la fuente]}`) |
| 10 | El área está asignada al proceso (`proceso_asignaturas`, como `validarClasificacion`, `questions/repository.go:45-55`) | `422 referencia_invalida` (`detalle.campos.asignatura_id`) |
| 11 | Se ejecuta `Parsear` sobre el `contenido` vigente, leído dentro de la transacción y nunca recibido del cliente, y se aplica `sin_pasaje` (§3). No debe superar los 1 000 bloques | `422 limite_preguntas` |
| 12 | Hay ≥ 1 válida | **`422 sin_validas`** |
| 13 | Inserciones (§4.3) y `audit_log` (§4.4) en la misma transacción (D12), y `COMMIT` | Un `unique_violation` de `uq_balotario_importaciones_version` (confirmación concurrente) da `409 ya_importado`. Cualquier otro error da `500 error_interno` y no crea nada |

- **`201`:**

  ```json
  { "importacion_id": 3, "fuente_id": 9, "version": 2, "creadas": 580,
    "con_problemas": 12, "unica": 570, "multiple": 10, "con_alertas": 34 }
  ```

- **Idempotencia** por `fuente_version_id` (D10): el paso 7 cubre el caso secuencial y la restricción única, el concurrente. Se puede importar una versión nueva de la misma fuente.

### 4.2 Qué se crea por cada pregunta válida

| Tabla | Valores |
|---|---|
| `preguntas` | `institucion_id` del proceso (`questions/service.go:27`), `proceso_id`, `asignatura_id` (área inicial), `especialidad_id`, `grado_id = NULL`, `tema_id = NULL`, `enunciado`, `tipo`, `dificultad = 'medio'` (`importer.go:255-257`), **`estado = 'borrador'`**, **`origen = 'balotario'`** |
| `respuestas` | Una por alternativa: `texto`, `es_correcta` y `orden` = 1…n |
| `pregunta_citas` | Los pasajes de la **misma versión** cuyo rango se solapa con el de la pregunta: `pa.linea_inicio <= fin AND pa.linea_fin >= inicio` (D8). Siempre hay ≥ 1, porque `sin_pasaje` ya los excluyó. Más de 20 (`limite_citas`, E3-07 §4.2) sería una violación del invariante: `500` |
| `pregunta_balotario` | `importacion_id`, `numero`, `linea_inicio`, `linea_fin` y `alertas_estructura`. Con el rango, el admin ve el texto original de la pregunta en la fuente (D8) |

### 4.3 Implementación por conjunto (obligatoria)

- **`internal/questions`** exporta `InsertarLote(ctx, tx *sqlx.Tx, lote []NuevaPregunta) ([]int64, error)`, con **una sentencia por tabla**:
  1. reserva n ids con `SELECT nextval('preguntas_id_seq') FROM generate_series(1, $n)`;
  2. `INSERT INTO preguntas … SELECT … FROM unnest(…)`;
  3. `INSERT INTO respuestas … FROM unnest(…)`.

  `CreatePregunta` no cambia, e `internal/questions` sigue sin importar `internal/fuentes` (E3-02 §5).
- **`internal/fuentes`** exporta, fuera del puerto (ADR 0007):
  - `VersionVigenteBalotario(ctx, q, fuenteID)`, que devuelve `{fuente_version_id, numero, contenido, tipo, estado, oficial_verificada, oficial_entidad, especialidad_ids}`;
  - `RangosPasajes(ctx, q, fuenteVersionID)`, que devuelve `[]{pasaje_id, linea_inicio, linea_fin}` por `orden`.
- **`internal/balotario`** orquesta la transacción:
  1. bloqueo y validaciones;
  2. `Parsear`;
  3. solape en memoria (un recorrido lineal, porque los dos lados vienen ordenados);
  4. `InsertarLote`;
  5. `INSERT … FROM unnest` en `pregunta_citas` y `pregunta_balotario`;
  6. auditoría.
- **Objetivo:** 1 000 preguntas con 6 alternativas en < 5 s en el CI, medido con un test (el `WriteTimeout` es de 15 s).

### 4.4 Auditoría (regla 5; D12)

En la misma transacción: `accion = "balotario.importar"`, `entidad = "fuente"`, `entidad_id = fuente_id`, `detalle = {"importacion_id", "version", "fuente_version_id", "proceso_id", "especialidad_id", "asignatura_id", "creadas", "con_problemas", "unica", "multiple", "con_alertas"}`. No lleva texto.

## 5. Modelo de datos — migración 067

```sql
-- +goose Up
CREATE TABLE balotario_importaciones (
    id                BIGSERIAL   PRIMARY KEY,
    fuente_id         BIGINT      NOT NULL REFERENCES fuentes(id),
    fuente_version_id BIGINT      NOT NULL,
    proceso_id        BIGINT      NOT NULL REFERENCES procesos(id),
    especialidad_id   BIGINT      NOT NULL REFERENCES especialidades(id),
    asignatura_id     BIGINT      NOT NULL REFERENCES asignaturas(id),   -- área inicial (opción A)
    creadas           INT         NOT NULL CHECK (creadas BETWEEN 1 AND 1000),
    con_problemas     INT         NOT NULL CHECK (con_problemas BETWEEN 0 AND 1000),
    creado_por        BIGINT      REFERENCES usuarios(id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_balotario_importaciones_version UNIQUE (fuente_version_id),
    -- la versión pertenece a la fuente (destino: uq_fuente_versiones_id_fuente, 062:64)
    CONSTRAINT fk_balotario_importaciones_version
        FOREIGN KEY (fuente_version_id, fuente_id) REFERENCES fuente_versiones (id, fuente_id)
);
CREATE INDEX idx_balotario_importaciones_fuente       ON balotario_importaciones (fuente_id);
CREATE INDEX idx_balotario_importaciones_proceso      ON balotario_importaciones (proceso_id);
CREATE INDEX idx_balotario_importaciones_especialidad ON balotario_importaciones (especialidad_id);
CREATE INDEX idx_balotario_importaciones_asignatura   ON balotario_importaciones (asignatura_id);
CREATE INDEX idx_balotario_importaciones_creado_por   ON balotario_importaciones (creado_por) WHERE creado_por IS NOT NULL;

CREATE TABLE pregunta_balotario (
    pregunta_id        BIGINT  PRIMARY KEY REFERENCES preguntas(id) ON DELETE CASCADE,
    importacion_id     BIGINT  NOT NULL REFERENCES balotario_importaciones(id),
    numero             INT     NOT NULL CHECK (numero BETWEEN 1 AND 9999),
    linea_inicio       INT     NOT NULL CHECK (linea_inicio >= 1),
    linea_fin          INT     NOT NULL,
    alertas_estructura TEXT[]  NOT NULL DEFAULT '{}',
    CONSTRAINT ck_pregunta_balotario_lineas  CHECK (linea_fin >= linea_inicio),
    CONSTRAINT ck_pregunta_balotario_alertas CHECK (alertas_estructura <@
        ARRAY['alternativas_separadas', 'vinetas_normalizadas', 'html_eliminado']::TEXT[])
);
CREATE INDEX idx_pregunta_balotario_importacion ON pregunta_balotario (importacion_id, linea_inicio);

-- +goose Down
DROP TABLE IF EXISTS pregunta_balotario;
DROP TABLE IF EXISTS balotario_importaciones;
```

- **Efecto sobre los datos existentes:** ninguno, porque las tablas son nuevas.
- **Qué pierde el Down:** el registro de importaciones (y con él la idempotencia), los rangos y las alertas. Las preguntas importadas **siguen** en `preguntas`, con `origen = 'balotario'` y sus citas, que son de la 065.
- **`numero` no es único:** la numeración puede volver a 1 en cada sección. La identidad de cada pregunta sale de `linea_inicio`.
- **`tuvo_conflicto_clave`** y los vínculos de duplicado los añade E3-06 (YAGNI aquí).

## 6. Seguridad

- Ambas rutas son **solo admin**; el rol `usuario` recibe `403` (escenario «Permisos»).
- Solo fuentes ya ingeridas: el contenido se lee en el servidor y el cliente nunca envía texto ni rutas.
- El estado, la versión, la idempotencia y las referencias se verifican en el servidor, dentro de la transacción y con la fuente bloqueada `FOR SHARE`.
- El contenido es **datos**: se guarda como texto plano (§2.3).
- Logs: solo ids, totales y códigos.

## 7. Errores

| Código | HTTP | Dónde | Clave i18n (README §8) |
|---|---|---|---|
| `parametro_invalido`, `cuerpo_invalido` | 400 | §3, §4 | módulo |
| `no_encontrado` | 404 | §3, §4 | recurso `balotario` |
| `version_desactualizada` | 409 | §4.1, paso 6 | módulo |
| `ya_importado` | 409 | §4.1, pasos 7 y 13 | módulo |
| `validacion`, `referencia_invalida` | 422 | §4.1 | recurso `balotario` |
| `tipo_fuente_invalido` | 422 | §3, §4 | módulo |
| `fuente_no_activa` | 422 | §3, §4 (ya existe en E3-07, con el mismo sentido) | módulo |
| `especialidad_invalida` | 422 | §4.1, paso 8b (ya existe en E1-04, con el mismo sentido) | módulo (se reutiliza) |
| `especialidad_no_corresponde` | 422 | §4.1, paso 9 | módulo |
| `limite_preguntas` | 422 | §3, §4 | **recurso `balotario`** (en E1-03 tiene otro sentido) |
| `sin_validas` | 422 | §4.1, paso 12 | módulo |
| `error_interno` | 500 | — | módulo |

Enums que traduce la UI:

- `dashboard.serumsIa.balotario.motivos.<motivo>`: los de §2.4 más `sin_pasaje`;
- `dashboard.serumsIa.balotario.alertas.<alerta>`: 3;
- `dashboard.serumsIa.balotario.rutaRevision.<ruta>`: `estructural` y `verificacion_clave`, cada una con su ayuda;
- `dashboard.serumsIa.balotario.noConfirmable.<motivo>`: `sin_validas` y `ya_importado`.

## 8. Issue #165 (decidido)

- **Sin índice único.** Chocaría con «mantener ambas» (E3-06 regla 5) y haría fallar entera una importación atómica.
- backend-dev quita de `importer.go:259-264` la rama inalcanzable y deja anotado en el issue que la detección de duplicados queda en E3-06 (#28).
- No lleva migración.

## 9. Decisiones del PO (cerradas el 2026-11-12; supuesto 25, PR #174 y #180)

| # | Decisión | Dónde |
|---|---|---|
| P-B1 | Opción A: un área inicial provisional al confirmar. E3-05 (regla 10) exige que la aprobación traiga el área confirmada (`422 falta_area`) | §4 |
| P-B2 | `conflicto_clave` (el paso 8, su escenario y su test) pasa a E3-06 | §1 B7; E3-07 §1 |
| P-B3 | #165 sin índice único; el PR #175 quita la rama muerta | §8 |
| P-B4 | Se acepta la ADR 0008: segmentación genérica con unión y cita por solape. Se revierte la Q7 (README §12) y se mantiene `sin_pasaje` | §3, §4.2 |
| P-B5 | `vinetas_normalizadas` solo con viñetas sin letra, estilos mezclados o letras fuera de orden. Las letras fuera de orden se reetiquetan por posición, y cada alternativa conserva su texto y su marca | §2.3 |
| P-B6 | Hay dos códigos distintos, a pedido del PO, para que la UI explique cuál de los dos casos falló. Si la especialidad no existe, está inactiva o no es de la institución del proceso, `422 especialidad_invalida` (como E1-04). Si no está entre las profesiones de la fuente, `422 especialidad_no_corresponde`. Un proceso inexistente o un área de otro proceso dan `422 referencia_invalida` | §4.1, pasos 8a–10 |
| P-B7 | Motivo `alternativa_vacia`: la lista cerrada pasa a 9 motivos | §2.4 |

## 10. Trazabilidad con los criterios de aceptación

| Historia | Escenario | Cómo se cumple |
|---|---|---|
| E3-01 | Clave única / Selección múltiple | §2.3, §2.4: el tipo depende del número de marcas |
| E3-01 | Alternativas pegadas | §2.2: 4 alternativas |
| E3-01 | Sin clave | §2.4, `sin_clave`, con su línea |
| E3-01 | Exceso de preguntas | §2.4, D3; §3, paso 5: `422 limite_preguntas` |
| E3-01 | Conservación del texto | §2.1, paso 2 |
| E3-01 | Resumen | §2.4, totales |
| E3-01 | Alerta de estructura / Sin intervención | §2.3: `alternativas_separadas`; la lista queda vacía si el parser no intervino |
| E3-03 | Vista previa | §3: totales, las 20 primeras y los problemas paginados |
| E3-03 | Confirmar | §4.2: `borrador`, `balotario` y la cita por solape |
| E3-03 | Fuente oficial verificada / no oficial | §3: `fuente.oficial_*` y `ruta_revision` |
| E3-03 | Doble importación | §4.1, pasos 7 y 13 |
| E3-03 | Versión desactualizada | §4.1, paso 6: `409 version_desactualizada` |
| E3-03 | Fallo parcial | §4.1, paso 13: una sola transacción; test con un fallo provocado |
| E3-03 | Sin válidas | §3, `puede_confirmar`; §4.1, paso 12 |
| E3-03 | Permisos | §6 |

## 11. Partición en tareas (1–3 h cada una)

### Backend

| # | Tarea | Depende de | h |
|---|---|---|---|
| P1 | `balotario/parser.go`, primera parte: preparación, gramática y rangos, con fixtures | — | 3 |
| P2 | `parser.go`, segunda parte: alternativas pegadas, marcas, HTML (`script`/`style`), entidades, alertas, motivos, tipo y límite, con los fixtures de §2.5 y el test de determinismo | P1 | 3 |
| I1 | Migración 067, Up y Down, con `up → down → up` | 065 y 066 en develop | 1 |
| I2 | `questions.InsertarLote` por conjunto, con un test de 1 000 preguntas en < 5 s y de rollback | 065 | 3 |
| I3 | `fuentes.VersionVigenteBalotario` y `RangosPasajes`, con tests | — | 1 |
| I4 | `GET …/balotario` (§3): `sin_pasaje`, paginación de `problemas`, `importacion`/`otras_importaciones` y `ruta_revision`, con un test por validación | P2, I1, I3 | 2 |
| I5 | `POST …/importaciones` (§4): bloqueo, validaciones 1–13, citas, `pregunta_balotario` y auditoría. Tests: idempotencia secuencial y concurrente, atomicidad, `version_desactualizada`, `especialidad_invalida`, `especialidad_no_corresponde`, `403` y retiro concurrente | I2, I4 | 3 |
| I6 | #165: quitar la rama muerta (§8). Es de backend-dev | — | 1 |

- P1 y P2 son puras y pueden ir a **backend-dev-2**. I2 e I5 tocan `internal/questions`, que es de **backend-dev**. Lo reparte el tech-lead.

### Frontend (frontend-dev)

| # | Tarea | h |
|---|---|---|
| F1 | Cliente `src/lib/serums-ia/balotario/api.ts` (`vistaPrevia(id, page, perPage)` y `confirmar`), con el manejo de errores del README §3 | 1 |
| F2 | Vista previa desde el detalle de una fuente `balotario` `activa`: totales, insignia de oficial, aviso de la ruta, tabla de las 20 primeras con sus alertas, lista **paginada** de problemas con línea y motivo traducido, y aviso de `otras_importaciones`. Estados de carga, vacío y error | 3 |
| F3 | Confirmación (formik + yup): proceso, especialidad y **área inicial** (con la ayuda «provisional, se confirma en la revisión»), botón deshabilitado con el motivo, y éxito con el número creado y el enlace al listado `origen=balotario`. Ante un `409` (`version_desactualizada`, `ya_importado`), recarga la vista previa | 2 |
| F4 | Claves i18n es/en de §7 | 1 |

- Todo con componentes y tokens de khipu. Si falta alguno, va primero el PR a khipu (CLAUDE.md).
