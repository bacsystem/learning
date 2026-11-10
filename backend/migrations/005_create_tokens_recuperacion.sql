-- +goose Up
CREATE TABLE tokens_recuperacion (
    id          BIGSERIAL PRIMARY KEY,
    usuario_id  BIGINT NOT NULL REFERENCES usuarios(id),
    token_hash  VARCHAR(255) NOT NULL UNIQUE,
    usado       BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE tokens_recuperacion;
