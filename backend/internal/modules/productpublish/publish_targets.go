package productpublish

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productcheck"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/opslabels"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

// PublishTargetShopDTO is one shop under a platform target.
type PublishTargetShopDTO struct {
	ShopID          string `json:"shopId"`
	ShopName        string `json:"shopName"`
	AuthStatus      string `json:"authStatus"`
	AuthStatusLabel string `json:"authStatusLabel"`
	Enabled         bool   `json:"enabled"`
}

// PublishTargetPlatformDTO is one platform with shops and capability.
type PublishTargetPlatformDTO struct {
	Platform         string                 `json:"platform"`
	PlatformLabel    string                 `json:"platformLabel"`
	Capability       string                 `json:"capability"`
	CapabilityLabel  string                 `json:"capabilityLabel"`
	Shops            []PublishTargetShopDTO `json:"shops"`
	SettingsGroupKey string                 `json:"settingsGroupKey,omitempty"`
	SettingsPath     string                 `json:"settingsPath,omitempty"`
}

// PublishTargetsResponse GET publish-targets.
type PublishTargetsResponse struct {
	ProductID uuid.UUID                  `json:"productId"`
	Platforms []PublishTargetPlatformDTO `json:"platforms"`
}

// PublishTargetRef identifies one publish destination.
type PublishTargetRef struct {
	Platform string  `json:"platform"`
	ShopID   *string `json:"shopId"`
}

// PublishTargetsCheckRequest POST check body.
type PublishTargetsCheckRequest struct {
	Targets       []PublishTargetRef `json:"targets"`
	CommonConfig  map[string]any     `json:"commonConfig,omitempty"`
	TargetConfigs map[string]any     `json:"targetConfigs,omitempty"`
}

// PublishTargetIssue is one localized check issue for a target.
type PublishTargetIssue struct {
	Code             string         `json:"code"`
	Title            string         `json:"title"`
	Message          string         `json:"message"`
	Severity         string         `json:"severity"`
	Suggestion       string         `json:"suggestion,omitempty"`
	TechnicalDetails map[string]any `json:"technicalDetails,omitempty"`
}

// PublishTargetCheckResult is readiness for one target.
type PublishTargetCheckResult struct {
	TargetKey     string               `json:"targetKey"`
	Platform      string               `json:"platform"`
	PlatformLabel string               `json:"platformLabel"`
	ShopID        string               `json:"shopId,omitempty"`
	ShopName      string               `json:"shopName,omitempty"`
	Capability    string               `json:"capability"`
	Status        string               `json:"status"`
	StatusLabel   string               `json:"statusLabel"`
	CanCreate     bool                 `json:"canCreateDraft"`
	Issues        []PublishTargetIssue `json:"issues"`
}

// PublishTargetsCheckSummary aggregates multi-target check.
type PublishTargetsCheckSummary struct {
	TargetCount  int `json:"targetCount"`
	ReadyCount   int `json:"readyCount"`
	WarningCount int `json:"warningCount"`
	BlockedCount int `json:"blockedCount"`
}

// PublishTargetsCheckResponse POST check response.
type PublishTargetsCheckResponse struct {
	Summary PublishTargetsCheckSummary `json:"summary"`
	Targets []PublishTargetCheckResult `json:"targets"`
}

// PublishTargetsCreateDraftsRequest POST create-drafts body.
type PublishTargetsCreateDraftsRequest struct {
	Targets         []PublishTargetRef `json:"targets"`
	CommonConfig    map[string]any     `json:"commonConfig,omitempty"`
	TargetConfigs   map[string]any     `json:"targetConfigs,omitempty"`
	OnlyReady       bool               `json:"onlyReady,omitempty"`
	RetryFailedOnly bool               `json:"retryFailedOnly,omitempty"`
	BatchID         string             `json:"batchId,omitempty"`
	Force           bool               `json:"force,omitempty"`
}

// PublishTargetTaskResult is one child task outcome.
type PublishTargetTaskResult struct {
	TargetKey         string `json:"targetKey"`
	Platform          string `json:"platform"`
	PlatformLabel     string `json:"platformLabel"`
	ShopID            string `json:"shopId,omitempty"`
	ShopName          string `json:"shopName,omitempty"`
	TaskID            string `json:"taskId,omitempty"`
	PublicationID     string `json:"publicationId,omitempty"`
	Status            string `json:"status"`
	StatusLabel       string `json:"statusLabel"`
	Capability        string `json:"capability"`
	LocalDraftOnly    bool   `json:"localDraftOnly"`
	ErrorCode         string `json:"errorCode,omitempty"`
	ErrorMessage      string `json:"errorMessage,omitempty"`
	PlatformProductID string `json:"platformProductId,omitempty"`
}

// PublishTargetsCreateDraftsResponse POST create-drafts response.
type PublishTargetsCreateDraftsResponse struct {
	BatchID      string                    `json:"batchId"`
	Status       string                    `json:"status"`
	StatusLabel  string                    `json:"statusLabel"`
	TargetCount  int                       `json:"targetCount"`
	SuccessCount int                       `json:"successCount"`
	FailedCount  int                       `json:"failedCount"`
	SkippedCount int                       `json:"skippedCount"`
	Targets      []PublishTargetTaskResult `json:"targets"`
}

func publishTargetKey(platform string, shopID *uuid.UUID) string {
	plat := strings.TrimSpace(strings.ToLower(platform))
	if shopID == nil || *shopID == uuid.Nil {
		return plat
	}
	return plat + ":" + shopID.String()
}

// ListPublishTargets returns platforms/shops available for publishing one product.
func (s *Service) ListPublishTargets(ctx context.Context, productID uuid.UUID) (*PublishTargetsResponse, error) {
	if s == nil || s.DB == nil || s.Shops == nil {
		return nil, fmt.Errorf("product publish unavailable")
	}
	if _, err := s.loadProductForPublish(ctx, productID); err != nil {
		return nil, err
	}

	providers := platformp.All()
	sort.Slice(providers, func(i, j int) bool {
		return opslabels.PlatformLabel(providers[i].Platform()) < opslabels.PlatformLabel(providers[j].Platform())
	})

	var shops []shop.Shop
	_ = s.DB.WithContext(ctx).Where("status <> ?", shop.StatusDisabled).Order("platform ASC, shop_name ASC").Find(&shops).Error
	shopsByPlat := map[string][]shop.Shop{}
	for _, sh := range shops {
		plat := strings.TrimSpace(strings.ToLower(sh.Platform))
		if plat == "" || plat == "manual" || plat == "mock" {
			continue
		}
		shopsByPlat[plat] = append(shopsByPlat[plat], sh)
	}

	out := make([]PublishTargetPlatformDTO, 0, len(providers))
	for _, prov := range providers {
		if prov == nil {
			continue
		}
		plat := strings.TrimSpace(strings.ToLower(prov.Platform()))
		if plat == "" || plat == "manual" || plat == "mock" {
			continue
		}
		if !platformp.HasCapability(prov, platformp.CapProductPublish) {
			continue
		}
		capability, capLabel := s.resolvePublishCapability(ctx, prov)
		shopDTOs := make([]PublishTargetShopDTO, 0)
		for _, sh := range shopsByPlat[plat] {
			shopDTOs = append(shopDTOs, PublishTargetShopDTO{
				ShopID:          sh.ID.String(),
				ShopName:        strings.TrimSpace(sh.ShopName),
				AuthStatus:      sh.AuthStatus,
				AuthStatusLabel: opslabels.StatusLabel(authStatusLabel(sh.AuthStatus)),
				Enabled:         strings.TrimSpace(strings.ToLower(sh.Status)) == shop.StatusActive,
			})
		}
		appSch := prov.AppConfigSchema()
		pubSch := prov.PublishConfigSchema()
		settingsKey := strings.TrimSpace(appSch.GroupKey)
		if settingsKey == "" {
			settingsKey = strings.TrimSpace(pubSch.GroupKey)
		}
		out = append(out, PublishTargetPlatformDTO{
			Platform:         plat,
			PlatformLabel:    opslabels.PlatformLabel(plat),
			Capability:       capability,
			CapabilityLabel:  capLabel,
			Shops:            shopDTOs,
			SettingsGroupKey: settingsKey,
			SettingsPath:     settingsPathForPlatform(plat),
		})
	}
	return &PublishTargetsResponse{ProductID: productID, Platforms: out}, nil
}

func authStatusLabel(st string) string {
	switch strings.TrimSpace(strings.ToLower(st)) {
	case shop.AuthAuthorized:
		return "authorized"
	case shop.AuthExpired:
		return "expired"
	case shop.AuthInvalid, shop.AuthError:
		return "failed"
	default:
		return st
	}
}

func settingsPathForPlatform(plat string) string {
	switch strings.TrimSpace(strings.ToLower(plat)) {
	case "douyin_shop":
		return "/settings/platforms"
	case "tiktok", "shopee", "lazada", "amazon":
		return "/settings/platform-publish"
	default:
		return "/settings/platforms"
	}
}

func (s *Service) resolvePublishCapability(ctx context.Context, prov platformp.Provider) (cap, label string) {
	if prov == nil {
		return CapDisabled, opslabels.PublishCapabilityLabel(CapDisabled)
	}
	plat := strings.TrimSpace(strings.ToLower(prov.Platform()))
	if prov.Status() == platformp.StatusDisabled {
		return CapDisabled, opslabels.PublishCapabilityLabel(CapDisabled)
	}
	pubImpl := platformp.ProductPublishImplementationStatus(prov)
	if pubImpl == platformp.StatusDisabled || pubImpl == platformp.StatusPlanned {
		return CapNotConfigured, opslabels.PublishCapabilityLabel(CapNotConfigured)
	}
	if s.Settings != nil {
		sch := prov.AppConfigSchema()
		gk := strings.TrimSpace(sch.GroupKey)
		if gk != "" {
			m, err := s.Settings.PlainByGroup(ctx, 0, gk)
			if err != nil || !partnerConfigLooksComplete(m, sch) {
				return CapNotConfigured, opslabels.PublishCapabilityLabel(CapNotConfigured)
			}
		}
	}
	if plat == "douyin_shop" {
		return CapRealDraftCreate, opslabels.PublishCapabilityLabel(CapRealDraftCreate)
	}
	if platformp.IsProductPublishRunnable(prov) {
		return CapLocalDraftOnly, opslabels.PublishCapabilityLabel(CapLocalDraftOnly)
	}
	return CapNotConfigured, opslabels.PublishCapabilityLabel(CapNotConfigured)
}

func partnerConfigLooksComplete(m map[string]string, sch platformp.PlatformAppConfigSchema) bool {
	if len(sch.Fields) == 0 {
		return true
	}
	for _, f := range sch.Fields {
		if !f.Required {
			continue
		}
		if strings.TrimSpace(m[f.Name]) == "" {
			return false
		}
	}
	return true
}

// CheckPublishTargets runs independent readiness checks per target (no side effects).
func (s *Service) CheckPublishTargets(ctx context.Context, productID uuid.UUID, req PublishTargetsCheckRequest) (*PublishTargetsCheckResponse, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product publish unavailable")
	}
	if len(req.Targets) == 0 {
		return nil, fmt.Errorf("targets required")
	}
	if _, err := s.loadProductForPublish(ctx, productID); err != nil {
		return nil, err
	}

	targets := make([]PublishTargetCheckResult, 0, len(req.Targets))
	var readyN, warnN, blockedN int
	for _, t := range req.Targets {
		res := s.checkOnePublishTarget(ctx, productID, t)
		targets = append(targets, res)
		switch res.Status {
		case statusReady:
			readyN++
		case statusWarning:
			warnN++
		default:
			blockedN++
		}
	}
	return &PublishTargetsCheckResponse{
		Summary: PublishTargetsCheckSummary{
			TargetCount:  len(targets),
			ReadyCount:   readyN,
			WarningCount: warnN,
			BlockedCount: blockedN,
		},
		Targets: targets,
	}, nil
}

const (
	statusReady   = "ready"
	statusWarning = "warning"
	statusBlocked = "blocked"
)

func (s *Service) checkOnePublishTarget(ctx context.Context, productID uuid.UUID, t PublishTargetRef) PublishTargetCheckResult {
	plat := strings.TrimSpace(strings.ToLower(t.Platform))
	var sid *uuid.UUID
	shopName := ""
	capability := CapLocalDraftOnly
	if t.ShopID != nil && strings.TrimSpace(*t.ShopID) != "" {
		if u, err := uuid.Parse(strings.TrimSpace(*t.ShopID)); err == nil {
			sid = &u
			if s.Shops != nil {
				if row, _, err := s.Shops.PlainAuthForProviderCtx(ctx, u); err == nil && row != nil {
					shopName = strings.TrimSpace(row.ShopName)
					if strings.TrimSpace(strings.ToLower(row.AuthStatus)) != shop.AuthAuthorized {
						return blockedTargetResult(plat, sid, shopName, capability, []PublishTargetIssue{
							issueFromCode("SHOP_NOT_AUTHORIZED", "error", "店铺尚未授权", "请前往店铺管理完成授权。"),
						})
					}
					if strings.TrimSpace(strings.ToLower(row.Status)) != shop.StatusActive {
						return blockedTargetResult(plat, sid, shopName, capability, []PublishTargetIssue{
							issueFromCode("SHOP_DISABLED", "error", "店铺已停用", "请在店铺管理中启用店铺。"),
						})
					}
				}
			}
		}
	}
	prov := platformp.Get(plat)
	if prov != nil {
		capability, _ = s.resolvePublishCapability(ctx, prov)
	}
	if capability == CapDisabled || capability == CapNotConfigured {
		return blockedTargetResult(plat, sid, shopName, capability, []PublishTargetIssue{
			issueFromCode("PUBLISH_CONFIG_MISSING", "error", "刊登配置未完成", "请先在平台接入设置中完成配置。"),
		})
	}

	issues := make([]PublishTargetIssue, 0, 8)
	if capability == CapLocalDraftOnly {
		issues = append(issues, issueFromCode("PLATFORM_NOT_SUPPORTED", "warning",
			"当前平台暂未接入真实发布", "将仅生成本地刊登草稿与任务快照，不会调用平台接口。"))
	}

	if s.Readiness != nil {
		rreq := productcheck.CheckProductReadinessRequest{
			ProductID: productID,
			Mode:      "publish",
		}
		if capability == CapRealDraftCreate {
			rreq.Platform = plat
			rreq.ShopID = sid
		}
		rres, err := s.Readiness.CheckProductReadiness(ctx, rreq)
		if err != nil {
			issues = append(issues, issueFromCode("PUBLISH_CHECK_FAILED", "error", "发布检查失败", err.Error()))
		} else if rres != nil {
			loc := productcheck.LocalizeReadinessResult(rres)
			for _, c := range loc.Checks {
				issues = append(issues, checkItemToIssue(c))
			}
			if capability == CapRealDraftCreate && plat == "douyin_shop" && sid != nil {
				issues = append(issues, s.douyinConfigIssues(ctx, productID, *sid)...)
			}
		}
	}

	status := statusReady
	statusLabel := "可以创建草稿"
	canCreate := true
	var errN, warnN int
	for _, iss := range issues {
		if iss.Severity == "error" {
			errN++
		} else {
			warnN++
		}
	}
	if errN > 0 {
		status = statusBlocked
		statusLabel = "暂不能创建草稿"
		canCreate = false
	} else if warnN > 0 {
		status = statusWarning
		statusLabel = "需要检查"
	}

	return PublishTargetCheckResult{
		TargetKey:     publishTargetKey(plat, sid),
		Platform:      plat,
		PlatformLabel: opslabels.PlatformLabel(plat),
		ShopID:        shopIDString(sid),
		ShopName:      shopName,
		Capability:    capability,
		Status:        status,
		StatusLabel:   statusLabel,
		CanCreate:     canCreate,
		Issues:        issues,
	}
}

func (s *Service) douyinConfigIssues(ctx context.Context, productID, shopID uuid.UUID) []PublishTargetIssue {
	var cfg product.ProductPlatformPublishConfig
	if err := s.DB.WithContext(ctx).Where("product_id = ? AND platform = ?", productID, "douyin_shop").First(&cfg).Error; err != nil {
		return []PublishTargetIssue{
			issueFromCode("PUBLISH_CONFIG_MISSING", "error", "抖店刊登配置未完成", "请先配置抖店类目与属性并生成刊登草稿。"),
		}
	}
	if cfg.ShopID == nil || *cfg.ShopID != shopID {
		return []PublishTargetIssue{
			issueFromCode("PUBLISH_CONFIG_MISSING", "error", "抖店店铺配置不一致", "请保存与所选店铺一致的抖店刊登配置。"),
		}
	}
	if cfg.LastMappedAt == nil || strings.TrimSpace(cfg.MappedTitle) == "" {
		return []PublishTargetIssue{
			issueFromCode("PUBLISH_CONFIG_MISSING", "warning", "抖店刊登草稿未生成", "请先生成并保存抖店刊登草稿。"),
		}
	}
	if strings.TrimSpace(cfg.CategoryID) == "" {
		return []PublishTargetIssue{
			issueFromCode("CATEGORY_REQUIRED", "error", "抖店类目未选择", "请选择抖店叶子类目。"),
		}
	}
	return nil
}

func blockedTargetResult(plat string, sid *uuid.UUID, shopName, capability string, issues []PublishTargetIssue) PublishTargetCheckResult {
	return PublishTargetCheckResult{
		TargetKey:     publishTargetKey(plat, sid),
		Platform:      plat,
		PlatformLabel: opslabels.PlatformLabel(plat),
		ShopID:        shopIDString(sid),
		ShopName:      shopName,
		Capability:    capability,
		Status:        statusBlocked,
		StatusLabel:   "暂不能创建草稿",
		CanCreate:     false,
		Issues:        issues,
	}
}

func checkItemToIssue(c productcheck.CheckItem) PublishTargetIssue {
	sev := strings.TrimSpace(strings.ToLower(c.Level))
	if sev == "failed" {
		sev = "error"
	}
	return PublishTargetIssue{
		Code:             c.Code,
		Title:            firstNonEmpty(c.Title, c.Message),
		Message:          c.Message,
		Severity:         sev,
		Suggestion:       c.Suggestion,
		TechnicalDetails: c.TechnicalDetails,
	}
}

func issueFromCode(code, severity, title, message string) PublishTargetIssue {
	loc := opslabels.LocalizeReadinessIssue(code, severity, message, "", "", "", "")
	return PublishTargetIssue{
		Code:             code,
		Title:            firstNonEmpty(loc.Title, title),
		Message:          firstNonEmpty(loc.Message, message),
		Severity:         severity,
		Suggestion:       loc.Suggestion,
		TechnicalDetails: loc.TechnicalDetails,
	}
}

func shopIDString(sid *uuid.UUID) string {
	if sid == nil || *sid == uuid.Nil {
		return ""
	}
	return sid.String()
}

func (s *Service) loadProductForPublish(ctx context.Context, productID uuid.UUID) (*product.Product, error) {
	var prod product.Product
	if err := s.DB.WithContext(ctx).First(&prod, "id = ?", productID).Error; err != nil {
		return nil, err
	}
	if prod.DeletedAt.Valid {
		return nil, fmt.Errorf("deleted product cannot be published")
	}
	return &prod, nil
}

// CreateDraftsForTargets creates a batch and one child task per target.
func (s *Service) CreateDraftsForTargets(c *gin.Context, productID uuid.UUID, req PublishTargetsCreateDraftsRequest, adminID *uuid.UUID) (*PublishTargetsCreateDraftsResponse, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product publish unavailable")
	}
	ctx := c.Request.Context()
	targets := req.Targets
	if req.RetryFailedOnly && strings.TrimSpace(req.BatchID) != "" {
		retryTargets, err := s.failedTargetsFromBatch(ctx, req.BatchID)
		if err != nil {
			return nil, err
		}
		if len(retryTargets) == 0 {
			return nil, fmt.Errorf("no failed targets to retry")
		}
		targets = retryTargets
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("targets required")
	}

	checkRes, err := s.CheckPublishTargets(ctx, productID, PublishTargetsCheckRequest{Targets: targets})
	if err != nil {
		return nil, err
	}
	checkByKey := map[string]PublishTargetCheckResult{}
	for _, t := range checkRes.Targets {
		checkByKey[t.TargetKey] = t
	}

	inRaw, _ := json.Marshal(req)
	batch := ProductPublishBatch{
		ProductID:   productID,
		Status:      BatchRunning,
		TargetCount: len(targets),
		Input:       datatypes.JSON(inRaw),
		CreatedBy:   adminID,
	}
	if err := s.DB.WithContext(ctx).Create(&batch).Error; err != nil {
		return nil, err
	}

	results := make([]PublishTargetTaskResult, 0, len(targets))
	var successN, failedN, skippedN int
	for _, t := range targets {
		plat := strings.TrimSpace(strings.ToLower(t.Platform))
		var sid *uuid.UUID
		if t.ShopID != nil && strings.TrimSpace(*t.ShopID) != "" {
			if u, err := uuid.Parse(strings.TrimSpace(*t.ShopID)); err == nil {
				sid = &u
			}
		}
		key := publishTargetKey(plat, sid)
		chk := checkByKey[key]
		if req.OnlyReady && (chk.Status == statusBlocked || !chk.CanCreate) {
			skippedN++
			results = append(results, PublishTargetTaskResult{
				TargetKey:     key,
				Platform:      plat,
				PlatformLabel: opslabels.PlatformLabel(plat),
				ShopID:        shopIDString(sid),
				ShopName:      chk.ShopName,
				Status:        "skipped",
				StatusLabel:   "已跳过",
				Capability:    chk.Capability,
			})
			continue
		}
		if chk.Status == statusBlocked {
			failedN++
			msg := "目标暂不能创建草稿"
			if len(chk.Issues) > 0 {
				msg = chk.Issues[0].Title
			}
			results = append(results, PublishTargetTaskResult{
				TargetKey:     key,
				Platform:      plat,
				PlatformLabel: opslabels.PlatformLabel(plat),
				ShopID:        shopIDString(sid),
				ShopName:      chk.ShopName,
				Status:        TaskFailed,
				StatusLabel:   opslabels.StatusLabel(TaskFailed),
				Capability:    chk.Capability,
				ErrorMessage:  msg,
			})
			continue
		}

		var taskRes PublishTargetTaskResult
		switch chk.Capability {
		case CapRealDraftCreate:
			if sid == nil {
				failedN++
				taskRes = PublishTargetTaskResult{
					TargetKey: key, Platform: plat, PlatformLabel: opslabels.PlatformLabel(plat),
					Status: TaskFailed, StatusLabel: opslabels.StatusLabel(TaskFailed),
					Capability: chk.Capability, ErrorMessage: "缺少店铺",
				}
			} else {
				taskRes = s.createDouyinDraftForTarget(c, productID, *sid, batch.ID, adminID, req.Force, chk)
				if taskRes.Status == TaskSuccess || taskRes.Status == TaskPending || taskRes.Status == TaskRunning {
					successN++
				} else {
					failedN++
				}
			}
		default:
			taskRes = s.createLocalDraftForTarget(ctx, productID, plat, sid, batch.ID, adminID, chk)
			if taskRes.Status == TaskSuccess {
				successN++
			} else {
				failedN++
			}
		}
		results = append(results, taskRes)
	}

	batchStatus := BatchSuccess
	if failedN > 0 && successN > 0 {
		batchStatus = BatchPartialSuccess
	} else if failedN > 0 && successN == 0 {
		batchStatus = BatchFailed
	}
	fin := time.Now().UTC()
	sumRaw, _ := json.Marshal(map[string]any{
		"targetCount": len(targets), "successCount": successN, "failedCount": failedN, "skippedCount": skippedN,
	})
	_ = s.DB.WithContext(ctx).Model(&ProductPublishBatch{}).Where("id = ?", batch.ID).Updates(map[string]any{
		"status":        batchStatus,
		"success_count": successN,
		"failed_count":  failedN,
		"target_count":  len(targets),
		"summary":       datatypes.JSON(sumRaw),
		"finished_at":   &fin,
	}).Error

	return &PublishTargetsCreateDraftsResponse{
		BatchID:      batch.ID.String(),
		Status:       batchStatus,
		StatusLabel:  opslabels.StatusLabel(batchStatus),
		TargetCount:  len(targets),
		SuccessCount: successN,
		FailedCount:  failedN,
		SkippedCount: skippedN,
		Targets:      results,
	}, nil
}

func (s *Service) createDouyinDraftForTarget(c *gin.Context, productID, shopID, batchID uuid.UUID, adminID *uuid.UUID, force bool, chk PublishTargetCheckResult) PublishTargetTaskResult {
	key := publishTargetKey("douyin_shop", &shopID)
	base := PublishTargetTaskResult{
		TargetKey:     key,
		Platform:      "douyin_shop",
		PlatformLabel: opslabels.PlatformLabel("douyin_shop"),
		ShopID:        shopID.String(),
		ShopName:      chk.ShopName,
		Capability:    CapRealDraftCreate,
	}
	out, err := s.CreateDouyinDraftTask(c, productID, DouyinCreateDraftBody{
		ShopID:      shopID.String(),
		PublishMode: PublishModeSaveAsPlatformDraft,
		Force:       force,
	}, adminID)
	if err != nil {
		base.Status = TaskFailed
		base.StatusLabel = opslabels.StatusLabel(TaskFailed)
		base.ErrorMessage = err.Error()
		var blocked *productcheck.BlockedError
		if errors.As(err, &blocked) && blocked.Result != nil {
			if len(blocked.Result.Checks) > 0 {
				base.ErrorMessage = blocked.Result.Checks[0].Message
			}
		}
		return base
	}
	_ = s.DB.WithContext(c.Request.Context()).Model(&ProductPublishTask{}).Where("id = ?", out.ID).
		Updates(map[string]any{"batch_id": batchID, "target_key": key}).Error
	base.TaskID = out.ID.String()
	base.Status = out.Status
	base.StatusLabel = opslabels.StatusLabel(out.Status)
	base.PlatformProductID = out.PlatformProductID
	var pub ProductPublication
	if err := s.DB.WithContext(c.Request.Context()).Where("publish_task_id = ?", out.ID).First(&pub).Error; err == nil {
		base.PublicationID = pub.ID.String()
	}
	return base
}

func (s *Service) createLocalDraftForTarget(ctx context.Context, productID uuid.UUID, plat string, sid *uuid.UUID, batchID uuid.UUID, adminID *uuid.UUID, chk PublishTargetCheckResult) PublishTargetTaskResult {
	key := publishTargetKey(plat, sid)
	res := PublishTargetTaskResult{
		TargetKey:      key,
		Platform:       plat,
		PlatformLabel:  opslabels.PlatformLabel(plat),
		ShopID:         shopIDString(sid),
		ShopName:       chk.ShopName,
		Capability:     CapLocalDraftOnly,
		LocalDraftOnly: true,
	}
	if sid == nil {
		res.Status = TaskFailed
		res.StatusLabel = opslabels.StatusLabel(TaskFailed)
		res.ErrorMessage = "本地草稿需要选择店铺"
		return res
	}

	var prod product.Product
	if err := s.DB.WithContext(ctx).
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order ASC, created_at ASC") }).
		Preload("SKUs", func(db *gorm.DB) *gorm.DB { return db.Order("created_at ASC") }).
		First(&prod, "id = ?", productID).Error; err != nil {
		res.Status = TaskFailed
		res.StatusLabel = opslabels.StatusLabel(TaskFailed)
		res.ErrorMessage = err.Error()
		return res
	}
	draft, err := BuildPlatformDraftFromProduct(prod)
	if err != nil {
		res.Status = TaskFailed
		res.StatusLabel = opslabels.StatusLabel(TaskFailed)
		res.ErrorMessage = err.Error()
		return res
	}

	fin := time.Now().UTC()
	pubRow := ProductPublication{
		ProductID:     productID,
		ShopID:        *sid,
		Platform:      plat,
		Status:        StatusDraft,
		PublishStatus: StatusDraftCreated,
		Title:         draft.Title,
		Currency:      draft.Currency,
		PublishMode:   PublishModeSaveAsPlatformDraft,
		CreatedBy:     adminID,
		LastSyncedAt:  &fin,
	}
	snap := map[string]any{
		"localDraftOnly": true,
		"capability":     CapLocalDraftOnly,
		"title":          draft.Title,
		"description":    draft.Description,
		"currency":       draft.Currency,
		"imageCount":     len(draft.Images),
		"skuCount":       len(draft.SKUs),
	}
	snapRaw, _ := json.Marshal(snap)
	pubRow.RawData = datatypes.JSON(snapRaw)
	if err := s.DB.WithContext(ctx).Create(&pubRow).Error; err != nil {
		res.Status = TaskFailed
		res.StatusLabel = opslabels.StatusLabel(TaskFailed)
		res.ErrorMessage = err.Error()
		return res
	}

	outSnap, _ := json.Marshal(snap)
	task := ProductPublishTask{
		ProductID:     productID,
		ShopID:        *sid,
		TargetStoreID: *sid,
		Platform:      plat,
		BatchID:       &batchID,
		TargetKey:     key,
		TaskType:      TaskTypeLocalDraftCreate,
		Status:        TaskSuccess,
		PublishStatus: StatusDraftCreated,
		Mode:          PublishModeSaveAsPlatformDraft,
		PublishMode:   PublishModeSaveAsPlatformDraft,
		Title:         draft.Title,
		Description:   draft.Description,
		Output:        datatypes.JSON(outSnap),
		FinishedAt:    &fin,
		CreatedBy:     adminID,
	}
	if err := s.DB.WithContext(ctx).Create(&task).Error; err != nil {
		res.Status = TaskFailed
		res.StatusLabel = opslabels.StatusLabel(TaskFailed)
		res.ErrorMessage = err.Error()
		return res
	}
	_ = s.DB.WithContext(ctx).Model(&ProductPublication{}).Where("id = ?", pubRow.ID).
		Updates(map[string]any{"publish_task_id": task.ID}).Error

	res.TaskID = task.ID.String()
	res.PublicationID = pubRow.ID.String()
	res.Status = TaskSuccess
	res.StatusLabel = opslabels.StatusLabel(TaskSuccess)
	return res
}

func (s *Service) failedTargetsFromBatch(ctx context.Context, batchID string) ([]PublishTargetRef, error) {
	bid, err := uuid.Parse(strings.TrimSpace(batchID))
	if err != nil {
		return nil, fmt.Errorf("invalid batchId")
	}
	var tasks []ProductPublishTask
	if err := s.DB.WithContext(ctx).Where("batch_id = ? AND status = ?", bid, TaskFailed).Find(&tasks).Error; err != nil {
		return nil, err
	}
	out := make([]PublishTargetRef, 0, len(tasks))
	for _, t := range tasks {
		sid := t.ShopID.String()
		out = append(out, PublishTargetRef{Platform: t.Platform, ShopID: &sid})
	}
	if len(out) == 0 {
		var batch ProductPublishBatch
		if err := s.DB.WithContext(ctx).First(&batch, "id = ?", bid).Error; err != nil {
			return nil, err
		}
		var in PublishTargetsCreateDraftsRequest
		_ = json.Unmarshal(batch.Input, &in)
		out = in.Targets
	}
	return out, nil
}

// GetPublishBatch returns batch summary with child tasks.
func (s *Service) GetPublishBatch(ctx context.Context, batchID uuid.UUID) (*ProductPublishBatch, []ProductPublishTask, error) {
	var batch ProductPublishBatch
	if err := s.DB.WithContext(ctx).First(&batch, "id = ?", batchID).Error; err != nil {
		return nil, nil, err
	}
	var tasks []ProductPublishTask
	_ = s.DB.WithContext(ctx).Where("batch_id = ?", batchID).Order("created_at ASC").Find(&tasks).Error
	return &batch, tasks, nil
}
