package productpublish

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

const ErrorPublishConfigInvalid = "PUBLISH_CONFIG_INVALID"

// PublishConfigInvalidError is returned when batch publish config fails validation.
type PublishConfigInvalidError struct {
	Title            string
	Message          string
	TechnicalDetails map[string]any
}

func (e *PublishConfigInvalidError) Error() string {
	if e == nil {
		return "刊登配置不正确"
	}
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	return "刊登配置不正确"
}

func newConfigInvalid(field, message string) *PublishConfigInvalidError {
	return &PublishConfigInvalidError{
		Title:   "刊登配置不正确",
		Message: message,
		TechnicalDetails: map[string]any{
			"field": field,
			"code":  ErrorPublishConfigInvalid,
		},
	}
}

var (
	validPriceStrategies = map[string]bool{
		"use_current_price": true, "cost_plus_fixed": true,
		"cost_plus_percent": true, "multiplier": true,
	}
	validDecimalHandling = map[string]bool{
		"round": true, "ceil": true, "floor": true, "two_decimal": true,
	}
	validMainImageStrategies = map[string]bool{
		"use_current": true, "prefer_ai_processed": true, "platform_synced_only": true,
	}
	validDetailImageStrategies = map[string]bool{
		"use_current": true, "prefer_ai_processed": true, "platform_synced_only": true, "skip": true,
	}
	validInventoryStrategies = map[string]bool{
		"use_current": true, "fixed_quantity": true,
		"skip_when_unknown": true, "mark_needs_check": true,
	}
	validOutOfStockActions = map[string]bool{
		"skip": true, "mark_needs_check": true, "zero": true,
	}
	validWeightUnits = map[string]bool{"kg": true, "g": true}
	validSizeUnits   = map[string]bool{"cm": true}
	// Legacy flat fields (A2 MVP).
	validLegacyImageStrategies = map[string]bool{
		"main_only": true, "main_and_detail": true,
	}
	validLegacyStockStrategies = map[string]bool{
		"sync_local": true, "fixed": true,
	}
)

const maxPackageDimension = 10000
const maxPackageWeight = 10000

func (s *Service) validateBatchPublishConfig(
	ctx context.Context,
	productIDs []uuid.UUID,
	targets []PublishTargetRef,
	common map[string]any,
	overrides PublishConfigOverrides,
) error {
	if err := validatePublishConfigLayer("commonConfig", common); err != nil {
		return err
	}
	allowedProducts := map[string]struct{}{}
	for _, id := range productIDs {
		allowedProducts[id.String()] = struct{}{}
	}
	allowedPlatforms, allowedShops, shopPlatform, err := s.batchTargetScope(ctx, targets)
	if err != nil {
		return err
	}

	if overrides.Products != nil {
		for pid, layer := range overrides.Products {
			if _, ok := allowedProducts[strings.TrimSpace(pid)]; !ok {
				return newConfigInvalid("overrides.products."+pid, "商品覆盖中的商品不在本次批量选择范围内。")
			}
			if err := validatePublishConfigLayer("overrides.products."+pid, layer); err != nil {
				return err
			}
		}
	}
	if overrides.Platforms != nil {
		for plat, layer := range overrides.Platforms {
			p := strings.TrimSpace(strings.ToLower(plat))
			if !allowedPlatforms[p] {
				return newConfigInvalid("overrides.platforms."+p, "平台覆盖中的平台不在本次刊登目标范围内。")
			}
			if !isRegisteredPublishPlatform(p) {
				return newConfigInvalid("overrides.platforms."+p, "平台「"+p+"」尚未注册或不支持刊登。")
			}
			if err := validatePublishConfigLayer("overrides.platforms."+p, layer); err != nil {
				return err
			}
		}
	}
	if overrides.Shops != nil {
		for sid, layer := range overrides.Shops {
			id := strings.TrimSpace(sid)
			if !allowedShops[id] {
				return newConfigInvalid("overrides.shops."+id, "店铺覆盖中的店铺不在本次刊登目标范围内。")
			}
			if err := validatePublishConfigLayer("overrides.shops."+id, layer); err != nil {
				return err
			}
		}
	}
	if overrides.ProductTargets != nil {
		for key, layer := range overrides.ProductTargets {
			pid, plat, sid, err := parseProductTargetOverrideKey(key)
			if err != nil {
				return newConfigInvalid("overrides.productTargets."+key, err.Error())
			}
			if _, ok := allowedProducts[pid]; !ok {
				return newConfigInvalid("overrides.productTargets."+key, "商品目标覆盖中的商品不在本次批量选择范围内。")
			}
			if !allowedPlatforms[plat] {
				return newConfigInvalid("overrides.productTargets."+key, "商品目标覆盖中的平台不在本次刊登目标范围内。")
			}
			if sid != "" && !allowedShops[sid] {
				return newConfigInvalid("overrides.productTargets."+key, "商品目标覆盖中的店铺不在本次刊登目标范围内。")
			}
			if sid != "" && shopPlatform[sid] != "" && shopPlatform[sid] != plat {
				return newConfigInvalid("overrides.productTargets."+key, "商品目标覆盖中的店铺与平台不匹配。")
			}
			if err := validatePublishConfigLayer("overrides.productTargets."+key, layer); err != nil {
				return err
			}
		}
	}
	return nil
}

func parseProductTargetOverrideKey(key string) (productID, platform, shopID string, err error) {
	parts := strings.Split(strings.TrimSpace(key), ":")
	if len(parts) < 2 || len(parts) > 3 {
		return "", "", "", fmt.Errorf("商品目标覆盖键格式不正确，应为「商品ID:平台」或「商品ID:平台:店铺ID」。")
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(strings.ToLower(parts[1])), strings.TrimSpace(parts[2]), nil
}

func (s *Service) batchTargetScope(ctx context.Context, targets []PublishTargetRef) (
	allowedPlatforms map[string]bool,
	allowedShops map[string]bool,
	shopPlatform map[string]string,
	err error,
) {
	allowedPlatforms = map[string]bool{}
	allowedShops = map[string]bool{}
	shopPlatform = map[string]string{}
	shopIDs := make([]uuid.UUID, 0)
	for _, t := range targets {
		plat := strings.TrimSpace(strings.ToLower(t.Platform))
		if plat == "" {
			continue
		}
		allowedPlatforms[plat] = true
		if t.ShopID != nil && strings.TrimSpace(*t.ShopID) != "" {
			if u, e := uuid.Parse(strings.TrimSpace(*t.ShopID)); e == nil {
				allowedShops[u.String()] = true
				shopIDs = append(shopIDs, u)
			}
		}
	}
	if s != nil && s.DB != nil && len(shopIDs) > 0 {
		var rows []shop.Shop
		if e := s.DB.WithContext(ctx).Where("id IN ?", shopIDs).Find(&rows).Error; e != nil {
			return nil, nil, nil, e
		}
		for _, row := range rows {
			sid := row.ID.String()
			shopPlatform[sid] = strings.TrimSpace(strings.ToLower(row.Platform))
			plat := shopPlatform[sid]
			if plat != "" && allowedPlatforms[plat] == false {
				// shop listed under a target but platform key mismatch
			}
			for _, t := range targets {
				tp := strings.TrimSpace(strings.ToLower(t.Platform))
				if t.ShopID != nil && strings.TrimSpace(*t.ShopID) == sid && tp != plat {
					return nil, nil, nil, newConfigInvalid(
						"targets",
						fmt.Sprintf("店铺「%s」不属于平台「%s」。", row.ShopName, tp),
					)
				}
			}
		}
	}
	return allowedPlatforms, allowedShops, shopPlatform, nil
}

func isRegisteredPublishPlatform(plat string) bool {
	for _, prov := range platformp.All() {
		if prov == nil {
			continue
		}
		if strings.TrimSpace(strings.ToLower(prov.Platform())) == plat &&
			platformp.HasCapability(prov, platformp.CapProductPublish) {
			return true
		}
	}
	return false
}

func validatePublishConfigLayer(prefix string, layer map[string]any) error {
	if len(layer) == 0 {
		return nil
	}
	for k, v := range layer {
		path := prefix + "." + k
		switch k {
		case "price":
			if m, ok := v.(map[string]any); ok {
				if err := validatePriceConfig(path, m); err != nil {
					return err
				}
			}
		case "image":
			if m, ok := v.(map[string]any); ok {
				if err := validateImageConfig(path, m); err != nil {
					return err
				}
			}
		case "inventory":
			if m, ok := v.(map[string]any); ok {
				if err := validateInventoryConfig(path, m); err != nil {
					return err
				}
			}
		case "package":
			if m, ok := v.(map[string]any); ok {
				if err := validatePackageConfig(path, m); err != nil {
					return err
				}
			}
		case "remark":
			// free text
		case "priceRule", "packageSize":
			// legacy free text
		case "imageStrategy":
			if s, ok := v.(string); ok && s != "" && !validLegacyImageStrategies[s] && !validMainImageStrategies[s] {
				return newConfigInvalid(path, "不支持的图片策略："+s)
			}
		case "stockStrategy":
			if s, ok := v.(string); ok && s != "" && !validLegacyStockStrategies[s] && !validInventoryStrategies[s] {
				return newConfigInvalid(path, "不支持的库存策略："+s)
			}
		case "packageWeight":
			if err := validatePositiveNumber(path, v, maxPackageWeight); err != nil {
				return err
			}
		default:
			if nested, ok := v.(map[string]any); ok {
				if err := validatePublishConfigLayer(path, nested); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func validatePriceConfig(prefix string, m map[string]any) error {
	if strat, ok := m["strategy"].(string); ok && strat != "" && !validPriceStrategies[strat] {
		return newConfigInvalid(prefix+".strategy", "不支持的价格策略："+strat)
	}
	if v, ok := m["markupValue"]; ok && v != nil {
		if err := validateNonNegativeNumber(prefix+".markupValue", v); err != nil {
			return err
		}
		if strat, _ := m["strategy"].(string); strat == "multiplier" {
			if f, ok := toFloat64(v); !ok || f <= 0 {
				return newConfigInvalid(prefix+".markupValue", "倍率必须大于 0。")
			}
		}
	}
	if v, ok := m["minProfitMargin"]; ok && v != nil {
		if err := validateNonNegativeNumber(prefix+".minProfitMargin", v); err != nil {
			return err
		}
	}
	if dh, ok := m["decimalHandling"].(string); ok && dh != "" && !validDecimalHandling[dh] {
		return newConfigInvalid(prefix+".decimalHandling", "不支持的小数处理方式："+dh)
	}
	return nil
}

func validateImageConfig(prefix string, m map[string]any) error {
	if s, ok := m["mainImageStrategy"].(string); ok && s != "" && !validMainImageStrategies[s] {
		return newConfigInvalid(prefix+".mainImageStrategy", "不支持的主图策略："+s)
	}
	if s, ok := m["detailImageStrategy"].(string); ok && s != "" && !validDetailImageStrategies[s] {
		return newConfigInvalid(prefix+".detailImageStrategy", "不支持的详情图策略："+s)
	}
	return nil
}

func validateInventoryConfig(prefix string, m map[string]any) error {
	if s, ok := m["strategy"].(string); ok && s != "" && !validInventoryStrategies[s] {
		return newConfigInvalid(prefix+".strategy", "不支持的库存策略："+s)
	}
	if v, ok := m["safetyStock"]; ok && v != nil {
		if err := validateNonNegativeInteger(prefix+".safetyStock", v); err != nil {
			return err
		}
	}
	if v, ok := m["fixedQuantity"]; ok && v != nil {
		if err := validateNonNegativeInteger(prefix+".fixedQuantity", v); err != nil {
			return err
		}
	}
	if s, ok := m["outOfStockAction"].(string); ok && s != "" && !validOutOfStockActions[s] {
		return newConfigInvalid(prefix+".outOfStockAction", "不支持的缺库存处理方式："+s)
	}
	return nil
}

func validatePackageConfig(prefix string, m map[string]any) error {
	if u, ok := m["weightUnit"].(string); ok && u != "" && !validWeightUnits[u] {
		return newConfigInvalid(prefix+".weightUnit", "重量单位仅支持 kg 或 g。")
	}
	if u, ok := m["sizeUnit"].(string); ok && u != "" && !validSizeUnits[u] {
		return newConfigInvalid(prefix+".sizeUnit", "尺寸单位仅支持 cm。")
	}
	for _, dim := range []string{"weight", "length", "width", "height"} {
		if v, ok := m[dim]; ok && v != nil {
			if err := validatePositiveNumber(prefix+"."+dim, v, maxPackageDimension); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateNonNegativeNumber(path string, v any) error {
	f, ok := toFloat64(v)
	if !ok {
		return newConfigInvalid(path, "必须是有效数字。")
	}
	if f < 0 {
		return newConfigInvalid(path, "数值不能为负数。")
	}
	return nil
}

func validatePositiveNumber(path string, v any, max float64) error {
	f, ok := toFloat64(v)
	if !ok {
		return newConfigInvalid(path, "必须是有效数字。")
	}
	if f <= 0 {
		return newConfigInvalid(path, "数值必须大于 0。")
	}
	if f > max {
		return newConfigInvalid(path, fmt.Sprintf("数值不能超过 %.0f。", max))
	}
	return nil
}

func validateNonNegativeInteger(path string, v any) error {
	f, ok := toFloat64(v)
	if !ok {
		return newConfigInvalid(path, "必须是有效整数。")
	}
	if f < 0 || math.Mod(f, 1) != 0 {
		return newConfigInvalid(path, "必须是大于或等于 0 的整数。")
	}
	return nil
}

func toFloat64(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case int32:
		return float64(x), true
	default:
		return 0, false
	}
}
