-- +goose Up
ALTER TABLE procesos ADD COLUMN nivel VARCHAR(30);

-- +goose Down
ALTER TABLE procesos DROP COLUMN nivel;
