-- SIA-E2-02: fuentes, versiones y pasajes del repositorio propio
-- (docs/api/serums-ia-e2-fuentes.md § Modelo de datos; ADR 0003).
--
-- Efecto sobre los datos existentes: ninguno; son tablas nuevas.
-- Qué pierde el Down: todas las fuentes, sus versiones y pasajes. Una
-- migración posterior con FK a pasajes (E3-02) se revierte antes por el orden
-- de goose.
--
-- pasajes.tsv = to_tsvector('spanish', texto.Plegar(texto)), calculado al
-- publicar; el texto original se guarda sin plegar y es el que se cita.

-- +goose Up
CREATE TABLE fuentes (
    id                 BIGSERIAL    PRIMARY KEY,
    titulo             VARCHAR(150) NOT NULL CHECK (char_length(titulo) BETWEEN 3 AND 150),
    tipo               VARCHAR(20)  NOT NULL CHECK (tipo IN ('balotario', 'guia', 'norma', 'otro')),
    estado             VARCHAR(20)  NOT NULL DEFAULT 'procesando'
                       CHECK (estado IN ('procesando', 'activa', 'fallida', 'retirada')),
    anio               INT          CHECK (anio BETWEEN 1900 AND 2100),
    proceso_id         BIGINT       REFERENCES procesos(id),
    version_vigente_id BIGINT,
    motivo_retiro      VARCHAR(500),
    creado_por         BIGINT       REFERENCES usuarios(id),
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_fuentes_vigente CHECK ((estado IN ('activa', 'retirada')) = (version_vigente_id IS NOT NULL)),
    CONSTRAINT ck_fuentes_motivo  CHECK ((estado = 'retirada') = (motivo_retiro IS NOT NULL))
);
CREATE INDEX idx_fuentes_estado_updated ON fuentes (estado, updated_at DESC);
CREATE INDEX idx_fuentes_proceso ON fuentes (proceso_id) WHERE proceso_id IS NOT NULL;

CREATE TABLE fuente_especialidades (
    fuente_id       BIGINT NOT NULL REFERENCES fuentes(id) ON DELETE CASCADE,
    especialidad_id BIGINT NOT NULL REFERENCES especialidades(id),
    PRIMARY KEY (fuente_id, especialidad_id)
);
CREATE INDEX idx_fuente_especialidades_especialidad ON fuente_especialidades (especialidad_id);

CREATE TABLE fuente_versiones (
    id                   BIGSERIAL    PRIMARY KEY,
    fuente_id            BIGINT       NOT NULL REFERENCES fuentes(id),
    numero               INT          NOT NULL CHECK (numero >= 1),
    estado               VARCHAR(20)  NOT NULL DEFAULT 'procesando'
                         CHECK (estado IN ('procesando', 'lista', 'fallida', 'archivada')),
    motivo_fallo         VARCHAR(30)  CHECK (motivo_fallo IN ('sin_contenido', 'limite_pasajes', 'error_interno')),
    nombre_archivo       VARCHAR(255) NOT NULL,
    sha256               CHAR(64)     NOT NULL,
    tamano_bytes         INT          NOT NULL CHECK (tamano_bytes BETWEEN 0 AND 2097152),
    lineas               INT          NOT NULL CHECK (lineas BETWEEN 0 AND 20000),
    contenido            TEXT,        -- original tal como se subió (UTF-8); el worker lo normaliza
    normalizador_version SMALLINT     NOT NULL DEFAULT 1,
    pasajes_count        INT          NOT NULL DEFAULT 0 CHECK (pasajes_count BETWEEN 0 AND 5000),
    intentos             INT          NOT NULL DEFAULT 0,
    claimed_at           TIMESTAMPTZ,
    oficial_verificada   BOOLEAN      NOT NULL DEFAULT false,
    oficial_entidad      VARCHAR(100),
    oficial_nota         VARCHAR(500),
    oficial_por          BIGINT       REFERENCES usuarios(id),
    oficial_at           TIMESTAMPTZ,
    creado_por           BIGINT       REFERENCES usuarios(id),
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    procesada_at         TIMESTAMPTZ,
    CONSTRAINT uq_fuente_versiones_numero    UNIQUE (fuente_id, numero),
    CONSTRAINT uq_fuente_versiones_id_fuente UNIQUE (id, fuente_id),        -- destino de la FK compuesta
    CONSTRAINT ck_fuente_versiones_contenido CHECK ((estado = 'archivada') = (contenido IS NULL)),
    CONSTRAINT ck_fuente_versiones_fallo     CHECK ((estado = 'fallida') = (motivo_fallo IS NOT NULL)),
    CONSTRAINT ck_fuente_versiones_oficial   CHECK (oficial_verificada =
        (oficial_entidad IS NOT NULL AND oficial_nota IS NOT NULL AND oficial_at IS NOT NULL))
);
-- Duplicado por hash (E2-03 regla 3): global entre todas las versiones de todas las fuentes, en cualquier estado.
CREATE UNIQUE INDEX uq_fuente_versiones_sha256 ON fuente_versiones (sha256);
-- Como máximo una versión en proceso por fuente; también es el índice del worker.
CREATE UNIQUE INDEX uq_fuente_versiones_una_procesando ON fuente_versiones (fuente_id) WHERE estado = 'procesando';
CREATE INDEX idx_fuente_versiones_oficial_por ON fuente_versiones (oficial_por) WHERE oficial_por IS NOT NULL;

-- La vigente pertenece a la misma fuente (MATCH SIMPLE: NULL no se comprueba).
ALTER TABLE fuentes ADD CONSTRAINT fk_fuentes_version_vigente
    FOREIGN KEY (version_vigente_id, id) REFERENCES fuente_versiones (id, fuente_id);

CREATE TABLE pasajes (
    id                BIGSERIAL    PRIMARY KEY,
    fuente_version_id BIGINT       NOT NULL REFERENCES fuente_versiones(id),
    orden             INT          NOT NULL CHECK (orden BETWEEN 1 AND 5000),
    linea_inicio      INT          NOT NULL CHECK (linea_inicio >= 1),
    linea_fin         INT          NOT NULL,
    seccion           VARCHAR(300),
    texto             TEXT         NOT NULL,
    palabras          INT          NOT NULL CHECK (palabras >= 1),
    tsv               TSVECTOR     NOT NULL,     -- to_tsvector('spanish', plegar(texto)), ver ADR 0003
    CONSTRAINT uq_pasajes_version_orden UNIQUE (fuente_version_id, orden),
    CONSTRAINT ck_pasajes_lineas CHECK (linea_fin >= linea_inicio)
);
CREATE INDEX idx_pasajes_tsv ON pasajes USING GIN (tsv);

-- +goose Down
ALTER TABLE fuentes DROP CONSTRAINT IF EXISTS fk_fuentes_version_vigente;
DROP TABLE IF EXISTS pasajes;
DROP TABLE IF EXISTS fuente_versiones;
DROP TABLE IF EXISTS fuente_especialidades;
DROP TABLE IF EXISTS fuentes;
