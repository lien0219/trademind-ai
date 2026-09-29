package douyinruntime

import (
	"context"
	"fmt"
	"strings"
	"time"

	douyinmetrics "github.com/trademind-ai/trademind/backend/internal/metrics/douyin"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter/failureclassifier"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

// ScanDouyinAlerts evaluates platform metrics/state for every tenant with a Douyin shop.
func (s *Service) ScanDouyinAlerts(ctx context.Context) (AlertScanSummary, error) {
	var sum AlertScanSummary
	if s == nil || s.DB == nil || s.Settings == nil {
		return sum, fmt.Errorf("douyinruntime: misconfigured")
	}
	var tenantIDs []int64
	if err := s.DB.WithContext(ctx).Model(&shop.Shop{}).
		Where("platform = ? AND deleted_at IS NULL", "douyin_shop").Distinct("tenant_id").Pluck("tenant_id", &tenantIDs).Error; err != nil {
		return sum, err
	}
	if len(tenantIDs) == 0 {
		tenantIDs = []int64{0}
	}
	for _, tenantID := range tenantIDs {
		part, err := s.scanDouyinAlertsForTenant(ctx, tenantID)
		if err != nil {
			return sum, err
		}
		sum.Scanned += part.Scanned
		sum.Generated += part.Generated
		sum.Resolved += part.Resolved
	}
	return sum, nil
}

func (s *Service) scanDouyinAlertsForTenant(ctx context.Context, tenantID int64) (AlertScanSummary, error) {
	var sum AlertScanSummary
	now := time.Now().UTC()
	cfgMap := douyinAlertSettings(ctx, s.Settings, tenantID)
	if !parseBoolSetting(cfgMap["alert_scan_enabled"], true) {
		return sum, nil
	}
	var scanErr error
	recordErr := func(err error) {
		if scanErr == nil && err != nil {
			scanErr = err
		}
	}

	metrics := douyinmetrics.GetSummary24h()
	rt, _, cfg, _ := platformdouyin.LoadRuntimeFromBridge(ctx)

	// Runtime emergency
	if rt.Status == platformdouyin.RuntimeEmergencyDisabled {
		sum.Generated++
		recordErr(s.UpsertDouyinAlertForTenant(ctx, tenantID, "global", AlertRuntimeEmergency, failureclassifier.SeverityCritical,
			"抖店已进入紧急停用", "平台运行状态为紧急停用", "在设置中恢复平台运行状态或排查原因后恢复", now))
	} else {
		sum.Resolved++
		recordErr(s.ResolveDouyinAlertForTenant(ctx, tenantID, "global", AlertRuntimeEmergency, now))
	}

	// Token refresh failures
	thToken := alertThresholdInt(cfgMap, "alert_token_refresh_fail_threshold", 3)
	if metrics.TokenRefreshFailedTotal >= int64(thToken) {
		sum.Generated++
		recordErr(s.UpsertDouyinAlertForTenant(ctx, tenantID, "global", AlertTokenRefreshFailed, failureclassifier.SeverityHigh,
			"访问令牌刷新连续失败", fmt.Sprintf("最近24小时访问令牌刷新失败 %d 次", metrics.TokenRefreshFailedTotal),
			"检查应用密钥、店铺授权与网络连通性", now))
	} else {
		sum.Resolved++
		recordErr(s.ResolveDouyinAlertForTenant(ctx, tenantID, "global", AlertTokenRefreshFailed, now))
	}

	// Auth expired / need_check shops
	var expired, needCheck int64
	recordErr(s.DB.WithContext(ctx).Model(&shop.Shop{}).Where("tenant_id = ? AND platform = ? AND deleted_at IS NULL AND auth_status = ?", tenantID, "douyin_shop", shop.AuthExpired).Count(&expired).Error)
	recordErr(s.DB.WithContext(ctx).Model(&shop.Shop{}).Where("tenant_id = ? AND platform = ? AND deleted_at IS NULL AND auth_status = ?", tenantID, "douyin_shop", shop.AuthNeedCheck).Count(&needCheck).Error)
	if expired > 0 {
		sum.Generated++
		recordErr(s.UpsertDouyinAlertForTenant(ctx, tenantID, "global", AlertAuthExpired, failureclassifier.SeverityHigh,
			"存在授权过期店铺", fmt.Sprintf("过期店铺数=%d", expired), "前往店铺管理重新授权", now))
	} else {
		sum.Resolved++
		recordErr(s.ResolveDouyinAlertForTenant(ctx, tenantID, "global", AlertAuthExpired, now))
	}
	if needCheck > 0 {
		sum.Generated++
		recordErr(s.UpsertDouyinAlertForTenant(ctx, tenantID, "global", AlertAuthNeedCheck, failureclassifier.SeverityMedium,
			"存在需检查授权店铺", fmt.Sprintf("需检查店铺数=%d", needCheck), "检查店铺授权与访问令牌状态", now))
	} else {
		sum.Resolved++
		recordErr(s.ResolveDouyinAlertForTenant(ctx, tenantID, "global", AlertAuthNeedCheck, now))
	}

	// Storage public access
	if s.Preflight != nil && s.Preflight.Storage != nil {
		probe := s.Preflight.Storage.ProbeConfiguredBase(ctx)
		if !probe.OK {
			sum.Generated++
			recordErr(s.UpsertDouyinAlertForTenant(ctx, tenantID, "global", AlertStoragePublicFailed, failureclassifier.SeverityHigh,
				"存储公网访问异常", probe.Message, "在存储设置中配置 HTTPS 公网访问地址并运行公网访问测试", now))
		} else {
			sum.Resolved++
			recordErr(s.ResolveDouyinAlertForTenant(ctx, tenantID, "global", AlertStoragePublicFailed, now))
		}
	}

	// Stale / backlog
	staleTh := alertThresholdInt(cfgMap, "alert_stale_tasks_threshold", 5)
	if metrics.StaleTasksTotal >= int64(staleTh) {
		sum.Generated++
		recordErr(s.UpsertDouyinAlertForTenant(ctx, tenantID, "global", AlertStaleTasksHigh, failureclassifier.SeverityHigh,
			"停滞任务数量偏高", fmt.Sprintf("最近24小时停滞标记 %d", metrics.StaleTasksTotal), "在任务中心处理停滞或结果暂无法确认的任务", now))
	} else {
		sum.Resolved++
		recordErr(s.ResolveDouyinAlertForTenant(ctx, tenantID, "global", AlertStaleTasksHigh, now))
	}
	backlogTh := alertThresholdInt(cfgMap, "alert_failure_backlog_threshold", 20)
	if metrics.FailureTasksPending >= int64(backlogTh) {
		sum.Generated++
		recordErr(s.UpsertDouyinAlertForTenant(ctx, tenantID, "global", AlertFailureBacklog, failureclassifier.SeverityMedium,
			"失败任务积压", fmt.Sprintf("待处理失败任务 %d", metrics.FailureTasksPending), "前往失败任务中心处理", now))
	} else {
		sum.Resolved++
		recordErr(s.ResolveDouyinAlertForTenant(ctx, tenantID, "global", AlertFailureBacklog, now))
	}

	// Rate limit spike
	rateTh := alertThresholdInt(cfgMap, "alert_rate_limit_threshold", 10)
	if metrics.APIRateLimitedTotal >= int64(rateTh) {
		sum.Generated++
		recordErr(s.UpsertDouyinAlertForTenant(ctx, tenantID, "global", AlertRateLimitSpike, failureclassifier.SeverityMedium,
			"抖店接口限流次数升高", fmt.Sprintf("最近24小时限流 %d 次", metrics.APIRateLimitedTotal), "降低并发、检查重试策略或联系平台", now))
	} else {
		sum.Resolved++
		recordErr(s.ResolveDouyinAlertForTenant(ctx, tenantID, "global", AlertRateLimitSpike, now))
	}

	// Product draft failures
	draftFailTh := alertThresholdInt(cfgMap, "alert_product_draft_fail_threshold", 3)
	if metrics.ProductDraftCreateFailed >= int64(draftFailTh) {
		sum.Generated++
		recordErr(s.UpsertDouyinAlertForTenant(ctx, tenantID, "global", AlertProductDraftFailures, failureclassifier.SeverityHigh,
			"商品草稿创建失败偏多", fmt.Sprintf("最近24小时失败 %d 次", metrics.ProductDraftCreateFailed), "检查类目、属性、图片与店铺授权", now))
	} else {
		sum.Resolved++
		recordErr(s.ResolveDouyinAlertForTenant(ctx, tenantID, "global", AlertProductDraftFailures, now))
	}

	// Inventory sync failures
	invFailTh := alertThresholdInt(cfgMap, "alert_inventory_sync_fail_threshold", 5)
	if metrics.InventorySyncFailedTotal >= int64(invFailTh) {
		sum.Generated++
		recordErr(s.UpsertDouyinAlertForTenant(ctx, tenantID, "global", AlertInventorySyncFailed, failureclassifier.SeverityMedium,
			"库存同步失败偏多", fmt.Sprintf("最近24小时失败 %d 次", metrics.InventorySyncFailedTotal), "检查 SKU 绑定与库存任务", now))
	} else {
		sum.Resolved++
		recordErr(s.ResolveDouyinAlertForTenant(ctx, tenantID, "global", AlertInventorySyncFailed, now))
	}

	_ = cfg
	if scanErr != nil {
		return sum, scanErr
	}
	sum.Scanned = 1
	return sum, nil
}

// AlertScanSummary counts alert scan actions.
type AlertScanSummary struct {
	Scanned   int `json:"scanned"`
	Generated int `json:"generated"`
	Resolved  int `json:"resolved"`
}

func parseBoolSetting(v string, def bool) bool {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return def
	}
}
