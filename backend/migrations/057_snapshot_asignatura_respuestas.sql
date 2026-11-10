-- +goose Up

-- H73: la asignatura de cada pregunta de un intento se fija AL CREARLO.
--
-- Simulacros y prácticas agrupaban sus resultados por la asignatura ACTUAL de
-- la pregunta (preguntas.asignatura_id). Si un administrador movía una
-- pregunta a otra asignatura durante un intento, la respuesta quedaba fuera
-- del snapshot de calificación (simulacro_asignaturas) y se perdía de la nota;
-- el desglose de prácticas ya completadas cambiaba retroactivamente.
--
-- Cada fila de simulacro_respuestas / practica_respuestas guarda ahora la
-- asignatura que tenía la pregunta al crear el intento; la calificación, el
-- desglose y la revisión agrupan por esa columna.

ALTER TABLE simulacro_respuestas ADD COLUMN asignatura_id BIGINT;
ALTER TABLE practica_respuestas  ADD COLUMN asignatura_id BIGINT;

-- Backfill: la mejor información disponible es la asignatura actual.
UPDATE simulacro_respuestas srr
SET asignatura_id = pq.asignatura_id
FROM preguntas pq
WHERE pq.id = srr.pregunta_id;

UPDATE practica_respuestas prr
SET asignatura_id = pq.asignatura_id
FROM preguntas pq
WHERE pq.id = prr.pregunta_id;

ALTER TABLE simulacro_respuestas
    ALTER COLUMN asignatura_id SET NOT NULL,
    ADD CONSTRAINT simulacro_respuestas_asignatura_id_fkey
        FOREIGN KEY (asignatura_id) REFERENCES asignaturas(id);

ALTER TABLE practica_respuestas
    ALTER COLUMN asignatura_id SET NOT NULL,
    ADD CONSTRAINT practica_respuestas_asignatura_id_fkey
        FOREIGN KEY (asignatura_id) REFERENCES asignaturas(id);

-- Índices para las FKs (borrar una asignatura no recorre las tablas enteras).
CREATE INDEX idx_simulacro_respuestas_asignatura ON simulacro_respuestas (asignatura_id);
CREATE INDEX idx_practica_respuestas_asignatura  ON practica_respuestas (asignatura_id);

-- +goose Down

DROP INDEX IF EXISTS idx_practica_respuestas_asignatura;
DROP INDEX IF EXISTS idx_simulacro_respuestas_asignatura;

ALTER TABLE practica_respuestas  DROP COLUMN IF EXISTS asignatura_id;
ALTER TABLE simulacro_respuestas DROP COLUMN IF EXISTS asignatura_id;
