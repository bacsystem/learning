-- +goose Up

ALTER TABLE respuestas ALTER COLUMN texto TYPE TEXT;

-- +goose Down
-- Vuelve al tipo original de 015. Falla (sin truncar en silencio) si existe
-- algún texto de más de 500 caracteres: en ese caso hay que decidir a mano.
ALTER TABLE respuestas ALTER COLUMN texto TYPE VARCHAR(500);
