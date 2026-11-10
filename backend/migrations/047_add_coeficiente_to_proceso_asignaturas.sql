-- +goose Up

-- NOTA (H41): la versión original de este Up hacía el DROP sin copiar los
-- valores y el coeficiente global se perdía (todas las asignaturas en 1.0).
-- El backfill de abajo lo copia antes del DROP. Una base local que ya había
-- aplicado la versión original conserva los coeficientes en 1.0.

-- Coeficiente por asignatura (multiplicador de peso en el cálculo de nota)
ALTER TABLE proceso_asignaturas
    ADD COLUMN coeficiente DECIMAL(5,2) NOT NULL DEFAULT 1.0;

-- Backfill: cada asignatura del proceso hereda el coeficiente global de la
-- configuración de examen del proceso (la activa más reciente si hay varias).
UPDATE proceso_asignaturas pa
SET coeficiente = ce.coeficiente
FROM (
    SELECT DISTINCT ON (proceso_id) proceso_id, coeficiente
    FROM configuracion_examen
    ORDER BY proceso_id, (estado = 'activo') DESC, updated_at DESC, id DESC
) ce
WHERE pa.proceso_id = ce.proceso_id;

-- Quitar coeficiente global de configuracion_examen (ahora es por asignatura)
ALTER TABLE configuracion_examen DROP COLUMN IF EXISTS coeficiente;

-- +goose Down
ALTER TABLE configuracion_examen
    ADD COLUMN coeficiente DECIMAL(5,2) NOT NULL DEFAULT 1.0;

-- Copia de vuelta: el coeficiente más frecuente entre las asignaturas del
-- proceso (exacto si eran todos iguales, que es lo que produce el Up). Si se
-- habían diferenciado por asignatura, esa granularidad no cabe en el esquema
-- anterior y se pierde.
UPDATE configuracion_examen ce
SET coeficiente = pa.coeficiente
FROM (
    SELECT proceso_id, MODE() WITHIN GROUP (ORDER BY coeficiente) AS coeficiente
    FROM proceso_asignaturas
    GROUP BY proceso_id
) pa
WHERE ce.proceso_id = pa.proceso_id;

ALTER TABLE proceso_asignaturas DROP COLUMN IF EXISTS coeficiente;
