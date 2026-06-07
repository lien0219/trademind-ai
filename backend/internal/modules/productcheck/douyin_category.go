package productcheck

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/gorm"
)

const douyinCacheStaleAfter = 7 * 24 * time.Hour

func (s *Service) checkDouyinListingConfig(ctx context.Context, p product.Product) []CheckItem {
	if s == nil || s.DB == nil {
		return nil
	}
	var out []CheckItem
	var cfg product.ProductPlatformPublishConfig
	err := s.DB.WithContext(ctx).Where("product_id = ? AND platform = ?", p.ID, "douyin_shop").First(&cfg).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return []CheckItem{{
				Group:      "platform",
				Code:       shop.DouyinCategoryNotSelected,
				Level:      levelError,
				Message:    "抖店类目未选择",
				Suggestion: "请先选择抖店商品类目。",
			}}
		}
		return nil
	}
	if cfg.ShopID == nil {
		out = append(out, CheckItem{
			Group:      "platform",
			Code:       "DOUYIN_SHOP_NOT_AUTHORIZED",
			Level:      levelError,
			Message:    "抖店店铺未授权",
			Suggestion: "请先选择并授权抖店店铺。",
		})
	}
	cid := strings.TrimSpace(cfg.CategoryID)
	if cid == "" {
		out = append(out, CheckItem{
			Group:      "platform",
			Code:       shop.DouyinCategoryNotSelected,
			Level:      levelError,
			Message:    "抖店类目未选择",
			Suggestion: "请先选择抖店商品类目。",
		})
		return out
	}
	var cat shop.PlatformCategory
	if err := s.DB.WithContext(ctx).Where("platform = ? AND category_id = ?", "douyin_shop", cid).First(&cat).Error; err != nil {
		out = append(out, CheckItem{
			Group:      "platform",
			Code:       shop.DouyinCategoryEmpty,
			Level:      levelError,
			Message:    "本地没有抖店类目缓存",
			Suggestion: "暂无抖店类目数据，请先点击「刷新类目」。",
		})
		return out
	}
	if !cat.IsLeaf {
		out = append(out, CheckItem{
			Group:      "platform",
			Code:       shop.DouyinCategoryNotLeaf,
			Level:      levelError,
			Message:    "抖店类目不是叶子类目",
			Suggestion: "请选择抖店叶子类目。",
		})
	}
	if cat.SyncedAt == nil || time.Since(*cat.SyncedAt) > douyinCacheStaleAfter {
		out = append(out, CheckItem{
			Group:      "platform",
			Code:       shop.DouyinCategoryCacheStale,
			Level:      levelWarning,
			Message:    "抖店类目缓存较旧",
			Suggestion: "类目缓存较旧，建议刷新。",
		})
	}
	var attrs []shop.PlatformCategoryAttribute
	if err := s.DB.WithContext(ctx).Where("platform = ? AND category_id = ?", "douyin_shop", cid).Find(&attrs).Error; err != nil {
		return out
	}
	attrValues := map[string]any{}
	if len(cfg.PlatformAttributes) > 0 {
		_ = json.Unmarshal(cfg.PlatformAttributes, &attrValues)
	}
	var newestAttr *time.Time
	matched := 0
	for _, attr := range attrs {
		if attr.SyncedAt != nil && (newestAttr == nil || attr.SyncedAt.After(*newestAttr)) {
			t := *attr.SyncedAt
			newestAttr = &t
		}
		v, ok := attrValues[attr.AttrID]
		if !ok {
			v, ok = attrValues[attr.Name]
		}
		if ok && valuePresent(v) {
			matched++
			continue
		}
		if attr.Required {
			out = append(out, CheckItem{
				Group:      "platform",
				Code:       shop.DouyinRequiredAttrMissing,
				Level:      levelError,
				Message:    "抖店必填属性未补齐：" + attr.Name,
				Suggestion: "请补全抖店要求的商品属性后再创建商品草稿。",
			})
		}
	}
	if len(attrs) > 0 && (newestAttr == nil || time.Since(*newestAttr) > douyinCacheStaleAfter) {
		out = append(out, CheckItem{
			Group:      "platform",
			Code:       shop.DouyinCategoryCacheStale,
			Level:      levelWarning,
			Message:    "抖店属性缓存较旧",
			Suggestion: "属性缓存较旧，建议刷新。",
		})
	}
	if len(attrs) > 0 && matched < maxInt(1, len(attrs)/5) {
		out = append(out, CheckItem{
			Group:      "platform",
			Code:       "DOUYIN_ATTRIBUTE_MATCH_LOW",
			Level:      levelWarning,
			Message:    "商品参数与抖店属性匹配较少",
			Suggestion: "商品参数与抖店属性匹配较少，建议人工检查。",
		})
	}
	out = append(out, s.checkSavedDouyinMapping(ctx, p)...)
	return out
}

func (s *Service) checkSavedDouyinMapping(ctx context.Context, p product.Product) []CheckItem {
	var cfg product.ProductPlatformPublishConfig
	if err := s.DB.WithContext(ctx).Where("product_id = ? AND platform = ?", p.ID, "douyin_shop").First(&cfg).Error; err != nil {
		return nil
	}
	m := product.DouyinDraftMappingFromConfig(cfg)
	m.Source = strings.TrimSpace(p.Source)
	product.ApplyDouyinDraftValidation(m, s.douyinMinProfit(ctx))
	issues := append([]product.DouyinMappingIssue{}, m.Errors...)
	issues = append(issues, m.Warnings...)
	out := make([]CheckItem, 0, len(issues))
	for _, item := range issues {
		out = append(out, CheckItem{
			Group:               "platform",
			Code:                item.Code,
			Level:               item.Level,
			Message:             item.Message,
			Suggestion:          item.Suggestion,
			RelatedResourceType: item.RelatedResourceType,
			RelatedResourceID:   item.RelatedResourceID,
		})
	}
	return out
}

func (s *Service) douyinMinProfit(ctx context.Context) float64 {
	_, minProfit := s.pricingProtection(ctx, "douyin_shop")
	return minProfit
}

func valuePresent(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(x) != ""
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	default:
		return true
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
