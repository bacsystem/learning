-- +goose Up
CREATE TABLE configuracion_sistema (
    clave      VARCHAR(100) PRIMARY KEY,
    valor      TEXT         NOT NULL,
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

INSERT INTO configuracion_sistema (clave, valor) VALUES ('max_sesiones', '2');

-- +goose Down
DROP TABLE configuracion_sistema;
