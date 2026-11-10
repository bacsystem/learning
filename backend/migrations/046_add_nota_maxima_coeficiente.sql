-- +goose Up
ALTER TABLE configuracion_examen
    ADD COLUMN nota_maxima   DECIMAL(5,2) NOT NULL DEFAULT 100.0,
    ADD COLUMN coeficiente   DECIMAL(5,2) NOT NULL DEFAULT 1.0;

-- +goose Down
ALTER TABLE configuracion_examen
    DROP COLUMN IF EXISTS nota_maxima,
    DROP COLUMN IF EXISTS coeficiente;
