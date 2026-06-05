package collect

import (
	"context"
	"errors"
	"strings"
)

// CollectProviderDTO is one row from Collector GET /v1/providers (and admin GET /api/v1/collect/providers).
type CollectProviderDTO struct {
	Source         string   `json:"source"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Status         string   `json:"status"`
	BatchSupported bool     `json:"batchSupported"`
	URLPatterns    []string `json:"urlPatterns"`
	Features       []string `json:"features"`
	Notes          string   `json:"notes"`
}

var (
	ErrUnknownCollectSource      = errors.New("unknown collect source")
	ErrProviderNotAvailable      = errors.New("Provider not available")
	ErrBatchCollectNotSupported  = errors.New("batch collect is not supported for this provider")
	ErrCollectURLNeedsHTTPScheme = errors.New("url must start with http:// or https://")
	ErrCollectURLsNeedHTTPScheme = errors.New("each url must start with http:// or https://")
)

func defaultCollectProvidersFallback() []CollectProviderDTO {
	return []CollectProviderDTO{
		{
			Source:         "1688",
			Name:           "1688采集器",
			Description:    "采集 1688 商品详情页，支持标题、主图、详情图、属性、SKU",
			Status:         "available",
			BatchSupported: true,
			URLPatterns:    []string{"https://detail.1688.com/offer/*.html"},
			Features:       []string{"title", "mainImages", "descriptionImages", "attributes", "skus"},
			Notes:          "",
		},
		{
			Source:         "pinduoduo",
			Name:           "拼多多采集器",
			Description:    "采集拼多多批发商品详情，支持标题、价格、主图、规格等基础字段。",
			Status:         "available",
			BatchSupported: true,
			URLPatterns: []string{
				"https://pifa.pinduoduo.com/goods/detail/?gid=*",
				"https://mobile.yangkeduo.com/goods.html?goods_id=*",
			},
			Features: []string{"title", "price", "mainImages", "descriptionImages", "attributes", "skus"},
			Notes:    "批量采集默认限速，建议先少量测试。",
		},
		{
			Source:         "taobao_tmall",
			Name:           "淘宝/天猫采集器",
			Description:    "采集淘宝、天猫商品详情，支持标题、价格、主图、详情图、商品参数。部分商品可能需要登录后采集。",
			Status:         "beta",
			BatchSupported: false,
			URLPatterns: []string{
				"https://item.taobao.com/item.htm?id=*",
				"https://detail.tmall.com/item.htm?id=*",
				"https://detail.tmall.hk/item.htm?id=*",
				"https://world.taobao.com/item/*.htm",
			},
			Features: []string{"title", "price", "mainImages", "descriptionImages", "attributes", "skus"},
			Notes:    "批量采集暂未开放。部分商品需要登录或手动完成安全验证。",
		},
		{
			Source:         "aliexpress",
			Name:           "速卖通采集器",
			Description:    "采集 AliExpress 商品详情页，提取标题、图片、属性、SKU 等信息",
			Status:         "beta",
			BatchSupported: false,
			URLPatterns: []string{
				"https://www.aliexpress.com/item/*.html",
				"https://*.aliexpress.com/item/*.html",
			},
			Features: []string{"title", "mainImages", "descriptionImages", "attributes", "skus"},
			Notes:    "",
		},
		{
			Source:         "shein_temu",
			Name:           "SHEIN/Temu采集器",
			Description:    "采集 SHEIN、Temu 等平台商品详情（规划中）。",
			Status:         "planned",
			BatchSupported: false,
			URLPatterns:    []string{"https://www.shein.com/…", "https://www.temu.com/…"},
			Features:       nil,
			Notes:          "",
		},
		{
			Source:         "custom",
			Name:           "自定义链接采集器",
			Description:    "适合采集没有专用采集器的网站商品页，可采集商品标题、价格、图片、参数等基础信息。",
			Status:         "beta",
			BatchSupported: false,
			URLPatterns:    []string{"https://example.com/product/..."},
			Features:       []string{"title", "price", "mainImages", "descriptionImages", "attributes"},
			Notes:          "商品规格、库存、动态价格不保证完整。使用前建议先测试采集规则。已支持的平台请优先使用专用采集器；自定义链接批量采集暂未开放。",
		},
	}
}

func findCollectProvider(list []CollectProviderDTO, source string) *CollectProviderDTO {
	key := strings.ToLower(strings.TrimSpace(source))
	if key == "" {
		return nil
	}
	for i := range list {
		if strings.ToLower(strings.TrimSpace(list[i].Source)) == key {
			return &list[i]
		}
	}
	return nil
}

// ResolveCollectProviders loads provider metadata from Collector when reachable; otherwise uses a built-in fallback.
func (s *Service) ResolveCollectProviders(ctx context.Context) []CollectProviderDTO {
	if s == nil {
		return defaultCollectProvidersFallback()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if s.Client == nil {
		return defaultCollectProvidersFallback()
	}
	list, err := s.Client.FetchProviders(ctx)
	if err != nil || len(list) == 0 {
		return defaultCollectProvidersFallback()
	}
	return list
}

// ValidateSourceForCollect ensures the source appears in Collector registry and passes status rules:
// single-link task → status may be available or beta; batch → strictly available plus batchSupported.
func (s *Service) ValidateSourceForCollect(ctx context.Context, source string, requireBatch bool) error {
	provs := s.ResolveCollectProviders(ctx)
	p := findCollectProvider(provs, source)
	if p == nil {
		return ErrUnknownCollectSource
	}
	status := strings.TrimSpace(strings.ToLower(p.Status))
	if requireBatch {
		if status != "available" {
			return ErrProviderNotAvailable
		}
		if !p.BatchSupported {
			return ErrBatchCollectNotSupported
		}
		return nil
	}
	if status != "available" && status != "beta" {
		return ErrProviderNotAvailable
	}
	return nil
}

func looksLikeCollectURL(u string) bool {
	s := strings.TrimSpace(u)
	ls := strings.ToLower(s)
	return strings.HasPrefix(ls, "http://") || strings.HasPrefix(ls, "https://")
}
