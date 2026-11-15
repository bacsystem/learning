package auth

import (
	"fmt"
	"html"
	"time"

	"github.com/cbaciliod/learning/internal/locale"
)

// emailBrand signs the header and footer of every e-mail.
const emailBrand = "Capacitate"

// emailInstitution is the institution line removed from the e-mails until the
// product owner confirms whether the client requires it. Restore it as a
// secondary line in emailLayout (header and footer) if so.
//
//nolint:unused // kept on purpose, see above.
const emailInstitution = "Ejército del Perú"

const emailLayout = `<!DOCTYPE html>
<html lang="%s">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>%s</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;">
  <table width="100%%" cellpadding="0" cellspacing="0" style="background-color:#f4f4f5;padding:40px 16px;">
    <tr><td align="center">
      <table width="100%%" cellpadding="0" cellspacing="0" style="max-width:560px;">

        <!-- Header -->
        <tr><td style="background-color:#4a5e3a;border-radius:12px 12px 0 0;padding:32px 40px;text-align:center;">
          <p style="margin:0;font-size:22px;font-weight:700;color:#ffffff;letter-spacing:-0.3px;">%s</p>
        </td></tr>

        <!-- Body -->
        <tr><td style="background-color:#ffffff;padding:40px 40px 32px;">%s</td></tr>

        <!-- Footer -->
        <tr><td style="background-color:#f9f9f9;border:1px solid #e4e4e7;border-top:none;border-radius:0 0 12px 12px;padding:20px 40px;text-align:center;">
          <p style="margin:0;font-size:12px;color:#71717a;">%s</p>
          <p style="margin:8px 0 0;font-size:12px;color:#a1a1aa;">© %d %s</p>
        </td></tr>

      </table>
    </td></tr>
  </table>
</body>
</html>`

// layoutAutoNotice is the "do not reply" line of the footer.
func layoutAutoNotice(loc string) string {
	if loc == locale.EN {
		return "This message was sent automatically — please do not reply to this e-mail."
	}
	return "Este mensaje fue enviado automáticamente — por favor no respondas a este correo."
}

func wrap(loc, subject, content string) string {
	return fmt.Sprintf(emailLayout, loc, subject, emailBrand, content, layoutAutoNotice(loc), time.Now().Year(), emailBrand)
}

// btnPrimary renders a call-to-action button. label is a trusted literal;
// url is escaped (it goes into an attribute and the visible text).
func btnPrimary(loc, label, url string) string {
	return fmt.Sprintf(`
    <table width="100%%" cellpadding="0" cellspacing="0" style="margin:28px 0 20px;">
      <tr><td align="center">
        <a href="%s" style="display:inline-block;background-color:#4a5e3a;color:#ffffff;text-decoration:none;font-size:15px;font-weight:600;padding:14px 36px;border-radius:8px;">%s</a>
      </td></tr>
    </table>
    <p style="margin:0;font-size:12px;color:#a1a1aa;text-align:center;">%s<br><span style="color:#4a5e3a;">%s</span></p>`,
		html.EscapeString(url), label, buttonFallback(loc), html.EscapeString(url))
}

func buttonFallback(loc string) string {
	if loc == locale.EN {
		return "If the button does not work, copy this link:"
	}
	return "Si el botón no funciona, copia este enlace:"
}

func infoBox(color, borderColor, text string) string {
	return fmt.Sprintf(`
    <table width="100%%" cellpadding="0" cellspacing="0" style="background:%s;border-left:4px solid %s;border-radius:0 8px 8px 0;margin:20px 0;">
      <tr><td style="padding:16px 20px;font-size:14px;color:#3f3f46;">%s</td></tr>
    </table>`, color, borderColor, text)
}

// Every value that comes from the database (names, username) is passed
// through html.EscapeString before it reaches the HTML (H130).

// ── Cuenta activada ──────────────────────────────────────────────────────────

func emailActivado(nombres, apellido, username, loginURL string) (subject, body string) {
	subject = "Tu cuenta ha sido activada"
	content := fmt.Sprintf(`
    <h1 style="margin:0 0 8px;font-size:24px;font-weight:700;color:#18181b;">¡Bienvenido, %s!</h1>
    <p style="margin:0 0 20px;font-size:15px;color:#52525b;">
      Tu cuenta ha sido <strong style="color:#4a5e3a;">activada exitosamente</strong>.
      Ya puedes acceder a la plataforma y comenzar tus capacitaciones.
    </p>

    <table width="100%%" cellpadding="0" cellspacing="0" style="background:#f0f4ec;border-radius:8px;margin-bottom:8px;">
      <tr><td style="padding:20px 24px;">
        <p style="margin:0 0 4px;font-size:11px;font-weight:600;color:#71717a;text-transform:uppercase;letter-spacing:0.5px;">Usuario asignado</p>
        <p style="margin:0;font-size:20px;font-weight:700;color:#4a5e3a;">@%s</p>
      </td></tr>
    </table>
    %s`,
		html.EscapeString(nombres+" "+apellido), html.EscapeString(username), btnPrimary(locale.ES, "Iniciar sesión", loginURL))
	body = wrap(locale.ES, subject, content)
	return
}

// ── Cuenta rechazada ─────────────────────────────────────────────────────────

func emailRechazado(nombres, apellido string) (subject, body string) {
	subject = "Actualización sobre tu solicitud de cuenta"
	content := fmt.Sprintf(`
    <h1 style="margin:0 0 8px;font-size:24px;font-weight:700;color:#18181b;">Hola, %s</h1>
    <p style="margin:0 0 4px;font-size:15px;color:#52525b;">
      Hemos revisado tu solicitud de acceso y lamentablemente
      <strong style="color:#dc2626;">no pudo ser aprobada</strong> en este momento.
    </p>
    %s
    <p style="margin:16px 0 0;font-size:14px;color:#71717a;">
      Puedes contactar al personal administrativo de tu unidad para obtener más información
      o presentar una nueva solicitud.
    </p>`,
		html.EscapeString(nombres+" "+apellido),
		infoBox("#fef2f2", "#dc2626", "Si crees que esto es un error, comunícate con el área de administración de tu unidad."))
	body = wrap(locale.ES, subject, content)
	return
}

// ── Cuenta desactivada ───────────────────────────────────────────────────────

func emailInactivo(nombres, apellido string) (subject, body string) {
	subject = "Tu cuenta ha sido desactivada"
	content := fmt.Sprintf(`
    <h1 style="margin:0 0 8px;font-size:24px;font-weight:700;color:#18181b;">Hola, %s</h1>
    <p style="margin:0 0 4px;font-size:15px;color:#52525b;">
      Tu cuenta ha sido <strong style="color:#d97706;">desactivada temporalmente</strong>
      por el administrador del sistema.
    </p>
    %s
    <p style="margin:16px 0 0;font-size:14px;color:#71717a;">
      Durante este período no podrás acceder a los recursos de capacitación.
      Contacta con el área de administración para solicitar la reactivación.
    </p>`,
		html.EscapeString(nombres+" "+apellido),
		infoBox("#fffbeb", "#d97706", "Si consideras que esta acción fue un error, comunícate con el área de administración de tu unidad."))
	body = wrap(locale.ES, subject, content)
	return
}

// ── Registro pendiente (al usuario) ─────────────────────────────────────────

func emailRegistroPendiente(loc, nombres, apellido, username string) (subject, body string) {
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
		html.EscapeString(username), infoBox("#f0f9ff", "#0ea5e9", c.notice), c.footer)
	body = wrap(loc, subject, content)
	return
}

type registroPendienteTexts struct{ subject, greeting, intro, usernameLabel, notice, footer string }

func registroPendienteCopy(loc string) registroPendienteTexts {
	if loc == locale.EN {
		return registroPendienteTexts{
			subject:       "Account request received",
			greeting:      "Hello, %s",
			intro:         "We have received your request to access the training platform. Your account is being reviewed by the administrative team.",
			usernameLabel: "Requested username",
			notice:        "⏳ Your request is <strong>under review</strong>. The usual response time is 1 to 2 business days.",
			footer:        "You will receive another e-mail when your account is activated. If you have questions, contact the administration area of your unit.",
		}
	}
	return registroPendienteTexts{
		subject:       "Solicitud de cuenta recibida",
		greeting:      "Hola, %s",
		intro:         "Hemos recibido tu solicitud de acceso a la plataforma de capacitación. Tu cuenta está siendo revisada por el equipo administrativo.",
		usernameLabel: "Usuario solicitado",
		notice:        "⏳ Tu solicitud está <strong>en revisión</strong>. El tiempo de respuesta habitual es de 1 a 2 días hábiles.",
		footer:        "Recibirás otro correo cuando tu cuenta sea activada. Si tienes dudas, contacta con el área de administración de tu unidad.",
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
		btnPrimary(loc, c.button, verifyURL), infoBox("#fffbeb", "#d97706", c.notice), c.footer)
	body = wrap(loc, subject, content)
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
		c.title, c.intro, btnPrimary(loc, c.button, resetURL), infoBox("#fffbeb", "#d97706", c.notice), c.footer)
	body = wrap(loc, subject, content)
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
		c.title, c.intro, btnPrimary(loc, c.button, loginURL), infoBox("#fef2f2", "#dc2626", c.notice), c.footer)
	body = wrap(loc, subject, content)
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
		infoBox("#fef2f2", "#dc2626", "🔒 Si no solicitaste este cambio, contacta al administrador de inmediato."))
	body = wrap(locale.ES, subject, content)
	return
}
