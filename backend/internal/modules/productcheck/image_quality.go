package productcheck

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
)

type cachedImageScore struct {
	OverallScore float64
	Issues       []string
}

type scoreJSONPayload struct {
	OverallScore float64  `json:"overallScore"`
	Issues       []string `json:"issues"`
}

func (s *Service) checkImageQualityHints(ctx context.Context, p product.Product) []CheckItem {
	var out []CheckItem
	if len(p.Images) == 0 {
		return out
	}

	hasDetail := false
	for _, im := range p.Images {
		imgType := normalizeImageType(im.ImageType)
		if imgType == product.ImageTypeDetail {
			hasDetail = true
			break
		}
	}
	if !hasDetail {
		out = append(out, CheckItem{
			Group:      "image",
			Code:       "image.detail_missing",
			Level:      levelWarning,
			Message:    "缺少详情图",
			Suggestion: "建议至少上传一张 detail 类型图片用于商品详情展示。",
		})
	}

	mainByID := map[uuid.UUID]product.ProductImage{}
	for _, im := range p.Images {
		imgType := normalizeImageType(im.ImageType)
		if imgType == product.ImageTypeMain || im.IsBestMain {
			mainByID[im.ID] = im
		}
	}
	if len(mainByID) == 0 {
		for _, im := range p.Images {
			if normalizeImageType(im.ImageType) != product.ImageTypeSKU {
				mainByID[im.ID] = im
				break
			}
		}
	}

	for id, im := range mainByID {
		scoreVal := im.Score
		cached := s.loadLatestScoreForImage(ctx, id)
		if cached != nil {
			if scoreVal == nil {
				v := cached.OverallScore
				scoreVal = &v
			}
		}
		if scoreVal != nil && *scoreVal < 60 {
			out = append(out, CheckItem{
				Group:               "image",
				Code:                "image.main_score_low",
				Level:               levelWarning,
				Message:             "主图评分低于 60",
				Suggestion:          "建议运行 AI 商品图评分或更换更清晰、无干扰元素的主图。",
				RelatedResourceType: "product_image",
				RelatedResourceID:   id.String(),
			})
		}
		issues := []string(nil)
		if cached != nil {
			issues = cached.Issues
		}
		if scoreIssuesContain(issues, "watermark", "水印") {
			out = append(out, CheckItem{
				Group:               "image",
				Code:                "image.main_watermark",
				Level:               levelWarning,
				Message:             "主图可能存在水印",
				Suggestion:          "建议使用 AI 去水印后再设为主图。",
				RelatedResourceType: "product_image",
				RelatedResourceID:   id.String(),
			})
		}
		if scoreIssuesContain(issues, "logo", "品牌", "商标") {
			out = append(out, CheckItem{
				Group:               "image",
				Code:                "image.main_logo",
				Level:               levelWarning,
				Message:             "主图可能存在 Logo",
				Suggestion:          "建议使用 AI 去 Logo 后再设为主图。",
				RelatedResourceType: "product_image",
				RelatedResourceID:   id.String(),
			})
		}
		if scoreIssuesContain(issues, "qrcode", "qr", "二维码", "条码") {
			out = append(out, CheckItem{
				Group:               "image",
				Code:                "image.main_qrcode",
				Level:               levelWarning,
				Message:             "主图可能存在二维码",
				Suggestion:          "建议使用 AI 去二维码后再设为主图。",
				RelatedResourceType: "product_image",
				RelatedResourceID:   id.String(),
			})
		}
	}
	return out
}

func normalizeImageType(raw string) string {
	t := strings.TrimSpace(strings.ToLower(raw))
	if t == product.ImageTypeDescription {
		return product.ImageTypeDetail
	}
	return t
}

func scoreIssuesContain(issues []string, keywords ...string) bool {
	for _, iss := range issues {
		lower := strings.ToLower(strings.TrimSpace(iss))
		for _, kw := range keywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				return true
			}
		}
	}
	return false
}

func (s *Service) loadLatestScoreForImage(ctx context.Context, imageID uuid.UUID) *cachedImageScore {
	if s == nil || s.DB == nil || imageID == uuid.Nil {
		return nil
	}
	type row struct {
		ScoreJSON []byte
	}
	var r row
	err := s.DB.WithContext(ctx).Table("ai_image_task_items").
		Select("score_json").
		Where("source_image_id = ? AND score_json IS NOT NULL AND status = ?", imageID, "success").
		Order("updated_at DESC").
		Limit(1).
		Scan(&r).Error
	if err != nil || len(r.ScoreJSON) == 0 {
		return nil
	}
	var payload scoreJSONPayload
	if err := json.Unmarshal(r.ScoreJSON, &payload); err != nil {
		return nil
	}
	return &cachedImageScore{
		OverallScore: payload.OverallScore,
		Issues:       payload.Issues,
	}
}
