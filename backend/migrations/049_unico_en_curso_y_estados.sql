-- +goose Up

-- H10: como máximo un simulacro y una práctica 'en_curso' por usuario.
--
-- Duplicados existentes (creados por POSTs concurrentes): se conserva el más
-- reciente (created_at, id) y los demás se cierran con un estado terminal que el
-- código ya maneja:
--   simulacros → 'completado' (sin nota: puntaje/nota quedan NULL, la UI los
--                muestra como 0), fecha_fin acotada al tiempo límite.
--   practicas  → 'abandonada' (el mismo estado que usa "abandonar").

UPDATE simulacros s
SET estado    = 'completado',
    fecha_fin = LEAST(NOW(), s.fecha_inicio + make_interval(secs => s.tiempo_limite_seg))
WHERE s.estado = 'en_curso'
  AND EXISTS (SELECT 1 FROM simulacros o
              WHERE o.usuario_id = s.usuario_id
                AND o.estado = 'en_curso'
                AND (o.created_at, o.id) > (s.created_at, s.id));

UPDATE practicas p
SET estado = 'abandonada', fecha_fin = NOW()
WHERE p.estado = 'en_curso'
  AND EXISTS (SELECT 1 FROM practicas o
              WHERE o.usuario_id = p.usuario_id
                AND o.estado = 'en_curso'
                AND (o.created_at, o.id) > (p.created_at, p.id));

CREATE UNIQUE INDEX uq_simulacros_usuario_en_curso
    ON simulacros (usuario_id) WHERE estado = 'en_curso';

CREATE UNIQUE INDEX uq_practicas_usuario_en_curso
    ON practicas (usuario_id) WHERE estado = 'en_curso';

-- Estados que escribe el código (practicas ya tiene su CHECK desde 018).
-- H94: antes el CHECK se creaba NOT VALID y se saltaba la validación si había
-- estados desconocidos, pero 050 hace UPDATE de todas las filas (re-comprueba
-- el CHECK) y abortaba. Ahora las filas históricas con un estado que el código
-- no conoce (p.ej. 'abandonado') se cierran como 'completado' sin nota (igual
-- que los duplicados de arriba; fecha_fin se conserva o se acota al tiempo
-- límite) y el CHECK se crea validado. Se editó el Up de una migración ya
-- escrita porque aún no hay producción.
UPDATE simulacros s
SET estado    = 'completado',
    fecha_fin = COALESCE(s.fecha_fin,
                         LEAST(NOW(), s.fecha_inicio + make_interval(secs => s.tiempo_limite_seg)))
WHERE s.estado NOT IN ('en_curso', 'completado');

ALTER TABLE simulacros
    ADD CONSTRAINT simulacros_estado_check
    CHECK (estado IN ('en_curso', 'completado'));

-- +goose Down
ALTER TABLE simulacros DROP CONSTRAINT IF EXISTS simulacros_estado_check;
DROP INDEX IF EXISTS uq_practicas_usuario_en_curso;
DROP INDEX IF EXISTS uq_simulacros_usuario_en_curso;
