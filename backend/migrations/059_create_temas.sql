-- SIA-E1-01: temas bajo asignatura (docs/api/serums-ia-e1-taxonomia.md §4).
--
-- Efecto sobre los datos existentes: crea dos tablas nuevas y añade
-- preguntas.tema_id, que nace NULL en todo el banco; ninguna fila se reescribe.
-- El índice parcial se crea sin CONCURRENTLY porque la columna es nueva y está
-- vacía, así que se construye al instante.
--
-- Qué pierde el Down: los temas, sus especialidades y la clasificación por
-- tema de las preguntas. El resto de columnas de preguntas queda intacto.

-- +goose Up
CREATE TABLE temas (
    id            BIGSERIAL    PRIMARY KEY,
    asignatura_id BIGINT       NOT NULL REFERENCES asignaturas(id),
    nombre        VARCHAR(120) NOT NULL CHECK (char_length(nombre) BETWEEN 3 AND 120),
    nombre_clave  VARCHAR(120) NOT NULL,          -- texto.Plegar(nombre), calculado en Go
    descripcion   VARCHAR(500),
    estado        VARCHAR(20)  NOT NULL DEFAULT 'activo' CHECK (estado IN ('activo', 'archivado')),
    orden         INT          NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_temas_asignatura_nombre_clave UNIQUE (asignatura_id, nombre_clave),
    CONSTRAINT uq_temas_id_asignatura UNIQUE (id, asignatura_id)   -- destino de la FK compuesta
);
CREATE INDEX idx_temas_asignatura_estado_orden ON temas (asignatura_id, estado, orden);

CREATE TABLE tema_especialidades (
    tema_id         BIGINT NOT NULL REFERENCES temas(id) ON DELETE CASCADE,
    especialidad_id BIGINT NOT NULL REFERENCES especialidades(id),
    PRIMARY KEY (tema_id, especialidad_id)
);
CREATE INDEX idx_tema_especialidades_especialidad ON tema_especialidades (especialidad_id);

ALTER TABLE preguntas ADD COLUMN tema_id BIGINT;
-- MATCH SIMPLE: con tema_id NULL no se comprueba (banco existente).
ALTER TABLE preguntas ADD CONSTRAINT fk_preguntas_tema
    FOREIGN KEY (tema_id, asignatura_id) REFERENCES temas (id, asignatura_id);
CREATE INDEX idx_preguntas_tema ON preguntas (tema_id) WHERE tema_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_preguntas_tema;
ALTER TABLE preguntas DROP CONSTRAINT IF EXISTS fk_preguntas_tema;
ALTER TABLE preguntas DROP COLUMN IF EXISTS tema_id;
DROP TABLE IF EXISTS tema_especialidades;
DROP TABLE IF EXISTS temas;
