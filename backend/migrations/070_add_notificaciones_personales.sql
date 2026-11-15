-- PLT-12: notificaciones personales (docs/api/plataforma-avisos-idioma.md §2.3; ADR 0011).
-- Expand: columnas nuevas NULL. Las filas existentes ('todos' o 'rol:*') tienen
-- usuario_id NULL y cumplen los CHECK, y el binario anterior no las muestra a los
-- postulantes (su destinatario es 'usuario'): es compatible hacia atrás. Salvedad:
-- el ListAll y el Delete del admin del binario anterior no filtran, así que en la
-- ventana de despliegue el panel admin puede listar y borrar las personales.
-- +goose Up
ALTER TABLE notificaciones ADD COLUMN usuario_id BIGINT REFERENCES usuarios(id) ON DELETE CASCADE;
ALTER TABLE notificaciones ADD COLUMN clave VARCHAR(60);
ALTER TABLE notificaciones ADD COLUMN parametros JSONB;
ALTER TABLE notificaciones ADD CONSTRAINT notificaciones_personal_check
    CHECK ((destinatario = 'usuario') = (usuario_id IS NOT NULL));
ALTER TABLE notificaciones ADD CONSTRAINT notificaciones_parametros_check
    CHECK (parametros IS NULL OR clave IS NOT NULL);
CREATE INDEX idx_notificaciones_usuario ON notificaciones (usuario_id, created_at DESC)
    WHERE usuario_id IS NOT NULL;

-- +goose Down
-- Sin las columnas, las personales quedarían como filas invisibles con un
-- destinatario que nadie lee: se borran primero (los avisos se pierden).
DELETE FROM notificaciones WHERE usuario_id IS NOT NULL;
DROP INDEX IF EXISTS idx_notificaciones_usuario;
ALTER TABLE notificaciones DROP CONSTRAINT IF EXISTS notificaciones_parametros_check;
ALTER TABLE notificaciones DROP CONSTRAINT IF EXISTS notificaciones_personal_check;
ALTER TABLE notificaciones DROP COLUMN IF EXISTS parametros;
ALTER TABLE notificaciones DROP COLUMN IF EXISTS clave;
ALTER TABLE notificaciones DROP COLUMN IF EXISTS usuario_id;
