-- +goose Up
DROP TABLE IF EXISTS telegram_queue;

-- +goose Down
-- Recrea exactamente el esquema de 008_create_telegram_queue.sql (los datos no
-- se recuperan).
CREATE TABLE telegram_queue (
    id            BIGSERIAL PRIMARY KEY,
    chat_id       BIGINT NOT NULL,
    message       TEXT NOT NULL,
    status        VARCHAR(20) NOT NULL DEFAULT 'pending',
    retry_count   INT NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_telegram_queue_status_retry ON telegram_queue(status, next_retry_at)
    WHERE status IN ('pending', 'failed');
