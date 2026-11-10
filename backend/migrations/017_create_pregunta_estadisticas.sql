-- +goose Up
CREATE TABLE pregunta_estadisticas (
    id BIGSERIAL PRIMARY KEY,
    pregunta_id BIGINT NOT NULL UNIQUE REFERENCES preguntas(id) ON DELETE CASCADE,
    total_intentos INT NOT NULL DEFAULT 0,
    total_aciertos INT NOT NULL DEFAULT 0,
    indice_dificultad DECIMAL(5,4) NOT NULL DEFAULT 0,
    indice_discriminacion DECIMAL(5,4) NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS pregunta_estadisticas;
