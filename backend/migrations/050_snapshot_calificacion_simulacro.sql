-- +goose Up

-- H33: la calificación de un simulacro usa la configuración vigente AL CREARLO
-- (no la del momento de finalizar) y el número de preguntas realmente asignadas.

ALTER TABLE simulacros
    ADD COLUMN nota_minima DECIMAL(5,2),
    ADD COLUMN nota_maxima DECIMAL(5,2);

UPDATE simulacros s
SET nota_minima = ce.nota_minima, nota_maxima = ce.nota_maxima
FROM configuracion_examen ce
WHERE ce.id = s.configuracion_id;

UPDATE simulacros
SET nota_minima = COALESCE(nota_minima, 55.0),
    nota_maxima = COALESCE(nota_maxima, 100.0);

ALTER TABLE simulacros
    ALTER COLUMN nota_minima SET DEFAULT 55.0,
    ALTER COLUMN nota_minima SET NOT NULL,
    ALTER COLUMN nota_maxima SET DEFAULT 100.0,
    ALTER COLUMN nota_maxima SET NOT NULL;

CREATE TABLE simulacro_asignaturas (
    simulacro_id     BIGINT       NOT NULL REFERENCES simulacros(id) ON DELETE CASCADE,
    asignatura_id    BIGINT       NOT NULL REFERENCES asignaturas(id),
    numero_preguntas INT          NOT NULL CHECK (numero_preguntas >= 0), -- asignadas de verdad
    peso             DECIMAL(5,2) NOT NULL,
    coeficiente      DECIMAL(5,2) NOT NULL,
    PRIMARY KEY (simulacro_id, asignatura_id)
);

-- Backfill: preguntas realmente asignadas por asignatura; peso del resultado
-- guardado si existe, si no el de la configuración actual del proceso.
INSERT INTO simulacro_asignaturas (simulacro_id, asignatura_id, numero_preguntas, peso, coeficiente)
SELECT srr.simulacro_id,
       pq.asignatura_id,
       COUNT(*),
       COALESCE(MAX(sr.peso), MAX(pa.peso_porcentual), 0),
       COALESCE(MAX(pa.coeficiente), 1.0)
FROM simulacro_respuestas srr
JOIN simulacros s  ON s.id  = srr.simulacro_id
JOIN preguntas  pq ON pq.id = srr.pregunta_id
LEFT JOIN proceso_asignaturas pa
       ON pa.proceso_id = s.proceso_id AND pa.asignatura_id = pq.asignatura_id
LEFT JOIN simulacro_resultados sr
       ON sr.simulacro_id = s.id AND sr.asignatura_id = pq.asignatura_id
GROUP BY srr.simulacro_id, pq.asignatura_id;

-- +goose Down
DROP TABLE IF EXISTS simulacro_asignaturas;
ALTER TABLE simulacros
    DROP COLUMN IF EXISTS nota_maxima,
    DROP COLUMN IF EXISTS nota_minima;
