-- +goose Up
ALTER TABLE personas ADD COLUMN email_hmac TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_personas_email_hmac ON personas(email_hmac);

-- +goose Down
DROP INDEX IF EXISTS idx_personas_email_hmac;
ALTER TABLE personas DROP COLUMN IF EXISTS email_hmac;
