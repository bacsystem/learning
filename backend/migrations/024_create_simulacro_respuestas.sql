-- +goose Up
CREATE TABLE simulacro_respuestas (
    id            BIGSERIAL PRIMARY KEY,
    simulacro_id  BIGINT NOT NULL REFERENCES simulacros(id) ON DELETE CASCADE,
    pregunta_id   BIGINT NOT NULL REFERENCES preguntas(id),
    respuesta_id  BIGINT REFERENCES respuestas(id),
    es_correcta   BOOLEAN,
    respondida_at TIMESTAMPTZ,
    orden         INT NOT NULL,
    UNIQUE(simulacro_id, pregunta_id)
);

CREATE INDEX idx_simulacro_resp_simulacro ON simulacro_respuestas(simulacro_id);

-- +goose Down
DROP TABLE IF EXISTS simulacro_respuestas;
