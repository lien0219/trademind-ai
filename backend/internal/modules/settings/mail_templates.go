package settings

import (
	"context"
	"encoding/json"
	"errors"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"github.com/trademind-ai/trademind/backend/internal/providers/email"
	"github.com/trademind-ai/trademind/backend/internal/providers/email/smtp"
	"gorm.io/gorm"
)

const templateGroup = "mail_templates"

var (
	mailTemplateVars = regexp.MustCompile(`\{\{([a-z_]+)\}\}`)
	mailTemplateUnsafe = regexp.MustCompile(`(?i)<\s*(script|iframe|object|embed|form|base|svg|meta|link)\b|on[a-z]+\s*=|javascript:|data:text/html|<\s*/?\s*style\b`)
)

var mailLocales = []string{"zh-CN", "en-US", "ja-JP", "zh-TW", "ko-KR", "th-TH"}

type MailEvent struct {
	Key          string   `json:"key"`
	Category     string   `json:"category"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Placeholders []string `json:"placeholders"`
}

var mailEvents = []MailEvent{
	{"auth.verify_code", "authentication", "邮箱验证码", "账号注册与邮箱验证", []string{"site_name", "recipient_email", "verification_code", "expires_in_minutes"}},
	{"auth.password_reset", "authentication", "密码重置", "自助重置密码链接", []string{"site_name", "recipient_email", "reset_url", "verification_code", "expires_in_minutes"}},
	{"auth.registration_welcome", "authentication", "注册欢迎", "邮箱注册完成通知", []string{"site_name", "recipient_email", "login_url"}},
	{"tenant.invitation", "organization", "企业邀请", "成员加入企业的邀请邮件", []string{"site_name", "recipient_email", "tenant_name", "invite_url", "expires_in_minutes"}},
	{"tenant.member_added", "organization", "成员加入通知", "企业成员创建后通知", []string{"site_name", "recipient_email", "tenant_name"}},
	{"security.account_alert", "security", "账号安全提醒", "密码变化或登录安全事件", []string{"site_name", "recipient_email", "event_name", "occurred_at"}},
	{"task.failed", "operations", "任务失败", "异步任务执行失败时提醒", []string{"site_name", "recipient_email", "task_name", "event_name"}},
	{"report.ready", "operations", "报表完成", "导出报表生成成功", []string{"site_name", "recipient_email", "report_name", "report_url"}},
	{"subscription.expiry", "billing", "订阅到期提醒", "套餐即将到期", []string{"site_name", "recipient_email", "expires_at", "renew_url"}},
	{"billing.payment", "billing", "支付成功通知", "付款成功或账单记录", []string{"site_name", "recipient_email", "amount", "order_no"}},
}

type MailTemplate struct {
	Event      string `json:"event"`
	Locale     string `json:"locale"`
	Subject    string `json:"subject"`
	HTML       string `json:"html"`
	Customized bool   `json:"customized"`
}

func lookupMailEvent(key string) (MailEvent, bool) {
	for _, event := range mailEvents {
		if event.Key == key {
			return event, true
		}
	}
	return MailEvent{}, false
}

func validMailLocale(locale string) bool {
	for _, item := range mailLocales {
		if locale == item {
			return true
		}
	}
	return false
}

func defaultMailTemplate(event, locale string) MailTemplate {
	ev, _ := lookupMailEvent(event)
	title, body := ev.Name, ev.Description
	if locale == "en-US" {
		switch event {
		case "auth.verify_code": title, body = "Email verification code", "Use this code to verify your account."
		case "auth.password_reset": title, body = "Reset your password", "You requested a password reset."
		case "tenant.invitation": title, body = "Organization invitation", "You have been invited to join an organization."
		default: title, body = "Notification", "This is an automated notification."
		}
	}
	if locale == "ja-JP" {
		title, body = "お知らせ", "アカウントへの通知です。"
	}
	if locale == "zh-TW" {
		title, body = "系統通知", "這是一則系統通知。"
	}
	if locale == "ko-KR" {
		title, body = "알림", "계정에 대한 알림입니다."
	}
	if locale == "th-TH" {
		title, body = "การแจ้งเตือน", "นี่คือการแจ้งเตือนจากระบบ"
	}
	primary := "{{event_name}}"
	switch event {
	case "auth.verify_code": primary = "{{verification_code}}"
	case "auth.password_reset": primary = "{{verification_code}}"
	case "tenant.invitation": primary = "{{invite_url}}"
	case "auth.registration_welcome": primary = "{{login_url}}"
	case "report.ready": primary = "{{report_url}}"
	case "subscription.expiry": primary = "{{expires_at}}"
	case "billing.payment": primary = "{{amount}}"
	case "task.failed": primary = "{{task_name}}"
	case "tenant.member_added": primary = "{{tenant_name}}"
	}
	return MailTemplate{
		Event: event, Locale: locale, Subject: "[{{site_name}}] " + title,
		HTML: "<!doctype html><html><head></head><body style=\"font-family:Arial,sans-serif;background:#f3f5fa;padding:24px\"><div style=\"max-width:560px;margin:auto;background:white;padding:32px;border-radius:12px;border:1px solid #e4e7ee\"><h2 style=\"color:#22306b\">{{site_name}}</h2><h3>" + title + "</h3><p>" + body + "</p><p style=\"padding:16px;background:#eef2ff;overflow-wrap:anywhere;font-weight:bold\">" + primary + "</p><p style=\"color:#667085;font-size:12px\">{{recipient_email}}</p></div></body></html>",
	}
}

func validateMailTemplate(template MailTemplate) error {
	ev, ok := lookupMailEvent(template.Event)
	if !ok || !validMailLocale(template.Locale) || len(template.Subject) == 0 ||
		len(template.Subject) > 300 || len(template.HTML) == 0 || len(template.HTML) > 100000 {
		return errors.New("invalid email template")
	}
	if strings.ContainsAny(template.Subject, "\r\n") || mailTemplateUnsafe.MatchString(template.HTML) {
		return errors.New("unsafe email template")
	}
	allow := map[string]bool{}
	for _, name := range ev.Placeholders {
		allow[name] = true
	}
	for _, part := range []string{template.Subject, template.HTML} {
		for _, match := range mailTemplateVars.FindAllStringSubmatch(part, -1) {
			if !allow[match[1]] {
				return errors.New("unknown email placeholder")
			}
		}
		if left := mailTemplateVars.ReplaceAllString(part, ""); strings.Contains(left, "{{") || strings.Contains(left, "}}") {
			return errors.New("invalid email placeholder")
		}
	}
	return nil
}

func RenderMailTemplate(t MailTemplate, values map[string]string) (string, string, error) {
	if err := validateMailTemplate(t); err != nil {
		return "", "", err
	}
	render := func(s string) string {
		return mailTemplateVars.ReplaceAllStringFunc(s, func(raw string) string {
			name := strings.TrimSuffix(strings.TrimPrefix(raw, "{{"), "}}")
			return html.EscapeString(values[name])
		})
	}
	return render(t.Subject), render(t.HTML), nil
}

func (s *Service) GetMailTemplate(ctx context.Context, event, locale string) (MailTemplate, error) {
	if _, ok := lookupMailEvent(event); !ok || !validMailLocale(locale) {
		return MailTemplate{}, errors.New("invalid email template selector")
	}
	var row Setting
	err := s.DB.WithContext(ctx).Where("tenant_id = 0 AND group_key = ? AND item_key = ?", templateGroup, event+"."+locale).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return defaultMailTemplate(event, locale), nil
	}
	if err != nil {
		return MailTemplate{}, err
	}
	var t MailTemplate
	if err := json.Unmarshal([]byte(row.ItemValue), &t); err != nil || t.Event != event || t.Locale != locale || validateMailTemplate(t) != nil {
		return MailTemplate{}, errors.New("stored email template invalid")
	}
	t.Customized = true
	return t, nil
}

func (h *Handler) MailTemplateOptions(c *gin.Context) {
	if !h.requireSettingsManage(c) { return }
	response.OK(c, gin.H{"events": mailEvents, "locales": mailLocales})
}

func (h *Handler) MailTemplateGet(c *gin.Context) {
	if !h.requireSettingsManage(c) { return }
	t, err := h.Svc.GetMailTemplate(c.Request.Context(), c.Query("event"), c.Query("locale"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid email template selection")
		return
	}
	response.OK(c, t)
}

func (h *Handler) MailTemplatePreview(c *gin.Context) {
	if !h.requireSettingsManage(c) { return }
	var t MailTemplate
	if err := c.ShouldBindJSON(&t); err != nil || validateMailTemplate(t) != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid email template")
		return
	}
	values := map[string]string{
		"site_name": "TradeMind", "recipient_email": "demo@example.com",
		"verification_code": "123456", "expires_in_minutes": "15", "reset_url": "https://example.com/reset",
		"invite_url": "https://example.com/invite", "login_url": "https://example.com/login",
		"tenant_name": "Demo Company", "event_name": "Security alert", "occurred_at": "2026-11-03",
		"task_name": "Export task", "report_name": "Revenue report", "report_url": "https://example.com/report",
		"expires_at": "2026-12-03", "renew_url": "https://example.com/renew",
		"amount": "USD 10.00", "order_no": "TM-001",
	}
	subject, body, err := RenderMailTemplate(t, values)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "template preview unavailable")
		return
	}
	response.OK(c, gin.H{"subject": subject, "html": body})
}

func (h *Handler) MailTemplateSave(c *gin.Context) {
	if !h.requireSettingsManage(c) { return }
	var t MailTemplate
	if err := c.ShouldBindJSON(&t); err != nil || validateMailTemplate(t) != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid email template")
		return
	}
	if h.OpLog == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, http.StatusServiceUnavailable, response.CodeInternalError, "email settings audit unavailable")
		return
	}
	payload, err := json.Marshal(t)
	if err != nil { response.HandleError(c, err); return }
	err = h.Svc.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := h.Svc.PutBulkTx(tx, []PutItem{{
			TenantID: 0, GroupKey: templateGroup, ItemKey: t.Event+"."+t.Locale,
			ItemValue: string(payload), ValueType: "json",
		}}); err != nil { return err }
		return h.OpLog.WriteTx(c, tx, operationlog.WriteOpts{
			Action: "mail.template.save", Resource: "mail_template",
			ResourceID: t.Event+"."+t.Locale, Status: "success",
		})
	})
	if err != nil { response.HandleError(c, err); return }
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) MailTemplateRestore(c *gin.Context) {
	if !h.requireSettingsManage(c) { return }
	var input struct { Event string `json:"event"`; Locale string `json:"locale"` }
	if err := c.ShouldBindJSON(&input); err != nil { response.Fail(c, 400, response.CodeBadRequest, "invalid input"); return }
	if _, ok := lookupMailEvent(input.Event); !ok || !validMailLocale(input.Locale) {
		response.Fail(c, 400, response.CodeBadRequest, "invalid email template selection")
		return
	}
	if h.OpLog == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, 503, response.CodeInternalError, "email settings audit unavailable")
		return
	}
	err := h.Svc.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tenant_id = 0 AND group_key = ? AND item_key = ?", templateGroup, input.Event+"."+input.Locale).Delete(&Setting{}).Error; err != nil {
			return err
		}
		return h.OpLog.WriteTx(c, tx, operationlog.WriteOpts{
			Action: "mail.template.restore", Resource: "mail_template",
			ResourceID: input.Event+"."+input.Locale, Status: "success",
		})
	})
	if err != nil { response.HandleError(c, err); return }
	response.OK(c, gin.H{"ok": true})
}


// SendEventEmail sends an event-specific HTML notification with the same
// server-side templates used by the editor. SMTP credentials never leave the
// backend and no delivery is attempted without verified TLS.
func (s *Service) SendEventEmail(ctx context.Context, event, locale, recipient string, values map[string]string) error {
	if !validMailLocale(locale) {
		locale = "zh-CN"
	}
	t, err := s.GetMailTemplate(ctx, event, locale)
	if err != nil {
		return err
	}
	if values == nil {
		values = make(map[string]string)
	}
	values["recipient_email"] = recipient
	if values["site_name"] == "" {
		values["site_name"] = "TradeMind"
	}
	subject, body, err := RenderMailTemplate(t, values)
	if err != nil {
		return err
	}
	m, err := s.PlainMailSettings(ctx)
	if err != nil {
		return err
	}
	if m["provider"] != "" && m["provider"] != "smtp" {
		return errors.New("unsupported email provider")
	}
	port, err := strconv.Atoi(m["smtp_port"])
	if err != nil {
		return errors.New("invalid mail port")
	}
	cfg := smtp.Config{
		Host: m["smtp_host"], Port: port, Username: m["smtp_username"],
		Password: m["smtp_password"], From: m["smtp_from"],
		FromName: m["smtp_from_name"],
		UseTLS: m["smtp_use_tls"] == "true", UseSSL: m["smtp_use_ssl"] == "true",
	}
	return smtp.NewProvider(cfg).Send(ctx, email.SendEmailRequest{
		To: recipient, Subject: subject, Content: body, ContentType: "text/html",
	})
}
