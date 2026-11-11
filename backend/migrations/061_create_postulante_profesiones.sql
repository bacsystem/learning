-- SIA-E1-04: profesión (proceso, especialidad) elegida por el postulante en el
-- módulo SERUMS IA (docs/api/serums-ia-e1-taxonomia.md §4).
--
-- Efecto sobre los datos existentes: ninguno; es una tabla nueva. La elección
-- vive solo aquí y no modifica personas (E1-04 regla 6).
--
-- Qué pierde el Down: los perfiles elegidos (dato de aprendizaje; SIA-E9-07 lo
-- incluirá en la exportación y el borrado).

-- +goose Up
CREATE TABLE postulante_profesiones (
    id              BIGSERIAL   PRIMARY KEY,
    usuario_id      BIGINT      NOT NULL REFERENCES usuarios(id),
    proceso_id      BIGINT      NOT NULL REFERENCES procesos(id),
    especialidad_id BIGINT      NOT NULL REFERENCES especialidades(id),
    estado          VARCHAR(20) NOT NULL DEFAULT 'activo' CHECK (estado IN ('activo', 'archivado')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archivado_at    TIMESTAMPTZ,
    CONSTRAINT ck_postulante_profesiones_archivado
        CHECK ((estado = 'archivado') = (archivado_at IS NOT NULL))
);
-- Regla 4: máximo un par (proceso, profesión) activo por usuario (patrón de 049:29-33).
CREATE UNIQUE INDEX uq_postulante_profesiones_usuario_activo
    ON postulante_profesiones (usuario_id) WHERE estado = 'activo';
CREATE INDEX idx_postulante_profesiones_usuario      ON postulante_profesiones (usuario_id, created_at DESC);
CREATE INDEX idx_postulante_profesiones_proceso      ON postulante_profesiones (proceso_id);
CREATE INDEX idx_postulante_profesiones_especialidad ON postulante_profesiones (especialidad_id);

-- +goose Down
DROP TABLE IF EXISTS postulante_profesiones;
