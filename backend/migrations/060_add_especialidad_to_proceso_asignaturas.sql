-- SIA-E1-03: pesos por (proceso, especialidad, asignatura) (ADR 0001).
--
-- Efecto sobre los datos existentes: añade proceso_asignaturas.especialidad_id,
-- que nace NULL en todas las filas (la configuración de compatibilidad); no se
-- modifica ningún valor. La UNIQUE antigua impedía duplicados, así que la
-- nueva se crea sin conflictos.
--
-- Requiere Postgres >= 15 (NULLS NOT DISTINCT); compose, CI y producción usan 16.
--
-- Qué pierde el Down: las configuraciones por especialidad, que el esquema
-- anterior no puede representar (quedan en audit_log, acción pesos.guardar).
-- Las filas sin especialidad quedan intactas.

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
DELETE FROM proceso_asignaturas WHERE especialidad_id IS NOT NULL;
DROP INDEX IF EXISTS idx_proceso_asignaturas_especialidad;
ALTER TABLE proceso_asignaturas DROP CONSTRAINT IF EXISTS uq_proceso_asignaturas_proceso_esp_asig;
ALTER TABLE proceso_asignaturas DROP COLUMN IF EXISTS especialidad_id;
ALTER TABLE proceso_asignaturas
    ADD CONSTRAINT proceso_asignaturas_proceso_id_asignatura_id_key
    UNIQUE (proceso_id, asignatura_id);
