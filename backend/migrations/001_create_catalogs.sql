-- +goose Up

CREATE TABLE instituciones (
    id         BIGSERIAL PRIMARY KEY,
    nombre     VARCHAR(200) NOT NULL UNIQUE,
    siglas     VARCHAR(20)  NOT NULL,
    estado     VARCHAR(20)  NOT NULL DEFAULT 'activo',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE grados (
    id             BIGSERIAL PRIMARY KEY,
    institucion_id BIGINT       NOT NULL REFERENCES instituciones(id),
    nombre         VARCHAR(100) NOT NULL,
    abreviatura    VARCHAR(30)  NOT NULL,
    nivel          VARCHAR(30)  NOT NULL,
    orden          INT          NOT NULL DEFAULT 0,
    estado         VARCHAR(20)  NOT NULL DEFAULT 'activo',
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE especialidades (
    id             BIGSERIAL PRIMARY KEY,
    institucion_id BIGINT       NOT NULL REFERENCES instituciones(id),
    nombre         VARCHAR(100) NOT NULL,
    codigo         VARCHAR(50)  NOT NULL,
    estado         VARCHAR(20)  NOT NULL DEFAULT 'activo',
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE asignaturas (
    id         BIGSERIAL PRIMARY KEY,
    nombre     VARCHAR(100) NOT NULL,
    codigo     VARCHAR(30)  NOT NULL UNIQUE,
    estado     VARCHAR(20)  NOT NULL DEFAULT 'activo',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE procesos (
    id             BIGSERIAL PRIMARY KEY,
    institucion_id BIGINT       NOT NULL REFERENCES instituciones(id),
    nombre         VARCHAR(200) NOT NULL,
    tipo           VARCHAR(50)  NOT NULL,
    grado_id       BIGINT       REFERENCES grados(id),
    descripcion    TEXT,
    anio           INT          NOT NULL,
    estado         VARCHAR(20)  NOT NULL DEFAULT 'activo',
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE proceso_asignaturas (
    id                 BIGSERIAL PRIMARY KEY,
    proceso_id         BIGINT         NOT NULL REFERENCES procesos(id),
    asignatura_id      BIGINT         NOT NULL REFERENCES asignaturas(id),
    numero_preguntas   INT            NOT NULL DEFAULT 0,
    peso_porcentual    NUMERIC(5,2)   NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    UNIQUE (proceso_id, asignatura_id)
);

-- +goose Down
DROP TABLE IF EXISTS proceso_asignaturas;
DROP TABLE IF EXISTS procesos;
DROP TABLE IF EXISTS asignaturas;
DROP TABLE IF EXISTS especialidades;
DROP TABLE IF EXISTS grados;
DROP TABLE IF EXISTS instituciones;
