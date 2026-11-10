package auth

import (
	"fmt"
	"html"
)

const emailLayout = `<!DOCTYPE html>
<html lang="es">
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
          <p style="margin:0;font-size:22px;font-weight:700;color:#ffffff;letter-spacing:-0.3px;">Sistema de Capacitación</p>
          <p style="margin:6px 0 0;font-size:13px;color:#c8d8b8;">Ejército del Perú</p>
        </td></tr>

        <!-- Body -->
        <tr><td style="background-color:#ffffff;padding:40px 40px 32px;">%s</td></tr>

        <!-- Footer -->
        <tr><td style="background-color:#f9f9f9;border:1px solid #e4e4e7;border-top:none;border-radius:0 0 12px 12px;padding:20px 40px;text-align:center;">
          <p style="margin:0;font-size:12px;color:#71717a;">Este mensaje fue enviado automáticamente — por favor no respondas a este correo.</p>
          <p style="margin:8px 0 0;font-size:12px;color:#a1a1aa;">© 2025 Sistema de Capacitación · Ejército del Perú</p>
        </td></tr>

      </table>
    </td></tr>
  </table>
</body>
</html>`

func wrap(subject, content string) string {
	return fmt.Sprintf(emailLayout, subject, content)
}

// btnPrimary renders a call-to-action button. label is a trusted literal;
// url is escaped (it goes into an attribute and the visible text).
func btnPrimary(label, url string) string {
	return fmt.Sprintf(`
    <table width="100%%" cellpadding="0" cellspacing="0" style="margin:28px 0 20px;">
      <tr><td align="center">
        <a href="%s" style="display:inline-block;background-color:#4a5e3a;color:#ffffff;text-decoration:none;font-size:15px;font-weight:600;padding:14px 36px;border-radius:8px;">%s</a>
      </td></tr>
    </table>
    <p style="margin:0;font-size:12px;color:#a1a1aa;text-align:center;">Si el botón no funciona, copia este enlace:<br><span style="color:#4a5e3a;">%s</span></p>`,
		html.EscapeString(url), label, html.EscapeString(url))
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
		html.EscapeString(nombres+" "+apellido), html.EscapeString(username), btnPrimary("Iniciar sesión", loginURL))
	body = wrap(subject, content)
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
	body = wrap(subject, content)
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
	body = wrap(subject, content)
	return
}

// ── Registro pendiente (al usuario) ─────────────────────────────────────────

func emailRegistroPendiente(nombres, apellido, username string) (subject, body string) {
	subject = "Solicitud de cuenta recibida"
	content := fmt.Sprintf(`
    <h1 style="margin:0 0 8px;font-size:24px;font-weight:700;color:#18181b;">Hola, %s</h1>
    <p style="margin:0 0 20px;font-size:15px;color:#52525b;">
      Hemos recibido tu solicitud de acceso a la plataforma de capacitación.
      Tu cuenta está siendo revisada por el equipo administrativo.
    </p>

    <table width="100%%" cellpadding="0" cellspacing="0" style="background:#f0f4ec;border-radius:8px;margin-bottom:20px;">
      <tr><td style="padding:20px 24px;">
        <p style="margin:0 0 4px;font-size:11px;font-weight:600;color:#71717a;text-transform:uppercase;letter-spacing:0.5px;">Usuario solicitado</p>
        <p style="margin:0;font-size:20px;font-weight:700;color:#4a5e3a;">@%s</p>
      </td></tr>
    </table>

    %s

    <p style="margin:20px 0 0;font-size:13px;color:#71717a;">
      Recibirás otro correo cuando tu cuenta sea activada. Si tienes dudas,
      contacta con el área de administración de tu unidad.
    </p>`,
		html.EscapeString(nombres+" "+apellido), html.EscapeString(username),
		infoBox("#f0f9ff", "#0ea5e9", "⏳ Tu solicitud está <strong>en revisión</strong>. El tiempo de respuesta habitual es de 1 a 2 días hábiles."))
	body = wrap(subject, content)
	return
}

// ── Verificación de correo (registro) ───────────────────────────────────────

func emailVerificarRegistro(nombres, apellido, verifyURL string) (subject, body string) {
	subject = "Confirma tu registro"
	content := fmt.Sprintf(`
    <h1 style="margin:0 0 8px;font-size:24px;font-weight:700;color:#18181b;">Hola, %s</h1>
    <p style="margin:0 0 4px;font-size:15px;color:#52525b;">
      Recibimos una solicitud de registro en la plataforma de capacitación para tu DNI.
      Para confirmar que este correo es tuyo y elegir tu usuario y contraseña, haz clic en el botón.
    </p>
    %s
    %s
    <p style="margin:20px 0 0;font-size:13px;color:#71717a;">
      Si no solicitaste este registro, ignora este mensaje: no se creará ninguna cuenta.
    </p>`,
		html.EscapeString(nombres+" "+apellido),
		btnPrimary("Completar registro", verifyURL),
		infoBox("#fffbeb", "#d97706", "⚠ Este enlace es válido por <strong>24 horas</strong> y solo puede usarse una vez. Si te registras de nuevo, el enlace anterior deja de funcionar."))
	body = wrap(subject, content)
	return
}

// ── Recuperación de contraseña ───────────────────────────────────────────────

func emailOlvidePassword(resetURL string) (subject, body string) {
	subject = "Recuperación de contraseña"
	content := fmt.Sprintf(`
    <h1 style="margin:0 0 8px;font-size:24px;font-weight:700;color:#18181b;">Restablecer contraseña</h1>
    <p style="margin:0 0 4px;font-size:15px;color:#52525b;">
      Recibimos una solicitud para restablecer la contraseña de tu cuenta.
      Haz clic en el botón para crear una nueva contraseña.
    </p>
    %s
    %s
    <p style="margin:20px 0 0;font-size:13px;color:#71717a;">
      Si no solicitaste restablecer tu contraseña, ignora este mensaje.
      Tu cuenta permanece segura.
    </p>`,
		btnPrimary("Restablecer contraseña", resetURL),
		infoBox("#fffbeb", "#d97706", "⚠ Este enlace es válido por <strong>1 hora</strong> y solo puede usarse una vez."))
	body = wrap(subject, content)
	return
}

// ── Contraseña restablecida (confirmación) ───────────────────────────────────

func emailPasswordRestablecida(loginURL string) (subject, body string) {
	subject = "Tu contraseña fue restablecida"
	content := fmt.Sprintf(`
    <h1 style="margin:0 0 8px;font-size:24px;font-weight:700;color:#18181b;">Contraseña actualizada</h1>
    <p style="margin:0 0 20px;font-size:15px;color:#52525b;">
      Tu contraseña ha sido restablecida exitosamente. Ya puedes iniciar sesión con tu nueva contraseña.
    </p>
    %s
    %s
    <p style="margin:20px 0 0;font-size:13px;color:#71717a;">
      Si no realizaste este cambio, contacta inmediatamente con el administrador del sistema.
    </p>`,
		btnPrimary("Iniciar sesión", loginURL),
		infoBox("#fef2f2", "#dc2626", "🔒 Si no fuiste tú quien cambió la contraseña, tu cuenta puede estar comprometida. Contacta al administrador de inmediato."))
	body = wrap(subject, content)
	return
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
	body = wrap(subject, content)
	return
}
