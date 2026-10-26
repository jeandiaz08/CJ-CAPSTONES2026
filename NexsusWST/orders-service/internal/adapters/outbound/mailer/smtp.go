// Package mailer - avisos de negocio por correo (DAS v1.1 §4.5: Nexus envía
// sus avisos con el mismo SMTP de westorage.cl). En desarrollo, Mailpit.
package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"
)

type Config struct {
	Host     string
	Port     string
	From     string
	User     string
	Password string
	StartTLS bool
}

type SMTP struct {
	cfg Config
	log *slog.Logger
}

func New(cfg Config, log *slog.Logger) *SMTP {
	return &SMTP{cfg: cfg, log: log.With("component", "mailer")}
}

func (m *SMTP) Send(ctx context.Context, to []string, subject, body string) error {
	if m.cfg.Host == "" {
		m.log.Info("SMTP no configurado; aviso no enviado por correo", "to", len(to), "subject", subject)
		return nil
	}
	clean := make([]string, 0, len(to))
	for _, t := range to {
		t = strings.TrimSpace(t)
		if t != "" && !strings.ContainsAny(t, "\r\n") {
			clean = append(clean, t)
		}
	}
	if len(clean) == 0 {
		return nil
	}
	headers := []string{
		"From: NexusWST · WeStorage <" + m.cfg.From + ">",
		"To: " + strings.Join(clean, ", "),
		"Subject: " + mime.QEncoding.Encode("utf-8", subject),
		"Date: " + time.Now().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Transfer-Encoding: 8bit",
	}
	msg := []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + strings.ReplaceAll(body, "\n", "\r\n"))

	d := net.Dialer{Timeout: 10 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(m.cfg.Host, m.cfg.Port))
	if err != nil {
		return fmt.Errorf("SMTP: %w", err)
	}
	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		return err
	}
	defer c.Close()
	if m.cfg.StartTLS {
		if err := c.StartTLS(&tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if m.cfg.User != "" {
		if err := c.Auth(smtp.PlainAuth("", m.cfg.User, m.cfg.Password, m.cfg.Host)); err != nil {
			return err
		}
	}
	if err := c.Mail(m.cfg.From); err != nil {
		return err
	}
	for _, t := range clean {
		if err := c.Rcpt(t); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
