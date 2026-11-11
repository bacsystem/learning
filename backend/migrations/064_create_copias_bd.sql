-- PLT-05: one row per run of the daily R2 backup cron (ADR 0004 § Decisión 11),
-- inserted when it ends. No personal data, bucket, path or error text.
-- The GRANT to backup_ro is provisioning, not goose (docs/api/plataforma-salud.md).
-- +goose Up
CREATE TABLE copias_bd (
    id           BIGSERIAL   PRIMARY KEY,
    iniciada_en  TIMESTAMPTZ NOT NULL,
    terminada_en TIMESTAMPTZ NOT NULL,
    resultado    VARCHAR(10) NOT NULL CHECK (resultado IN ('correcta', 'fallida')),
    tamano_bytes BIGINT      CHECK (tamano_bytes >= 0),
    CONSTRAINT ck_copias_bd_tamano CHECK ((resultado = 'correcta') = (tamano_bytes IS NOT NULL)),
    CONSTRAINT ck_copias_bd_tiempos CHECK (terminada_en >= iniciada_en)
);
CREATE INDEX idx_copias_bd_terminada ON copias_bd (terminada_en DESC);
CREATE INDEX idx_copias_bd_correcta  ON copias_bd (terminada_en DESC) WHERE resultado = 'correcta';

-- +goose Down
DROP TABLE IF EXISTS copias_bd;
