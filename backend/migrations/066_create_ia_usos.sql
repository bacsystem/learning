-- SIA-E6-01: one row per AI invocation (docs/api/serums-ia-e6-01-servicio-ia.md
-- §6), also for rejections, written outside the consumer's transaction. It
-- never stores the prompt, the answer, the detected text nor error messages.
-- actor_id links to usuarios, so SIA-E9-07 (export and erasure) includes it.
-- +goose Up
CREATE TABLE ia_usos (
    id              BIGSERIAL     PRIMARY KEY,
    operacion       VARCHAR(30)   NOT NULL CHECK (operacion IN
                    ('clasificar', 'generar_preguntas', 'generar_material', 'generar_explicacion')),
    actor_rol       VARCHAR(10)   NOT NULL CHECK (actor_rol IN ('admin', 'usuario', 'sistema')),
    actor_id        BIGINT        REFERENCES usuarios(id),
    proveedor       VARCHAR(30)   NOT NULL,              -- 'doble' o el adaptador; '' si no_disponible
    modelo          VARCHAR(80),
    tokens_entrada  INT           NOT NULL DEFAULT 0 CHECK (tokens_entrada >= 0),
    tokens_salida   INT           NOT NULL DEFAULT 0 CHECK (tokens_salida >= 0),
    costo_usd       NUMERIC(12,6) NOT NULL DEFAULT 0 CHECK (costo_usd >= 0),
    duracion_ms     INT           NOT NULL CHECK (duracion_ms >= 0),
    intentos        SMALLINT      NOT NULL DEFAULT 0 CHECK (intentos BETWEEN 0 AND 6),
    resultado       VARCHAR(30)   NOT NULL CHECK (resultado IN ('ok', 'no_disponible',
                    'entrada_demasiado_grande', 'datos_personales', 'timeout', 'cancelado', 'salida_invalida', 'error_proveedor')),
    desde_cache     BOOLEAN       NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_ia_usos_actor CHECK ((actor_rol = 'sistema') = (actor_id IS NULL))
);
CREATE INDEX idx_ia_usos_created   ON ia_usos (created_at);
CREATE INDEX idx_ia_usos_actor     ON ia_usos (actor_id, created_at) WHERE actor_id IS NOT NULL;
CREATE INDEX idx_ia_usos_operacion ON ia_usos (operacion, created_at);

-- +goose Down
DROP TABLE IF EXISTS ia_usos;
