# Contrato API — SERUMS IA, SIA-E3-02 Ciclo de vida y trazabilidad de preguntas

- **Historia:** [SIA-E3-02](../stories/serums-ia/E3-02-borrador-y-trazabilidad.md)
- **Estado:** borrador para revisión del tech-lead (2026-11-11). Las preguntas P1–P4 al PO quedaron **decididas** (§8).
- **Release:** E3-02 no sale en un release sin SIA-E3-07 «Revisión básica de preguntas» (decisión del PO): es la UI de admin que permite publicar.
- **Convenciones:** [README.md](README.md). Los endpoints nuevos usan el formato de error 2. Los existentes de `internal/questions` conservan sus errores heredados, salvo lo que se indica.
- **Depende de:** el retiro S5 del contrato E2, con `SuscriptorRetiro` (PR #138, aprobado y sin mergear). E3-02 no se mergea antes que el #138.
- **Migración reservada:** **065**.

## 1. Impacto en el código existente

| # | Lugar | Hoy | Cambio |
|---|---|---|---|
| I1 | `questions/servible.go:7-9` (`PreguntaServible`) | `estado = 'activo'` | `estado IN ('aprobada', 'activo')`. Es la única definición, y su comentario ya lo anticipa |
| I2 | `questions/repository.go:726` (`GenerateSet`, que selecciona para prácticas y simulacros mediante sus adaptadores) | usa `PreguntaServible` | Sin cambios: hereda I1 |
| I3 | `catalogs/pesos_repository.go:135` (`AreasServibles`) | usa `PreguntaServible` | Sin cambios: hereda I1 |
| I4 | `simulations/repository.go:275` (pantalla previa: `preguntas_disponibles`) | `p.estado = 'activo'` **fijo** | Pasa a `questions.PreguntaServible("p")` |
| I5 | `catalogs/repository.go:767` (listado de áreas: `preguntas_disponibles`) | `p.estado = 'activo'` **fijo** | Pasa a `questions.PreguntaServible("p")` |
| I6 | Respuesta durante un intento en curso (`practices/repository.go:398-405`; simulacros, análogo) | Busca la pregunta por id **dentro del intento**, sin filtrar por estado | Sin cambios. Cumple «Intento en curso»: el intento conserva la pregunta aunque cambie de estado |
| I7 | Rutas `/preguntas*` (`questions/routes.go:31-40`) | Todas son **admin** | Sin cambios. El alumno no puede pedir una pregunta por id (sección Seguridad) |
| I8 | `POST /preguntas`, `POST /preguntas/import` | `CreatePregunta` escribe el literal `'activo'` en el INSERT (`questions/repository.go:130-132`). Es el único INSERT de `internal/questions`, y la importación pasa por él | El literal pasa a `'borrador'` y se agrega `origen = 'manual'` (ver §8, P1). El DEFAULT de la columna también cambia a `borrador` en la 065, pero el código no depende de él |
| I12 | Fixture de tests `questions/questionstest/seed.go:197-200` (usado por 14 archivos de test) y seed de demo `cmd/seed/demo/sembrar.go:359-381` | El fixture inserta **sin** `estado` y depende del DEFAULT `activo`. La demo crea preguntas con `CreatePregunta` | Con la 065, el fixture daría `borrador` y la demo quedaría **sin preguntas servibles**. Tarea B8 (§9) |
| I9 | `DELETE /preguntas/{id}` (`questions/repository.go:575`) | Elimina en cualquier estado | Solo desde los estados permitidos (§3); si no, `409 estado_invalido` (ver §8, P2) |
| I10 | `internal/fuentes` (`extension.go:12-26`, `service.go:25-27`) | `Citantes` = `sinCitantes`; sin suscriptores | E3-02 registra en `cmd/server/main.go` un `Citantes` y un `SuscriptorRetiro` reales (§5) |
| I11 | Frontend `src/lib/questions/api.ts:81` | `estado: "activo" \| "eliminada"` | El tipo se amplía con los 4 estados nuevos, más `origen` y `citas`. Hay que mostrar el estado en el listado admin (tarea de frontend-dev) |

## 2. Modelo de datos — migración 065

```sql
-- +goose Up
-- El CHECK de 014:14-15 no tiene nombre: Postgres le pone preguntas_estado_check.
-- Sin IF EXISTS a propósito: si el nombre no coincide, la migración falla (como en la ADR 0001).
ALTER TABLE preguntas DROP CONSTRAINT preguntas_estado_check;
ALTER TABLE preguntas ADD CONSTRAINT ck_preguntas_estado CHECK (estado IN
    ('activo', 'eliminada', 'borrador', 'en_revision', 'aprobada', 'rechazada'));
ALTER TABLE preguntas ALTER COLUMN estado SET DEFAULT 'borrador';

ALTER TABLE preguntas ADD COLUMN origen VARCHAR(10) NOT NULL DEFAULT 'manual'
    CONSTRAINT ck_preguntas_origen CHECK (origen IN ('manual', 'balotario', 'ia'));
ALTER TABLE preguntas ADD COLUMN motivo_rechazo VARCHAR(500);
ALTER TABLE preguntas ADD CONSTRAINT ck_preguntas_motivo_rechazo
    CHECK (estado <> 'rechazada' OR motivo_rechazo IS NOT NULL);

CREATE TABLE pregunta_citas (
    pregunta_id BIGINT      NOT NULL REFERENCES preguntas(id) ON DELETE CASCADE,
    pasaje_id   BIGINT      NOT NULL REFERENCES pasajes(id),   -- sin ON DELETE: un pasaje citado no se borra
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (pregunta_id, pasaje_id)
);
CREATE INDEX idx_pregunta_citas_pasaje ON pregunta_citas (pasaje_id);

-- +goose Down
DROP TABLE IF EXISTS pregunta_citas;
-- Los estados nuevos no caben en el esquema anterior.
UPDATE preguntas SET estado = 'activo'    WHERE estado = 'aprobada';
UPDATE preguntas SET estado = 'eliminada' WHERE estado IN ('borrador', 'en_revision', 'rechazada');
ALTER TABLE preguntas DROP CONSTRAINT IF EXISTS ck_preguntas_motivo_rechazo;
ALTER TABLE preguntas DROP COLUMN IF EXISTS motivo_rechazo;
ALTER TABLE preguntas DROP COLUMN IF EXISTS origen;
ALTER TABLE preguntas ALTER COLUMN estado SET DEFAULT 'activo';
ALTER TABLE preguntas DROP CONSTRAINT IF EXISTS ck_preguntas_estado;
ALTER TABLE preguntas ADD CONSTRAINT preguntas_estado_check CHECK (estado IN ('activo', 'eliminada'));
```

- **Compatibilidad del Up** (escenario «Compatibilidad con el banco existente»): ninguna fila se reescribe. El banco existente queda `activo`, con `origen = 'manual'` y sin citas, y se sigue sirviendo por I1.
- **Down:**
  - `aprobada` vuelve a `activo`.
  - `borrador`, `en_revision` y `rechazada` pasan a `eliminada`, para que no se sirvan; incluye una `activo` heredada que el retiro de una fuente pasó a `en_revision`.
  - Se pierden las citas, el origen y el motivo de rechazo; las transiciones quedan en `audit_log`.
  - El ciclo `up → down → up` del CI no tiene filas en estados nuevos, así que el banco existente queda intacto (escenario «Migración reversible»).
- **Citas:**
  - `pasaje_id` identifica la tupla (fuente, versión, pasaje): `pasajes.fuente_version_id` → `fuente_versiones.fuente_id`. Las versiones son inmutables (ADR 0003), así que la cita no cambia aunque la fuente tenga versiones nuevas.
  - La FK sin `ON DELETE` impide borrar un pasaje citado. Es la red de seguridad del archivado de versiones, que ya consulta `Citantes.VersionCitada` (`fuentes/repository.go:346`).

## 3. Máquina de estados (regla 4)

| Desde \ Hacia | `aprobada` | `rechazada` | `borrador` | `en_revision` | `eliminada` |
|---|---|---|---|---|---|
| `borrador` | sí: aprobar (§4.1) | sí: rechazar, con motivo | — | no | no (P2) |
| `en_revision` | sí: revalidar (§4.1) | sí: rechazar, con motivo | no | — | no (P2) |
| `rechazada` | no | — | sí: reabrir (borra `motivo_rechazo`) | no | sí |
| `aprobada` | — | no | no | sí: admin, o retiro de la fuente | sí |
| `activo` (heredado) | no | no | no | sí: admin, o retiro de la fuente | sí (P2) |
| `eliminada` | no | no | no | no | — |

- Toda otra transición, o pedir el estado actual, da **409 `estado_invalido`**.
- `activo` nunca es destino: nada nuevo nace ni vuelve a `activo`.
- La regla de 3 reportes (SIA-E6-09) llegará con su historia: aquí no hay nada automático aparte del retiro.

## 4. Endpoints (todos admin; `RequireAuth` + `RequireAdmin`)

### 4.1 `PATCH /preguntas/{id}/estado` (nuevo)

> **Actualizado por SIA-E3-07** ([contrato §3](serums-ia-e3-07-revision-basica.md#3-patch-preguntasidestado-versión-definitiva)): el cuerpo definitivo trae `estado`, `motivo`, `verificado` y `nota` desde el inicio. La tabla de validaciones de abajo queda **sustituida** por la de E3-07 §3.2, que añade `revision_especializada`, `nota_obligatoria` y `verificacion_requerida` y fija el orden total. Se mantienen el bloqueo, la respuesta y la auditoría de este §, con el `detalle` de E3-07 §3.3. Donde difieran, manda E3-07.

```json
{ "estado": "rechazada", "motivo": "La clave no coincide con la NTS vigente.", "verificado": null, "nota": null }
```

| # | Validación, en orden | Error |
|---|---|---|
| 1 | Cuerpo JSON válido sin campos desconocidos; `{id}` entero > 0 | 400 `cuerpo_invalido` / `parametro_invalido` |
| 2 | `estado` ∈ `aprobada` \| `rechazada` \| `borrador` \| `en_revision`. `eliminada` va por `DELETE` | 422 `validacion` |
| 3 | `motivo` obligatorio para `rechazada`, 5–500 runes tras `TrimSpace`. Opcional para `en_revision` (≤ 500). En otro caso se ignora | 422 `validacion` (`detalle.campos.motivo`) |
| 4 | La pregunta existe y no está `eliminada` | 404 `no_encontrado` |
| 5 | La transición está permitida (§3) | 409 `estado_invalido` (`detalle = {"estado_actual"}`) |
| 6 | Solo para `aprobada`: ≥ 1 cita a un pasaje de una fuente en estado `activa`, de cualquier versión. Se exige en **todos los procesos**, no solo en SERUMS (decisión del usuario, 2026-11-11). Una pregunta nueva de cualquier proceso necesita una fuente cargada para publicarse | **422 `falta_cita`** (escenario «Aprobar sin cita») |

- **Origen `balotario`** (regla 3): su cita es a la propia fuente del balotario, que crea SIA-E3-01. El requisito es el mismo (≥ 1 cita a una fuente `activa`), así que no hay rama especial en el código. La excepción vale solo si la fuente del balotario está `activa` (P4).
- **Revalidar** (`en_revision` → `aprobada`) exige lo mismo: ≥ 1 cita a una fuente `activa` (P4, decidida).
- **Bloqueo, en orden fijo** (lo comparten aprobar, editar citas y el retiro):
  1. Leer los `fuente_id` de las citas de la pregunta.
  2. `SELECT … FROM fuentes WHERE id = ANY($1) ORDER BY id FOR SHARE`.
  3. `SELECT … FROM preguntas WHERE id = $1 FOR UPDATE`.
  4. Volver a leer las citas: si el conjunto de fuentes cambió entre los pasos 1 y 3, responder `409 estado_invalido` («la pregunta cambió, recarga»).
  5. Validar y escribir.

  El retiro (#138) bloquea primero la fuente (`bloquearParaCambio`) y después actualiza preguntas: el orden es el mismo, fuente → pregunta, así que no hay deadlock. Y una aprobación nunca se confirma contra una fuente que se retira a la vez.
- `200` con el objeto pregunta (§4.4).
- Auditoría, en la misma transacción: `accion = "pregunta.estado"`, `entidad = "pregunta"`, `detalle = {"estado_anterior", "estado_nuevo", "motivo"}` (regla 7). Con `rechazada`, `motivo` va a `motivo_rechazo`; con `borrador` (reabrir), `motivo_rechazo` pasa a `NULL`.

### 4.2 `PUT /preguntas/{id}/citas` (nuevo)

> **Actualizado por SIA-E3-07** ([contrato §4](serums-ia-e3-07-revision-basica.md#4-put-preguntasidcitas-versión-definitiva)): las reglas de estados y validación de abajo quedan **sustituidas** por E3-07 §4. En resumen: se permite también en `aprobada`; más de 20 ids da `422 limite_citas`; las citas **nuevas** exigen una fuente `activa` (`422 fuente_no_activa`); en una `aprobada`, quitar exige que quede una cita a una fuente activa (`409 ultima_cita_activa`). Se mantienen el cuerpo, el reemplazo, el bloqueo, la respuesta y la auditoría. Donde difieran, manda E3-07.

```json
{ "pasaje_ids": [9012, 9013] }
```

- Reemplaza el conjunto de citas. `pasaje_ids` es obligatorio, sin repetidos y todos existentes (`422 validacion` / `422 referencia_invalida`); el máximo de 20 da `422 limite_citas` (E3-07 §4.2).
- ~~Se permite en `borrador`, `en_revision`, `rechazada` y `activo`. En `aprobada` responde `409 estado_invalido`. Se puede citar un pasaje de una fuente `retirada`.~~ **Sustituido por E3-07 §4.1–§4.2:** se permite también en `aprobada` (con `ultima_cita_activa`) y una cita **nueva** exige una fuente `activa` (`fuente_no_activa`). Una cita ya existente a una fuente retirada se puede conservar, pero no cuenta para aprobar (§4.1, regla 6).
- **Bloqueo** (corrección H2 del review): se bloquean las fuentes de las citas **actuales y las nuevas**.
  1. Resolver los `fuente_id` de los `pasaje_ids` del cuerpo y unirlos con los de las citas actuales de la pregunta.
  2. `SELECT … FROM fuentes WHERE id = ANY($unión) ORDER BY id FOR SHARE`.
  3. `SELECT … FROM preguntas WHERE id = $1 FOR UPDATE`.
  4. Volver a leer las citas actuales: si su conjunto de fuentes ya no está dentro de la unión bloqueada, responder `409 estado_invalido`.
  5. Validar y escribir.

  Bloquear también las fuentes **nuevas** evita dos carreras. Una: que se agregue una cita a una fuente que se retira a la vez, y que una pregunta servida quede citando solo una fuente retirada. Otra: que `liberarVersion` (`fuentes/repository.go:326`) archive esa versión y borre sus pasajes mientras se inserta la cita, lo que daría un error de FK convertido en 500. El orden fuente → pregunta es el mismo que en §4.1 y en el retiro.
- `200` con el objeto pregunta.
- Auditoría: `pregunta.citas`, `detalle = {"antes": [ids], "despues": [ids]}`.

### 4.3 Endpoints existentes que cambian

| Endpoint | Cambio |
|---|---|
| `POST /preguntas`, `POST /preguntas/import` | Nacen `estado = 'borrador'`, `origen = 'manual'` (P1, decidida: **toda** pregunta nueva nace en `borrador`, cualquiera sea su origen). El cuerpo **no** acepta `estado` ni `origen`: el origen `ia` lo fijan los generadores (E6) y `balotario` la importación de SIA-E3-01 |
| `GET /preguntas` | Filtros opcionales nuevos `estado` y `origen`. `estado` es un valor **exacto** entre `borrador`, `en_revision`, `aprobada`, `rechazada` y `activo`: `activo` (heredado) se filtra aparte de `aprobada`. Sin filtro se listan todas las no eliminadas, también los borradores, porque el admin tiene que verlos. Cada item del listado suma `estado`, `origen` y `citas_count` (número de citas), pero **no** el array `citas`, que va solo en `GET /preguntas/{id}` |
| `GET /preguntas/{id}` | La respuesta suma los campos de §4.4 |
| `PUT /preguntas/{id}` | La respuesta suma los campos de §4.4. Efecto sobre el estado (P3, decidida) en §4.5 |
| `DELETE /preguntas/{id}` | Solo desde `aprobada`, `rechazada` o `activo`; si no, `409 estado_invalido` con el formato 2 (P2, decidida). Un `borrador` o una `en_revision` se rechazan primero. Audita `pregunta.estado` con `estado_nuevo = "eliminada"` |
| `GET /preguntas/stats` y `preguntas_count` de temas (`temas/repository.go:48,277`) | Siguen contando las no eliminadas, borradores incluidos: son vistas de admin del banco completo. Si stats desglosa por estado, incluye los nuevos (backend-dev decide la forma). Lo «servible» solo lo cuenta `PreguntaServible` |

### 4.4 Objeto pregunta: campos nuevos

```json
{
  "estado": "borrador",
  "origen": "manual",
  "motivo_rechazo": null,
  "citas": [
    { "pasaje_id": 9012, "fuente_id": 9, "version": 2, "fuente_titulo": "Balotario SERUMS 2025", "fuente_estado": "activa" }
  ]
}
```

`citas` va ordenado por `fuente_id, version, pasaje_id`. No incluye el texto completo del pasaje, que se resuelve con `ResolverCita` del puerto (ADR 0003). **SIA-E3-07 §5** suma a cada cita `seccion`, `linea_inicio`, `linea_fin`, `extracto` (≤ 300 runes, `texto.Extracto`) y `texto`, el pasaje completo de la versión citada (decisión del PO).

### 4.5 `PUT /preguntas/{id}`: efecto sobre el estado (P3, decidida por el PO)

| Estado actual | Qué se edita | Resultado |
|---|---|---|
| `aprobada` o `activo` | **Contenido**: `enunciado`, `tipo`, alternativas (texto o cuál es correcta), retroalimentación o explicación | Se guarda y la pregunta pasa a **`en_revision`**, en la misma transacción. Se audita `pregunta.estado` con `motivo = "edición de contenido"` |
| `aprobada` o `activo` | Solo **metadatos**: `asignatura_id`, `tema_id`, `etiqueta_ids`, `dificultad`, `grado_id`, `especialidad_id` | Se guarda y el estado no cambia |
| `borrador`, `en_revision` o `rechazada` | Cualquier campo | El estado no cambia |
| `eliminada` | — | `404 no_encontrado` (como hoy) |

- **Qué es «contenido»:** se compara con lo guardado, no con lo enviado. Reenviar el mismo texto no cuenta como cambio: si el `PUT` trae el enunciado y las alternativas idénticas, el estado no cambia.
- **Origen `balotario`:** la clave oficial no se puede cambiar. Si el `PUT` cambia cuál alternativa es correcta, responde **`422 clave_oficial`** y no guarda nada. El resto sigue la tabla.
- **Riesgo aceptado por el PO:** corregir una errata en una `activo` heredada sin citas la saca de circulación hasta que se cite y se revalide (E3-07).
- **Bloqueo:** el `PUT` ya bloquea la pregunta `FOR UPDATE` (`questions/repository.go:479`). No toca fuentes, así que no afecta el orden de §4.1.

## 5. Integración con fuentes (regla 8)

Implementaciones en `internal/questions`, registradas en `cmd/server/main.go` al construir `fuentes.NewService`:

- **Cableado** (extensión del #138): `fuentes.NewService(repo, citantes, suscriptores…)` en `main.go`. `internal/questions` implementa las dos interfaces sin importar `internal/fuentes`.
- **`Citantes`:**
  - `ContarPorFuente`: preguntas en `aprobada` o `activo` con alguna cita a esa fuente. Son exactamente las que el retiro pasaría a `en_revision`, así que el diálogo de retiro (`contenidos_citantes`, S4) coincide con el `contenidos_marcados` de S5.
  - `VersionCitada`: hay alguna cita a un pasaje de esa versión.
- **`SuscriptorRetiro`**, con `Nombre() = "preguntas"` y `FuenteRetirada(ctx, tx, fuenteID, actorID)`. Corre **dentro** de la transacción del retiro, con la fuente ya bloqueada. Hace **dos sentencias por conjunto**, nunca un bucle por pregunta:

  ```sql
  -- $1 = fuente_id, $2 = actor del retiro
  WITH objetivo AS (
    SELECT p.id, p.estado FROM preguntas p
    WHERE p.estado IN ('aprobada', 'activo')
      AND EXISTS (SELECT 1 FROM pregunta_citas pc
                  JOIN pasajes pa ON pa.id = pc.pasaje_id
                  JOIN fuente_versiones fv ON fv.id = pa.fuente_version_id
                  WHERE pc.pregunta_id = p.id AND fv.fuente_id = $1)
    FOR UPDATE OF p
  ), afectadas AS (
    UPDATE preguntas p SET estado = 'en_revision', updated_at = NOW()
    FROM objetivo o WHERE p.id = o.id
    RETURNING p.id, o.estado AS estado_anterior
  )
  INSERT INTO audit_log (usuario_id, accion, entidad, entidad_id, detalle)
  SELECT $2, 'pregunta.estado', 'pregunta', a.id,
         jsonb_build_object('estado_anterior', a.estado_anterior, 'estado_nuevo', 'en_revision',
                            'motivo', 'fuente retirada', 'fuente_id', $1::bigint)
  FROM afectadas a;
  ```

  - `objetivo` toma el estado **anterior** antes de actualizar. Un `RETURNING` con subconsulta leería el valor nuevo.
  - Las columnas son las de `audit_log` (`006_create_audit_log.sql:2-12`). `ip` y `user_agent` quedan `NULL`, porque el suscriptor solo recibe el actor. El retiro mismo ya audita `fuente.retirar` con IP y user agent. El test verifica una fila por pregunta, con `estado_anterior` y el actor.
  - Devuelve `afectados` = número de preguntas cambiadas. Suma a `contenidos_marcados` de S5.
  - Las preguntas en `borrador`, `rechazada` y `en_revision` no cambian.
  - Si falla, `Retirar` revierte todo (#138, `SuscriptorError`): la fuente sigue `activa` y el admin recibe `500 error_interno`.
- **Restaurar no reaprueba** (regla 8; E2-06 regla 5): el suscriptor solo actúa al retirar.

## 6. Seguridad

- El estado se valida siempre en el servidor. La selección para el alumno usa solo `PreguntaServible`.
- Ninguna ruta de alumno devuelve una pregunta por id fuera de un intento suyo (I6, I7).
- Logs: solo ids y códigos, nunca el enunciado ni el motivo.

## 7. Errores

| Código | HTTP | Dónde |
|---|---|---|
| `cuerpo_invalido`, `parametro_invalido` | 400 | §4 |
| `no_encontrado` | 404 | §4 |
| `estado_invalido` | 409 | §3, §4 |
| `validacion`, `referencia_invalida` | 422 | §4 |
| `falta_cita` | 422 | §4.1 |
| `clave_oficial` | 422 | §4.5 (origen `balotario`) |
| `revision_especializada`, `ultima_cita_activa`, `conflicto_clave` (reservado) | 409 | SIA-E3-07 §3, §4 |
| `verificacion_requerida`, `nota_obligatoria`, `limite_citas`, `fuente_no_activa`, `consulta_invalida` | 422 | SIA-E3-07 §2, §3, §4 |

Los códigos de esta tabla usan el **formato 2** (`{"error": "<codigo>", "message": "…", "detalle"?}`, README §3). También usa el formato 2 el `409 estado_invalido` del `DELETE` existente. El resto de los errores heredados de `internal/questions` no cambia.

## 8. Decisiones del PO (2026-11-11)

| # | Pregunta | Decisión |
|---|---|---|
| P1 | ¿El alta manual y la importación Excel también nacen en `borrador`? | **Sí.** Toda pregunta nueva nace en `borrador`: manual, Excel, balotario e IA. Si naciera aprobada, el alta manual saltaría la revisión. Para publicar, el PO crea **SIA-E3-07 «Revisión básica de preguntas»**, la UI de admin que usa `PATCH …/estado` y `PUT …/citas` de este contrato. E3-02 no sale en un release sin E3-07 |
| P2 | ¿Desde qué estados se elimina? | Desde `aprobada`, `rechazada` y `activo`. Un `borrador` o una `en_revision` se rechazan primero |
| P3 | ¿Editar una `aprobada` o `activo` cambia su estado? | Editar el **contenido** la pasa a `en_revision`; editar metadatos no. En origen `balotario` la clave no se cambia (`422 clave_oficial`). Detalle en §4.5 |
| P4 | ¿Revalidar exige una cita a una fuente activa? | **Sí.** La excepción de `balotario` vale solo si su propia fuente está `activa` |

**Para SIA-E3-07** (respuesta al PO): los endpoints para cambiar el estado (§4.1) y para poner o quitar citas (§4.2) **ya existen** en este contrato. Lo que falta, y va en el contrato de E3-07, es un endpoint para que el admin **elija un pasaje**. Hoy no hay ninguna ruta HTTP que liste los pasajes de una fuente: `ObtenerPasajes` y `BuscarPasajes` existen solo en el puerto (ADR 0003).

## 9. Partición en tareas (backend-dev, 1–3 h cada una; después del merge del #138)

| # | Tarea | h |
|---|---|---|
| B1 | Migración 065, Up y Down, con un test `up → down → up` sobre filas en estados nuevos | 2 |
| B2 | I1, I4 e I5 (`PreguntaServible` y los dos conteos), con los tests «Borrador no se sirve», «Compatibilidad» e «Intento en curso» | 2 |
| B3 | I8: alta e importación en `borrador`/`manual`; campos de §4.4 en las respuestas; filtros de `GET /preguntas` | 3 |
| B4 | **Sustituida por B4a y B4b de SIA-E3-07 §9** (§4.1 con las guardas de E3-07) | — |
| B5 | **Sustituida por B5 de SIA-E3-07 §9** (§4.2 con las reglas de E3-07, más `Citantes`) | — |
| B6 | I9, `DELETE` según el estado | 1 |
| B6b | §4.5: `PUT` con contenido → `en_revision` (comparando con lo guardado) y `422 clave_oficial` en `balotario` | 2 |
| B7 | §5, `SuscriptorRetiro`: registro en `main` y tests de «Retirar fuente citada» (12 aprobadas y 3 en borrador), «Restaurar no reaprueba» y «todo o nada» con un fallo provocado | 3 |
| B8 | I12 (vía acordada con backend-dev):<br>• **seed de demo:** siembra una fuente activa titulada **«Fuente demo (no oficial)»** por el puerto (`GuardarFuente` + `PublicarVersion`). Usa texto sintético, nunca de `assets/`, y va **sin** marca de oficial verificada. **Nunca** se siembra en producción: el comando se niega a correr con `ENV=production` (condiciones del PO). Después cita cada pregunta (uno o pocos pasajes) y la aprueba con la transición de B4, sin crear ninguna en `activo`.<br>• **fixture `questionstest/seed.go:198`:** lleva `estado` **explícito** en el INSERT (`activo`, que representa el banco heredado), más casos aparte con `aprobada`, para no depender del DEFAULT | 2 |
| F1 | frontend-dev: tipos de `questions/api.ts`, estado visible en el listado admin y claves i18n es/en de los 6 estados y del origen. La rama adelanta además eliminar según el estado y el aviso al editar: se aceptan contra E3-10 (alcance y reparto en SIA-E3-07 §9) | 2 |
