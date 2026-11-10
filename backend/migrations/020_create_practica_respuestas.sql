-- +goose Up
CREATE TABLE practica_respuestas (
    id             BIGSERIAL PRIMARY KEY,
    practica_id    BIGINT       NOT NULL REFERENCES practicas(id) ON DELETE CASCADE,
    pregunta_id    BIGINT       NOT NULL REFERENCES preguntas(id),
    respuesta_id   BIGINT       REFERENCES respuestas(id),
    es_correcta    BOOLEAN,
    respondida_at  TIMESTAMPTZ,
    orden          INT          NOT NULL
);

CREATE INDEX idx_practica_resp_practica ON practica_respuestas(practica_id);
CREATE INDEX idx_practica_resp_pregunta ON practica_respuestas(practica_id, pregunta_id);

-- +goose Down
DROP TABLE IF EXISTS practica_respuestas;
