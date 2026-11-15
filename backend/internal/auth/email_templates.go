package auth

import (
	"fmt"
	"html"

	"github.com/cbaciliod/learning/internal/correo"
	"github.com/cbaciliod/learning/internal/locale"
)

// Every value that comes from the database (names, username) is passed
// through html.EscapeString before it reaches the HTML (H130).

// ── Cuenta activada ──────────────────────────────────────────────────────────

func emailActivado(loc, nombres, apellido, username, loginURL string) (subject, body string) {
	c := activadoCopy(loc)
	subject = c.subject
	content := fmt.Sprintf(`
    <h1 style="margin:0 0 8px;font-size:24px;font-weight:700;color:#18181b;">%s</h1>
    <p style="margin:0 0 20px;font-size:15px;color:#52525b;">%s</p>

    <table width="100%%" cellpadding="0" cellspacing="0" style="background:#f0f4ec;border-radius:8px;margin-bottom:8px;">
      <tr><td style="padding:20px 24px;">
        <p style="margin:0 0 4px;font-size:11px;font-weight:600;color:#71717a;text-transform:uppercase;letter-spacing:0.5px;">%s</p>
        <p style="margin:0;font-size:20px;font-weight:700;color:#4a5e3a;">@%s</p>
      </td></tr>
    </table>
    %s`,
		fmt.Sprintf(c.greeting, html.EscapeString(nombres+" "+apellido)), c.intro, c.usernameLabel,
		html.EscapeString(username), correo.BtnPrimary(loc, c.button, loginURL))
	body = correo.Wrap(loc, subject, content)
	return
}

type activadoTexts struct{ subject, greeting, intro, usernameLabel, button string }

func activadoCopy(loc string) activadoTexts {
	if loc == locale.EN {
		return activadoTexts{
			subject:       "Your account has been activated",
			greeting:      "Welcome, %s!",
			intro:         `Your account has been <strong style="color:#4a5e3a;">activated successfully</strong>. You can now access the platform and start your training.`,
			usernameLabel: "Assigned username",
			button:        "Sign in",
		}
	}
	return activadoTexts{
		subject:       "Tu cuenta ha sido activada",
		greeting:      "¡Bienvenido, %s!",
		intro:         `Tu cuenta ha sido <strong style="color:#4a5e3a;">activada exitosamente</strong>. Ya puedes acceder a la plataforma y comenzar tus capacitaciones.`,
		usernameLabel: "Usuario asignado",
		button:        "Iniciar sesión",
	}
}

// ── Cuenta rechazada ─────────────────────────────────────────────────────────

func emailRechazado(loc, nombres, apellido string) (subject, body string) {
	c := rechazadoCopy(loc)
	subject = c.subject
	content := fmt.Sprintf(`
    <h1 style="margin:0 0 8px;font-size:24px;font-weight:700;color:#18181b;">%s</h1>
    <p style="margin:0 0 4px;font-size:15px;color:#52525b;">%s</p>
    %s
    <p style="margin:16px 0 0;font-size:14px;color:#71717a;">%s</p>`,
		fmt.Sprintf(c.greeting, html.EscapeString(nombres+" "+apellido)), c.intro,
		correo.InfoBox("#fef2f2", "#dc2626", c.notice), c.footer)
	body = correo.Wrap(loc, subject, content)
	return
}

type rechazadoTexts struct{ subject, greeting, intro, notice, footer string }

func rechazadoCopy(loc string) rechazadoTexts {
	if loc == locale.EN {
		return rechazadoTexts{
			subject:  "Update on your account request",
			greeting: "Hello, %s",
			intro:    `We have reviewed your access request and unfortunately it <strong style="color:#dc2626;">could not be approved</strong> at this time.`,
			notice:   "If you think this is a mistake, contact the administration area of your institution.",
			footer:   "You can contact the administrative staff of your institution for more information or submit a new request.",
		}
	}
	return rechazadoTexts{
		subject:  "Actualización sobre tu solicitud de cuenta",
		greeting: "Hola, %s",
		intro:    `Hemos revisado tu solicitud de acceso y lamentablemente <strong style="color:#dc2626;">no pudo ser aprobada</strong> en este momento.`,
		notice:   "Si crees que esto es un error, comunícate con el área de administración de tu institución.",
		footer:   "Puedes contactar al personal administrativo de tu institución para obtener más información o presentar una nueva solicitud.",
	}
}

// ── Cuenta desactivada ───────────────────────────────────────────────────────

func emailInactivo(loc, nombres, apellido string) (subject, body string) {
	c := inactivoCopy(loc)
	subject = c.subject
	content := fmt.Sprintf(`
    <h1 style="margin:0 0 8px;font-size:24px;font-weight:700;color:#18181b;">%s</h1>
    <p style="margin:0 0 4px;font-size:15px;color:#52525b;">%s</p>
    %s
    <p style="margin:16px 0 0;font-size:14px;color:#71717a;">%s</p>`,
		fmt.Sprintf(c.greeting, html.EscapeString(nombres+" "+apellido)), c.intro,
		correo.InfoBox("#fffbeb", "#d97706", c.notice), c.footer)
	body = correo.Wrap(loc, subject, content)
	return
}

type inactivoTexts struct{ subject, greeting, intro, notice, footer string }

func inactivoCopy(loc string) inactivoTexts {
	if loc == locale.EN {
		return inactivoTexts{
			subject:  "Your account has been deactivated",
			greeting: "Hello, %s",
			intro:    `Your account has been <strong style="color:#d97706;">temporarily deactivated</strong> by the system administrator.`,
			notice:   "If you think this action was a mistake, contact the administration area of your institution.",
			footer:   "During this period you will not be able to access the training resources. Contact the administration area to request reactivation.",
		}
	}
	return inactivoTexts{
		subject:  "Tu cuenta ha sido desactivada",
		greeting: "Hola, %s",
		intro:    `Tu cuenta ha sido <strong style="color:#d97706;">desactivada temporalmente</strong> por el administrador del sistema.`,
		notice:   "Si consideras que esta acción fue un error, comunícate con el área de administración de tu institución.",
		footer:   "Durante este período no podrás acceder a los recursos de capacitación. Contacta con el área de administración para solicitar la reactivación.",
	}
}

// ── Registro pendiente (al usuario) ─────────────────────────────────────────

// emailRegistroPendiente confirms the request. contact is the support address
// shown for questions (config CONTACT_EMAIL); empty omits that sentence.
func emailRegistroPendiente(loc, contact, nombres, apellido, username string) (subject, body string) {
	c := registroPendienteCopy(loc)
	subject = c.subject
	content := fmt.Sprintf(`
    <h1 style="margin:0 0 8px;font-size:24px;font-weight:700;color:#18181b;">%s</h1>
    <p style="margin:0 0 20px;font-size:15px;color:#52525b;">%s</p>

    <table width="100%%" cellpadding="0" cellspacing="0" style="background:#f0f4ec;border-radius:8px;margin-bottom:20px;">
      <tr><td style="padding:20px 24px;">
        <p style="margin:0 0 4px;font-size:11px;font-weight:600;color:#71717a;text-transform:uppercase;letter-spacing:0.5px;">%s</p>
        <p style="margin:0;font-size:20px;font-weight:700;color:#4a5e3a;">@%s</p>
      </td></tr>
    </table>

    %s

    <p style="margin:20px 0 0;font-size:13px;color:#71717a;">%s</p>`,
		fmt.Sprintf(c.greeting, html.EscapeString(nombres+" "+apellido)), c.intro, c.usernameLabel,
		html.EscapeString(username), correo.InfoBox("#f0f9ff", "#0ea5e9", c.notice), c.footerWithContact(contact))
	body = correo.Wrap(loc, subject, content)
	return
}

type registroPendienteTexts struct{ subject, greeting, intro, usernameLabel, notice, footer, contactLine string }

// footerWithContact appends the contact sentence (its %s is the mailto link).
func (c registroPendienteTexts) footerWithContact(contact string) string {
	if contact == "" {
		return c.footer
	}
	link := fmt.Sprintf(`<a href="mailto:%[1]s" style="color:#4a5e3a;">%[1]s</a>`, html.EscapeString(contact))
	return c.footer + " " + fmt.Sprintf(c.contactLine, link)
}

func registroPendienteCopy(loc string) registroPendienteTexts {
	if loc == locale.EN {
		return registroPendienteTexts{
			subject:       "Account request received",
			greeting:      "Hello, %s",
			intro:         "We have received your request to access the training platform. Your account is being reviewed by the administrative team.",
			usernameLabel: "Requested username",
			notice:        "⏳ Your request is <strong>under review</strong>.",
			footer:        "You will receive another e-mail when your account is activated.",
			contactLine:   "If you have questions, write to %s.",
		}
	}
	return registroPendienteTexts{
		subject:       "Solicitud de cuenta recibida",
		greeting:      "Hola, %s",
		intro:         "Hemos recibido tu solicitud de acceso a la plataforma de capacitación. Tu cuenta está siendo revisada por el equipo administrativo.",
		usernameLabel: "Usuario solicitado",
		notice:        "⏳ Tu solicitud está <strong>en revisión</strong>.",
		footer:        "Recibirás otro correo cuando tu cuenta sea activada.",
		contactLine:   "Si tienes dudas, escribe a %s.",
	}
}

// ── Verificación de correo (registro) ───────────────────────────────────────

func emailVerificarRegistro(loc, nombres, apellido, verifyURL string) (subject, body string) {
	c := verificarRegistroCopy(loc)
	subject = c.subject
	content := fmt.Sprintf(`
    <h1 style="margin:0 0 8px;font-size:24px;font-weight:700;color:#18181b;">%s</h1>
    <p style="margin:0 0 4px;font-size:15px;color:#52525b;">%s</p>
    %s
    %s
    <p style="margin:20px 0 0;font-size:13px;color:#71717a;">%s</p>`,
		fmt.Sprintf(c.greeting, html.EscapeString(nombres+" "+apellido)), c.intro,
		correo.BtnPrimary(loc, c.button, verifyURL), correo.InfoBox("#fffbeb", "#d97706", c.notice), c.footer)
	body = correo.Wrap(loc, subject, content)
	return
}

type verificarRegistroTexts struct{ subject, greeting, intro, button, notice, footer string }

func verificarRegistroCopy(loc string) verificarRegistroTexts {
	if loc == locale.EN {
		return verificarRegistroTexts{
			subject:  "Confirm your registration",
			greeting: "Hello, %s",
			intro:    "We received a registration request on the training platform for your ID number. To confirm this e-mail is yours and choose your username and password, click the button.",
			button:   "Complete registration",
			notice:   "⚠ This link is valid for <strong>24 hours</strong> and can be used only once. If you register again, the previous link stops working.",
			footer:   "If you did not request this registration, ignore this message: no account will be created.",
		}
	}
	return verificarRegistroTexts{
		subject:  "Confirma tu registro",
		greeting: "Hola, %s",
		intro:    "Recibimos una solicitud de registro en la plataforma de capacitación para tu DNI. Para confirmar que este correo es tuyo y elegir tu usuario y contraseña, haz clic en el botón.",
		button:   "Completar registro",
		notice:   "⚠ Este enlace es válido por <strong>24 horas</strong> y solo puede usarse una vez. Si te registras de nuevo, el enlace anterior deja de funcionar.",
		footer:   "Si no solicitaste este registro, ignora este mensaje: no se creará ninguna cuenta.",
	}
}

// ── Recuperación de contraseña ───────────────────────────────────────────────

func emailOlvidePassword(loc, resetURL string) (subject, body string) {
	c := olvidePasswordCopy(loc)
	subject = c.subject
	content := fmt.Sprintf(`
    <h1 style="margin:0 0 8px;font-size:24px;font-weight:700;color:#18181b;">%s</h1>
    <p style="margin:0 0 4px;font-size:15px;color:#52525b;">%s</p>
    %s
    %s
    <p style="margin:20px 0 0;font-size:13px;color:#71717a;">%s</p>`,
		c.title, c.intro, correo.BtnPrimary(loc, c.button, resetURL), correo.InfoBox("#fffbeb", "#d97706", c.notice), c.footer)
	body = correo.Wrap(loc, subject, content)
	return
}

type olvidePasswordTexts struct{ subject, title, intro, button, notice, footer string }

func olvidePasswordCopy(loc string) olvidePasswordTexts {
	if loc == locale.EN {
		return olvidePasswordTexts{
			subject: "Password recovery",
			title:   "Reset your password",
			intro:   "We received a request to reset the password of your account. Click the button to create a new password.",
			button:  "Reset password",
			notice:  "⚠ This link is valid for <strong>1 hour</strong> and can be used only once.",
			footer:  "If you did not request a password reset, ignore this message. Your account remains safe.",
		}
	}
	return olvidePasswordTexts{
		subject: "Recuperación de contraseña",
		title:   "Restablecer contraseña",
		intro:   "Recibimos una solicitud para restablecer la contraseña de tu cuenta. Haz clic en el botón para crear una nueva contraseña.",
		button:  "Restablecer contraseña",
		notice:  "⚠ Este enlace es válido por <strong>1 hora</strong> y solo puede usarse una vez.",
		footer:  "Si no solicitaste restablecer tu contraseña, ignora este mensaje. Tu cuenta permanece segura.",
	}
}

// ── Contraseña restablecida (confirmación) ───────────────────────────────────

func emailPasswordRestablecida(loc, loginURL string) (subject, body string) {
	c := passwordRestablecidaCopy(loc)
	subject = c.subject
	content := fmt.Sprintf(`
    <h1 style="margin:0 0 8px;font-size:24px;font-weight:700;color:#18181b;">%s</h1>
    <p style="margin:0 0 20px;font-size:15px;color:#52525b;">%s</p>
    %s
    %s
    <p style="margin:20px 0 0;font-size:13px;color:#71717a;">%s</p>`,
		c.title, c.intro, correo.BtnPrimary(loc, c.button, loginURL), correo.InfoBox("#fef2f2", "#dc2626", c.notice), c.footer)
	body = correo.Wrap(loc, subject, content)
	return
}

type passwordRestablecidaTexts struct{ subject, title, intro, button, notice, footer string }

func passwordRestablecidaCopy(loc string) passwordRestablecidaTexts {
	if loc == locale.EN {
		return passwordRestablecidaTexts{
			subject: "Your password was reset",
			title:   "Password updated",
			intro:   "Your password has been reset successfully. You can now sign in with your new password.",
			button:  "Sign in",
			notice:  "🔒 If you did not change the password, your account may be compromised. Contact the administrator immediately.",
			footer:  "If you did not make this change, contact the system administrator immediately.",
		}
	}
	return passwordRestablecidaTexts{
		subject: "Tu contraseña fue restablecida",
		title:   "Contraseña actualizada",
		intro:   "Tu contraseña ha sido restablecida exitosamente. Ya puedes iniciar sesión con tu nueva contraseña.",
		button:  "Iniciar sesión",
		notice:  "🔒 Si no fuiste tú quien cambió la contraseña, tu cuenta puede estar comprometida. Contacta al administrador de inmediato.",
		footer:  "Si no realizaste este cambio, contacta inmediatamente con el administrador del sistema.",
	}
}

// emailCorreoCambiado is sent to the OLD address when an administrator
// changes the recovery e-mail of the account. It never contains the new
// address.
func emailCorreoCambiado() (subject, body string) {
	subject = "Tu correo de recuperación fue cambiado"
	content := fmt.Sprintf(`
    <h1 style="margin:0 0 8px;font-size:24px;font-weight:700;color:#18181b;">Correo actualizado</h1>
    <p style="margin:0 0 20px;font-size:15px;color:#52525b;">
      Un administrador cambió el correo electrónico asociado a tu cuenta. Desde ahora los avisos y la
      recuperación de contraseña se enviarán a la nueva dirección, y los enlaces enviados antes a este
      correo ya no son válidos. Por seguridad se cerraron tus sesiones abiertas.
    </p>
    %s`,
		correo.InfoBox("#fef2f2", "#dc2626", "🔒 Si no solicitaste este cambio, contacta al administrador de inmediato."))
	body = correo.Wrap(locale.ES, subject, content)
	return
}
