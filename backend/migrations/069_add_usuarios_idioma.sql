-- PLT-11: idioma guardado del usuario (docs/api/plataforma-avisos-idioma.md §1.1; ADR 0011).
-- Expand: el DEFAULT constante no reescribe la tabla y deja a los usuarios
-- existentes en 'es'; el INSERT del código viejo sigue funcionando. El CHECK
-- recorre la tabla (pequeña) bajo el bloqueo del ALTER; goose corre todo en una
-- sola transacción, así que no se intenta evitarlo.
-- Despliegue: migrar justo antes de arrancar el binario nuevo. El binario previo
-- lee usuarios con SELECT *, que falla con la columna nueva; el nuevo ya no.
-- +goose Up
ALTER TABLE usuarios ADD COLUMN idioma VARCHAR(2) NOT NULL DEFAULT 'es';
ALTER TABLE usuarios ADD CONSTRAINT usuarios_idioma_check CHECK (idioma IN ('es', 'en'));

-- +goose Down
ALTER TABLE usuarios DROP CONSTRAINT IF EXISTS usuarios_idioma_check;
ALTER TABLE usuarios DROP COLUMN IF EXISTS idioma;
