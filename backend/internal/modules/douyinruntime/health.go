package douyinruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	douyinmetrics "github.com/trademind-ai/trademind/backend/internal/metrics/douyin"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

const (
	HealthHealthy   = "healthy"
	HealthDegraded  = "degraded"
	HealthUnhealthy = "unhealthy"
	HealthDisabled  = "disabled"
)

// HealthDTO aggregates Douyin platform health for the runtime page.
type HealthDTO struct {
	OverallStatus string            `json:"overallStatus"`
	OverallLabel  string            `json:"overallLabel"`
	CheckedAt     string            `json:"checkedAt"`
	Config        HealthSection     `json:"config"`
	Auth          HealthSection     `json:"auth"`
	Storage       HealthSection     `json:"storage"`
	Tasks         HealthSection     `json:"tasks"`
	API           HealthSection     `json:"api"`
	Runtime       *RuntimeStatusDTO `json:"runtime,omitempty"`
	GrayRelease   GrayReleaseDTO    `json:"grayRelease"`
}

type HealthSection struct {
	Status  string         `json:"status"`
	Label   string         `json:"label"`
	Details map[string]any `json:"details,omitempty"`
}

type GrayReleaseDTO struct {
	Enabled                       bool     `json:"enabled"`
	WriteOperationsEnabled        bool     `json:"writeOperationsEnabled"`
	ScheduledOrderSyncEnabled     bool     `json:"scheduledOrderSyncEnabled"`
	ScheduledInventorySyncEnabled bool     `json:"scheduledInventorySyncEnabled"`
	ShopIDs                       []string `json:"shopIds,omitempty"`
}

// GetHealth aggregates config/auth/storage/tasks/api health.
func (s *Service) GetHealth(ctx context.Context) (*HealthDTO, error) {
	if s == nil || s.Settings == nil {
		return nil, fmt.Errorf("douyinruntime: misconfigured")
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, groupKey)
	if err != nil {
		return nil, err
	}
	cfg, cfgErr := platformdouyin.RuntimeFromMergedMap(m)
	rt, _ := platformdouyin.RuntimeStateFromMergedMap(m)
	metrics := douyinmetrics.GetSummary24h()

	out := &HealthDTO{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		GrayRelease: GrayReleaseDTO{
			Enabled:                       cfg.GrayReleaseEnabled,
			WriteOperationsEnabled:        cfg.WriteOperationsEnabled,
			ScheduledOrderSyncEnabled:     cfg.ScheduledOrderSyncEnabled,
			ScheduledInventorySyncEnabled: cfg.ScheduledInventorySyncEnabled,
			ShopIDs:                       cfg.GrayShopIDs,
		},
	}
	if rs, err := s.GetRuntimeStatus(ctx); err == nil {
		out.Runtime = rs
	}

	out.Config = s.healthConfig(cfg, cfgErr)
	out.Auth = s.healthAuth(ctx, metrics)
	out.Storage = s.healthStorage(ctx)
	out.Tasks = s.healthTasks(ctx, metrics)
	out.API = s.healthAPI(metrics)

	out.OverallStatus, out.OverallLabel = aggregateOverall(out, rt, cfg)
	return out, nil
}

func (s *Service) healthConfig(cfg platformdouyin.RuntimeConfig, cfgErr error) HealthSection {
	details := map[string]any{
		"environment":    cfg.Environment,
		"realApiEnabled": cfg.RealAPIEnabled,
		"authBaseUrl":    cfg.AuthBaseURL,
		"apiBaseUrl":     cfg.APIBaseURL,
		"redirectUri":    cfg.RedirectURI,
	}
	if cfgErr != nil {
		details["error"] = cfgErr.Error()
		return HealthSection{Status: HealthUnhealthy, Label: "配置不完整", Details: details}
	}
	if !cfg.RealAPIEnabled {
		return HealthSection{Status: HealthDisabled, Label: "真实接口未启用", Details: details}
	}
	return HealthSection{Status: HealthHealthy, Label: "配置正常", Details: details}
}

func (s *Service) healthAuth(ctx context.Context, metrics douyinmetrics.Summary24h) HealthSection {
	if s == nil || s.DB == nil {
		return HealthSection{Status: HealthDegraded, Label: "无法读取店铺授权", Details: map[string]any{}}
	}
	var authorized, expiring, expired, needCheck int64
	_ = s.DB.WithContext(ctx).Model(&shop.Shop{}).Where("platform = ? AND auth_status = ?", "douyin_shop", shop.AuthAuthorized).Count(&authorized).Error
	_ = s.DB.WithContext(ctx).Model(&shop.Shop{}).Where("platform = ? AND auth_status = ?", "douyin_shop", shop.AuthExpired).Count(&expired).Error
	_ = s.DB.WithContext(ctx).Model(&shop.Shop{}).Where("platform = ? AND auth_status = ?", "douyin_shop", shop.AuthNeedCheck).Count(&needCheck).Error
	douyinmetrics.SetAuthorizationsExpiring(expiring)

	details := map[string]any{
		"authorizedShops":       authorized,
		"expiringShops":         expiring,
		"expiredShops":          expired,
		"needCheckShops":        needCheck,
		"tokenRefreshFailed24h": metrics.TokenRefreshFailedTotal,
	}
	switch {
	case expired > 0:
		return HealthSection{Status: HealthUnhealthy, Label: "存在授权过期店铺", Details: details}
	case needCheck > 0 || metrics.TokenRefreshFailedTotal >= 3:
		return HealthSection{Status: HealthDegraded, Label: "部分授权需要检查", Details: details}
	default:
		return HealthSection{Status: HealthHealthy, Label: "授权正常", Details: details}
	}
}

func (s *Service) healthStorage(ctx context.Context) HealthSection {
	if s == nil || s.Preflight == nil || s.Preflight.Storage == nil {
		return HealthSection{Status: HealthDegraded, Label: "存储检查不可用", Details: map[string]any{}}
	}
	probe := s.Preflight.Storage.ProbeConfiguredBase(ctx)
	details := map[string]any{
		"ok":        probe.OK,
		"message":   probe.Message,
		"errorCode": probe.ErrorCode,
	}
	if !probe.OK {
		return HealthSection{Status: HealthDegraded, Label: "存储公网访问需检查", Details: details}
	}
	return HealthSection{Status: HealthHealthy, Label: "存储正常", Details: details}
}

func (s *Service) healthTasks(ctx context.Context, metrics douyinmetrics.Summary24h) HealthSection {
	details := map[string]any{
		"stale24h":          metrics.StaleTasksTotal,
		"runtimeBlocked24h": metrics.RuntimeBlockedTasksTotal,
		"recoveryRequired":  int64(0),
		"resultUnknown":     int64(0),
		"failedPending":     metrics.FailureTasksPending,
	}
	if s != nil && s.DB != nil {
		var recovery, unknown int64
		_ = s.DB.WithContext(ctx).Model(&productpublish.ProductPublishTask{}).
			Where("platform = ? AND status IN ? AND output::text LIKE ?", "douyin_shop", []string{"failed", "running"}, "%recovery_required%").
			Count(&recovery).Error
		_ = s.DB.WithContext(ctx).Model(&productpublish.ProductPublishTask{}).
			Where("platform = ? AND status IN ? AND output::text LIKE ?", "douyin_shop", []string{"failed", "running"}, "%result_unknown%").
			Count(&unknown).Error
		details["recoveryRequired"] = recovery
		details["resultUnknown"] = unknown
	}
	switch {
	case metrics.StaleTasksTotal >= 5 || metrics.FailureTasksPending >= 20:
		return HealthSection{Status: HealthUnhealthy, Label: "任务积压或停滞过多", Details: details}
	case metrics.StaleTasksTotal > 0 || metrics.FailureTasksPending > 0:
		return HealthSection{Status: HealthDegraded, Label: "存在需处理任务", Details: details}
	default:
		return HealthSection{Status: HealthHealthy, Label: "任务正常", Details: details}
	}
}

func (s *Service) healthAPI(metrics douyinmetrics.Summary24h) HealthSection {
	details := map[string]any{
		"requests24h":    metrics.APIRequestsTotal,
		"successRate24h": metrics.APISuccessRate,
		"avgDurationMs":  metrics.APIDurationAvgMs,
		"timeout24h":     metrics.APITimeoutTotal,
		"rateLimited24h": metrics.APIRateLimitedTotal,
		"retry24h":       metrics.APIRetryTotal,
	}
	if metrics.APIRequestsTotal == 0 {
		return HealthSection{Status: HealthDegraded, Label: "暂无接口调用数据", Details: details}
	}
	switch {
	case metrics.APISuccessRate < 90 || metrics.APITimeoutTotal >= 10:
		return HealthSection{Status: HealthUnhealthy, Label: "接口成功率偏低", Details: details}
	case metrics.APISuccessRate < 98 || metrics.APIRateLimitedTotal >= 5:
		return HealthSection{Status: HealthDegraded, Label: "接口存在异常波动", Details: details}
	default:
		return HealthSection{Status: HealthHealthy, Label: "接口运行正常", Details: details}
	}
}

func aggregateOverall(out *HealthDTO, rt platformdouyin.RuntimeState, cfg platformdouyin.RuntimeConfig) (status, label string) {
	if rt.Status == platformdouyin.RuntimeEmergencyDisabled || !cfg.RealAPIEnabled {
		return HealthDisabled, healthLabel(HealthDisabled)
	}
	if rt.Status == platformdouyin.RuntimePaused {
		return HealthDegraded, "平台已暂停"
	}
	sections := []string{out.Config.Status, out.Auth.Status, out.Storage.Status, out.Tasks.Status, out.API.Status}
	if containsStatus(sections, HealthUnhealthy) {
		return HealthUnhealthy, healthLabel(HealthUnhealthy)
	}
	if containsStatus(sections, HealthDegraded) || containsStatus(sections, HealthDisabled) {
		return HealthDegraded, healthLabel(HealthDegraded)
	}
	return HealthHealthy, healthLabel(HealthHealthy)
}

func containsStatus(list []string, target string) bool {
	for _, s := range list {
		if s == target {
			return true
		}
	}
	return false
}

func healthLabel(status string) string {
	switch strings.TrimSpace(status) {
	case HealthHealthy:
		return "运行正常"
	case HealthDegraded:
		return "部分能力需要检查"
	case HealthUnhealthy:
		return "当前不可用"
	case HealthDisabled:
		return "当前已停用"
	default:
		return status
	}
}

func (s *Service) saveHealthSnapshot(ctx context.Context, h *HealthDTO) {
	if s == nil || s.Settings == nil || h == nil {
		return
	}
	b, _ := json.Marshal(h)
	_ = s.Settings.PutBulk(ctx, []settings.PutItem{{
		TenantID: 0, GroupKey: groupKey, ItemKey: "health_snapshot", ItemValue: string(b), ValueType: "json",
	}})
}
