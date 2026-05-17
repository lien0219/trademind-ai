package settings

import (
	"context"

	"gorm.io/gorm"
)

// EnsureAlertNotifyDefaults seeds alert outbound notification settings (idempotent).
func EnsureAlertNotifyDefaults(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return nil
	}
	type def struct {
		key       string
		val       string
		enc       bool
		valueType string
		remark    string
	}
	defs := []def{
		{"enabled", "false", false, "string", "总开关：关闭时不发送外部通知（手动通知仍可尝试，视告警存在性）"},
		{"channels", "[]", false, "string", "展示的可用通道列表（JSON 数组，与 taskcenter.notification_channels 协同）"},
		{"mail_enabled", "false", false, "string", ""},
		{"mail_to", "", false, "string", "逗号分隔收件人"},
		{"mail_cc", "", false, "string", ""},
		{"mail_subject_prefix", "", false, "string", ""},
		{"webhook_enabled", "false", false, "string", ""},
		{"webhook_url", "", true, "string", ""},
		{"webhook_method", "", false, "string", ""},
		{"webhook_secret", "", true, "string", ""},
		{"webhook_timeout_seconds", "", false, "string", "HTTP 超时秒数，留空则后端使用引擎默认值"},
		{"webhook_template", "", false, "string", "预留：首版未使用自定义模板"},
		{"feishu_enabled", "false", false, "string", "预留，发送为 planned"},
		{"feishu_webhook_url", "", true, "string", ""},
		{"feishu_secret", "", true, "string", ""},
		{"wecom_enabled", "false", false, "string", "预留，发送为 planned"},
		{"wecom_webhook_url", "", true, "string", ""},
	}
	for _, d := range defs {
		var n int64
		if err := db.WithContext(ctx).Model(&Setting{}).
			Where("tenant_id = ? AND group_key = ? AND item_key = ?", 0, "alert_notify", d.key).
			Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		row := Setting{
			TenantID:    0,
			GroupKey:    "alert_notify",
			ItemKey:     d.key,
			ItemValue:   d.val,
			ValueType:   d.valueType,
			IsEncrypted: d.enc,
			Remark:      d.remark,
		}
		if err := db.WithContext(ctx).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}
