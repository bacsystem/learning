-- +goose Up
CREATE TABLE personas (
    id              BIGSERIAL PRIMARY KEY,
    dni             TEXT NOT NULL,
    nombres         TEXT NOT NULL,
    apellido_paterno TEXT NOT NULL,
    apellido_materno TEXT NOT NULL,
    grado_id        BIGINT NOT NULL REFERENCES grados(id),
    especialidad_id BIGINT NOT NULL REFERENCES especialidades(id),
    institucion_id  BIGINT NOT NULL REFERENCES instituciones(id),
    telefono        TEXT NOT NULL,
    email           TEXT NOT NULL,
    estado          VARCHAR(20) NOT NULL DEFAULT 'activo',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_personas_dni ON personas(dni);

-- +goose Down
DROP TABLE personas;
