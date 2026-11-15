-- #261: el nombre de una etiqueta no distingue mayúsculas ni espacios
-- alrededor. El UNIQUE(asignatura_id, nombre) de la 013 deja convivir «zz»,
-- «ZZ» y « zz ». Antes de crear el índice se fusionan las que ya chocan: se
-- conserva la de menor id, las preguntas de las demás pasan a ella y el
-- nombre queda recortado. Si ya no hay choques, la fusión no toca nada.
-- +goose Up
-- +goose StatementBegin
DO $$
BEGIN
    CREATE TEMP TABLE etiqueta_fusion ON COMMIT DROP AS
    SELECT id AS viejo,
           MIN(id) OVER (PARTITION BY asignatura_id, lower(btrim(nombre))) AS nuevo
    FROM etiquetas_pregunta;

    INSERT INTO pregunta_etiquetas (pregunta_id, etiqueta_id)
    SELECT pe.pregunta_id, f.nuevo
    FROM pregunta_etiquetas pe
    JOIN etiqueta_fusion f ON f.viejo = pe.etiqueta_id
    WHERE f.viejo <> f.nuevo
    ON CONFLICT DO NOTHING;

    -- ON DELETE CASCADE quita también los vínculos viejos de pregunta_etiquetas.
    DELETE FROM etiquetas_pregunta e
    USING etiqueta_fusion f
    WHERE e.id = f.viejo AND f.viejo <> f.nuevo;

    UPDATE etiquetas_pregunta SET nombre = btrim(nombre) WHERE nombre <> btrim(nombre);
END
$$;
-- +goose StatementEnd

CREATE UNIQUE INDEX uq_etiquetas_pregunta_asignatura_nombre
    ON etiquetas_pregunta (asignatura_id, lower(btrim(nombre)));

-- +goose Down
DROP INDEX IF EXISTS uq_etiquetas_pregunta_asignatura_nombre;
