package email

import "context"

type SendEmailRequest struct {
	To          string
	Subject     string
	Content     string
	ContentType string // "text/plain" or "text/html"
}

type Provider interface {
	Name() string
	Send(ctx context.Context, req SendEmailRequest) error
}
