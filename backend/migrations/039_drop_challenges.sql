-- +goose Up
-- Drop challenges module tables in reverse dependency order
DROP TABLE IF EXISTS usuario_insignias;
DROP TABLE IF EXISTS insignias;
DROP TABLE IF EXISTS usuario_gamificacion;
DROP TABLE IF EXISTS reto_intento_respuestas;
DROP TABLE IF EXISTS reto_intentos;
DROP TABLE IF EXISTS reto_preguntas;
DROP TABLE IF EXISTS retos;

-- +goose Down
-- IRREVERSIBLE (no-op a propósito). Las tablas del módulo de retos/gamificación
-- tenían FKs a preguntas/respuestas, pero las migraciones que las creaban
-- (028-033) fueron eliminadas del repositorio. Recrearlas aquí dejaba objetos
-- huérfanos que dependen de preguntas/respuestas y hacían fallar el Down de
-- 015/014 (SQLSTATE 2BP01), rompiendo `goose down-to 0`. Los datos borrados por
-- el Up no se pueden recuperar con un Down; si se necesitan, restaurar backup.
SELECT 1;
