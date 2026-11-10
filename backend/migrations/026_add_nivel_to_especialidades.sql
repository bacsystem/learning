-- +goose Up
ALTER TABLE especialidades ADD COLUMN IF NOT EXISTS nivel VARCHAR(30);

-- +goose Down
ALTER TABLE especialidades DROP COLUMN nivel;
