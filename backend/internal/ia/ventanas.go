package ia

import "time"

// zonaNegocio is the business timezone: days and months of the quota run in
// America/Lima (contract E6-02 §2.1). It needs the tz database, which
// cmd/server embeds with time/tzdata.
const zonaNegocio = "America/Lima"

// Ventanas of the quota.
const (
	VentanaDia = "dia"
	VentanaMes = "mes"
)

// ventana is one calendar window in Lima: [Inicio, Fin).
type ventana struct {
	Inicio time.Time
	Fin    time.Time // the start of the next window: reinicia_at and expira_en
}

// ventanas returns the day and the month of Lima that contain now.
func ventanas(now time.Time, lima *time.Location) (dia, mes ventana) {
	l := now.In(lima)
	d := time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, lima)
	m := time.Date(l.Year(), l.Month(), 1, 0, 0, 0, 0, lima)
	return ventana{d.UTC(), d.AddDate(0, 0, 1).UTC()}, ventana{m.UTC(), m.AddDate(0, 1, 0).UTC()}
}
