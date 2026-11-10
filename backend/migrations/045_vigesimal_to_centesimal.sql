-- +goose Up

-- NOTA (H42): la versión anterior de este Up hacía ALTER a DECIMAL(5,2) y
-- después ×5. Para datos DECIMAL(4,2) el resultado es idéntico; solo cambia
-- el orden (NUMERIC sin escala → ×5 → DECIMAL(5,2)) para
-- que los valores con 4 decimales que deja el Down (p.ej. 11.0660) no se
-- redondeen antes de multiplicar, de modo que up(down(x)) == x.

-- NUMERIC sin límite: ni redondea ni desborda durante el ×5
ALTER TABLE simulacros ALTER COLUMN nota_vigesimal TYPE NUMERIC;
ALTER TABLE configuracion_examen ALTER COLUMN nota_minima TYPE NUMERIC;

-- Convertir valores existentes (×5: escala 20 → 100)
UPDATE simulacros SET nota_vigesimal = ROUND(nota_vigesimal * 5, 2) WHERE nota_vigesimal IS NOT NULL;
UPDATE configuracion_examen SET nota_minima = ROUND(nota_minima * 5, 2);

-- Precisión final para soportar 100.00
ALTER TABLE simulacros ALTER COLUMN nota_vigesimal TYPE DECIMAL(5,2);
ALTER TABLE configuracion_examen ALTER COLUMN nota_minima TYPE DECIMAL(5,2);

-- Renombrar columna
ALTER TABLE simulacros RENAME COLUMN nota_vigesimal TO nota_centesimal;

-- Actualizar default de nota_minima en schema
ALTER TABLE configuracion_examen ALTER COLUMN nota_minima SET DEFAULT 55.0;

-- +goose Down
-- Sin pérdida: x.xx / 5 tiene como mucho 3 decimales, así que se guarda en
-- DECIMAL(7,4) (no en el DECIMAL(4,2) original) sin redondear. Así 55.33 →
-- 11.0660 → 55.33 en un nuevo Up (antes: 11.07 → 55.35).
ALTER TABLE configuracion_examen ALTER COLUMN nota_minima SET DEFAULT 11.0;
ALTER TABLE simulacros RENAME COLUMN nota_centesimal TO nota_vigesimal;
ALTER TABLE simulacros ALTER COLUMN nota_vigesimal TYPE DECIMAL(7,4);
ALTER TABLE configuracion_examen ALTER COLUMN nota_minima TYPE DECIMAL(7,4);
UPDATE configuracion_examen SET nota_minima = nota_minima / 5;
UPDATE simulacros SET nota_vigesimal = nota_vigesimal / 5 WHERE nota_vigesimal IS NOT NULL;
