# Contrato API — SERUMS IA, SIA-E3-02 Ciclo de vida y trazabilidad de preguntas

- **Historia:** [SIA-E3-02](../stories/serums-ia/E3-02-borrador-y-trazabilidad.md)
- **Estado:** borrador para revisión del tech-lead (2026-11-11). Hay preguntas abiertas al PO (§8) con decisión por defecto.
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
| I8 | `POST /preguntas`, `POST /preguntas/import` | Insertan con `estado` `activo` (default de `014:14`) | Nacen en `borrador` con `origen = 'manual'` (ver §8, P1) |
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

```json
{ "estado": "rechazada", "motivo": "La clave no coincide con la NTS vigente." }
```

| # | Validación, en orden | Error |
|---|---|---|
| 1 | Cuerpo JSON válido sin campos desconocidos; `{id}` entero > 0 | 400 `cuerpo_invalido` / `parametro_invalido` |
| 2 | `estado` ∈ `aprobada` \| `rechazada` \| `borrador` \| `en_revision`. `eliminada` va por `DELETE` | 422 `validacion` |
| 3 | `motivo` obligatorio para `rechazada`, 5–500 runes tras `TrimSpace`. Opcional para `en_revision` (≤ 500). En otro caso se ignora | 422 `validacion` (`detalle.campos.motivo`) |
| 4 | La pregunta existe y no está `eliminada` | 404 `no_encontrado` |
| 5 | La transición está permitida (§3) | 409 `estado_invalido` (`detalle = {"estado_actual"}`) |
| 6 | Solo para `aprobada`: ≥ 1 cita a un pasaje de una fuente en estado `activa`, de cualquier versión | **422 `falta_cita`** (escenario «Aprobar sin cita») |

- **Origen `balotario`** (regla 3): su cita es a la propia fuente del balotario, que crea SIA-E3-01. El requisito es el mismo (≥ 1 cita a una fuente `activa`), así que no hay rama especial en el código.
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

```json
{ "pasaje_ids": [9012, 9013] }
```

- Reemplaza el conjunto de citas. `pasaje_ids` es obligatorio, con 0–20 ids, sin repetidos y todos existentes. Si no, `422 validacion` / `422 referencia_invalida`.
- Se permite en `borrador`, `en_revision`, `rechazada` y `activo` (agregar trazabilidad al banco heredado).
- En `aprobada` responde `409 estado_invalido`: una aprobada no puede quedar sin su cita. Para cambiarla se pasa antes a `en_revision`.
- Se puede citar un pasaje de una fuente `retirada`, pero esa cita no cuenta para aprobar (§4.1, regla 6).
- Mismo orden de bloqueo que §4.1.
- `200` con el objeto pregunta.
- Auditoría: `pregunta.citas`, `detalle = {"antes": [ids], "despues": [ids]}`.

### 4.3 Endpoints existentes que cambian

| Endpoint | Cambio |
|---|---|
| `POST /preguntas`, `POST /preguntas/import` | Nacen `estado = 'borrador'`, `origen = 'manual'` (P1). El cuerpo **no** acepta `estado` ni `origen`: el origen `ia` lo fijan los generadores (E6) y `balotario` la importación de SIA-E3-01 |
| `GET /preguntas` | Filtros opcionales nuevos `estado` y `origen`. `estado` es un valor **exacto** entre `borrador`, `en_revision`, `aprobada`, `rechazada` y `activo`: `activo` (heredado) se filtra aparte de `aprobada`. Sin filtro se listan todas las no eliminadas, también los borradores, porque el admin tiene que verlos. Cada item del listado suma `estado`, `origen` y `citas_count` (número de citas), pero **no** el array `citas`, que va solo en `GET /preguntas/{id}` |
| `GET /preguntas/{id}`, `PUT /preguntas/{id}` | La respuesta suma los campos de §4.4. Por defecto, `PUT` no cambia el estado (P3) |
| `DELETE /preguntas/{id}` | Solo desde `aprobada`, `rechazada` o `activo`; si no, `409 estado_invalido` con el formato 2 (P2). Audita `pregunta.estado` con `estado_nuevo = "eliminada"` |
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

`citas` va ordenado por `fuente_id, version, pasaje_id`. No incluye el texto del pasaje, que se resuelve con `ResolverCita` del puerto (ADR 0003).

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

Los códigos de esta tabla usan el **formato 2** (`{"error": "<codigo>", "message": "…", "detalle"?}`, README §3). También usa el formato 2 el `409 estado_invalido` del `DELETE` existente. El resto de los errores heredados de `internal/questions` no cambia.

## 8. Preguntas abiertas al PO (con decisión por defecto)

| # | Pregunta | Decisión por defecto |
|---|---|---|
| P1 | ¿El alta manual (`POST /preguntas`) y la importación Excel también nacen en `borrador`? La regla 2 dice «importada o generada», y el objetivo de la historia dice «ninguna pregunta nueva». **Consecuencia:** hasta SIA-E3-05 (UI de revisión) el admin solo puede publicar por API (`PATCH …/estado`), y siempre con una cita a una fuente activa | Sí, todas nacen en `borrador`. El tech-lead debería ordenar E3-05 cerca de E3-02 |
| P2 | El diagrama no tiene `borrador → eliminada`, `en_revision → eliminada` ni `activo → eliminada`, pero hoy `DELETE` elimina en cualquier estado | Se permite desde `aprobada`, `rechazada` y `activo` (compatibilidad). `borrador` y `en_revision` se rechazan primero |
| P3 | ¿Editar con `PUT` una pregunta `aprobada` o `activo` cambia su estado? | No cambia (como hoy). Recomendación del architect: que editar enunciado o respuestas de una `aprobada` la pase a `en_revision` |
| P4 | Revalidar (`en_revision` → `aprobada`) cuando sus citas siguen apuntando a una fuente `retirada` | Se exige ≥ 1 cita a una fuente `activa` (la misma regla 6). Así una pregunta nunca se sirve citando solo una fuente retirada |

## 9. Partición en tareas (backend-dev, 1–3 h cada una; después del merge del #138)

| # | Tarea | h |
|---|---|---|
| B1 | Migración 065, Up y Down, con un test `up → down → up` sobre filas en estados nuevos | 2 |
| B2 | I1, I4 e I5 (`PreguntaServible` y los dos conteos), con los tests «Borrador no se sirve», «Compatibilidad» e «Intento en curso» | 2 |
| B3 | I8: alta e importación en `borrador`/`manual`; campos de §4.4 en las respuestas; filtros de `GET /preguntas` | 3 |
| B4 | §4.1, `PATCH …/estado`: transiciones, motivo, `falta_cita`, orden de bloqueo y auditoría. Un test por fila de §3 | 3 |
| B5 | §4.2, `PUT …/citas`, más la implementación de `Citantes` | 2 |
| B6 | I9, `DELETE` según el estado | 1 |
| B7 | §5, `SuscriptorRetiro`: registro en `main` y tests de «Retirar fuente citada» (12 aprobadas y 3 en borrador), «Restaurar no reaprueba» y «todo o nada» con un fallo provocado | 3 |
| F1 | frontend-dev: tipos de `questions/api.ts`, estado visible en el listado admin y claves i18n es/en de los 6 estados y del origen | 2 |
