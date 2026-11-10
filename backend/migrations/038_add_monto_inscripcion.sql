-- +goose Up
CREATE TABLE configuracion_pago (
    anio       INT           PRIMARY KEY,
    monto      DECIMAL(10,2) NOT NULL,
    updated_at TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);
ALTER TABLE inscripciones ADD COLUMN monto DECIMAL(10,2);
INSERT INTO configuracion_pago (anio, monto) VALUES (2025, 0.00);

-- +goose Down
ALTER TABLE inscripciones DROP COLUMN IF EXISTS monto;
DROP TABLE IF EXISTS configuracion_pago;
