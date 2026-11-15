package inscripciones

import (
	"fmt"
	"html"

	"github.com/cbaciliod/learning/internal/correo"
	"github.com/cbaciliod/learning/internal/locale"
)

// Decisions an admin can take on a pendiente inscripción.
const (
	estadoAprobado  = "aprobado"
	estadoRechazado = "rechazado"
)

// Keys of the personal notification; the bell translates them (PLT-12).
const (
	claveAprobada  = "inscripcion_aprobada"
	claveRechazada = "inscripcion_rechazada"
)

// Column limits: email_queue.subject and notificaciones.titulo.
const (
	maxAsunto = 255
	maxTitulo = 200
)

// datosAviso is what the notice of a decision needs, read in the decision's
// transaction. Motivo is the admin's observación, as written.
type datosAviso struct {
	UsuarioID     int64
	InscripcionID int64
	Idioma        string
	Email         string
	Nombres       string
	Proceso       string
	Institucion   string
	Estado        string
	Motivo        string
}

// aviso is the rendered notice: the e-mail and the in-app notification, in the
// user's language.
type aviso struct {
	Asunto     string
	CorreoHTML string
	Clave      string
	Titulo     string
	Cuerpo     string
	Parametros map[string]string
}

// textos are the words of one notice in one language. The wording is neutral:
// the inscripción is the app's, not a procedure before the institución. asunto
// takes the proceso; intro takes the proceso and the institución.
type textos struct {
	asunto, titulo, saludo, intro, boton, motivo string
}

func textosDe(loc, estado string) textos {
	en := loc == locale.EN
	switch {
	case en && estado == estadoAprobado:
		return textos{"Your enrollment was approved: %s", "Enrollment approved", "Hello, %s",
			"Your enrollment in the practice of the process %s (%s) was approved. You can now practice and take mock exams.",
			"Go to the processes", "Reason"}
	case en:
		return textos{"Your enrollment was rejected: %s", "Enrollment rejected", "Hello, %s",
			"Your enrollment in the practice of the process %s (%s) was rejected.",
			"Go to the processes", "Reason"}
	case estado == estadoAprobado:
		return textos{"Tu inscripción fue aprobada: %s", "Inscripción aprobada", "Hola, %s",
			"Tu inscripción a la práctica del proceso %s (%s) fue aprobada. Ya puedes practicar y rendir simulacros.",
			"Ir a los procesos", "Motivo"}
	default:
		return textos{"Tu inscripción fue rechazada: %s", "Inscripción rechazada", "Hola, %s",
			"Tu inscripción a la práctica del proceso %s (%s) fue rechazada.",
			"Ir a los procesos", "Motivo"}
	}
}

// renderAviso renders the notice of a decision. An absent or unknown idioma
// falls back to Spanish. Every value from the database is escaped before it
// reaches the HTML (H130); the motivo is data, never markup.
func renderAviso(d datosAviso, frontendBaseURL string) aviso {
	loc := locale.Normalize(d.Idioma)
	tx := textosDe(loc, d.Estado)
	conMotivo := d.Estado == estadoRechazado && d.Motivo != ""

	clave := claveAprobada
	if d.Estado == estadoRechazado {
		clave = claveRechazada
	}
	params := map[string]string{"proceso": d.Proceso, "institucion": d.Institucion}
	cuerpo := fmt.Sprintf(tx.intro, d.Proceso, d.Institucion)
	contenido := fmt.Sprintf(`
    <h1 style="margin:0 0 8px;font-size:24px;font-weight:700;color:#18181b;">%s</h1>
    <p style="margin:0 0 20px;font-size:15px;color:#52525b;">%s</p>`,
		html.EscapeString(fmt.Sprintf(tx.saludo, d.Nombres)),
		fmt.Sprintf(tx.intro, "<strong>"+html.EscapeString(d.Proceso)+"</strong>", html.EscapeString(d.Institucion)))
	if conMotivo {
		params["motivo"] = d.Motivo
		cuerpo += " " + tx.motivo + ": " + d.Motivo
		contenido += correo.InfoBox("#fef2f2", "#dc2626",
			"<strong>"+tx.motivo+":</strong> "+html.EscapeString(d.Motivo))
	}
	contenido += correo.BtnPrimary(loc, tx.boton, frontendBaseURL+"/procesos")

	asunto := recortar(fmt.Sprintf(tx.asunto, d.Proceso), maxAsunto)
	return aviso{
		Asunto:     asunto,
		CorreoHTML: correo.Wrap(loc, html.EscapeString(asunto), contenido),
		Clave:      clave,
		Titulo:     recortar(tx.titulo, maxTitulo),
		Cuerpo:     cuerpo,
		Parametros: params,
	}
}

// recortar cuts s to at most max characters without splitting one.
func recortar(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
