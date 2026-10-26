package taskcenter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter/notify"
    "github.com/trademind-ai/trademind/backend/internal/modules/tenantconfig"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// alertNotifyCandidate ties a freshly touched alert row to scan metadata.
type alertNotifyCandidate struct {
	Alert TaskAlert
	IsNew bool
}

func (s *Service) alertNotifyPlain(ctx context.Context) (map[string]string, error) {
	if s == nil || s.Settings == nil {
		return map[string]string{}, nil
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "alert_notify")
	if err != nil || m == nil {
		return map[string]string{}, err
	}
	return m, err
}

func parseJSONStringSlice(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var arr []string
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		v = strings.TrimSpace(strings.ToLower(v))
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func normalizeChannelFilter(in []string) []string {
	var out []string
	for _, c := range in {
		c = strings.TrimSpace(strings.ToLower(c))
		if c != "" {
			out = append(out, c)
		}
	}
	return out
}

func webhookNotifyTimeoutSeconds(an map[string]string) int {
	const engineDefault = 15
	const maxSec = 300
	raw := strings.TrimSpace(an["webhook_timeout_seconds"])
	if raw == "" {
		return engineDefault
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return engineDefault
	}
	if n > maxSec {
		return maxSec
	}
	return n
}

func (s *Service) lastSuccessNotification(ctx context.Context, alertID uuid.UUID, channel string) (*TaskAlertNotification, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("taskcenter: no db")
	}
	var row TaskAlertNotification
	err := s.DB.WithContext(ctx).
		Where("alert_id = ? AND channel = ? AND status = ?", alertID, channel, TaskAlertNotifStatusSuccess).
		Order("created_at DESC").
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func parseAlertCountSnapshot(raw datatypes.JSON) int {
	if len(raw) == 0 {
		return 0
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return 0
	}
	v, ok := m["alertCount"]
	if !ok {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	default:
		return 0
	}
}

func (s *Service) shouldAutoNotifyChannel(
	ctx context.Context,
	alert TaskAlert,
	channel string,
	isNew bool,
	tc map[string]string,
	manual bool,
) (bool, string) {
	if manual {
		return true, ""
	}
	if !strings.EqualFold(strings.TrimSpace(alert.Status), TaskAlertStatusOpen) {
		return false, "alert not open"
	}
	if !manual && !parseBoolTaskCenter(tc["notify_on_alert_generated"], false) && isNew {
		return false, "notify_on_alert_generated off"
	}
	if !isNew && !parseBoolTaskCenter(tc["notify_on_repeated_alert"], false) {
		return false, "notify_on_repeated_alert off"
	}
	last, err := s.lastSuccessNotification(ctx, alert.ID, channel)
	if err != nil {
		return false, "lookup notify history"
	}
	if last == nil {
		return true, ""
	}
	prev := parseAlertCountSnapshot(last.RawSummary)
	if alert.AlertCount <= prev {
		return false, "already notified for alert_count"
	}
	if !parseBoolTaskCenter(tc["notify_on_repeated_alert"], false) {
		return false, "repeated notify disabled"
	}
	return true, ""
}

func (s *Service) notifySeverityOK(alert TaskAlert, minSev string) bool {
	return severityOrder(alert.Severity) >= severityOrder(minSev)
}

func (s *Service) buildNotifyPayload(a TaskAlert, detailBase string) notify.AlertNotificationPayload {
	detail := detailURL(a.TaskType, a.SourceID)
	base := strings.TrimRight(strings.TrimSpace(detailBase), "/")
	if base != "" && detail != "" {
		detail = base + detail
	}
	return notify.AlertNotificationPayload{
		AlertID:           a.ID.String(),
		Severity:          a.Severity,
		FailureCategory:   a.FailureCategory,
		Title:             truncateRunes(a.Title, 240),
		Message:           truncateRunes(a.Message, 500),
		SuggestedAction:   truncateRunes(a.SuggestedAction, 400),
		TaskType:          a.TaskType,
		SourceID:          a.SourceID,
		DetailURL:         detail,
		OccurredAtRFC3339: a.LastSeenAt.UTC().Format(time.RFC3339),
	}
}

func (s *Service) mailDeps(ctx context.Context, an map[string]string) (notify.MailDeps, error) {
	var deps notify.MailDeps
	if s == nil || s.Settings == nil {
		return deps, errors.New("no settings")
	}
	m, err := s.Settings.PlainMailSettings(ctx)
	if err != nil {
		return deps, err
	}
	port := 587
	if p := strings.TrimSpace(m["smtp_port"]); p != "" {
		var parsed int
		fmt.Sscanf(p, "%d", &parsed)
		if parsed > 0 {
			port = parsed
		}
	}
	deps = notify.MailDeps{
		SMTPHost:          strings.TrimSpace(m["smtp_host"]),
		SMTPPort:          port,
		SMTPUser:          strings.TrimSpace(m["smtp_username"]),
		SMTPPassword:      m["smtp_password"],
		FromName:          strings.TrimSpace(m["smtp_from_name"]),
		From:              strings.TrimSpace(m["smtp_from"]),
		UseTLS:            strings.TrimSpace(m["smtp_use_tls"]) == "true",
		UseSSL:            strings.TrimSpace(m["smtp_use_ssl"]) == "true",
		MailTo:            strings.TrimSpace(an["mail_to"]),
		MailCC:            strings.TrimSpace(an["mail_cc"]),
		MailSubjectPrefix: strings.TrimSpace(an["mail_subject_prefix"]),
	}
	return deps, nil
}

func validateMailToBasic(s string) bool {
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, err := mail.ParseAddress(p); err != nil {
			return false
		}
	}
	return strings.TrimSpace(s) != ""
}

func (s *Service) persistNotification(ctx context.Context, alertID uuid.UUID, res notify.AlertNotificationResult) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("taskcenter: notification persistence unavailable")
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return s.persistNotificationTx(ctx, tx, alertID, res)
	})
}

func (s *Service) persistNotificationTx(ctx context.Context, tx *gorm.DB, alertID uuid.UUID, res notify.AlertNotificationResult) error {
	if tx == nil {
		return fmt.Errorf("taskcenter: notification persistence unavailable")
	}
	id := uuid.New()
	now := time.Now().UTC()
	sent := &now
	if res.Status != TaskAlertNotifStatusSuccess {
		sent = nil
	}
	var dj datatypes.JSON
	if len(res.RawSummary) > 0 {
		if b, err := json.Marshal(res.RawSummary); err == nil {
			dj = b
		}
	}
	row := TaskAlertNotification{
		ID:           id,
		AlertID:      alertID,
		Channel:      res.Channel,
		Status:       res.Status,
		Target:       truncateRunes(res.Target, 500),
		SentAt:       sent,
		ErrorMessage: truncateRunes(res.ErrorMessage, 2000),
		RawSummary:   dj,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return tx.WithContext(ctx).Create(&row).Error
}

func (s *Service) persistNotificationOutcome(
	ctx context.Context,
	ginC *gin.Context,
	tenantID int64,
	alertID uuid.UUID,
	channel string,
	res notify.AlertNotificationResult,
	action string,
	status string,
	message string,
) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("taskcenter: notification persistence unavailable")
	}
	if err := s.requireAudit(); err != nil {
		return errors.Join(errTaskAuditOutcomeUnavailable, err)
	}
	if res.Channel == "" {
		res.Channel = channel
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.persistNotificationTx(ctx, tx, alertID, res); err != nil {
			return err
		}
		opts := operationlog.WriteOpts{
			TenantID:   tenantID,
			Action:     action,
			Resource:   "task_alert_notification",
			ResourceID: alertID.String(),
			Status:     status,
			Message:    truncateRunes(fmt.Sprintf("alertId=%s channel=%s status=%s %s", alertID.String(), channel, status, message), 480),
		}
		if ginC != nil {
			return s.writeRequiredAuditTx(ginC, tx, opts)
		}
		return s.writeRequiredAuditBackgroundTx(ctx, tx, opts)
	})
}

func (s *Service) persistNotificationOutcomeOrAuditFailure(
	ctx context.Context,
	ginC *gin.Context,
	tenantID int64,
	alertID uuid.UUID,
	channel string,
	res notify.AlertNotificationResult,
	action string,
	status string,
	message string,
) error {
	if err := s.persistNotificationOutcome(ctx, ginC, tenantID, alertID, channel, res, action, status, message); err == nil {
		return nil
	} else {
		outcomeErr := errors.Join(errTaskAuditOutcomeUnavailable, err)
		failureErr := s.writeNotifyOpLog(ctx, ginC, tenantID, "task_center.alert.notify.failed", alertID, channel, "failed", "notification outcome persistence failed")
		if failureErr != nil {
			return errors.Join(outcomeErr, failureErr)
		}
		return outcomeErr
	}
}

func (s *Service) writeNotifyOpLog(ctx context.Context, ginC *gin.Context, tenantID int64, action string, alertID uuid.UUID, channel, status, msg string) error {
	if err := s.requireAudit(); err != nil {
		return errors.Join(errTaskAuditOutcomeUnavailable, err)
	}
	msg = truncateRunes(fmt.Sprintf("alertId=%s channel=%s status=%s %s", alertID.String(), channel, status, msg), 480)
	opts := operationlog.WriteOpts{
		TenantID:   tenantID,
		Action:     action,
		Resource:   "task_alert_notification",
		ResourceID: alertID.String(),
		Status:     status,
		Message:    msg,
	}
	var err error
	if ginC != nil {
		err = s.writeRequiredAudit(ginC, opts)
	} else {
		err = s.writeRequiredAuditBackground(ctx, opts)
	}
	if err != nil {
		return errors.Join(errTaskAuditOutcomeUnavailable, err)
	}
	return nil
}

// NotifyGeneratedAlerts sends external notifications for scan/generate candidates.
func (s *Service) NotifyGeneratedAlerts(ctx context.Context, candidates []alertNotifyCandidate, manual bool, ginC *gin.Context, channelFilter []string) error {
	if s == nil {
		return nil
	}
	filt := normalizeChannelFilter(channelFilter)
	if manual && len(filt) == 0 {
		return fmt.Errorf("%w: channels must contain at least one channel", errTaskInvalidRequest)
	}
	if len(candidates) == 0 {
		return nil
	}
	tc, err := taskCenterPlain(ctx, s.Settings)
	if err != nil {
		return errors.Join(errTaskDependencyUnavailable, err)
	}
	if manual && s.Settings == nil {
		return errTaskDependencyUnavailable
	}

	var effective []string
	if len(filt) > 0 {
		effective = filt
	} else {
		if !manual {
			if !parseBoolTaskCenter(tc["enable_external_notifications"], false) {
				return nil
			}
			if strings.TrimSpace(tc["notification_min_severity"]) == "" {
				return nil
			}
		}
		effective = parseJSONStringSlice(tc["notification_channels"])
	}
	if len(effective) == 0 {
		return nil
	}

	minSev := strings.TrimSpace(tc["notification_min_severity"])
	an, err := s.alertNotifyPlain(ctx)
	if err != nil {
		return errors.Join(errTaskDependencyUnavailable, err)
	}
	if len(an) == 0 {
		an = map[string]string{}
	}
	detailBase := strings.TrimSpace(tc["alert_detail_public_base"])

	for _, cand := range candidates {
		alert := cand.Alert
        if !manual && alert.TenantID > 0 {
            // Check the authenticated tenant's policy before any outbound
            // send. Missing migrations and SQL errors fail closed.
            if s.DB == nil {
                return errors.Join(errTaskDependencyUnavailable, errors.New("tenant notification database unavailable"))
            }
            prefs, _, err := tenantconfig.ReadNotifications(ctx, s.DB, alert.TenantID)
            if err != nil {
                return errors.Join(errTaskDependencyUnavailable, err)
            }
            if !prefs.BusinessAlertsEnabled {
                continue
            }
        }
		if !manual && minSev != "" && !s.notifySeverityOK(alert, minSev) {
			continue
		}
		payload := s.buildNotifyPayload(alert, detailBase)

		for _, ch := range effective {
			ch = strings.TrimSpace(strings.ToLower(ch))
			if ch == "" {
				continue
			}
			if !channelEnabled(an, ch) {
				res := notify.AlertNotificationResult{
					Channel:      ch,
					Status:       TaskAlertNotifStatusSkipped,
					ErrorMessage: "channel_disabled",
				}
				if err := s.persistNotificationOutcomeOrAuditFailure(ctx, ginC, alert.TenantID, alert.ID, ch, res, "task_center.alert.notify.skipped", "skipped", res.ErrorMessage); err != nil {
					return err
				}
				continue
			}
			ok, reason := s.shouldAutoNotifyChannel(ctx, alert, ch, cand.IsNew, tc, manual)
			if !ok {
				res := notify.AlertNotificationResult{
					Channel:      ch,
					Status:       TaskAlertNotifStatusSkipped,
					ErrorMessage: reason,
					Target:       ch + ":dedupe",
					RawSummary:   map[string]any{"reason": reason},
				}
				if err := s.persistNotificationOutcomeOrAuditFailure(
					ctx, ginC, alert.TenantID, alert.ID, ch, res,
					"task_center.alert.notify.skipped", "skipped", reason,
				); err != nil {
					return err
				}
				continue
			}
			requested := operationlog.WriteOpts{
				TenantID: alert.TenantID, Action: "task_center.alert.notify.channel.requested",
				Resource: "task_alert_notification", ResourceID: alert.ID.String(), Status: "requested",
				Message: truncateRunes(fmt.Sprintf("channel=%s", ch), 240),
			}
			var auditErr error
			if ginC != nil {
				auditErr = s.writeRequiredAudit(ginC, requested)
			} else {
				auditErr = s.writeRequiredAuditBackground(ctx, requested)
			}
			if auditErr != nil {
				return auditErr
			}

			var res notify.AlertNotificationResult
			switch ch {
			case "mail":
				if !parseBoolTaskCenter(an["mail_enabled"], false) {
					res = notify.AlertNotificationResult{Channel: ch, Status: TaskAlertNotifStatusSkipped, ErrorMessage: "mail_disabled"}
					break
				}
				if !validateMailToBasic(an["mail_to"]) {
					res = notify.AlertNotificationResult{Channel: ch, Status: TaskAlertNotifStatusSkipped, ErrorMessage: "mail_to invalid"}
					break
				}
				md, err := s.mailDeps(ctx, an)
				if err != nil {
					res = notify.AlertNotificationResult{Channel: ch, Status: TaskAlertNotifStatusSkipped, ErrorMessage: "mail settings"}
					break
				}
				res = notify.SendMail(ctx, md, payload)
				if res.Status == TaskAlertNotifStatusSuccess {
					if res.RawSummary == nil {
						res.RawSummary = map[string]any{}
					}
					res.RawSummary["alertCount"] = alert.AlertCount
				}
			case "webhook":
				if !parseBoolTaskCenter(an["webhook_enabled"], false) {
					res = notify.AlertNotificationResult{Channel: ch, Status: TaskAlertNotifStatusSkipped, ErrorMessage: "webhook_disabled"}
					break
				}
				sec := time.Duration(webhookNotifyTimeoutSeconds(an)) * time.Second
				allowHTTP := s.Cfg != nil && s.Cfg.AppEnv != "production"
				res = notify.SendWebhook(ctx, notify.WebhookDeps{
					URL:       strings.TrimSpace(an["webhook_url"]),
					Method:    strings.TrimSpace(an["webhook_method"]),
					Secret:    strings.TrimSpace(an["webhook_secret"]),
					Timeout:   sec,
					AllowHTTP: allowHTTP,
				}, payload)
				if res.Status == TaskAlertNotifStatusSuccess {
					merged := map[string]any{"alertCount": alert.AlertCount}
					if res.RawSummary != nil {
						for k, v := range res.RawSummary {
							merged[k] = v
						}
					}
					res.RawSummary = merged
				}
			case "feishu":
				sec := time.Duration(webhookNotifyTimeoutSeconds(an)) * time.Second
				allowHTTP := s.Cfg != nil && s.Cfg.AppEnv != "production"
				res = notify.SendFeishu(ctx, notify.FeishuDeps{
					URL:       strings.TrimSpace(an["feishu_webhook_url"]),
					Secret:    strings.TrimSpace(an["feishu_secret"]),
					Timeout:   sec,
					AllowHTTP: allowHTTP,
				}, payload)
				res = withAlertCount(res, alert.AlertCount)
			case "wecom":
				sec := time.Duration(webhookNotifyTimeoutSeconds(an)) * time.Second
				allowHTTP := s.Cfg != nil && s.Cfg.AppEnv != "production"
				res = notify.SendWeCom(ctx, notify.WeComDeps{
					URL:       strings.TrimSpace(an["wecom_webhook_url"]),
					Timeout:   sec,
					AllowHTTP: allowHTTP,
				}, payload)
				res = withAlertCount(res, alert.AlertCount)
			default:
				res = notify.AlertNotificationResult{Channel: ch, Status: TaskAlertNotifStatusSkipped, ErrorMessage: "unknown channel"}
			}

			action := "task_center.alert.notify.skipped"
			status := TaskAlertNotifStatusSkipped
			message := res.ErrorMessage
			switch res.Status {
			case TaskAlertNotifStatusSuccess:
				action = "task_center.alert.notify.success"
				status = "success"
				message = ""
			case TaskAlertNotifStatusFailed:
				action = "task_center.alert.notify.failed"
				status = "failed"
				message = "notification_failed"
			}
			if err := s.persistNotificationOutcomeOrAuditFailure(ctx, ginC, alert.TenantID, alert.ID, ch, res, action, status, message); err != nil {
				return err
			}
		}
	}
	return nil
}

func withAlertCount(res notify.AlertNotificationResult, alertCount int) notify.AlertNotificationResult {
	if res.Status != TaskAlertNotifStatusSuccess {
		return res
	}
	if res.RawSummary == nil {
		res.RawSummary = map[string]any{}
	}
	res.RawSummary["alertCount"] = alertCount
	return res
}

func channelEnabled(an map[string]string, ch string) bool {
	if !parseBoolTaskCenter(an["enabled"], false) {
		return false
	}
	switch ch {
	case "mail":
		return parseBoolTaskCenter(an["mail_enabled"], false)
	case "webhook":
		return parseBoolTaskCenter(an["webhook_enabled"], false)
	case "feishu":
		return parseBoolTaskCenter(an["feishu_enabled"], false)
	case "wecom":
		return parseBoolTaskCenter(an["wecom_enabled"], false)
	default:
		return true
	}
}

// --- list / manual API helpers ---

// ListAlertNotificationsParams binds GET /alert-notifications.
type ListAlertNotificationsParams struct {
	TenantID          int64
	AllowedShopIDs    []uuid.UUID
	RestrictShopScope bool
	AlertID           *uuid.UUID
	Channel           string
	Status            string
	Start             *time.Time
	End               *time.Time
	Page              int
	PageSz            int
}

// TaskAlertNotificationDTO is API shape for one notify audit row.
type TaskAlertNotificationDTO struct {
	ID           string         `json:"id"`
	AlertID      string         `json:"alertId"`
	Channel      string         `json:"channel"`
	Status       string         `json:"status"`
	Target       string         `json:"target,omitempty"`
	SentAt       *time.Time     `json:"sentAt,omitempty"`
	ErrorMessage string         `json:"errorMessage,omitempty"`
	RetryCount   int            `json:"retryCount"`
	RawSummary   map[string]any `json:"rawSummary,omitempty"`
	CreatedAt    time.Time      `json:"createdAt"`
}

// ListAlertNotificationsResult is paged notifications.
type ListAlertNotificationsResult struct {
	List  []TaskAlertNotificationDTO `json:"list"`
	Total int64                      `json:"total"`
}

func clampPageNotif(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

// ListAlertNotifications page.
func (s *Service) ListAlertNotifications(ctx context.Context, p ListAlertNotificationsParams) (ListAlertNotificationsResult, error) {
	var out ListAlertNotificationsResult
	if s == nil || s.DB == nil {
		return out, fmt.Errorf("taskcenter: no db")
	}
	page, ps := clampPageNotif(p.Page, p.PageSz)
	q := s.DB.WithContext(ctx).Model(&TaskAlertNotification{})
	alerts := s.DB.WithContext(ctx).Model(&TaskAlert{}).Where("tenant_id = ?", p.TenantID)
	alerts = applyTaskAlertStoreScope(alerts, p.TenantID, p.RestrictShopScope, p.AllowedShopIDs)
	q = q.Where("alert_id IN (?)", alerts.Select("id"))
	if p.AlertID != nil && *p.AlertID != uuid.Nil {
		q = q.Where("alert_id = ?", *p.AlertID)
	}
	if ch := strings.TrimSpace(p.Channel); ch != "" {
		q = q.Where("channel = ?", ch)
	}
	if st := strings.TrimSpace(p.Status); st != "" {
		q = q.Where("status = ?", st)
	}
	if p.Start != nil {
		q = q.Where("created_at >= ?", *p.Start)
	}
	if p.End != nil {
		q = q.Where("created_at <= ?", *p.End)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return out, err
	}
	var rows []TaskAlertNotification
	off := (page - 1) * ps
	if err := q.Order("created_at DESC").Offset(off).Limit(ps).Find(&rows).Error; err != nil {
		return out, err
	}
	list := make([]TaskAlertNotificationDTO, 0, len(rows))
	for i := range rows {
		dto := TaskAlertNotificationDTO{
			ID:           rows[i].ID.String(),
			AlertID:      rows[i].AlertID.String(),
			Channel:      rows[i].Channel,
			Status:       rows[i].Status,
			Target:       rows[i].Target,
			SentAt:       rows[i].SentAt,
			ErrorMessage: rows[i].ErrorMessage,
			RetryCount:   rows[i].RetryCount,
			CreatedAt:    rows[i].CreatedAt,
		}
		if len(rows[i].RawSummary) > 0 {
			var m map[string]any
			if err := json.Unmarshal(rows[i].RawSummary, &m); err == nil {
				dto.RawSummary = m
			}
		}
		list = append(list, dto)
	}
	out.List = list
	out.Total = total
	return out, nil
}

// NotifyTaskAlertManual triggers outbound notifications for one alert (honours channel list).
func (s *Service) NotifyTaskAlertManual(ctx context.Context, c *gin.Context, alertID uuid.UUID, channels []string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("taskcenter: no db")
	}
	filter := normalizeChannelFilter(channels)
	if len(filter) == 0 {
		return fmt.Errorf("%w: channels must contain at least one channel", errTaskInvalidRequest)
	}
	var a TaskAlert
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	if err := s.DB.WithContext(ctx).Where("id = ? AND tenant_id = ?", alertID, tenantID).First(&a).Error; err != nil {
		return err
	}
	if err := s.ensureTaskAlertStoreScope(c, a, true); err != nil {
		return err
	}
	if err := s.writeRequiredAudit(c, operationlog.WriteOpts{
		TenantID: tenantID, Action: "task_center.alert.notify.requested", Resource: "task_alert_notification",
		ResourceID: a.ID.String(), Status: "requested",
		Message: truncateRunes("channels="+strings.Join(filter, ","), 240),
	}); err != nil {
		return err
	}
	return s.NotifyGeneratedAlerts(ctx, []alertNotifyCandidate{{Alert: a, IsNew: false}}, true, c, filter)
}

func (s *Service) notificationBadgeMap(ctx context.Context, ids []uuid.UUID) map[uuid.UUID]string {
	out := map[uuid.UUID]string{}
	if s == nil || s.DB == nil || len(ids) == 0 {
		return out
	}
	for _, id := range ids {
		out[id] = "none"
	}

	type row struct {
		AlertID uuid.UUID `gorm:"column:alert_id"`
		Status  string    `gorm:"column:status"`
	}
	var rows []row
	err := s.DB.WithContext(ctx).Raw(`
		SELECT n.alert_id, n.status
		FROM task_alert_notifications n
		INNER JOIN (
			SELECT alert_id, MAX(created_at) AS mc
			FROM task_alert_notifications
			WHERE alert_id IN ?
			GROUP BY alert_id
		) t ON t.alert_id = n.alert_id AND t.mc = n.created_at
	`, ids).Scan(&rows).Error
	if err != nil {
		return out
	}
	for _, r := range rows {
		switch strings.ToLower(strings.TrimSpace(r.Status)) {
		case TaskAlertNotifStatusFailed:
			out[r.AlertID] = "failed"
		case TaskAlertNotifStatusSuccess:
			out[r.AlertID] = "ok"
		default:
			out[r.AlertID] = "none"
		}
	}
	return out
}
