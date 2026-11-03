package smtp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

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

type Provider struct { cfg Config }

func NewProvider(cfg Config) *Provider { return &Provider{cfg: cfg} }
func (p *Provider) Name() string { return "smtp" }

const smtpTimeout = 15 * time.Second

func (p *Provider) Send(ctx context.Context, req email.SendEmailRequest) error {
	cfg := p.cfg
	if ctx == nil {
		return errors.New("smtp: missing context")
	}
	if cfg.UseSSL && cfg.UseTLS {
		return errors.New("smtp: select SSL or STARTTLS, not both")
	}
	if !cfg.UseSSL && !cfg.UseTLS {
		return errors.New("smtp: encrypted transport required")
	}
	if cfg.Port < 1 || cfg.Port > 65535 || strings.TrimSpace(cfg.Host) == "" ||
		strings.ContainsAny(cfg.Host, "\r\n\t @/\\") {
		return errors.New("smtp: invalid server configuration")
	}
	from, err := mail.ParseAddress(cfg.From)
	if err != nil || from == nil || from.Address != cfg.From || strings.ContainsAny(cfg.From, "\r\n") {
		return errors.New("smtp: invalid sender")
	}
	to, err := mail.ParseAddress(req.To)
	if err != nil || to == nil || to.Address != req.To || strings.ContainsAny(req.To, "\r\n") {
		return errors.New("smtp: invalid recipient")
	}
	if strings.ContainsAny(req.Subject, "\r\n") || strings.ContainsAny(cfg.FromName, "\r\n") {
		return errors.New("smtp: invalid message header")
	}
	contentType := req.ContentType
	if contentType == "" {
		contentType = "text/plain"
	}
	if contentType != "text/plain" && contentType != "text/html" {
		return errors.New("smtp: invalid content type")
	}
	host := strings.TrimSpace(cfg.Host)
	address := net.JoinHostPort(host, fmt.Sprint(cfg.Port))
	dialer := &net.Dialer{Timeout: smtpTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("smtp: connect: %w", err)
	}
	defer conn.Close()
	deadline := time.Now().Add(smtpTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("smtp: set deadline: %w", err)
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: host}
	if cfg.UseSSL {
		tlsConn := tls.Client(conn, tlsConfig)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return fmt.Errorf("smtp: secure handshake: %w", err)
		}
		conn = tlsConn
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp: session: %w", err)
	}
	defer client.Close()
	if cfg.UseTLS {
		ok, _ := client.Extension("STARTTLS")
		if !ok {
			return errors.New("smtp: server does not support required STARTTLS")
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("smtp: STARTTLS: %w", err)
		}
	}
	if cfg.Username != "" {
		if cfg.Password == "" {
			return errors.New("smtp: authentication password missing")
		}
		if err := client.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, host)); err != nil {
			return fmt.Errorf("smtp: authentication failed: %w", err)
		}
	}
	if err := client.Mail(from.Address); err != nil {
		return fmt.Errorf("smtp: MAIL FROM: %w", err)
	}
	if err := client.Rcpt(to.Address); err != nil {
		return fmt.Errorf("smtp: recipient rejected: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp: DATA: %w", err)
	}
	name := strings.TrimSpace(cfg.FromName)
	fromHeader := from.Address
	if name != "" {
		fromHeader = mime.QEncoding.Encode("utf-8", name) + " <" + from.Address + ">"
	}
	subject := mime.QEncoding.Encode("utf-8", req.Subject)
	msg := "From: " + fromHeader + "\r\n" +
		"To: " + to.Address + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: " + contentType + "; charset=utf-8\r\n" +
		"Content-Transfer-Encoding: 8bit\r\n\r\n" +
		strings.ReplaceAll(strings.ReplaceAll(req.Content, "\r\n", "\n"), "\n", "\r\n")
	_, err = io.WriteString(w, msg)
	if err != nil {
		_ = w.Close()
		return fmt.Errorf("smtp: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp: finalize: %w", err)
	}
	return client.Quit()
}
