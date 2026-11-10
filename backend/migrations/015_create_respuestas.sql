-- +goose Up
CREATE TABLE respuestas (
    id BIGSERIAL PRIMARY KEY,
    pregunta_id BIGINT NOT NULL REFERENCES preguntas(id) ON DELETE CASCADE,
    texto VARCHAR(500) NOT NULL,
    es_correcta BOOLEAN NOT NULL DEFAULT FALSE,
    orden INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_respuestas_pregunta ON respuestas(pregunta_id);

-- +goose Down
DROP TABLE IF EXISTS respuestas;
