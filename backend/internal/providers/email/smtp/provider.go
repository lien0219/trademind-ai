package smtp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"

	"github.com/trademind-ai/trademind/backend/internal/providers/email"
)

type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	FromName string
	From     string
	UseTLS   bool
	UseSSL   bool
}

type Provider struct {
	cfg Config
}

func NewProvider(cfg Config) *Provider {
	return &Provider{cfg: cfg}
}

func (p *Provider) Name() string {
	return "smtp"
}

func (p *Provider) Send(ctx context.Context, req email.SendEmailRequest) error {
	addr := fmt.Sprintf("%s:%d", p.cfg.Host, p.cfg.Port)

	// Build the message
	contentType := req.ContentType
	if contentType == "" {
		contentType = "text/plain"
	}

	header := make(map[string]string)
	header["From"] = fmt.Sprintf("%s <%s>", p.cfg.FromName, p.cfg.From)
	header["To"] = req.To
	header["Subject"] = req.Subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = fmt.Sprintf("%s; charset=\"utf-8\"", contentType)

	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + req.Content

	var auth smtp.Auth
	if p.cfg.Username != "" && p.cfg.Password != "" {
		auth = smtp.PlainAuth("", p.cfg.Username, p.cfg.Password, p.cfg.Host)
	}

	if p.cfg.UseSSL {
		tlsconfig := &tls.Config{
			InsecureSkipVerify: true, // We should perhaps not skip verify in prod, but for an MVP it's common
			ServerName:         p.cfg.Host,
		}
		conn, err := tls.Dial("tcp", addr, tlsconfig)
		if err != nil {
			return fmt.Errorf("smtp connect: %w", err)
		}
		defer conn.Close()

		c, err := smtp.NewClient(conn, p.cfg.Host)
		if err != nil {
			return fmt.Errorf("smtp new client: %w", err)
		}
		defer c.Close()

		if auth != nil {
			if err = c.Auth(auth); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}

		if err = c.Mail(p.cfg.From); err != nil {
			return fmt.Errorf("smtp mail from: %w", err)
		}
		if err = c.Rcpt(req.To); err != nil {
			return fmt.Errorf("smtp rcpt to: %w", err)
		}

		w, err := c.Data()
		if err != nil {
			return fmt.Errorf("smtp data: %w", err)
		}
		_, err = w.Write([]byte(message))
		if err != nil {
			return fmt.Errorf("smtp write data: %w", err)
		}
		err = w.Close()
		if err != nil {
			return fmt.Errorf("smtp close data: %w", err)
		}
		return c.Quit()
	}

	// Non-SSL (maybe TLS via STARTTLS inside SendMail)
	err := smtp.SendMail(addr, auth, p.cfg.From, []string{req.To}, []byte(message))
	if err != nil {
		return fmt.Errorf("smtp send mail: %w", err)
	}
	return nil
}
