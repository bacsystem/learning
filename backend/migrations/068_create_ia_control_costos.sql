-- SIA-E6-02: configuración, cuotas, gasto mensual y caché de la IA
-- (docs/api/serums-ia-e6-02-control-costos.md §6; ADR 0009 y ADR 0002).
-- ia_cuotas e ia_gasto_mes no llevan usuario_id ni FK: SIA-E9-07 no los borra.
-- El CHECK de resultado de la 066 no tiene nombre: Postgres lo llama
-- ia_usos_resultado_check. Sin IF EXISTS a propósito, como en la 065.
-- +goose Up
CREATE TABLE ia_configuracion (
    id                      SMALLINT      PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    limite_usuario_dia      INT           NOT NULL DEFAULT 5   CHECK (limite_usuario_dia BETWEEN 0 AND 50),
    limite_usuario_mes      INT           NOT NULL DEFAULT 60  CHECK (limite_usuario_mes BETWEEN 0 AND 1000),
    limite_admin_dia        INT           NOT NULL DEFAULT 200 CHECK (limite_admin_dia BETWEEN 1 AND 1000),
    max_preguntas_solicitud INT           NOT NULL DEFAULT 20  CHECK (max_preguntas_solicitud BETWEEN 1 AND 50),
    lote_clasificacion      INT           NOT NULL DEFAULT 20  CHECK (lote_clasificacion BETWEEN 1 AND 50),
    presupuesto_mensual_usd NUMERIC(10,2) NOT NULL DEFAULT 40.00 CHECK (presupuesto_mensual_usd BETWEEN 1 AND 1000),
    updated_by              BIGINT        REFERENCES usuarios(id),
    updated_at              TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_ia_configuracion_mes CHECK (limite_usuario_dia <= limite_usuario_mes)
);
INSERT INTO ia_configuracion (id) VALUES (1);

CREATE TABLE ia_cuotas (
    clave          TEXT        NOT NULL CHECK (clave LIKE 'ia_cuota:%'),
    ventana        VARCHAR(3)  NOT NULL CHECK (ventana IN ('dia', 'mes')),
    ventana_inicio TIMESTAMPTZ NOT NULL,
    usados         INT         NOT NULL CHECK (usados >= 0),
    expira_en      TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (clave, ventana, ventana_inicio)
);
CREATE INDEX idx_ia_cuotas_expira ON ia_cuotas (expira_en);

CREATE TABLE ia_gasto_mes (
    mes                       DATE          PRIMARY KEY CHECK (EXTRACT(DAY FROM mes) = 1),
    gastado_usd               NUMERIC(12,6) NOT NULL DEFAULT 0 CHECK (gastado_usd >= 0),
    alerta_80_presupuesto_usd NUMERIC(10,2),
    alerta_80_at              TIMESTAMPTZ,
    updated_at                TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_ia_gasto_mes_alerta CHECK ((alerta_80_presupuesto_usd IS NULL) = (alerta_80_at IS NULL))
);

CREATE TABLE ia_cache (
    clave       CHAR(64)    PRIMARY KEY,
    operacion   VARCHAR(30) NOT NULL CHECK (operacion IN
                ('clasificar', 'generar_preguntas', 'generar_material', 'generar_explicacion')),
    salida      JSONB       NOT NULL,
    fuente_ids  BIGINT[]    NOT NULL DEFAULT '{}',
    vigente_ids BIGINT[]    NOT NULL DEFAULT '{}',
    expira_en   TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_ia_cache_fuentes CHECK (cardinality(fuente_ids) = cardinality(vigente_ids))
);
CREATE INDEX idx_ia_cache_expira ON ia_cache (expira_en);

ALTER TABLE ia_usos DROP CONSTRAINT ia_usos_resultado_check;
ALTER TABLE ia_usos ADD CONSTRAINT ck_ia_usos_resultado CHECK (resultado IN ('ok', 'no_disponible',
    'entrada_demasiado_grande', 'datos_personales', 'timeout', 'cancelado', 'salida_invalida',
    'error_proveedor', 'presupuesto_agotado'));

-- +goose Down
UPDATE ia_usos SET resultado = 'no_disponible' WHERE resultado = 'presupuesto_agotado';
ALTER TABLE ia_usos DROP CONSTRAINT IF EXISTS ck_ia_usos_resultado;
ALTER TABLE ia_usos ADD CONSTRAINT ia_usos_resultado_check CHECK (resultado IN ('ok', 'no_disponible',
    'entrada_demasiado_grande', 'datos_personales', 'timeout', 'cancelado', 'salida_invalida', 'error_proveedor'));
DROP TABLE IF EXISTS ia_cache;
DROP TABLE IF EXISTS ia_gasto_mes;
DROP TABLE IF EXISTS ia_cuotas;
DROP TABLE IF EXISTS ia_configuracion;
