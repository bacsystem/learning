-- +goose Up
CREATE TABLE testimonios (
  id           BIGSERIAL PRIMARY KEY,
  usuario_id   BIGINT NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
  contenido    TEXT NOT NULL CHECK (char_length(contenido) BETWEEN 10 AND 300),
  calificacion SMALLINT NOT NULL CHECK (calificacion BETWEEN 1 AND 5),
  estado       VARCHAR(20) NOT NULL DEFAULT 'pendiente'
               CHECK (estado IN ('pendiente', 'aprobado', 'rechazado')),
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (usuario_id)
);

CREATE INDEX idx_testimonios_estado ON testimonios(estado);

-- +goose Down
DROP TABLE IF EXISTS testimonios;
