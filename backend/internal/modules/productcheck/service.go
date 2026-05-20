package productcheck

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/httppublic"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/gorm"
)

const (
	levelWarning = "warning"
	levelError   = "error"
)

const (
	statusReady   = "ready"
	statusWarning = "warning"
	statusBlocked = "blocked"
)

// Service evaluates product readiness without mutating data or calling platform APIs.
type Service struct {
	DB       *gorm.DB
	Settings *settings.Service
	Shops    *shop.Service
}

// CheckProductReadiness runs all applicable checks for one product.
func (s *Service) CheckProductReadiness(ctx context.Context, req CheckProductReadinessRequest) (*CheckProductReadinessResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product check unavailable")
	}
	var prod product.Product
	if err := s.DB.WithContext(ctx).
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order ASC, created_at ASC") }).
		Preload("SKUs", func(db *gorm.DB) *gorm.DB { return db.Order("created_at ASC") }).
		First(&prod, "id = ?", req.ProductID).Error; err != nil {
		return nil, err
	}

	out := &CheckProductReadinessResult{
		ProductID: req.ProductID,
		Mode:      strings.TrimSpace(req.Mode),
		Checks:    nil,
	}
	plat := strings.ToLower(strings.TrimSpace(req.Platform))
	if plat != "" {
		out.Platform = plat
	}
	if req.ShopID != nil && *req.ShopID != uuid.Nil {
		sid := *req.ShopID
		out.ShopID = &sid
	}

	checks := make([]CheckItem, 0, 32)
	checks = append(checks, checkProductBasics(prod)...)
	checks = append(checks, checkSKUBasics(prod)...)
	checks = append(checks, checkSKUPricing(prod)...)
	checks = append(checks, checkPinduoduoCollectHints(prod)...)

	imgChecks, mainURLs := checkImages(prod, plat)
	checks = append(checks, imgChecks...)

	checks = append(checks, checkInventoryHints(prod)...)

	if plat != "" {
		pc, err := s.checkPlatform(ctx, plat, req.ShopID, prod, req.PublishOptions)
		if err != nil {
			return nil, err
		}
		checks = append(checks, pc...)
	}

	out.Checks = checks
	var errN, warnN int
	for _, c := range checks {
		switch c.Level {
		case levelError:
			errN++
		case levelWarning:
			warnN++
		}
	}
	out.ErrorCount = errN
	out.WarningCount = warnN
	out.CanPublish = errN == 0
	switch {
	case errN > 0:
		out.Status = statusBlocked
	case warnN > 0:
		out.Status = statusWarning
	default:
		out.Status = statusReady
	}
	out.Score = readinessScore(errN, warnN)
	_ = mainURLs // reserved if we need cross-checks later
	return out, nil
}

func readinessScore(errN, warnN int) int {
	sc := 100 - errN*20 - warnN*5
	if sc < 0 {
		return 0
	}
	return sc
}

func checkProductBasics(p product.Product) []CheckItem {
	var out []CheckItem
	title := strings.TrimSpace(p.Title)
	aiTitle := strings.TrimSpace(p.AITitle)
	if title == "" && aiTitle == "" {
		out = append(out, CheckItem{
			Group:      "product",
			Code:       "product.title_missing",
			Level:      levelError,
			Message:    "商品标题缺失",
			Suggestion: "请填写商品标题或应用 AI 标题。",
		})
	}
	desc := strings.TrimSpace(p.Description)
	aiDesc := strings.TrimSpace(p.AIDescription)
	if desc == "" && aiDesc == "" {
		out = append(out, CheckItem{
			Group:      "product",
			Code:       "product.description_missing",
			Level:      levelWarning,
			Message:    "商品描述缺失",
			Suggestion: "建议填写描述或生成并应用 AI 描述后再刊登。",
		})
	}
	if strings.TrimSpace(p.Currency) == "" {
		out = append(out, CheckItem{
			Group:      "product",
			Code:       "product.currency_missing",
			Level:      levelError,
			Message:    "币种未填写",
			Suggestion: "请在基础信息中填写 currency。",
		})
	}
	if strings.TrimSpace(p.Status) == product.StatusArchived {
		out = append(out, CheckItem{
			Group:      "product",
			Code:       "product.archived",
			Level:      levelError,
			Message:    "商品已归档",
			Suggestion: "请先恢复为草稿或就绪状态后再尝试刊登。",
		})
	}
	return out
}

func checkSKUBasics(p product.Product) []CheckItem {
	var out []CheckItem
	if len(p.SKUs) == 0 {
		out = append(out, CheckItem{
			Group:      "sku",
			Code:       "sku.none",
			Level:      levelError,
			Message:    "未配置 SKU",
			Suggestion: "请在 SKU 标签页至少添加一个 SKU。",
		})
		return out
	}
	for _, s := range p.SKUs {
		sid := s.ID.String()
		if s.Price == nil {
			continue
		}
		pr := *s.Price
		if pr <= 0 {
			out = append(out, CheckItem{
				Group:               "pricing",
				Code:                "pricing.price_invalid",
				Level:               levelError,
				Message:             "SKU 价格无效（需大于 0）",
				Suggestion:          "请为每个 SKU 填写有效售价或应用定价规则。",
				RelatedResourceType: "product_sku",
				RelatedResourceID:   sid,
			})
		}
		st := 0
		if s.Stock != nil {
			st = *s.Stock
		}
		if s.Stock != nil && st < 0 {
			out = append(out, CheckItem{
				Group:               "sku",
				Code:                "sku.stock_negative",
				Level:               levelError,
				Message:             "SKU 库存不能为负数",
				Suggestion:          "请修正库存数量。",
				RelatedResourceType: "product_sku",
				RelatedResourceID:   sid,
			})
		}
		if strings.TrimSpace(s.SKUCode) == "" {
			out = append(out, CheckItem{
				Group:               "sku",
				Code:                "sku.code_missing",
				Level:               levelWarning,
				Message:             "SKU 编码为空",
				Suggestion:          "建议填写 SKUCode 便于平台映射与库存同步。",
				RelatedResourceType: "product_sku",
				RelatedResourceID:   sid,
			})
		}
	}
	return out
}

func checkSKUPricing(p product.Product) []CheckItem {
	var out []CheckItem
	for _, s := range p.SKUs {
		sid := s.ID.String()
		if s.Price == nil {
			out = append(out, CheckItem{
				Group:               "pricing",
				Code:                "pricing.price_missing",
				Level:               levelError,
				Message:             "SKU 销售价未设置",
				Suggestion:          "请填写 SKU 销售价，或在 SKU 标签页应用定价规则。",
				RelatedResourceType: "product_sku",
				RelatedResourceID:   sid,
			})
			continue
		}
		pr := *s.Price
		if pr <= 0 {
			out = append(out, CheckItem{
				Group:               "pricing",
				Code:                "pricing.price_invalid",
				Level:               levelError,
				Message:             "SKU 销售价无效（需大于 0）",
				Suggestion:          "请修正 SKU 销售价或应用定价规则。",
				RelatedResourceType: "product_sku",
				RelatedResourceID:   sid,
			})
		}
		if s.CostPrice != nil && *s.CostPrice > 0 && pr < *s.CostPrice {
			out = append(out, CheckItem{
				Group:               "pricing",
				Code:                "pricing.price_below_cost",
				Level:               levelWarning,
				Message:             "销售价低于成本价",
				Suggestion:          "建议检查加价规则或手动调整销售价。",
				RelatedResourceType: "product_sku",
				RelatedResourceID:   sid,
			})
		}
		if s.MinPublishPrice != nil && *s.MinPublishPrice > 0 && pr < *s.MinPublishPrice {
			out = append(out, CheckItem{
				Group:               "pricing",
				Code:                "pricing.price_below_min_publish_price",
				Level:               levelWarning,
				Message:             "销售价低于最低发布价保护",
				Suggestion:          "请提高销售价或调整 SKU 最低发布价。",
				RelatedResourceType: "product_sku",
				RelatedResourceID:   sid,
			})
		}
		if s.CompareAtPrice != nil && *s.CompareAtPrice > 0 && *s.CompareAtPrice < pr {
			out = append(out, CheckItem{
				Group:               "pricing",
				Code:                "pricing.compare_at_below_price",
				Level:               levelWarning,
				Message:             "划线价低于当前销售价",
				Suggestion:          "请检查 compare_at_price 是否填写正确。",
				RelatedResourceType: "product_sku",
				RelatedResourceID:   sid,
			})
		}
	}
	return out
}

// checkImages returns findings and resolved main image URLs for optional reuse.
func checkImages(p product.Product, platform string) ([]CheckItem, []string) {
	var out []CheckItem
	imgs := append([]product.ProductImage(nil), p.Images...)
	if len(imgs) == 0 {
		out = append(out, CheckItem{
			Group:      "image",
			Code:       "image.main_missing",
			Level:      levelError,
			Message:    "缺少商品图片",
			Suggestion: "请在图片标签页至少上传一张主图。",
		})
		return out, nil
	}
	sort.SliceStable(imgs, func(i, j int) bool {
		if imgs[i].SortOrder == imgs[j].SortOrder {
			return imgs[i].CreatedAt.Before(imgs[j].CreatedAt)
		}
		return imgs[i].SortOrder < imgs[j].SortOrder
	})

	hasMain := false
	var mainURLs []string
	for _, im := range imgs {
		imgType := strings.TrimSpace(strings.ToLower(im.ImageType))
		if imgType == product.ImageTypeDescription {
			imgType = product.ImageTypeDetail
		}
		pub := strings.TrimSpace(im.PublicURL)
		orig := strings.TrimSpace(im.OriginURL)
		key := strings.TrimSpace(im.ObjectKey)
		if pub == "" && orig == "" && key == "" {
			out = append(out, CheckItem{
				Group:               "image",
				Code:                "image.asset_missing",
				Level:               levelError,
				Message:             "图片记录缺少可解析的 URL 或存储关联",
				Suggestion:          "请为该图片补充 publicUrl、originUrl 或已上传文件的 objectKey。",
				RelatedResourceType: "product_image",
				RelatedResourceID:   im.ID.String(),
			})
			continue
		}
		if imgType == product.ImageTypeMain {
			hasMain = true
			mainURLs = append(mainURLs, bestImageURLForPublicCheck(pub, orig, key))
		}
	}
	if !hasMain {
		for i := range imgs {
			imgType := strings.TrimSpace(strings.ToLower(imgs[i].ImageType))
			if imgType == product.ImageTypeDescription {
				imgType = product.ImageTypeDetail
			}
			if imgType == product.ImageTypeSKU {
				continue
			}
			hasMain = true
			pub := strings.TrimSpace(imgs[i].PublicURL)
			orig := strings.TrimSpace(imgs[i].OriginURL)
			key := strings.TrimSpace(imgs[i].ObjectKey)
			mainURLs = append(mainURLs, bestImageURLForPublicCheck(pub, orig, key))
			break
		}
	}
	if !hasMain {
		out = append(out, CheckItem{
			Group:      "image",
			Code:       "image.main_missing",
			Level:      levelError,
			Message:    "缺少主图",
			Suggestion: "请至少指定一张 main 类型图片，或添加非 SKU 图作为封面。",
		})
		return out, nil
	}

	if platform != "" {
		for _, raw := range mainURLs {
			out = append(out, classifyImagePublicness(raw, platform)...)
		}
	}
	return out, mainURLs
}

func bestImageURLForPublicCheck(pub, orig, key string) string {
	if pub != "" {
		return pub
	}
	if orig != "" {
		return orig
	}
	return key
}

func classifyImagePublicness(raw string, platform string) []CheckItem {
	s := strings.TrimSpace(raw)
	if s == "" {
		return []CheckItem{{
			Group:      "image",
			Code:       "image.not_public_unknown",
			Level:      levelWarning,
			Message:    "无法判断主图是否公网可访问",
			Suggestion: "请配置带 http(s) 的公开图片地址，或确认云端 public_url/CDN。",
		}}
	}
	// Only object_key without URL → platforms that need migrate generally require public URLs.
	if !strings.Contains(s, "://") {
		if strings.ToLower(strings.TrimSpace(platform)) == "amazon" {
			return []CheckItem{{
				Group:      "image",
				Code:       "image.not_public_amazon",
				Level:      levelError,
				Message:    "Amazon 刊登要求主图为公网可访问 URL",
				Suggestion: "请使用可公网访问的 https 图片地址（非仅 object_key）。",
			}}
		}
		return []CheckItem{{
			Group:      "image",
			Code:       "image.not_public",
			Level:      levelWarning,
			Message:    "主图可能非公网 URL，部分平台刊登可能失败",
			Suggestion: "建议使用公网 https 图片或配置存储的稳定外链/CDN。",
		}}
	}
	if httppublic.IsPublicHTTPURL(s) {
		return nil
	}
	if strings.ToLower(strings.TrimSpace(platform)) == "amazon" {
		return []CheckItem{{
			Group:      "image",
			Code:       "image.not_public_amazon",
			Level:      levelError,
			Message:    "Amazon 刊登要求主图为公网可访问 URL",
			Suggestion: "请更换为公网 https 图片地址。",
		}}
	}
	return []CheckItem{{
		Group:      "image",
		Code:       "image.not_public",
		Level:      levelWarning,
		Message:    "主图可能非公网可访问",
		Suggestion: "请确认第三方可拉取该图片 URL；localhost / 内网地址不可用于平台上图。",
	}}
}

func checkInventoryHints(p product.Product) []CheckItem {
	var out []CheckItem
	for _, s := range p.SKUs {
		sid := s.ID.String()
		st := 0
		if s.Stock != nil {
			st = *s.Stock
		}
		if st == 0 {
			out = append(out, CheckItem{
				Group:               "inventory",
				Code:                "inventory.stock_zero",
				Level:               levelWarning,
				Message:             "SKU 库存为 0",
				Suggestion:          "仍可刊登，但请注意缺货风险。",
				RelatedResourceType: "product_sku",
				RelatedResourceID:   sid,
			})
		}
		warn := s.WarningStock
		safe := s.SafetyStock
		if safe > 0 && st <= safe {
			out = append(out, CheckItem{
				Group:               "inventory",
				Code:                "inventory.below_safety_stock",
				Level:               levelWarning,
				Message:             "SKU 库存低于安全库存线",
				Suggestion:          "可在库存标签页调整阈值或补货；不阻止刊登。",
				RelatedResourceType: "product_sku",
				RelatedResourceID:   sid,
			})
		}
		if warn > 0 && st <= warn && st > 0 {
			out = append(out, CheckItem{
				Group:               "inventory",
				Code:                "inventory.below_warning_stock",
				Level:               levelWarning,
				Message:             "SKU 库存低于预警线",
				Suggestion:          "请关注补货；不阻止刊登。",
				RelatedResourceType: "product_sku",
				RelatedResourceID:   sid,
			})
		}
	}
	return out
}

func (s *Service) checkPlatform(ctx context.Context, plat string, shopID *uuid.UUID, prod product.Product, publishOpts map[string]any) ([]CheckItem, error) {
	var out []CheckItem
	prov := platformp.Get(plat)
	if prov == nil {
		out = append(out, CheckItem{
			Group:      "platform",
			Code:       "platform.unknown",
			Level:      levelError,
			Message:    "未知平台",
			Suggestion: "请选择 tiktok / shopee / lazada / amazon / mock 等平台。",
		})
		return out, nil
	}
	pubImpl := platformp.ProductPublishImplementationStatus(prov)
	if pubImpl != platformp.StatusAvailable && pubImpl != platformp.StatusBeta {
		out = append(out, CheckItem{
			Group:      "platform",
			Code:       "platform.publish_not_runnable",
			Level:      levelError,
			Message:    "当前平台商品刊登能力未开放或为规划中状态",
			Suggestion: "该平台暂未接入真实刊登或处于 disabled/planned，请改用已支持平台或等待后续版本。",
		})
	}
	if plat != "mock" && plat != "manual" {
		sch := prov.AppConfigSchema()
		gk := strings.TrimSpace(sch.GroupKey)
		if gk != "" {
			m, err := s.Settings.PlainByGroup(ctx, 0, gk)
			if err != nil {
				return nil, err
			}
			if err := ensurePartnerOpenConfigPlain(m, sch); err != nil {
				out = append(out, CheckItem{
					Group:      "platform",
					Code:       "platform.partner_incomplete",
					Level:      levelError,
					Message:    "平台开放应用配置不完整",
					Suggestion: "请到「设置 → 平台开放配置」补齐必填项（密钥已配置可填脱敏占位）。",
				})
			}
		}
	}

	pubSch := prov.PublishConfigSchema()
	pubGK := strings.TrimSpace(pubSch.GroupKey)
	if pubGK == "" && plat != "mock" && plat != "manual" {
		out = append(out, CheckItem{
			Group:      "platform",
			Code:       "platform.publish_schema_missing",
			Level:      levelError,
			Message:    "平台未配置刊登设置分组",
			Suggestion: "请联系管理员检查 Provider 的 PublishConfigSchema。",
		})
	} else if pubGK != "" {
		curPub, err := s.Settings.PlainByGroup(ctx, 0, pubGK)
		if err != nil {
			return nil, err
		}
		merged := mergePublishBaselineFromSchema(pubSch, curPub)
		merged = applyPublishOptionsStrings(merged, publishOpts)
		if err := validateMergedPublishAgainstSchema(pubSch, merged); err != nil {
			out = append(out, CheckItem{
				Group:      "platform",
				Code:       "platform.publish_config_incomplete",
				Level:      levelError,
				Message:    "平台刊登预设不完整",
				Suggestion: "请到「设置 → 平台刊登配置」补齐必填字段。",
			})
		}
		for _, k := range supplementalPublishRequiredKeys(plat) {
			if strings.TrimSpace(publishPickField(merged, k)) == "" {
				out = append(out, CheckItem{
					Group:      "platform",
					Code:       "platform.publish_field_missing",
					Level:      levelError,
					Message:    fmt.Sprintf("缺少刊登关键字段：%s", k),
					Suggestion: "请在平台刊登配置中补充该字段。",
				})
			}
		}
	}

	if shopID == nil || *shopID == uuid.Nil {
		out = append(out, CheckItem{
			Group:      "platform",
			Code:       "platform.shop_required",
			Level:      levelError,
			Message:    "请选择店铺以校验授权与刊登条件",
			Suggestion: "请选择目标店铺后重新检查，或直接前往店铺管理完成授权。",
		})
		return out, nil
	}

	row, plainAuth, err := s.Shops.PlainAuthForProviderCtx(ctx, *shopID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			out = append(out, CheckItem{
				Group:      "platform",
				Code:       "platform.shop_not_found",
				Level:      levelError,
				Message:    "店铺不存在",
				Suggestion: "请刷新后选择有效店铺。",
			})
			return out, nil
		}
		return nil, err
	}
	if row == nil {
		out = append(out, CheckItem{
			Group:      "platform",
			Code:       "platform.shop_not_found",
			Level:      levelError,
			Message:    "店铺不存在",
			Suggestion: "请刷新后选择有效店铺。",
		})
		return out, nil
	}
	shopPlat := strings.ToLower(strings.TrimSpace(row.Platform))
	if shopPlat != plat {
		out = append(out, CheckItem{
			Group:      "platform",
			Code:       "platform.shop_platform_mismatch",
			Level:      levelError,
			Message:    "店铺平台与所选平台不一致",
			Suggestion: "请选择对应平台的店铺。",
		})
	}
	if prod.TenantID != 0 && row.TenantID != 0 && prod.TenantID != row.TenantID {
		out = append(out, CheckItem{
			Group:      "platform",
			Code:       "platform.tenant_mismatch",
			Level:      levelError,
			Message:    "商品与店铺租户不一致",
			Suggestion: "请改用同租户下的店铺。",
		})
	}
	if strings.TrimSpace(row.Status) != shop.StatusActive {
		out = append(out, CheckItem{
			Group:      "platform",
			Code:       "platform.shop_inactive",
			Level:      levelError,
			Message:    "店铺未处于启用状态",
			Suggestion: "请先在店铺管理中启用店铺。",
		})
	}
	if strings.TrimSpace(row.AuthStatus) != shop.AuthAuthorized {
		out = append(out, CheckItem{
			Group:      "platform",
			Code:       "platform.shop_not_authorized",
			Level:      levelError,
			Message:    "店铺未完成授权",
			Suggestion: "请前往店铺管理完成 OAuth 授权。",
		})
	}
	if plat != "mock" {
		if strings.TrimSpace(plainAuth.AccessToken) == "" && strings.TrimSpace(plainAuth.RefreshToken) == "" {
			out = append(out, CheckItem{
				Group:      "platform",
				Code:       "platform.shop_token_missing",
				Level:      levelError,
				Message:    "店铺缺少有效授权凭证",
				Suggestion: "请重新授权店铺以写入 Access / Refresh Token。",
			})
		}
	}
	return out, nil
}
