-- +goose Up
-- The email worker now claims rows atomically (status 'sending' + claimed_at,
-- via FOR UPDATE SKIP LOCKED) so several replicas never send the same email;
-- a claim older than the stale timeout is reclaimed. status has no CHECK
-- constraint, so 'sending' needs no constraint change.
ALTER TABLE email_queue ADD COLUMN claimed_at TIMESTAMPTZ;

CREATE INDEX idx_email_queue_sending_claimed ON email_queue (claimed_at)
    WHERE status = 'sending';

-- +goose Down
DROP INDEX IF EXISTS idx_email_queue_sending_claimed;
-- Rows caught mid-send go back to the queue for the previous worker version.
UPDATE email_queue SET status = 'pending' WHERE status = 'sending';
ALTER TABLE email_queue DROP COLUMN IF EXISTS claimed_at;
