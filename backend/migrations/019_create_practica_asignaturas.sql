-- +goose Up
CREATE TABLE practica_asignaturas (
    id               BIGSERIAL PRIMARY KEY,
    practica_id      BIGINT NOT NULL REFERENCES practicas(id) ON DELETE CASCADE,
    asignatura_id    BIGINT NOT NULL REFERENCES asignaturas(id),
    numero_preguntas INT    NOT NULL CHECK (numero_preguntas > 0)
);

-- +goose Down
DROP TABLE IF EXISTS practica_asignaturas;
