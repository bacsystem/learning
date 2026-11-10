-- +goose Up

ALTER TABLE sesiones ADD COLUMN last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE INDEX idx_sesiones_last_seen ON sesiones(last_seen_at);

-- +goose Down

DROP INDEX IF EXISTS idx_sesiones_last_seen;
ALTER TABLE sesiones DROP COLUMN IF EXISTS last_seen_at;
