-- +goose NO TRANSACTION
-- H61: índices para FKs que no eran la columna líder de ningún índice. Sin
-- ellos, cada DELETE/UPDATE en la tabla referenciada (preguntas, respuestas,
-- usuarios, configuracion_examen) hace un seq scan de la tabla hija, y los
-- JOIN por esas columnas también.
--
-- CONCURRENTLY para no bloquear escrituras en tablas grandes de producción;
-- requiere NO TRANSACTION (goose ejecuta cada sentencia por separado).
-- Si una construcción concurrente falla deja un índice INVALID que el
-- IF NOT EXISTS saltaría en el siguiente intento: el bloque final aborta la
-- migración (H99) si alguno de estos índices quedó INVALID; en ese caso,
-- DROP INDEX <nombre> y volver a correr.
--
-- Índices de una sola columna. Desde 048 la selección de respuestas del
-- simulacro vive en simulacro_respuesta_selecciones (su PK empieza por
-- simulacro_id), así que el FK a respuestas se indexa ahí.
--
-- H99: no se crea practica_respuestas(pregunta_id): 048 ya creó
-- idx_practica_resp_pregunta_respuesta (pregunta_id, respuesta_id), que lo
-- cubre (columna líder). Los dos índices por respuesta_id SÍ se mantienen:
-- los FK a respuestas son compuestos (pregunta_id, respuesta_id) y quedan
-- cubiertos por los índices de 048, pero questions/respuestas_sync.go filtra
-- solo por respuesta_id (WHERE respuesta_id = ANY($1)) y Postgres 16 no puede
-- usar un índice cuya columna líder es pregunta_id para eso (no hay skip
-- scan). Editado sin producción.

-- +goose Up
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_simulacros_configuracion ON simulacros(configuracion_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_practica_resp_respuesta_id ON practica_respuestas(respuesta_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_simulacro_resp_pregunta_id ON simulacro_respuestas(pregunta_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_simulacro_sel_respuesta_id ON simulacro_respuesta_selecciones(respuesta_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tokens_recuperacion_usuario ON tokens_recuperacion(usuario_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_inscripciones_revisado_por ON inscripciones(revisado_por);

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
      AND c.relname IN ('idx_simulacros_configuracion', 'idx_practica_resp_respuesta_id',
                        'idx_simulacro_resp_pregunta_id', 'idx_simulacro_sel_respuesta_id',
                        'idx_tokens_recuperacion_usuario', 'idx_inscripciones_revisado_por')
      AND NOT (i.indisvalid AND i.indisready);
    IF invalid IS NOT NULL THEN
        RAISE EXCEPTION '054: índice(s) INVALID (CREATE INDEX CONCURRENTLY fallido): %', invalid
            USING HINT = 'Ejecute DROP INDEX CONCURRENTLY <nombre> para cada uno y vuelva a correr las migraciones.';
    END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS idx_inscripciones_revisado_por;
DROP INDEX CONCURRENTLY IF EXISTS idx_tokens_recuperacion_usuario;
DROP INDEX CONCURRENTLY IF EXISTS idx_simulacro_sel_respuesta_id;
DROP INDEX CONCURRENTLY IF EXISTS idx_simulacro_resp_pregunta_id;
DROP INDEX CONCURRENTLY IF EXISTS idx_practica_resp_respuesta_id;
-- Ya no lo crea el Up (H99); se mantiene para limpiar bases migradas antes.
DROP INDEX CONCURRENTLY IF EXISTS idx_practica_resp_pregunta_id;
DROP INDEX CONCURRENTLY IF EXISTS idx_simulacros_configuracion;
