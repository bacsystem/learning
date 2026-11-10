-- +goose Up
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

-- +goose Down
DROP TABLE telegram_queue;
