-- +goose NO TRANSACTION
-- H149: simulacro_asignaturas.asignatura_id (050) es FK a asignaturas pero no
-- es columna líder de ningún índice (la PK empieza por simulacro_id). Sin él,
-- cada DELETE/UPDATE de la clave en asignaturas hace un seq scan de la tabla,
-- que crece con cada simulacro.
--
-- Mismo patrón que 054: CONCURRENTLY para no bloquear escrituras (requiere NO
-- TRANSACTION, así que esta migración no se revierte si se interrumpe) y un
-- bloque final que aborta si el índice quedó INVALID, porque IF NOT EXISTS lo
-- saltaría en el siguiente intento. En ese caso, revisar
-- pg_stat_progress_create_index (docs/puesta-en-marcha.md), DROP INDEX
-- CONCURRENTLY idx_simulacro_asignaturas_asignatura y volver a migrar.

-- +goose Up
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_simulacro_asignaturas_asignatura ON simulacro_asignaturas(asignatura_id);

-- +goose StatementBegin
DO $$
DECLARE
    invalid TEXT;
BEGIN
    SELECT string_agg(c.relname, ', ' ORDER BY c.relname) INTO invalid
    FROM pg_index i
    JOIN pg_class c ON c.oid = i.indexrelid
    JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname = current_schema()
      AND c.relname IN ('idx_simulacro_asignaturas_asignatura')
      AND NOT (i.indisvalid AND i.indisready);
    IF invalid IS NOT NULL THEN
        RAISE EXCEPTION '058: índice(s) INVALID (CREATE INDEX CONCURRENTLY fallido): %', invalid
            USING HINT = 'Revise pg_stat_progress_create_index, ejecute DROP INDEX CONCURRENTLY <nombre> y vuelva a correr las migraciones.';
    END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS idx_simulacro_asignaturas_asignatura;
