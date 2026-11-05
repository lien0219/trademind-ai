package taskcenter

import (
	"context"
	"encoding/json"
	"errors"
	"net/mail"
	"strconv"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter/notify"
	"github.com/trademind-ai/trademind/backend/internal/modules/tenantconfig"
	"gorm.io/gorm"
)

// taskAlertMailLocale follows the tenant's language without trusting request
// headers. Database errors fail closed instead of using stale tenant settings.
func taskAlertMailLocale(ctx context.Context, db *gorm.DB, tenantID int64) (string, error) {
	if tenantID == 0 {
		return "zh-CN", nil
	}
	snapshot, err := tenantconfig.Read(ctx, db, tenantID, tenantconfig.ModuleGeneral)
	if err != nil {
		return "", err
	}
	var cfg tenantconfig.General
	if err := json.Unmarshal(snapshot.Values, &cfg); err != nil {
		return "", err
	}
	if cfg.Language == "" {
		return "", errors.New("tenant mail locale unavailable")
	}
	return cfg.Language, nil
}

type taskMailTemplateSender func(context.Context, string, string, string, map[string]string) error

// deliverTaskFailureTemplate reuses the admin mail-template center. The caller
// remains responsible for opt-in, destination authorization, dedupe, audit
// and delivery history; this helper never logs a transport error or secret.
func deliverTaskFailureTemplate(ctx context.Context, recipients, locale string, payload notify.AlertNotificationPayload, send taskMailTemplateSender) notify.AlertNotificationResult {
	result := notify.AlertNotificationResult{Channel: "mail"}
	if send == nil || strings.TrimSpace(recipients) == "" {
		result.Status = TaskAlertNotifStatusSkipped
		result.ErrorMessage = "mail delivery unavailable"
		return result
	}
	targets := make([]string, 0)
	for _, raw := range strings.Split(recipients, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		addr, err := mail.ParseAddress(raw)
		if err != nil || addr.Address == "" {
			result.Status = TaskAlertNotifStatusSkipped
			result.ErrorMessage = "mail_to invalid"
			return result
		}
		targets = append(targets, addr.Address)
	}
	if len(targets) == 0 {
		result.Status = TaskAlertNotifStatusSkipped
		result.ErrorMessage = "mail_to empty"
		return result
	}
	result.Target = maskedTaskMailRecipients(targets)
	taskName := strings.TrimSpace(payload.TaskType)
	if taskName == "" {
		taskName = strings.TrimSpace(payload.Title)
	}
	if taskName == "" {
		taskName = "Task"
	}
	description := strings.Join([]string{
		payload.Title, payload.FailureCategory, payload.Message,
		payload.SuggestedAction, payload.DetailURL,
	}, " | ")
	for _, to := range targets {
		if err := send(ctx, "task.failed", locale, to, map[string]string{
			"task_name": taskName, "event_name": description,
		}); err != nil {
			result.Status = TaskAlertNotifStatusFailed
			result.ErrorMessage = "template mail delivery failed"
			return result
		}
	}
	result.Status = TaskAlertNotifStatusSuccess
	result.RawSummary = map[string]any{"recipients": len(targets), "templateEvent": "task.failed"}
	return result
}

func maskedTaskMailRecipients(addresses []string) string {
	if len(addresses) == 0 {
		return ""
	}
	first := strings.SplitN(addresses[0], "@", 2)
	if len(first) != 2 || len(first[0]) == 0 {
		return "***"
	}
	runes := []rune(first[0])
	masked := string(runes[0]) + "***@" + first[1]
	if len(addresses) > 1 {
		masked += " +" + strconv.Itoa(len(addresses)-1)
	}
	return masked
}
