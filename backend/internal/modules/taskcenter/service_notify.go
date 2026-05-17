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

func (s *Service) persistNotification(ctx context.Context, alertID uuid.UUID, res notify.AlertNotificationResult) {
	if s == nil || s.DB == nil {
		return
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
	_ = s.DB.WithContext(ctx).Create(&row).Error
}

func (s *Service) writeNotifyOpLog(ginC *gin.Context, action string, alertID uuid.UUID, channel, status, msg string) {
	if s == nil || s.OpLog == nil {
		return
	}
	msg = truncateRunes(fmt.Sprintf("alertId=%s channel=%s status=%s %s", alertID.String(), channel, status, msg), 480)
	opts := operationlog.WriteOpts{
		Action:     action,
		Resource:   "task_alert_notification",
		ResourceID: alertID.String(),
		Status:     status,
		Message:    msg,
	}
	if ginC != nil {
		_ = s.OpLog.Write(ginC, opts)
		return
	}
	_ = s.OpLog.WriteBackground(context.Background(), opts)
}

// NotifyGeneratedAlerts sends external notifications for scan/generate candidates (best-effort).
func (s *Service) NotifyGeneratedAlerts(ctx context.Context, candidates []alertNotifyCandidate, manual bool, ginC *gin.Context, channelFilter []string) {
	if s == nil || len(candidates) == 0 {
		return
	}
	tc := taskCenterPlain(ctx, s.Settings)
	filt := normalizeChannelFilter(channelFilter)

	var effective []string
	if len(filt) > 0 {
		effective = filt
	} else {
		if !manual {
			if !parseBoolTaskCenter(tc["enable_external_notifications"], false) {
				return
			}
			if strings.TrimSpace(tc["notification_min_severity"]) == "" {
				return
			}
		}
		effective = parseJSONStringSlice(tc["notification_channels"])
	}
	if len(effective) == 0 {
		return
	}

	minSev := strings.TrimSpace(tc["notification_min_severity"])
	an, err := s.alertNotifyPlain(ctx)
	if err != nil || len(an) == 0 {
		an = map[string]string{}
	}
	detailBase := strings.TrimSpace(tc["alert_detail_public_base"])

	for _, cand := range candidates {
		alert := cand.Alert
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
				continue
			}
			ok, reason := s.shouldAutoNotifyChannel(ctx, alert, ch, cand.IsNew, tc, manual)
			if !ok {
				s.persistNotification(ctx, alert.ID, notify.AlertNotificationResult{
					Channel:      ch,
					Status:       TaskAlertNotifStatusSkipped,
					ErrorMessage: reason,
					Target:       ch + ":dedupe",
					RawSummary:   map[string]any{"reason": reason},
				})
				continue
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
				res = notify.PlannedSender{Channel: "feishu", Reason: "feishu integration planned"}.Send(ctx, payload)
			case "wecom":
				res = notify.PlannedSender{Channel: "wecom", Reason: "wecom integration planned"}.Send(ctx, payload)
			default:
				res = notify.AlertNotificationResult{Channel: ch, Status: TaskAlertNotifStatusSkipped, ErrorMessage: "unknown channel"}
			}

			s.persistNotification(ctx, alert.ID, res)

			switch res.Status {
			case TaskAlertNotifStatusSuccess:
				s.writeNotifyOpLog(ginC, "task_center.alert.notify.success", alert.ID, ch, "success", "")
			case TaskAlertNotifStatusFailed:
				s.writeNotifyOpLog(ginC, "task_center.alert.notify.failed", alert.ID, ch, "failed", res.ErrorMessage)
			}
		}
	}
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
	AlertID *uuid.UUID
	Channel string
	Status  string
	Start   *time.Time
	End     *time.Time
	Page    int
	PageSz  int
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
	var a TaskAlert
	if err := s.DB.WithContext(ctx).First(&a, "id = ?", alertID).Error; err != nil {
		return err
	}
	filter := make([]string, 0, len(channels))
	for _, ch := range channels {
		ch = strings.TrimSpace(strings.ToLower(ch))
		if ch != "" {
			filter = append(filter, ch)
		}
	}
	var filt []string
	if len(filter) > 0 {
		filt = filter
	}
	s.NotifyGeneratedAlerts(ctx, []alertNotifyCandidate{{Alert: a, IsNew: false}}, true, c, filt)
	return nil
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
