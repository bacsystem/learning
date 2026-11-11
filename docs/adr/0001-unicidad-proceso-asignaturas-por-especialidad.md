# ADR 0001: unicidad de `proceso_asignaturas` con `especialidad_id` nulo

- **Estado:** propuesto (pendiente de revisión del tech-lead). Decisiones de producto confirmadas por el PO el 2026-11-11. Revisado el 2026-11-11: la versión de Postgres de producción pasa de supuesto a decisión (Postgres 16).
- **Fecha:** 2026-11-11
- **Historias:** SIA-E1-03 (#13); la consumen SIA-E1-04, E4-01, E5-04, E5-05, E7-02 y E9-05
- **Decisión de producto de origen:** P6 (issue #65), supuesto 1 de `docs/stories/serums-ia/README.md:128`

## Contexto

`proceso_asignaturas` (`backend/migrations/001_create_catalogs.sql:56-64`) guarda una fila por (proceso, asignatura) con `numero_preguntas` y `peso_porcentual`. El `coeficiente` se agregó en `047_add_coeficiente_to_proceso_asignaturas.sql:9-10`. La unicidad es `UNIQUE (proceso_id, asignatura_id)` (`001:63`). La restricción no tiene nombre explícito, así que Postgres le pone el nombre por defecto `proceso_asignaturas_proceso_id_asignatura_id_key`. Hoy ningún código cita ese nombre: `grep` en `backend/` solo encuentra `usuarios_*_key` en `internal/auth/repository.go:380-382`.

SIA-E1-03 pide añadir `especialidad_id` (opcional) con esta regla de unicidad:

- una sola fila por (proceso, especialidad, asignatura);
- una sola fila «de compatibilidad», sin especialidad, por (proceso, asignatura).

Las filas existentes quedan sin especialidad y sus valores no cambian (regla 2).

Con un `NULL` en la clave, una `UNIQUE` estándar trata los `NULL` como distintos y no impide dos filas de compatibilidad iguales.

### Versión de Postgres

| Entorno | Imagen | Evidencia |
|---|---|---|
| Local (`docker compose`) | `postgres:16-alpine` | `docker-compose.yml:24` |
| CI de backend | `postgres:16-alpine` | `.github/workflows/backend.yml:31` |
| Guía de puesta en marcha | Postgres 16 | `docs/puesta-en-marcha.md:222` |

`UNIQUE NULLS NOT DISTINCT` existe desde Postgres 15.

### Lectores y escritores actuales de `proceso_asignaturas`

Cuando existan filas con especialidad, cada uno de estos puntos devuelve duplicados o deja de funcionar:

| # | Lugar | Uso | Qué se rompe |
|---|---|---|---|
| L1 | `internal/catalogs/repository.go:701-707` (`sumPesoPorcentual`) | suma de pesos para activar el proceso (`:682`) y para `checkPesoActivo` (`:727-739`) | suma las filas de todas las especialidades: deja de dar 100 |
| L2 | `internal/catalogs/repository.go:743-771` (`ListProcesoAsignaturas`, rutas `GET /procesos/{id}/asignaturas` y `GET /admin/procesos/{id}/asignaturas`, `catalogs/routes.go:58-59`) | listado de áreas del proceso | asignaturas repetidas |
| L3 | `internal/catalogs/repository.go:788-792` (`AsignarAsignatura`) | `INSERT … ON CONFLICT (proceso_id, asignatura_id) DO NOTHING` | sin la UNIQUE antigua, el `ON CONFLICT` no encuentra índice árbitro y falla (`there is no unique or exclusion constraint matching the ON CONFLICT specification`) |
| L4 | `internal/catalogs/repository.go:818-825` (`UpdateProcesoAsignatura`) | `UPDATE … WHERE proceso_id AND asignatura_id` | modifica también las filas de todas las especialidades |
| L5 | `internal/catalogs/repository.go:828-834` (`DeleteProcesoAsignatura`) | `DELETE … WHERE proceso_id AND asignatura_id` | borra también las filas de todas las especialidades |
| L6 | `internal/simulations/repository.go:259-301` (`GetConfiguracionDetalle`) | pantalla previa al simulacro | asignaturas repetidas |
| L7 | `internal/simulations/repository.go:324-346` (`GetProcesoAsignaturas`, usado en `simulations/service.go:108`) | arma el simulacro y su snapshot | dos filas de la misma asignatura: rompe la PK `(simulacro_id, asignatura_id)` de `simulacro_asignaturas` (`050_snapshot_calificacion_simulacro.sql:31`) y la `UNIQUE (simulacro_id, asignatura_id)` de `simulacro_resultados` (`023_create_simulacro_resultados.sql:12`) |
| L8 | `internal/questions/repository.go:45-55` (`validarClasificacion`) | `EXISTS` (proceso, asignatura) | nada: sigue siendo correcto (una asignatura de cualquier configuración pertenece al proceso) |
| L9 | Seeds de prueba `internal/questions/questionstest/seed.go:153`, `internal/db/dbtest/dbtest.go:103,135` | inserts y deletes de prueba | nada: insertan filas sin especialidad y borran por proceso o por asignatura |

## Opciones consideradas

### A. `UNIQUE NULLS NOT DISTINCT (proceso_id, especialidad_id, asignatura_id)`

- Una sola restricción cubre las dos reglas: con `NULLS NOT DISTINCT`, dos filas `(P, NULL, A)` chocan.
- `ON CONFLICT (proceso_id, especialidad_id, asignatura_id)` usa esa restricción como árbitro tanto para filas con especialidad como sin ella. Así se mantiene una sola forma de insertar.
- Un único nombre de restricción para mapear la violación 23505 a `peso_duplicado` (ya existe un helper con nombre de restricción: `internal/examen/examen.go:124-127`).
- Requiere Postgres ≥ 15. Producción usará Postgres 16, igual que compose y CI (ver «Versión de Postgres en producción»). Con una versión menor, la migración fallaría al aplicarse: no se corrompe nada en silencio.

### B. Dos índices únicos parciales

```sql
CREATE UNIQUE INDEX … (proceso_id, especialidad_id, asignatura_id) WHERE especialidad_id IS NOT NULL;
CREATE UNIQUE INDEX … (proceso_id, asignatura_id) WHERE especialidad_id IS NULL;
```

- Funciona en cualquier versión de Postgres.
- Hay dos nombres de índice que mapear.
- `ON CONFLICT` necesita inferir el índice parcial con su predicado (`ON CONFLICT (proceso_id, asignatura_id) WHERE especialidad_id IS NULL`). Eso da dos formas de insertar, y es fácil equivocarse al copiar código.
- Descartada: su única ventaja era funcionar en Postgres < 15, y ningún entorno lo usa ni lo usará (ver «Versión de Postgres en producción»).

### C. Valor centinela `especialidad_id = 0 NOT NULL`

Descartada: rompe la FK a `especialidades(id)` o obliga a crear una especialidad ficticia que aparecería en los catálogos.

### D. Tabla separada `proceso_especialidad_asignaturas`

- No toca las consultas actuales.
- Contradice la decisión P6 («extiende `proceso_asignaturas`», README:128 y SIA-E1-03 regla 1).
- Obliga a resolver la configuración con un `UNION` entre dos tablas que tienen la misma forma. Descartada.

## Decisión

**Opción A.** La restricción se llama `uq_proceso_asignaturas_proceso_esp_asig`.

### Versión de Postgres en producción

El usuario confirmó (vía tech-lead, 2026-11-11) que todavía **no existe** base de datos de producción. Por eso esto pasa de supuesto a decisión:

- **Producción usará Postgres 16**, la misma versión que `docker-compose.yml:24` y el CI (`.github/workflows/backend.yml:31`).
- **Mínimo soportado: Postgres 15**, por `UNIQUE NULLS NOT DISTINCT`. Cualquier entorno nuevo (staging, producción) se crea con 16; bajar de 15 exige un ADR nuevo.
- Si por error se aplicara la migración 060 sobre Postgres < 15, falla al aplicarse y no deja datos a medias.

### Nivel de resolución: por conjunto, nunca por fila

Una configuración por especialidad es un **conjunto** de áreas que suma 100 %. Mezclar por asignatura (la fila de la especialidad donde exista y la de compatibilidad en las demás) da conjuntos que no suman 100 y áreas que el examen de esa profesión no tiene. Por eso:

> Para (proceso P, especialidad E): si existe al menos una fila (P, E), se usan **solo** las filas (P, E). Si no existe ninguna, se usan **solo** las filas de compatibilidad (P, NULL). Nunca se mezclan.

Como la restricción garantiza como máximo una fila por (P, E-o-NULL, asignatura), el conjunto resuelto tiene como máximo una fila por asignatura. Eso protege la PK de `simulacro_asignaturas` (`050:31`) y la UNIQUE de `simulacro_resultados` (`023:12`).

Consulta canónica (la escribe catalogs y la reutilizan simulations y el servicio de pesos):

```sql
-- $1 = proceso_id, $2 = especialidad_id (NULL o 0 = sin especialidad)
WITH esp AS (
  SELECT CASE WHEN EXISTS (
           SELECT 1 FROM proceso_asignaturas
           WHERE proceso_id = $1 AND especialidad_id = NULLIF($2, 0))
         THEN NULLIF($2, 0) END AS id
)
SELECT pa.asignatura_id, pa.numero_preguntas, pa.peso_porcentual, pa.coeficiente
FROM proceso_asignaturas pa, esp
WHERE pa.proceso_id = $1
  AND pa.especialidad_id IS NOT DISTINCT FROM esp.id
ORDER BY pa.asignatura_id;
```

### Dónde vive el estado `completa` / `incompleta`

Es un **dato derivado** y no se guarda en una tabla aparte:

- `completa` ⇔ existe al menos una fila (P, E).
- `incompleta` ⇔ no existe ninguna.

Esto vale porque el único escritor de filas con especialidad es el guardado atómico `PUT /admin/procesos/{id}/pesos/{especialidadId}` (ver `docs/api/serums-ia-e1-taxonomia.md`). En una sola transacción, ese guardado:

- bloquea el proceso con `lockProceso`, `catalogs/repository.go:714-721`;
- valida que la suma sea 100 ± 0,01, con el mismo criterio que `pesoValido` en `catalogs/repository.go:711`, y que Σ `numero_preguntas` ≤ 200;
- reemplaza el conjunto completo.

Un conjunto a medias nunca llega a la base de datos. Los endpoints de filas existentes (L3–L5) solo escriben filas de compatibilidad.

El PO confirmó el 2026-11-11 que no hay borradores parciales y que el estado es derivado (SIA-E1-03 reglas 4, 6 y 7; supuesto 23c del README de historias). El guardado reemplaza el conjunto completo: un área omitida sale. Tampoco hay borrado de configuraciones: `incompleta` significa que nunca se guardó ninguna.

### Qué especialidad usan los flujos existentes

Simulacro y pantalla previa (L6, L7, y L2 en la ruta de usuario) resuelven con la especialidad que ya usan para filtrar preguntas: `claims.EspecialidadID` (`simulations/handler.go:111,325`, `catalogs/handler.go:653-657`), que viene de `personas.especialidad_id` (`002_create_personas.sql:9`). La ruta admin `GET /admin/procesos/{id}/asignaturas` muestra **solo** las filas de compatibilidad. Las configuraciones por especialidad se ven en su propio endpoint.

Los flujos nuevos del módulo SERUMS IA usan la especialidad del perfil del postulante (SIA-E1-04).

### Cambios obligatorios en el código existente (los hace backend-dev en SIA-E1-03)

| # | Cambio |
|---|---|
| L1 | `sumPesoPorcentual`: `AND especialidad_id IS NULL`. La activación del proceso (`:682`) y `checkPesoActivo` siguen validando solo la configuración de compatibilidad. Un proceso SERUMS necesita también su configuración general al 100 % para poder activarse; además, esa configuración es la de respaldo |
| L2 | Ruta admin: `AND pa.especialidad_id IS NULL`. Ruta de usuario: consulta canónica con `claims.EspecialidadID`. Hoy las dos rutas comparten el handler (`catalogs/handler.go:647`), así que hay que separar el modo |
| L3 | `ON CONFLICT (proceso_id, especialidad_id, asignatura_id) DO NOTHING` (el insert no envía `especialidad_id`, que queda `NULL`). El 409 lleva el código `peso_duplicado` |
| L4, L5 | `AND especialidad_id IS NULL` en el `WHERE` |
| L6, L7 | Consulta canónica. `GetProcesoAsignaturas` recibe `especialidadID` (desde `simulations/service.go:108`) |
| L8, L9 | Sin cambios |

Tests obligatorios:

- unicidad de los dos tipos de fila (violación 23505 en `uq_proceso_asignaturas_proceso_esp_asig` → 409 `peso_duplicado`);
- una fila de compatibilidad duplicada (con `especialidad_id` `NULL`) por `AsignarAsignatura`. El `ON CONFLICT (proceso_id, especialidad_id, asignatura_id) DO NOTHING` debe usar la restricción `NULLS NOT DISTINCT` como árbitro y devolver 409, no insertar una segunda fila. Este comportamiento no se ejecutó al escribir el ADR: el test es la verificación;
- resolución por conjunto: una especialidad con conjunto propio no hereda filas de compatibilidad;
- creación de simulacro con conjunto de especialidad y sin él;
- un proceso sin filas con especialidad se comporta igual que hoy (escenario «Compatibilidad» de E1-03).

### Migración (goose, número propuesto 060)

```sql
-- +goose Up
ALTER TABLE proceso_asignaturas
    ADD COLUMN especialidad_id BIGINT REFERENCES especialidades(id);

-- Sin IF EXISTS a propósito: si el nombre por defecto no coincide, la
-- migración falla en vez de dejar viva la UNIQUE antigua.
ALTER TABLE proceso_asignaturas
    DROP CONSTRAINT proceso_asignaturas_proceso_id_asignatura_id_key;

ALTER TABLE proceso_asignaturas
    ADD CONSTRAINT uq_proceso_asignaturas_proceso_esp_asig
    UNIQUE NULLS NOT DISTINCT (proceso_id, especialidad_id, asignatura_id);

-- Índice de la FK (convención de 054_add_fk_indexes.sql).
CREATE INDEX idx_proceso_asignaturas_especialidad
    ON proceso_asignaturas (especialidad_id) WHERE especialidad_id IS NOT NULL;

-- +goose Down
-- Las configuraciones por especialidad no caben en el esquema anterior: se
-- borran (quedan en audit_log, acción pesos.guardar, con sus valores).
DELETE FROM proceso_asignaturas WHERE especialidad_id IS NOT NULL;
DROP INDEX IF EXISTS idx_proceso_asignaturas_especialidad;
ALTER TABLE proceso_asignaturas DROP CONSTRAINT IF EXISTS uq_proceso_asignaturas_proceso_esp_asig;
ALTER TABLE proceso_asignaturas DROP COLUMN IF EXISTS especialidad_id;
ALTER TABLE proceso_asignaturas
    ADD CONSTRAINT proceso_asignaturas_proceso_id_asignatura_id_key
    UNIQUE (proceso_id, asignatura_id);
```

- **Compatibilidad del Up:** las filas existentes quedan con `especialidad_id = NULL` y no cambia ningún valor. No puede haber duplicados porque la UNIQUE antigua los impedía.
- **Down:** las filas sin especialidad quedan intactas, como pide el escenario «Migración reversible» de E1-03. El ciclo `up → down-to 0 → up` del CI (`.github/workflows/backend.yml:71-73`) lo verifica.
- **Snapshots:** `simulacro_asignaturas` copia los valores al crear el simulacro (`050:25-32`), así que los simulacros históricos no dependen de esta tabla.

## Consecuencias

- **Positivas:** una sola restricción, una sola forma de `ON CONFLICT`, y un estado `completa` que no puede desincronizarse porque no se guarda. El simulacro nunca recibe dos filas de la misma asignatura.
- **Negativas:**
  - Requiere Postgres ≥ 15 en todo entorno; producción se crea con Postgres 16 (ver «Versión de Postgres en producción»).
  - El Down pierde las configuraciones por especialidad (inevitable: el esquema anterior no las representa).
  - Un proceso SERUMS no se puede activar sin una configuración de compatibilidad al 100 %.
- **Riesgo de orden:** la migración 060 debe aplicarse después de cualquier migración que ya esté en `develop`. goose falla ante versiones faltantes, así que el número definitivo lo fija el tech-lead al mergear.
