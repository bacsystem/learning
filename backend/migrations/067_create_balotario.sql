-- SIA-E3-03: record of balotario imports and per-question traceability
-- (line range, structure alerts). Contract
-- docs/api/serums-ia-e3-01-e3-03-balotario.md §5. New tables only: no existing
-- row is touched. The Down drops the import registry (and with it the
-- idempotency); the imported questions stay in preguntas with their citations.

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
