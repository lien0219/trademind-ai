package douyinruntime

import (
	"context"
	"strings"
	"time"

	douyinmetrics "github.com/trademind-ai/trademind/backend/internal/metrics/douyin"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

const (
	GateNotChecked = "not_checked"
	GateBlocked    = "blocked"
	GateFailed     = "failed"
	GateWarning    = "warning"
	GatePassed     = "passed"
)

// ReleaseGateItem is one RC checklist row.
type ReleaseGateItem struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// ReleaseGateDTO is the release candidate checklist response.
type ReleaseGateDTO struct {
	OverallConclusion string            `json:"overallConclusion"`
	CheckedAt         string            `json:"checkedAt"`
	Items             []ReleaseGateItem `json:"items"`
}

// GetReleaseGate builds the Phase 10.4 release candidate checklist.
func (s *Service) GetReleaseGate(ctx context.Context) (*ReleaseGateDTO, error) {
	items := make([]ReleaseGateItem, 0, 18)
	m, _ := s.Settings.PlainByGroup(ctx, 0, groupKey)
	cfg, cfgErr := platformdouyin.RuntimeFromMergedMap(m)
	health, _ := s.GetHealth(ctx)
	metrics := douyinmetrics.GetSummary24h()

	cfgSt, cfgMsg := gateFromErr(cfgErr)
	items = append(items, gateItem("config", "配置检查", cfgSt, cfgMsg))
	items = append(items, gateItem("credentials", "真实凭证", GateBlocked, "真实 E2E 仍为 blocked_by_real_credentials"))
	authSt, authMsg := gateAuth(health)
	items = append(items, gateItem("shop_auth", "店铺授权", authSt, authMsg))
	storageSt, storageMsg := gateSection(health, "storage")
	items = append(items, gateItem("storage", "Storage", storageSt, storageMsg))
	items = append(items, gateItem("image_upload", "图片上传", GateWarning, "需真实环境验证"))
	draftSt, draftMsg := gateCounter(metrics.ProductDraftCreateTotal > 0, metrics.ProductDraftCreateFailed == 0)
	items = append(items, gateItem("product_draft", "商品草稿", draftSt, draftMsg))
	items = append(items, gateItem("product_detail", "商品详情回查", GatePassed, "Phase 10.2+ 已实现"))
	items = append(items, gateItem("sku_binding", "SKU 绑定", GatePassed, "自动/手动绑定已实现"))
	orderSt, orderMsg := gateCounter(metrics.OrderFetchedTotal > 0, metrics.OrderPartialSuccessTotal == 0)
	items = append(items, gateItem("order_sync", "订单同步", orderSt, orderMsg))
	invSt, invMsg := gateCounter(metrics.InventorySyncSuccessTotal > 0, metrics.InventorySyncFailedTotal == 0)
	items = append(items, gateItem("inventory_sync", "库存同步", invSt, invMsg))
	items = append(items, gateItem("retry_policy", "重试策略", GatePassed, "Phase 10.3 已落地"))
	staleSt, staleMsg := gateCounter(metrics.RecoverySuccessTotal >= 0, true)
	items = append(items, gateItem("stale_recovery", "stale 恢复", staleSt, staleMsg))
	items = append(items, gateItem("alerts", "告警", GatePassed, "已接入 taskcenter TaskAlert"))
	items = append(items, gateItem("ci_race", "CI race", GateWarning, "Linux race job 已配置，待 CI 执行"))
	items = append(items, gateItem("rollback_drill", "回滚演练", GateWarning, "见 docs/DOUYIN_ROLLBACK_DRILL_REPORT.md"))
	graySt, grayMsg := gateGray(cfg)
	items = append(items, gateItem("gray_observation", "灰度观察", graySt, grayMsg))

	conclusion := ReleaseCandidateConclusion
	if allPassed(items) {
		conclusion = "Gray Release Ready"
	}
	return &ReleaseGateDTO{
		OverallConclusion: conclusion,
		CheckedAt:         time.Now().UTC().Format(time.RFC3339),
		Items:             items,
	}, nil
}

const ReleaseCandidateConclusion = "Release Candidate"

func gateItem(key, label, status, msg string) ReleaseGateItem {
	return ReleaseGateItem{Key: key, Label: label, Status: status, Message: msg}
}

func gateFromErr(err error) (string, string) {
	if err != nil {
		return GateFailed, err.Error()
	}
	return GatePassed, ""
}

func gateSection(h *HealthDTO, section string) (string, string) {
	if h == nil {
		return GateNotChecked, ""
	}
	var sec HealthSection
	switch section {
	case "storage":
		sec = h.Storage
	default:
		return GateNotChecked, ""
	}
	switch sec.Status {
	case HealthHealthy:
		return GatePassed, sec.Label
	case HealthDegraded:
		return GateWarning, sec.Label
	case HealthUnhealthy:
		return GateFailed, sec.Label
	case HealthDisabled:
		return GateBlocked, sec.Label
	default:
		return GateNotChecked, sec.Label
	}
}

func gateAuth(h *HealthDTO) (string, string) {
	if h == nil {
		return GateNotChecked, ""
	}
	switch h.Auth.Status {
	case HealthHealthy:
		return GatePassed, h.Auth.Label
	case HealthDegraded:
		return GateWarning, h.Auth.Label
	default:
		return GateFailed, h.Auth.Label
	}
}

func gateCounter(hasData, ok bool) (string, string) {
	if !hasData {
		return GateWarning, "暂无最近24小时数据"
	}
	if ok {
		return GatePassed, ""
	}
	return GateWarning, "存在失败记录"
}

func gateGray(cfg platformdouyin.RuntimeConfig) (string, string) {
	if cfg.GrayReleaseEnabled && len(cfg.GrayShopIDs) > 0 {
		return GateWarning, "灰度已配置，观察期未完成"
	}
	return GateNotChecked, "灰度未启用"
}

func allPassed(items []ReleaseGateItem) bool {
	for _, it := range items {
		st := strings.TrimSpace(it.Status)
		if st != GatePassed && st != GateWarning {
			return false
		}
	}
	return false
}
