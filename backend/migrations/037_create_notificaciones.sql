-- +goose Up
CREATE TABLE notificaciones (
    id          BIGSERIAL PRIMARY KEY,
    titulo      VARCHAR(200) NOT NULL,
    cuerpo      TEXT NOT NULL,
    tipo        VARCHAR(50)  NOT NULL DEFAULT 'sistema',
    destinatario VARCHAR(50) NOT NULL DEFAULT 'todos',
    creado_por  BIGINT REFERENCES usuarios(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE notificacion_leidas (
    notificacion_id BIGINT NOT NULL REFERENCES notificaciones(id) ON DELETE CASCADE,
    usuario_id      BIGINT NOT NULL REFERENCES usuarios(id)      ON DELETE CASCADE,
    leida_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (notificacion_id, usuario_id)
);

CREATE INDEX idx_notificaciones_created_at   ON notificaciones(created_at DESC);
CREATE INDEX idx_notif_leidas_usuario        ON notificacion_leidas(usuario_id);

-- +goose Down
DROP TABLE IF EXISTS notificacion_leidas;
DROP TABLE IF EXISTS notificaciones;
