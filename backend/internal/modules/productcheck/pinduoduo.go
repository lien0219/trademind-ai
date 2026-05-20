package productcheck

import (
	"encoding/json"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/product"
)

func isPinduoduoProductSource(src string) bool {
	s := strings.TrimSpace(strings.ToLower(src))
	return s == "pinduoduo" || s == "pdd"
}

func pinduoduoWarningCodesFromRaw(rawData json.RawMessage) []string {
	if len(rawData) == 0 {
		return nil
	}
	var root map[string]any
	if json.Unmarshal(rawData, &root) != nil {
		return nil
	}
	inner, _ := root["raw"].(map[string]any)
	if inner == nil {
		return nil
	}
	w, _ := inner["warnings"].([]any)
	out := make([]string, 0, len(w))
	for _, item := range w {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

func checkPinduoduoCollectHints(p product.Product) []CheckItem {
	if !isPinduoduoProductSource(p.Source) {
		return nil
	}
	var out []CheckItem
	warnCodes := pinduoduoWarningCodesFromRaw(json.RawMessage(p.RawData))
	if len(warnCodes) > 0 {
		out = append(out, CheckItem{
			Group:      "collect",
			Code:       "collect.pinduoduo.warnings",
			Level:      levelWarning,
			Message:    "拼多多采集存在字段识别告警，请发布前人工核对",
			Suggestion: "请检查标题、价格、主图、规格与库存是否与页面一致。",
		})
	}
	hasDetail := false
	for _, im := range p.Images {
		t := strings.TrimSpace(strings.ToLower(im.ImageType))
		if t == product.ImageTypeDetail || t == product.ImageTypeDescription {
			hasDetail = true
			break
		}
	}
	if !hasDetail {
		out = append(out, CheckItem{
			Group:      "image",
			Code:       "collect.pinduoduo.detail_images_missing",
			Level:      levelWarning,
			Message:    "未识别到详情图",
			Suggestion: "可后续在图片管理中补充，或使用 AI 生成描述。",
		})
	}
	attrCount := 0
	if len(p.RawData) > 0 {
		var root map[string]any
		if json.Unmarshal(json.RawMessage(p.RawData), &root) == nil {
			if inner, ok := root["raw"].(map[string]any); ok {
				if attrs, ok := inner["attributes"].(map[string]any); ok {
					attrCount = len(attrs)
				}
			}
		}
	}
	if attrCount == 0 {
		out = append(out, CheckItem{
			Group:      "product",
			Code:       "collect.pinduoduo.attributes_missing",
			Level:      levelWarning,
			Message:    "未识别到商品参数",
			Suggestion: "可在商品描述或属性中手动补充。",
		})
	}
	for _, sku := range p.SKUs {
		if sku.Stock != nil {
			continue
		}
		out = append(out, CheckItem{
			Group:               "inventory",
			Code:                "collect.pinduoduo.stock_unknown",
			Level:               levelWarning,
			Message:             "部分 SKU 库存未识别",
			Suggestion:          "请人工确认库存后再刊登。",
			RelatedResourceType: "product_sku",
			RelatedResourceID:   sku.ID.String(),
		})
	}
	return out
}
