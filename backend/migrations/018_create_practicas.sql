-- +goose Up
CREATE TABLE practicas (
    id                BIGSERIAL PRIMARY KEY,
    usuario_id        BIGINT NOT NULL REFERENCES usuarios(id),
    institucion_id    BIGINT NOT NULL REFERENCES instituciones(id),
    proceso_id        BIGINT NOT NULL REFERENCES procesos(id),
    modo_calificacion VARCHAR(20) NOT NULL CHECK (modo_calificacion IN ('inmediato', 'al_final')),
    tiempo_limite_seg INT,
    fecha_inicio      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    fecha_fin         TIMESTAMPTZ,
    total_preguntas   INT NOT NULL DEFAULT 0,
    correctas         INT NOT NULL DEFAULT 0,
    incorrectas       INT NOT NULL DEFAULT 0,
    sin_responder     INT NOT NULL DEFAULT 0,
    estado            VARCHAR(20) NOT NULL DEFAULT 'en_curso'
                      CHECK (estado IN ('en_curso','completada','abandonada','eliminada')),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_practicas_usuario ON practicas(usuario_id);
CREATE INDEX idx_practicas_estado  ON practicas(estado);

-- +goose Down
DROP TABLE IF EXISTS practicas;
