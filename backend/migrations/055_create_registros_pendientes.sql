-- Registro en dos pasos. POST /auth/register {dni, email} solo valida la
-- identidad y deja aquí una solicitud pendiente con un enlace enviado al correo
-- de la persona (el que tiene registrado, o el ingresado si no tiene). Las
-- credenciales (usuario y contraseña) se eligen recién en POST
-- /auth/verify-email, que bloquea la fila (FOR UPDATE), crea el usuario y la
-- marca consumida en una sola transacción. Así nunca hay credenciales elegidas
-- por un atacante esperando a que alguien abra el enlace.
--
--   - token_hash: SHA-256 hex del token; el token en claro nunca se guarda.
--   - email: correo destino del enlace, normalizado y CIFRADO (como el resto
--     de PII); se escribe en personas al verificar si la persona no tenía.
--     email_hmac es su HMAC normalizado.
--   - uq_registros_pendientes_persona_activo: a lo sumo una solicitud sin
--     consumir por persona. El registro hace UPSERT sobre ella, así una nueva
--     solicitud reemplaza el token anterior también bajo concurrencia.
--   - Las filas vencidas se borran de forma perezosa al crear otra solicitud
--     para la misma persona (sin cron).

-- +goose Up
CREATE TABLE registros_pendientes (
    id            BIGSERIAL PRIMARY KEY,
    persona_id    BIGINT NOT NULL REFERENCES personas(id) ON DELETE CASCADE,
    email         TEXT NOT NULL,
    email_hmac    TEXT NOT NULL,
    token_hash    TEXT NOT NULL,
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    consumed_at   TIMESTAMPTZ,
    CONSTRAINT uq_registros_pendientes_token_hash UNIQUE (token_hash)
);

CREATE UNIQUE INDEX uq_registros_pendientes_persona_activo
    ON registros_pendientes(persona_id) WHERE consumed_at IS NULL;
CREATE INDEX idx_registros_pendientes_persona ON registros_pendientes(persona_id);

-- +goose Down
DROP TABLE registros_pendientes;
