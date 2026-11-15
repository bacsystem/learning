// Package correo holds the HTML layout shared by every e-mail of the app
// (header with the brand, body, footer, button and info box), in Spanish and
// English. Each domain renders its own content and wraps it with Wrap.
package correo

import (
	"fmt"
	"html"
	"time"

	"github.com/cbaciliod/learning/internal/locale"
)

// Brand signs the header and footer of every e-mail.
const Brand = "Capacitate"

const layout = `<!DOCTYPE html>
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

// AutoNotice is the "do not reply" line of the footer.
func AutoNotice(loc string) string {
	if loc == locale.EN {
		return "This message was sent automatically — please do not reply to this e-mail."
	}
	return "Este mensaje fue enviado automáticamente — por favor no respondas a este correo."
}

// Wrap puts content (trusted HTML, already escaped by the caller) inside the
// layout. subject goes into <title> unescaped, as it always did: pass a
// subject built from trusted text or escape it first.
func Wrap(loc, subject, content string) string {
	return fmt.Sprintf(layout, loc, subject, Brand, content, AutoNotice(loc), time.Now().Year(), Brand)
}

// BtnPrimary renders a call-to-action button. label is a trusted literal;
// url is escaped (it goes into an attribute and the visible text).
func BtnPrimary(loc, label, url string) string {
	return fmt.Sprintf(`
    <table width="100%%" cellpadding="0" cellspacing="0" style="margin:28px 0 20px;">
      <tr><td align="center">
        <a href="%s" style="display:inline-block;background-color:#4a5e3a;color:#ffffff;text-decoration:none;font-size:15px;font-weight:600;padding:14px 36px;border-radius:8px;">%s</a>
      </td></tr>
    </table>
    <p style="margin:0;font-size:12px;color:#a1a1aa;text-align:center;">%s<br><span style="color:#4a5e3a;">%s</span></p>`,
		html.EscapeString(url), label, ButtonFallback(loc), html.EscapeString(url))
}

// ButtonFallback is the line that offers the link when the button fails.
func ButtonFallback(loc string) string {
	if loc == locale.EN {
		return "If the button does not work, copy this link:"
	}
	return "Si el botón no funciona, copia este enlace:"
}

// InfoBox renders a highlighted box; text is trusted HTML.
func InfoBox(color, borderColor, text string) string {
	return fmt.Sprintf(`
    <table width="100%%" cellpadding="0" cellspacing="0" style="background:%s;border-left:4px solid %s;border-radius:0 8px 8px 0;margin:20px 0;">
      <tr><td style="padding:16px 20px;font-size:14px;color:#3f3f46;">%s</td></tr>
    </table>`, color, borderColor, text)
}
