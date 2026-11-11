-- +goose NO TRANSACTION
-- PLT-03: the correo health check reads MAX(updated_at) of the sent e-mails,
-- most of the rows, which had no index (docs/api/plataforma-salud.md).
--
-- CONCURRENTLY so the build does not take a SHARE lock on email_queue during
-- the deploy (it would block enqueuing and the e-mail worker); it requires NO
-- TRANSACTION. A failed concurrent build leaves an INVALID index that IF NOT
-- EXISTS would skip next time, so the final block aborts the migration if it
-- is INVALID (same pattern as 054 and 058): DROP INDEX CONCURRENTLY it and
-- run the migrations again.

-- +goose Up
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_email_queue_sent_updated ON email_queue (updated_at) WHERE status = 'sent';

-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_index i
        JOIN pg_class c ON c.oid = i.indexrelid
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = current_schema()
          AND c.relname = 'idx_email_queue_sent_updated'
          AND NOT (i.indisvalid AND i.indisready)
    ) THEN
        RAISE EXCEPTION '063: índice INVALID (CREATE INDEX CONCURRENTLY fallido): idx_email_queue_sent_updated'
            USING HINT = 'Ejecute DROP INDEX CONCURRENTLY idx_email_queue_sent_updated y vuelva a correr las migraciones.';
    END IF;
END
$$;
-- +goose StatementEnd

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS idx_email_queue_sent_updated;
