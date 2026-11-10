-- Contadores de intentos fallidos de registro, compartidos entre réplicas y
-- persistentes ante reinicios (antes vivían en memoria por proceso).
--
-- Ventana fija: ventana_inicio = NOW() truncado a múltiplos de la duración de
-- la ventana (reloj de Postgres, el mismo para todas las réplicas). La reserva
-- es un único INSERT ... ON CONFLICT DO UPDATE ... WHERE intentos < límite
-- RETURNING: el bloqueo de fila serializa los intentos concurrentes, así que
-- el límite no se puede saltar. El reembolso decrementa la misma
-- (clave, ventana_inicio) que se reservó.
--
--   - clave: siempre derivada de HMACs (dni_hmac y HMAC de la IP), nunca DNI
--     ni IP en claro. Lleva el prefijo del contador ("reg_dni_ip:", "reg_dni:").
--   - expira_en: ventana_inicio + duración; las filas vencidas se borran de
--     forma perezosa (como mucho una vez por minuto por proceso).

-- +goose Up
CREATE TABLE auth_intentos_fallidos (
    clave          TEXT NOT NULL,
    ventana_inicio TIMESTAMPTZ NOT NULL,
    intentos       INT NOT NULL CHECK (intentos >= 0),
    expira_en      TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (clave, ventana_inicio)
);

CREATE INDEX idx_auth_intentos_fallidos_expira ON auth_intentos_fallidos(expira_en);

-- +goose Down
DROP TABLE auth_intentos_fallidos;
