-- +goose Up
CREATE TABLE inscripciones (
    id           BIGSERIAL PRIMARY KEY,
    usuario_id   BIGINT       NOT NULL REFERENCES usuarios(id),
    proceso_id   BIGINT       NOT NULL REFERENCES procesos(id),
    estado       VARCHAR(20)  NOT NULL DEFAULT 'pendiente',
    observacion  TEXT,
    revisado_por BIGINT       REFERENCES usuarios(id),
    revisado_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (usuario_id, proceso_id)
);

CREATE INDEX idx_inscripciones_estado    ON inscripciones(estado);
CREATE INDEX idx_inscripciones_proceso   ON inscripciones(proceso_id);
CREATE INDEX idx_inscripciones_usuario   ON inscripciones(usuario_id);

-- +goose Down
DROP TABLE inscripciones;
