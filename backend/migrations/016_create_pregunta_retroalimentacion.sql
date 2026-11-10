-- +goose Up
CREATE TABLE pregunta_retroalimentacion (
    id BIGSERIAL PRIMARY KEY,
    pregunta_id BIGINT NOT NULL UNIQUE REFERENCES preguntas(id) ON DELETE CASCADE,
    explicacion TEXT NOT NULL,
    referencia TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS pregunta_retroalimentacion;
