-- PLT-11: idioma guardado del usuario (docs/api/plataforma-avisos-idioma.md §1.1; ADR 0011).
-- Expand seguro: el DEFAULT constante no reescribe la tabla y deja a los usuarios
-- existentes en 'es'; el código viejo, que no conoce la columna, sigue insertando.
-- El CHECK se crea NOT VALID y se valida aparte para no mantener el bloqueo fuerte
-- durante el escaneo de la tabla.
-- +goose Up
ALTER TABLE usuarios ADD COLUMN idioma VARCHAR(2) NOT NULL DEFAULT 'es';
ALTER TABLE usuarios ADD CONSTRAINT usuarios_idioma_check CHECK (idioma IN ('es', 'en')) NOT VALID;
ALTER TABLE usuarios VALIDATE CONSTRAINT usuarios_idioma_check;

-- +goose Down
ALTER TABLE usuarios DROP CONSTRAINT IF EXISTS usuarios_idioma_check;
ALTER TABLE usuarios DROP COLUMN IF EXISTS idioma;
