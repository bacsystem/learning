-- +goose Up
CREATE TABLE configuracion_examen (
    id             BIGSERIAL PRIMARY KEY,
    proceso_id     BIGINT NOT NULL REFERENCES procesos(id),
    tiempo_total_seg INT NOT NULL,
    total_preguntas  INT NOT NULL,
    nota_minima      DECIMAL(4,2) NOT NULL DEFAULT 11.0,
    descripcion      VARCHAR(500),
    estado           VARCHAR(20) NOT NULL DEFAULT 'activo',
    vigente_desde    DATE,
    vigente_hasta    DATE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_config_examen_proceso ON configuracion_examen(proceso_id);
CREATE INDEX idx_config_examen_estado ON configuracion_examen(estado);

-- +goose Down
DROP TABLE IF EXISTS configuracion_examen;
