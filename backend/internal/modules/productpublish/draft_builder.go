package productpublish

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

// BuildPlatformDraftFromProduct maps a hydrated product.Product into a provider-neutral listing draft (no encryption).
func BuildPlatformDraftFromProduct(p product.Product) (platformp.PlatformProductDraft, error) {
	title := strings.TrimSpace(p.Title)
	if title == "" {
		title = strings.TrimSpace(p.AITitle)
	}
	if title == "" {
		title = strings.TrimSpace(p.OriginalTitle)
	}
	desc := strings.TrimSpace(p.Description)
	if desc == "" {
		desc = strings.TrimSpace(p.AIDescription)
	}
	curr := strings.TrimSpace(p.Currency)
	if curr == "" {
		curr = "USD"
	}
	if title == "" {
		return platformp.PlatformProductDraft{}, fmt.Errorf("product title is required for publish")
	}
	imgs := p.Images
	if len(imgs) == 0 {
		return platformp.PlatformProductDraft{}, fmt.Errorf("product main image required for publish")
	}
	sort.SliceStable(imgs, func(i, j int) bool {
		if imgs[i].SortOrder == imgs[j].SortOrder {
			return imgs[i].CreatedAt.Before(imgs[j].CreatedAt)
		}
		return imgs[i].SortOrder < imgs[j].SortOrder
	})

	hasMain := false
	plImgs := make([]platformp.PlatformProductImage, 0, len(imgs))
	for _, im := range imgs {
		imgType := strings.TrimSpace(strings.ToLower(im.ImageType))
		if imgType == product.ImageTypeDescription {
			imgType = product.ImageTypeDetail
		}
		url := strings.TrimSpace(im.PublicURL)
		if url == "" {
			url = strings.TrimSpace(im.OriginURL)
		}
		if url == "" {
			continue
		}
		if imgType == product.ImageTypeMain {
			hasMain = true
		}
		plImgs = append(plImgs, platformp.PlatformProductImage{
			URL:       url,
			Type:      imgType,
			SortOrder: im.SortOrder,
		})
	}
	if !hasMain {
		for i := range plImgs {
			if strings.TrimSpace(plImgs[i].Type) != product.ImageTypeSKU {
				plImgs[i].Type = product.ImageTypeMain
				hasMain = true
				break
			}
		}
	}
	if !hasMain {
		return platformp.PlatformProductDraft{}, fmt.Errorf("product main image required for publish")
	}

	if len(p.SKUs) == 0 {
		return platformp.PlatformProductDraft{}, fmt.Errorf("product SKU is required for publish")
	}

	var attrs map[string]any
	if len(p.RawData) > 0 {
		var top map[string]any
		_ = json.Unmarshal(p.RawData, &top)
		if attrs == nil && top != nil {
			if raw, ok := top["attributes"].([]any); ok {
				attrs = map[string]any{"attributes": raw}
			} else if a, ok := top["attrs"].(map[string]any); ok {
				attrs = a
			}
		}
	}

	skus := make([]platformp.PlatformProductSKU, 0, len(p.SKUs))
	for _, s := range p.SKUs {
		pr := 0.0
		if s.Price != nil {
			pr = *s.Price
		}
		st := 0
		if s.Stock != nil {
			st = *s.Stock
		}
		var skuAttrs map[string]any
		if len(s.Attrs) > 0 {
			_ = json.Unmarshal(s.Attrs, &skuAttrs)
		}
		skus = append(skus, platformp.PlatformProductSKU{
			LocalSKUID: s.ID,
			SKUCode:    strings.TrimSpace(s.SKUCode),
			SKUName:    strings.TrimSpace(s.SKUName),
			Attrs:      skuAttrs,
			Price:      pr,
			Stock:      st,
			ImageURL:   strings.TrimSpace(s.ImageURL),
		})
	}

	srcRow := platformp.TrimRawMap(map[string]any{
		"id":             p.ID.String(),
		"titleLen":       len([]rune(title)),
		"descriptionLen": len([]rune(desc)),
		"skuCount":       len(skus),
		"imageCount":     len(plImgs),
	}, 12, 120)

	return platformp.PlatformProductDraft{
		ProductID:        p.ID,
		Title:            title,
		Description:      desc,
		Currency:         curr,
		Images:           plImgs,
		SKUs:             skus,
		Attributes:       attrs,
		SourceProductRow: srcRow,
	}, nil
}
