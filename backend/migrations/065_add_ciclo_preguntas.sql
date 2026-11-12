-- SIA-E3-02: question life cycle (borrador → aprobada / rechazada,
-- en_revision) and question-source traceability. Contract
-- docs/api/serums-ia-e3-02-ciclo-preguntas.md §2. The existing bank keeps
-- 'activo' (served like 'aprobada'); no row is rewritten by the Up.

-- +goose Up
-- The CHECK of 014 has no name: Postgres calls it preguntas_estado_check.
-- No IF EXISTS on purpose: if the name differs, the migration fails.
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
    pasaje_id   BIGINT      NOT NULL REFERENCES pasajes(id),   -- no ON DELETE: a cited passage is never deleted
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (pregunta_id, pasaje_id)
);
CREATE INDEX idx_pregunta_citas_pasaje ON pregunta_citas (pasaje_id);

-- +goose Down
DROP TABLE IF EXISTS pregunta_citas;
-- The new states do not fit the old schema: an approved question stays served
-- as 'activo'; the rest stop being served.
UPDATE preguntas SET estado = 'activo'    WHERE estado = 'aprobada';
UPDATE preguntas SET estado = 'eliminada' WHERE estado IN ('borrador', 'en_revision', 'rechazada');
ALTER TABLE preguntas DROP CONSTRAINT IF EXISTS ck_preguntas_motivo_rechazo;
ALTER TABLE preguntas DROP COLUMN IF EXISTS motivo_rechazo;
ALTER TABLE preguntas DROP COLUMN IF EXISTS origen;
ALTER TABLE preguntas ALTER COLUMN estado SET DEFAULT 'activo';
ALTER TABLE preguntas DROP CONSTRAINT IF EXISTS ck_preguntas_estado;
ALTER TABLE preguntas ADD CONSTRAINT preguntas_estado_check CHECK (estado IN ('activo', 'eliminada'));
