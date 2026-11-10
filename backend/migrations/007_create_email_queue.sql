-- +goose Up
CREATE TABLE email_queue (
    id            BIGSERIAL PRIMARY KEY,
    to_address    TEXT NOT NULL,
    subject       VARCHAR(255) NOT NULL,
    body_html     TEXT NOT NULL,
    status        VARCHAR(20) NOT NULL DEFAULT 'pending',
    retry_count   INT NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_email_queue_status_retry ON email_queue(status, next_retry_at)
    WHERE status IN ('pending', 'failed');

-- +goose Down
DROP TABLE email_queue;
