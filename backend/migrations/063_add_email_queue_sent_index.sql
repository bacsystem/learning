-- PLT-03: the correo health check reads MAX(updated_at) of the sent e-mails,
-- most of the rows, which had no index (docs/api/plataforma-salud.md).
-- +goose Up
CREATE INDEX idx_email_queue_sent_updated ON email_queue (updated_at) WHERE status = 'sent';

-- +goose Down
DROP INDEX IF EXISTS idx_email_queue_sent_updated;
