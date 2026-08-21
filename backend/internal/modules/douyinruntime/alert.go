package douyinruntime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter/failureclassifier"
	"gorm.io/gorm"
)

const taskTypeDouyinPlatform = "douyin_platform"

// Douyin alert type keys stored in TaskAlert.failure_category.
const (
	AlertTokenRefreshFailed     = "douyin_token_refresh_failed"
	AlertAuthExpiring           = "douyin_auth_expiring"
	AlertAuthExpired            = "douyin_auth_expired"
	AlertAuthNeedCheck          = "douyin_auth_need_check"
	AlertProductDraftFailures   = "douyin_product_draft_failures"
	AlertProductResultUnknown   = "douyin_product_result_unknown"
	AlertProductRecoveryFailed  = "douyin_product_recovery_failed"
	AlertImageUploadFailureRate = "douyin_image_upload_failure_rate"
	AlertStoragePublicFailed    = "douyin_storage_public_failed"
	AlertOrderSyncFailed        = "douyin_order_sync_failed"
	AlertOrderPartialStale      = "douyin_order_partial_stale"
	AlertInventorySyncFailed    = "douyin_inventory_sync_failed"
	AlertInventoryStale         = "douyin_inventory_stale"
	AlertRuntimeEmergency       = "douyin_runtime_emergency_disabled"
	AlertStaleTasksHigh         = "douyin_stale_tasks_high"
	AlertFailureBacklog         = "douyin_failure_backlog"
	AlertRateLimitSpike         = "douyin_rate_limit_spike"
)

// UpsertDouyinAlert upserts a legacy system-scoped alert (tenant 0).
// Tenant-aware callers should use UpsertDouyinAlertForTenant.
func (s *Service) UpsertDouyinAlert(ctx context.Context, sourceID, alertType, severity, title, message, suggested string, now time.Time) error {
	return s.UpsertDouyinAlertForTenant(ctx, 0, sourceID, alertType, severity, title, message, suggested, now)
}

// UpsertDouyinAlertForTenant upserts a platform-level alert with tenant-scoped dedup.
func (s *Service) UpsertDouyinAlertForTenant(ctx context.Context, tenantID int64, sourceID, alertType, severity, title, message, suggested string, now time.Time) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("douyinruntime: no db")
	}
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		sourceID = "global"
	}
	alertType = strings.TrimSpace(alertType)
	if alertType == "" {
		return fmt.Errorf("alert type required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	title = truncateAlertText(title, 255)
	message = truncateAlertText(message, 1200)
	suggested = truncateAlertText(suggested, 600)

	var cur taskcenter.TaskAlert
	err := s.DB.WithContext(ctx).
		Where("tenant_id = ? AND task_type = ? AND source_id = ? AND failure_category = ?", tenantID, taskTypeDouyinPlatform, sourceID, alertType).
		First(&cur).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row := taskcenter.TaskAlert{
			ID:              uuid.New(),
			TenantID:        tenantID,
			TaskType:        taskTypeDouyinPlatform,
			SourceID:        sourceID,
			SourceTable:     "douyin_platform",
			FailureCategory: alertType,
			Severity:        severity,
			Platform:        "douyin_shop",
			Title:           title,
			Message:         message,
			SuggestedAction: suggested,
			Status:          taskcenter.TaskAlertStatusOpen,
			AlertCount:      1,
			FirstSeenAt:     now,
			LastSeenAt:      now,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		return s.DB.WithContext(ctx).Create(&row).Error
	}
	if err != nil {
		return err
	}
	if strings.EqualFold(cur.Status, taskcenter.TaskAlertStatusIgnored) || strings.EqualFold(cur.Status, taskcenter.TaskAlertStatusHandled) {
		return nil
	}
	return s.DB.WithContext(ctx).Model(&taskcenter.TaskAlert{}).Where("id = ? AND tenant_id = ?", cur.ID, tenantID).
		Updates(map[string]any{
			"last_seen_at":     now,
			"updated_at":       now,
			"severity":         severity,
			"message":          message,
			"suggested_action": suggested,
			"title":            title,
			"status":           taskcenter.TaskAlertStatusOpen,
			"alert_count":      gorm.Expr("alert_count + 1"),
		}).Error
}

// ResolveDouyinAlert marks a legacy system-scoped alert as resolved (tenant 0).
// Tenant-aware callers should use ResolveDouyinAlertForTenant.
func (s *Service) ResolveDouyinAlert(ctx context.Context, sourceID, alertType string, now time.Time) error {
	return s.ResolveDouyinAlertForTenant(ctx, 0, sourceID, alertType, now)
}

// ResolveDouyinAlertForTenant resolves an open alert within one tenant only.
func (s *Service) ResolveDouyinAlertForTenant(ctx context.Context, tenantID int64, sourceID, alertType string, now time.Time) error {
	if s == nil || s.DB == nil {
		return nil
	}
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		sourceID = "global"
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return s.DB.WithContext(ctx).Model(&taskcenter.TaskAlert{}).
		Where("tenant_id = ? AND task_type = ? AND source_id = ? AND failure_category = ? AND status = ?",
			tenantID, taskTypeDouyinPlatform, sourceID, strings.TrimSpace(alertType), taskcenter.TaskAlertStatusOpen).
		Updates(map[string]any{
			"status":     taskcenter.TaskAlertStatusResolved,
			"updated_at": now,
		}).Error
}

func truncateAlertText(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len([]rune(s)) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max])
}

func douyinAlertSettings(ctx context.Context, settingsSvc *settings.Service, tenantID int64) map[string]string {
	if settingsSvc == nil {
		return map[string]string{}
	}
	base, baseErr := settingsSvc.PlainByGroup(ctx, 0, "platform_douyin_shop")
	if tenantID == 0 {
		if baseErr != nil || base == nil {
			return map[string]string{}
		}
		return base
	}
	overrides, overrideErr := settingsSvc.PlainByGroup(ctx, tenantID, "platform_douyin_shop")
	if baseErr != nil && overrideErr != nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(base)+len(overrides))
	for k, v := range base {
		out[k] = v
	}
	if overrideErr == nil {
		for k, v := range overrides {
			out[k] = v
		}
	}
	return out
}

func alertThresholdInt(m map[string]string, key string, def int) int {
	raw := strings.TrimSpace(m[key])
	if raw == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(raw, "%d", &n); err != nil || n <= 0 {
		return def
	}
	return n
}

func alertSeverity(level string) string {
	switch strings.TrimSpace(strings.ToLower(level)) {
	case failureclassifier.SeverityCritical, failureclassifier.SeverityHigh, failureclassifier.SeverityMedium, failureclassifier.SeverityLow:
		return level
	default:
		return failureclassifier.SeverityMedium
	}
}
