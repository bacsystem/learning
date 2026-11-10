-- +goose Up

-- H5/H7/H32/H37: integridad de las respuestas de los intentos.
--
-- 1. (pregunta_id, id) es único en respuestas para que las tablas de historial
--    puedan referenciar el PAR: una respuesta solo puede registrarse para la
--    pregunta a la que pertenece.
-- 2. Las FKs de historial pasan a ON DELETE RESTRICT: borrar una alternativa
--    con intentos registrados ya no anula en silencio las respuestas.
-- 3. simulacro_respuestas deja de tener una sola respuesta_id: las selecciones
--    viven en simulacro_respuesta_selecciones (una fila por alternativa marcada),
--    y simulacro_respuestas queda como una fila por pregunta (orden,
--    respondida_at, es_correcta a nivel de pregunta).

ALTER TABLE respuestas
    ADD CONSTRAINT respuestas_pregunta_id_id_key UNIQUE (pregunta_id, id);

-- ── practica_respuestas ──────────────────────────────────────────────────────

-- Datos existentes inválidos: selección de una alternativa de otra pregunta →
-- se descarta (la fila queda como "sin responder").
UPDATE practica_respuestas prr
SET respuesta_id = NULL, es_correcta = NULL
WHERE prr.respuesta_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM respuestas r
                  WHERE r.id = prr.respuesta_id AND r.pregunta_id = prr.pregunta_id);

-- Selecciones duplicadas (toggles concurrentes): se conserva la fila más antigua.
DELETE FROM practica_respuestas a
USING practica_respuestas b
WHERE a.respuesta_id IS NOT NULL
  AND a.practica_id = b.practica_id
  AND a.pregunta_id = b.pregunta_id
  AND a.respuesta_id = b.respuesta_id
  AND a.id > b.id;

ALTER TABLE practica_respuestas
    DROP CONSTRAINT practica_respuestas_respuesta_id_fkey,
    ADD  CONSTRAINT practica_respuestas_pregunta_respuesta_fkey
         FOREIGN KEY (pregunta_id, respuesta_id)
         REFERENCES respuestas (pregunta_id, id) ON DELETE RESTRICT;

CREATE UNIQUE INDEX uq_practica_respuestas_seleccion
    ON practica_respuestas (practica_id, pregunta_id, respuesta_id)
    WHERE respuesta_id IS NOT NULL;

CREATE INDEX idx_practica_resp_pregunta_respuesta
    ON practica_respuestas (pregunta_id, respuesta_id);

-- ── simulacro_respuesta_selecciones ──────────────────────────────────────────

CREATE TABLE simulacro_respuesta_selecciones (
    simulacro_id BIGINT      NOT NULL,
    pregunta_id  BIGINT      NOT NULL,
    respuesta_id BIGINT      NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (simulacro_id, pregunta_id, respuesta_id),
    CONSTRAINT simulacro_sel_simulacro_pregunta_fkey
        FOREIGN KEY (simulacro_id, pregunta_id)
        REFERENCES simulacro_respuestas (simulacro_id, pregunta_id) ON DELETE CASCADE,
    CONSTRAINT simulacro_sel_pregunta_respuesta_fkey
        FOREIGN KEY (pregunta_id, respuesta_id)
        REFERENCES respuestas (pregunta_id, id) ON DELETE RESTRICT
);

CREATE INDEX idx_simulacro_sel_pregunta_respuesta
    ON simulacro_respuesta_selecciones (pregunta_id, respuesta_id);

-- Migrar las respuestas existentes (solo las válidas: misma pregunta).
INSERT INTO simulacro_respuesta_selecciones (simulacro_id, pregunta_id, respuesta_id)
SELECT srr.simulacro_id, srr.pregunta_id, srr.respuesta_id
FROM simulacro_respuestas srr
JOIN respuestas r ON r.id = srr.respuesta_id AND r.pregunta_id = srr.pregunta_id;

-- Respuestas inválidas de intentos aún en curso dejan de contar como respondidas.
UPDATE simulacro_respuestas srr
SET respondida_at = NULL
WHERE srr.respuesta_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM respuestas r
                  WHERE r.id = srr.respuesta_id AND r.pregunta_id = srr.pregunta_id)
  AND EXISTS (SELECT 1 FROM simulacros s WHERE s.id = srr.simulacro_id AND s.estado = 'en_curso');

ALTER TABLE simulacro_respuestas DROP COLUMN respuesta_id;

-- +goose Down

ALTER TABLE simulacro_respuestas ADD COLUMN respuesta_id BIGINT;

-- Una sola respuesta por pregunta en el esquema anterior: se conserva la menor.
UPDATE simulacro_respuestas srr
SET respuesta_id = sel.respuesta_id
FROM (SELECT simulacro_id, pregunta_id, MIN(respuesta_id) AS respuesta_id
      FROM simulacro_respuesta_selecciones
      GROUP BY simulacro_id, pregunta_id) sel
WHERE sel.simulacro_id = srr.simulacro_id AND sel.pregunta_id = srr.pregunta_id;

ALTER TABLE simulacro_respuestas
    ADD CONSTRAINT simulacro_respuestas_respuesta_id_fkey
        FOREIGN KEY (respuesta_id) REFERENCES respuestas(id) ON DELETE SET NULL;

DROP TABLE IF EXISTS simulacro_respuesta_selecciones;

DROP INDEX IF EXISTS idx_practica_resp_pregunta_respuesta;
DROP INDEX IF EXISTS uq_practica_respuestas_seleccion;

ALTER TABLE practica_respuestas
    DROP CONSTRAINT practica_respuestas_pregunta_respuesta_fkey,
    ADD  CONSTRAINT practica_respuestas_respuesta_id_fkey
         FOREIGN KEY (respuesta_id) REFERENCES respuestas(id) ON DELETE SET NULL;

ALTER TABLE respuestas DROP CONSTRAINT IF EXISTS respuestas_pregunta_id_id_key;
