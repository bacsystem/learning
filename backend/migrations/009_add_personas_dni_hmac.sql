-- +goose Up

-- H60: la versión original hacía `TRUNCATE personas CASCADE` para descartar
-- filas sembradas en texto plano, lo que borraba en cascada usuarios,
-- sesiones, inscripciones y simulacros en cualquier `down-to` < 9 + `up`.
-- Se eliminó: en una base nueva la tabla está vacía en este punto, y las
-- personas se siembran ya cifradas (cmd/seed). Si quedaran filas en texto
-- plano de un entorno antiguo, se corrigen con cmd/fix-encrypt-personas.

DROP INDEX IF EXISTS idx_personas_dni;
ALTER TABLE personas ADD COLUMN dni_hmac TEXT NOT NULL DEFAULT '';
-- H98: si ya hay filas (p.ej. tras un down-to 8 + up), todas quedarían con ''
-- y el índice único fallaría con >= 2 personas. Se rellena un marcador único
-- por fila; cmd/fix-encrypt-personas recalcula el HMAC real (cualquier
-- dni_hmac que no coincida con HMAC(dni) se corrige). En una base nueva la
-- tabla está vacía y no hace nada. Editado sin producción.
UPDATE personas SET dni_hmac = 'legacy:' || id WHERE dni_hmac = '';
CREATE UNIQUE INDEX idx_personas_dni_hmac ON personas(dni_hmac);

-- +goose Down
-- No borra filas (no hay TRUNCATE en el Down). Se pierde dni_hmac: al volver
-- a subir, el Up pone el marcador 'legacy:<id>' y hay que recalcularlo con
-- cmd/fix-encrypt-personas (make fix-encrypt-personas) antes de usar la app.
-- Ojo: el índice idx_personas_dni que se recrea exige DNI únicos; con DNI
-- cifrados (nonce aleatorio) eso se cumple siempre.
DROP INDEX IF EXISTS idx_personas_dni_hmac;
ALTER TABLE personas DROP COLUMN IF EXISTS dni_hmac;
CREATE UNIQUE INDEX idx_personas_dni ON personas(dni);
