package notifier

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"gopkg.in/gomail.v2"
)

// smtpSender delivers one message per connection with a hard deadline on every
// network operation (dial, TLS handshake, AUTH, DATA). gomail's Dialer only
// bounds the dial (10s) and can then block forever on a silent server, so the
// SMTP conversation is driven here with net/smtp on a net.Conn whose deadline
// comes from the per-message context; gomail is still used to build the MIME
// message. (H69)
type smtpSender struct {
	host     string
	port     int
	user     string
	password string
	from     string
	// implicitTLS mirrors gomail.NewDialer: port 465 means SMTPS.
	implicitTLS bool
	tlsConfig   *tls.Config
}

func newSMTPSender(host string, port int, user, password, from string) *smtpSender {
	return &smtpSender{
		host: host, port: port, user: user, password: password, from: from,
		implicitTLS: port == 465,
	}
}

func (s *smtpSender) tlsCfg() *tls.Config {
	if s.tlsConfig != nil {
		return s.tlsConfig
	}
	return &tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12}
}

func (s *smtpSender) Send(ctx context.Context, to, subject, htmlBody string) (err error) {
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("smtp: send requires a context deadline")
	}
	m := gomail.NewMessage()
	m.SetHeader("From", s.from)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", htmlBody)

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(s.host, fmt.Sprint(s.port)))
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	dl, _ := ctx.Deadline()
	if err := conn.SetDeadline(dl); err != nil {
		conn.Close()
		return fmt.Errorf("smtp deadline: %w", err)
	}
	// Cancellation (not only the deadline) also aborts blocked I/O.
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Unix(1, 0)) })
	defer stop()

	if s.implicitTLS {
		conn = tls.Client(conn, s.tlsCfg())
	}
	c, err := smtp.NewClient(conn, s.host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp greeting: %w", err)
	}
	defer c.Close()

	if !s.implicitTLS {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(s.tlsCfg()); err != nil {
				return fmt.Errorf("smtp starttls: %w", err)
			}
		}
	}
	if s.user != "" {
		if ok, mechs := c.Extension("AUTH"); ok {
			if err := c.Auth(s.auth(mechs)); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
	}
	if err := c.Mail(s.from); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := m.WriteTo(w); err != nil {
		w.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp data end: %w", err)
	}
	return c.Quit()
}

// auth picks the mechanism the same way gomail does.
func (s *smtpSender) auth(mechs string) smtp.Auth {
	switch {
	case strings.Contains(mechs, "CRAM-MD5"):
		return smtp.CRAMMD5Auth(s.user, s.password)
	case strings.Contains(mechs, "LOGIN") && !strings.Contains(mechs, "PLAIN"):
		return &loginAuth{user: s.user, password: s.password, host: s.host}
	default:
		return smtp.PlainAuth("", s.user, s.password, s.host)
	}
}

// loginAuth implements AUTH LOGIN (as gomail does), only over TLS or to
// localhost.
type loginAuth struct{ user, password, host string }

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS && server.Name != "localhost" && server.Name != "127.0.0.1" && server.Name != "::1" {
		return "", nil, errors.New("smtp: unencrypted connection")
	}
	if server.Name != a.host {
		return "", nil, errors.New("smtp: wrong host name")
	}
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch strings.ToLower(strings.TrimSpace(string(fromServer))) {
	case "username:":
		return []byte(a.user), nil
	case "password:":
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("smtp: unexpected server challenge %q", fromServer)
	}
}
