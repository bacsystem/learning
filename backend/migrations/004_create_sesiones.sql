-- +goose Up
CREATE TABLE sesiones (
    id          BIGSERIAL PRIMARY KEY,
    usuario_id  BIGINT NOT NULL REFERENCES usuarios(id),
    token_hash  VARCHAR(255) NOT NULL,
    ip          VARCHAR(45),
    user_agent  TEXT,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sesiones_usuario_id ON sesiones(usuario_id);
CREATE INDEX idx_sesiones_token_hash ON sesiones(token_hash);

-- +goose Down
DROP TABLE sesiones;
