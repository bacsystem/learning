-- +goose Up

ALTER TABLE usuarios ADD COLUMN tour_completado BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down

ALTER TABLE usuarios DROP COLUMN IF EXISTS tour_completado;
