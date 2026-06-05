package productcheck

import (
	"encoding/json"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/product"
)

func isTaobaoTmallProductSource(src string) bool {
	s := strings.TrimSpace(strings.ToLower(src))
	return s == "taobao_tmall" || s == "taobao"
}

func taobaoTmallWarningCodesFromRaw(rawData json.RawMessage) []string {
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
	out := make([]string, 0, 8)
	for _, key := range []string{"qualityWarnings", "warnings"} {
		w, _ := inner[key].([]any)
		for _, item := range w {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

func checkTaobaoTmallCollectHints(p product.Product) []CheckItem {
	if !isTaobaoTmallProductSource(p.Source) {
		return nil
	}
	var out []CheckItem
	warnCodes := taobaoTmallWarningCodesFromRaw(json.RawMessage(p.RawData))
	if len(warnCodes) > 0 {
		out = append(out, CheckItem{
			Group:      "collect",
			Code:       "collect.taobao_tmall.warnings",
			Level:      levelWarning,
			Message:    "淘宝/天猫采集存在字段识别告警，请发布前人工核对",
			Suggestion: "请检查标题、价格、主图、规格与库存是否与页面一致。",
		})
	}
	for _, code := range warnCodes {
		switch strings.ToUpper(strings.TrimSpace(code)) {
		case "PRICE_NOT_FOUND":
			out = append(out, CheckItem{
				Group:      "price",
				Code:       "collect.taobao_tmall.price_missing",
				Level:      levelError,
				Message:    "未识别到商品价格",
				Suggestion: "请手动填写价格后再发布。",
			})
		case "SKU_INCOMPLETE":
			out = append(out, CheckItem{
				Group:      "sku",
				Code:       "collect.taobao_tmall.sku_incomplete",
				Level:      levelWarning,
				Message:    "商品规格识别不完整",
				Suggestion: "请人工核对规格、价格与库存。",
			})
		case "DETAIL_IMAGES_INCOMPLETE":
			out = append(out, CheckItem{
				Group:      "image",
				Code:       "collect.taobao_tmall.detail_images_incomplete",
				Level:      levelWarning,
				Message:    "详情图可能未完全加载",
				Suggestion: "请核对商品介绍区域图片，必要时手动补充。",
			})
		}
	}
	return out
}
