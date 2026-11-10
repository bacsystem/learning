-- +goose Up

-- practica_respuestas: al borrar una respuesta → SET NULL (nullable)
ALTER TABLE practica_respuestas
    DROP CONSTRAINT practica_respuestas_respuesta_id_fkey,
    ADD  CONSTRAINT practica_respuestas_respuesta_id_fkey
         FOREIGN KEY (respuesta_id) REFERENCES respuestas(id) ON DELETE SET NULL;

-- practica_respuestas: al borrar una pregunta → CASCADE (elimina el registro)
ALTER TABLE practica_respuestas
    DROP CONSTRAINT practica_respuestas_pregunta_id_fkey,
    ADD  CONSTRAINT practica_respuestas_pregunta_id_fkey
         FOREIGN KEY (pregunta_id) REFERENCES preguntas(id) ON DELETE CASCADE;

-- simulacro_respuestas: al borrar una respuesta → SET NULL (nullable)
ALTER TABLE simulacro_respuestas
    DROP CONSTRAINT simulacro_respuestas_respuesta_id_fkey,
    ADD  CONSTRAINT simulacro_respuestas_respuesta_id_fkey
         FOREIGN KEY (respuesta_id) REFERENCES respuestas(id) ON DELETE SET NULL;

-- simulacro_respuestas: al borrar una pregunta → CASCADE (elimina el registro)
ALTER TABLE simulacro_respuestas
    DROP CONSTRAINT simulacro_respuestas_pregunta_id_fkey,
    ADD  CONSTRAINT simulacro_respuestas_pregunta_id_fkey
         FOREIGN KEY (pregunta_id) REFERENCES preguntas(id) ON DELETE CASCADE;

-- +goose Down

ALTER TABLE practica_respuestas
    DROP CONSTRAINT practica_respuestas_respuesta_id_fkey,
    ADD  CONSTRAINT practica_respuestas_respuesta_id_fkey
         FOREIGN KEY (respuesta_id) REFERENCES respuestas(id);

ALTER TABLE practica_respuestas
    DROP CONSTRAINT practica_respuestas_pregunta_id_fkey,
    ADD  CONSTRAINT practica_respuestas_pregunta_id_fkey
         FOREIGN KEY (pregunta_id) REFERENCES preguntas(id);

ALTER TABLE simulacro_respuestas
    DROP CONSTRAINT simulacro_respuestas_respuesta_id_fkey,
    ADD  CONSTRAINT simulacro_respuestas_respuesta_id_fkey
         FOREIGN KEY (respuesta_id) REFERENCES respuestas(id);

ALTER TABLE simulacro_respuestas
    DROP CONSTRAINT simulacro_respuestas_pregunta_id_fkey,
    ADD  CONSTRAINT simulacro_respuestas_pregunta_id_fkey
         FOREIGN KEY (pregunta_id) REFERENCES preguntas(id);
