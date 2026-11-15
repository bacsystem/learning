package inscripciones

import (
	"testing"
	"time"
)

func TestToAdminDTOs_DatesAreRFC3339(t *testing.T) {
	lima := time.FixedZone("PET", -5*60*60)
	created := time.Date(2026, 11, 15, 0, 49, 7, 0, lima)
	reviewed := time.Date(2026, 11, 16, 9, 30, 0, 0, time.UTC)

	tests := []struct {
		name         string
		row          InscripcionDetail
		wantSolicitu string
		wantRevisado *string
	}{
		{
			name:         "pending has no revisado_at",
			row:          InscripcionDetail{CreatedAt: created},
			wantSolicitu: "2026-11-15T00:49:07-05:00",
		},
		{
			name:         "decided keeps instant and offset",
			row:          InscripcionDetail{CreatedAt: created, RevisadoAt: &reviewed},
			wantSolicitu: "2026-11-15T00:49:07-05:00",
			wantRevisado: ptr("2026-11-16T09:30:00Z"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := toAdminDTOs([]InscripcionDetail{tc.row})[0]
			if got.FechaSolicitud != tc.wantSolicitu {
				t.Errorf("fecha_solicitud = %q, want %q", got.FechaSolicitud, tc.wantSolicitu)
			}
			if (got.RevisadoAt == nil) != (tc.wantRevisado == nil) ||
				(got.RevisadoAt != nil && *got.RevisadoAt != *tc.wantRevisado) {
				t.Errorf("revisado_at = %v, want %v", got.RevisadoAt, tc.wantRevisado)
			}
			if _, err := time.Parse(time.RFC3339, got.FechaSolicitud); err != nil {
				t.Errorf("fecha_solicitud not RFC 3339: %v", err)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }
