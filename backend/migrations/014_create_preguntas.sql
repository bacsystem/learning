-- +goose Up
CREATE TABLE preguntas (
    id BIGSERIAL PRIMARY KEY,
    institucion_id BIGINT NOT NULL REFERENCES instituciones(id),
    proceso_id BIGINT NOT NULL REFERENCES procesos(id),
    asignatura_id BIGINT NOT NULL REFERENCES asignaturas(id),
    grado_id BIGINT REFERENCES grados(id),
    especialidad_id BIGINT REFERENCES especialidades(id),
    enunciado TEXT NOT NULL,
    tipo VARCHAR(20) NOT NULL DEFAULT 'unica'
        CHECK (tipo IN ('unica', 'multiple')),
    dificultad VARCHAR(20) NOT NULL DEFAULT 'medio'
        CHECK (dificultad IN ('facil', 'medio', 'dificil')),
    estado VARCHAR(20) NOT NULL DEFAULT 'activo'
        CHECK (estado IN ('activo', 'eliminada')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_preguntas_proceso ON preguntas(proceso_id);
CREATE INDEX idx_preguntas_asignatura ON preguntas(asignatura_id);
CREATE INDEX idx_preguntas_grado ON preguntas(grado_id) WHERE grado_id IS NOT NULL;
CREATE INDEX idx_preguntas_especialidad ON preguntas(especialidad_id) WHERE especialidad_id IS NOT NULL;
CREATE INDEX idx_preguntas_dificultad ON preguntas(dificultad);
CREATE INDEX idx_preguntas_estado ON preguntas(estado);
CREATE INDEX idx_preguntas_multi_tenant ON preguntas(institucion_id, proceso_id, grado_id, especialidad_id, asignatura_id);
CREATE INDEX idx_preguntas_enunciado_trgm ON preguntas USING GIN (enunciado gin_trgm_ops);

CREATE TABLE pregunta_etiquetas (
    pregunta_id BIGINT NOT NULL REFERENCES preguntas(id) ON DELETE CASCADE,
    etiqueta_id BIGINT NOT NULL REFERENCES etiquetas_pregunta(id) ON DELETE CASCADE,
    PRIMARY KEY (pregunta_id, etiqueta_id)
);

-- +goose Down
DROP TABLE IF EXISTS pregunta_etiquetas;
DROP TABLE IF EXISTS preguntas;
