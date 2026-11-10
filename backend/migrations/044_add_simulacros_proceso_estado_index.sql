-- +goose Up

-- computePosicion queries simulacros WHERE proceso_id=$1 AND estado='completado'
CREATE INDEX idx_simulacros_proceso_estado ON simulacros(proceso_id, estado);

-- +goose Down
DROP INDEX IF EXISTS idx_simulacros_proceso_estado;
