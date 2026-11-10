-- +goose Up
CREATE TABLE simulacros (
    id               BIGSERIAL PRIMARY KEY,
    usuario_id       BIGINT NOT NULL REFERENCES usuarios(id),
    institucion_id   BIGINT NOT NULL REFERENCES instituciones(id),
    proceso_id       BIGINT NOT NULL REFERENCES procesos(id),
    configuracion_id BIGINT NOT NULL REFERENCES configuracion_examen(id),
    tiempo_limite_seg INT NOT NULL,
    fecha_inicio     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    fecha_fin        TIMESTAMPTZ,
    puntaje_total    DECIMAL(8,2),
    puntaje_maximo   DECIMAL(8,2),
    nota_vigesimal   DECIMAL(4,2),
    aprobado         BOOLEAN,
    estado           VARCHAR(20) NOT NULL DEFAULT 'en_curso',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_simulacros_usuario ON simulacros(usuario_id);
CREATE INDEX idx_simulacros_estado ON simulacros(estado);
CREATE INDEX idx_simulacros_usuario_estado ON simulacros(usuario_id, estado);

-- +goose Down
DROP TABLE IF EXISTS simulacros;
