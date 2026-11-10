-- +goose Up
CREATE TABLE simulacro_resultados (
    id              BIGSERIAL PRIMARY KEY,
    simulacro_id    BIGINT NOT NULL REFERENCES simulacros(id) ON DELETE CASCADE,
    asignatura_id   BIGINT NOT NULL REFERENCES asignaturas(id),
    total_preguntas INT NOT NULL,
    correctas       INT NOT NULL DEFAULT 0,
    incorrectas     INT NOT NULL DEFAULT 0,
    sin_responder   INT NOT NULL DEFAULT 0,
    puntaje         DECIMAL(8,2) NOT NULL DEFAULT 0,
    peso            DECIMAL(5,2) NOT NULL,
    UNIQUE(simulacro_id, asignatura_id)
);

-- +goose Down
DROP TABLE IF EXISTS simulacro_resultados;
