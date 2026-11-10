-- +goose Up

-- practica_asignaturas: sin índice en practica_id (592 seq_scans, 0 idx_scans)
CREATE INDEX idx_practica_asignaturas_practica ON practica_asignaturas(practica_id);

-- practicas: índice compuesto para GetPracticaEnCurso (WHERE usuario_id=$1 AND estado='en_curso')
CREATE INDEX idx_practicas_usuario_estado ON practicas(usuario_id, estado);

-- notificaciones: índice en destinatario+created_at para queries de listado
CREATE INDEX idx_notificaciones_destinatario ON notificaciones(destinatario, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_practica_asignaturas_practica;
DROP INDEX IF EXISTS idx_practicas_usuario_estado;
DROP INDEX IF EXISTS idx_notificaciones_destinatario;
