-- +goose Up
CREATE TABLE usuarios (
    id              BIGSERIAL PRIMARY KEY,
    persona_id      BIGINT NOT NULL UNIQUE REFERENCES personas(id),
    username        VARCHAR(30) NOT NULL UNIQUE,
    password_hash   VARCHAR(255) NOT NULL,
    rol             VARCHAR(20) NOT NULL DEFAULT 'usuario',
    estado          VARCHAR(20) NOT NULL DEFAULT 'pendiente',
    login_intentos  INT NOT NULL DEFAULT 0,
    bloqueado_hasta TIMESTAMPTZ,
    activated_at    TIMESTAMPTZ,
    activated_by    BIGINT REFERENCES usuarios(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_usuarios_username ON usuarios(username);
CREATE INDEX idx_usuarios_estado ON usuarios(estado);
CREATE INDEX idx_usuarios_bloqueado_hasta ON usuarios(bloqueado_hasta)
    WHERE bloqueado_hasta IS NOT NULL;

-- +goose Down
DROP TABLE usuarios;
