package notify

import (
	"context"
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/providers/email"
	"github.com/trademind-ai/trademind/backend/internal/providers/email/smtp"
)

// SendMail sends one alert mail to comma-separated mail_to (same body per recipient).
func SendMail(ctx context.Context, d MailDeps, payload AlertNotificationPayload) AlertNotificationResult {
	res := AlertNotificationResult{Channel: "mail"}
	toRaw := strings.TrimSpace(d.MailTo)
	if toRaw == "" {
		res.Status = "skipped"
		res.ErrorMessage = "mail_to empty"
		return res
	}
	parts := splitEmails(toRaw)
	if len(parts) == 0 {
		res.Status = "skipped"
		res.ErrorMessage = "mail_to empty"
		return res
	}
	if d.SMTPHost == "" || d.From == "" {
		res.Status = "skipped"
		res.ErrorMessage = "mail config incomplete"
		res.Target = maskMailListTarget(parts)
		return res
	}
	port := d.SMTPPort
	if port <= 0 {
		port = 587
	}
	provider := smtp.NewProvider(smtp.Config{
		Host:     d.SMTPHost,
		Port:     port,
		Username: d.SMTPUser,
		Password: d.SMTPPassword,
		FromName: d.FromName,
		From:     d.From,
		UseTLS:   d.UseTLS,
		UseSSL:   d.UseSSL,
	})

	prefix := strings.TrimSpace(d.MailSubjectPrefix)
	sev := strings.ToLower(payload.Severity)
	cat := strings.ToLower(payload.FailureCategory)
	ti := truncateStr(payload.Title, 80)
	var subj string
	if prefix != "" {
		subj = fmt.Sprintf("%s [%s][%s] %s", prefix, sev, cat, ti)
	} else {
		subj = fmt.Sprintf("[%s][%s] %s", sev, cat, ti)
	}
	body := buildMailBody(payload)

	var lastErr error
	for _, to := range parts {
		if err := provider.Send(ctx, email.SendEmailRequest{
			To:      to,
			Subject: subj,
			Content: body,
		}); err != nil {
			lastErr = err
		}
	}
	res.Target = maskMailListTarget(parts)
	if lastErr != nil {
		res.Status = "failed"
		res.ErrorMessage = truncateStr(lastErr.Error(), 500)
		return res
	}
	res.Status = "success"
	res.RawSummary = map[string]any{"recipients": len(parts)}
	return res
}

func buildMailBody(p AlertNotificationPayload) string {
	var b strings.Builder
	b.WriteString("Task alert\n\n")
	b.WriteString("严重等级: " + p.Severity + "\n")
	b.WriteString("失败分类: " + p.FailureCategory + "\n")
	b.WriteString("任务类型: " + p.TaskType + "\n")
	b.WriteString("建议处理: " + truncateStr(p.SuggestedAction, 400) + "\n\n")
	b.WriteString("摘要: " + truncateStr(p.Message, 400) + "\n\n")
	b.WriteString("后台详情路径: " + p.DetailURL + "\n")
	b.WriteString("告警时间(UTC): " + p.OccurredAtRFC3339 + "\n")
	return b.String()
}

func splitEmails(s string) []string {
	for _, sep := range []string{",", ";"} {
		s = strings.ReplaceAll(s, sep, ",")
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.Contains(p, "@") {
			out = append(out, p)
		}
	}
	return out
}

func maskMailListTarget(emails []string) string {
	if len(emails) == 0 {
		return ""
	}
	first := maskOneEmail(emails[0])
	if len(emails) == 1 {
		return first
	}
	return fmt.Sprintf("%s +%d", first, len(emails)-1)
}

func maskOneEmail(email string) string {
	email = strings.TrimSpace(email)
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		return "***"
	}
	local := email[:at]
	dom := email[at+1:]
	if local == "" {
		return "***@" + dom
	}
	runes := []rune(local)
	prefix := string(runes[0])
	return prefix + "***@" + dom
}
