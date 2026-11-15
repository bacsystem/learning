package questionstest

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// ProcesoDeOtraInstitucion creates a second institución with an active proceso
// and an approved inscripción of the fixture's main user (a cross-institution
// inscripción as they existed before PLT-06) and returns the proceso id.
func (f *Fixture) ProcesoDeOtraInstitucion(t *testing.T) int64 {
	t.Helper()
	ctx := context.Background()
	var instID, procesoID int64
	must(t, f.DB.QueryRowContext(ctx,
		`INSERT INTO instituciones (nombre, siglas) VALUES ($1, $2) RETURNING id`,
		"Otra "+f.Suffix, fmt.Sprintf("O%x", time.Now().UnixNano()+seq.Add(1))).Scan(&instID), "otra institucion")
	t.Cleanup(func() {
		for _, s := range []string{
			`DELETE FROM inscripciones WHERE proceso_id = $1`,
			`DELETE FROM procesos WHERE id = $1`,
		} {
			if _, err := f.DB.ExecContext(ctx, s, procesoID); err != nil {
				t.Errorf("cleanup %q: %v", s, err)
			}
		}
		if _, err := f.DB.ExecContext(ctx, `DELETE FROM instituciones WHERE id = $1`, instID); err != nil {
			t.Errorf("cleanup otra institucion: %v", err)
		}
	})
	must(t, f.DB.QueryRowContext(ctx,
		`INSERT INTO procesos (institucion_id, nombre, tipo, anio) VALUES ($1, $2, 'ascenso', 2026) RETURNING id`,
		instID, "Proceso otra "+f.Suffix).Scan(&procesoID), "proceso de otra institucion")
	_, err := f.DB.ExecContext(ctx,
		`INSERT INTO inscripciones (usuario_id, proceso_id, estado) VALUES ($1, $2, 'aprobado')`, f.UsuarioID, procesoID)
	must(t, err, "inscripcion cruzada")
	return procesoID
}
