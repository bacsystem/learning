-- +goose Up
-- The frontend used to send destinatario 'rol:user', but usuarios.rol is
-- 'usuario' and readers match "rol:" || rol, so those notifications were never
-- shown to anyone. Rewrite them to the canonical value.
UPDATE notificaciones SET destinatario = 'rol:usuario' WHERE destinatario = 'rol:user';

-- +goose Down
-- Intentionally a no-op (H85): the Up is a data fix that cannot be told apart
-- afterwards — rewriting every 'rol:usuario' back to 'rol:user' would also hit
-- notifications created after the Up (by then the canonical value) and hide
-- them from every user again. 'rol:usuario' is valid for older code too.
SELECT 1;
