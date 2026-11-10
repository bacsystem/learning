-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE etiquetas_pregunta (
    id BIGSERIAL PRIMARY KEY,
    asignatura_id BIGINT NOT NULL REFERENCES asignaturas(id),
    nombre VARCHAR(100) NOT NULL,
    estado VARCHAR(20) NOT NULL DEFAULT 'activo'
        CHECK (estado IN ('activo', 'inactivo')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(asignatura_id, nombre)
);

-- +goose Down
DROP TABLE IF EXISTS etiquetas_pregunta;
