-- +goose Up
ALTER TABLE instituciones ADD CONSTRAINT instituciones_siglas_key UNIQUE (siglas);
ALTER TABLE grados ADD CONSTRAINT grados_inst_nombre_key UNIQUE (institucion_id, nombre);
ALTER TABLE grados ADD CONSTRAINT grados_inst_orden_key  UNIQUE (institucion_id, orden);
ALTER TABLE grados ADD CONSTRAINT grados_nivel_check     CHECK  (nivel IN ('suboficial', 'oficial'));
ALTER TABLE especialidades ADD CONSTRAINT especialidades_inst_codigo_key UNIQUE (institucion_id, codigo);

-- +goose Down
ALTER TABLE especialidades DROP CONSTRAINT IF EXISTS especialidades_inst_codigo_key;
ALTER TABLE grados         DROP CONSTRAINT IF EXISTS grados_nivel_check;
ALTER TABLE grados         DROP CONSTRAINT IF EXISTS grados_inst_orden_key;
ALTER TABLE grados         DROP CONSTRAINT IF EXISTS grados_inst_nombre_key;
ALTER TABLE instituciones  DROP CONSTRAINT IF EXISTS instituciones_siglas_key;
