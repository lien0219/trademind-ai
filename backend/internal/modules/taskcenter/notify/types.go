package notify

import (
	"context"
	"time"
)

// AlertNotificationPayload is sent to external channels (no secrets, trimmed text).
type AlertNotificationPayload struct {
	AlertID           string `json:"alertId"`
	Severity          string `json:"severity"`
	FailureCategory   string `json:"failureCategory"`
	Title             string `json:"title"`
	Message           string `json:"message"`
	SuggestedAction   string `json:"suggestedAction"`
	TaskType          string `json:"taskType"`
	SourceID          string `json:"sourceId"`
	DetailURL         string `json:"detailUrl"`
	OccurredAtRFC3339 string `json:"occurredAt"`
}

// AlertNotificationResult is a trimmed audit record (no full gateway bodies).
type AlertNotificationResult struct {
	Channel           string         `json:"channel"`
	Status            string         `json:"status"` // success, failed, skipped
	ExternalMessageID string         `json:"externalMessageId,omitempty"`
	ErrorMessage      string         `json:"errorMessage,omitempty"`
	RawSummary        map[string]any `json:"rawSummary,omitempty"`
	Target            string         `json:"target,omitempty"`
}

// MailDeps is decrypted mail + alert_notify mail_* fields.
type MailDeps struct {
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	FromName     string
	From         string
	UseTLS       bool
	UseSSL       bool

	MailTo            string // comma-separated
	MailCC            string
	MailSubjectPrefix string
}

// WebhookDeps is decrypted webhook settings.
type WebhookDeps struct {
	URL       string
	Method    string
	Secret    string
	Timeout   time.Duration
	AllowHTTP bool
}

// PlannedSender is a no-op channel reserved for future vendor SDKs.
type PlannedSender struct {
	Channel string
	Reason  string
}

// SendPlanned returns a skipped result.
func (p PlannedSender) Send(_ context.Context, _ AlertNotificationPayload) AlertNotificationResult {
	ch := p.Channel
	if ch == "" {
		ch = "unknown"
	}
	return AlertNotificationResult{
		Channel: ch,
		Status:  "skipped",
		Target:  ch + ":planned",
		RawSummary: map[string]any{
			"implementation": "planned",
			"reason":         p.Reason,
		},
	}
}
