package taskcenter

import (
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproductimage"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func aiImageDetailURL(batchID, itemID string) string {
	if batchID == "" {
		return ""
	}
	path := "/product/ai-image-batches/" + url.PathEscape(batchID)
	if itemID != "" {
		q := url.Values{}
		q.Set("itemId", itemID)
		path += "?" + q.Encode()
	}
	return path
}

func aiImageFailureCategory(item *aiproductimage.AIProductImageItem) string {
	if item == nil {
		return CategoryAIImageProcessFailed
	}
	switch strings.TrimSpace(item.Status) {
	case aiproductimage.ItemConflict:
		return CategoryAIImageApplyConflict
	case aiproductimage.ItemFailed:
		code := strings.TrimSpace(strings.ToLower(item.ErrorCode))
		if code == "apply_failed" {
			return CategoryAIImageApplyFailed
		}
		if code == "undo_failed" {
			return CategoryAIImageUndoFailed
		}
		return CategoryAIImageProcessFailed
	case aiproductimage.ItemPendingReview, aiproductimage.ItemSuccess:
		if hasQualityWarningsJSON(item.QualityWarnings) {
			return CategoryAIImageQualityWarn
		}
	}
	return CategoryAIImageProcessFailed
}

func hasQualityWarningsJSON(raw datatypes.JSON) bool {
	if len(raw) == 0 {
		return false
	}
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" || s == "[]" {
		return false
	}
	var warnings []struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(raw, &warnings); err != nil {
		return false
	}
	return len(warnings) > 0
}

func aiImageFailureUserMessage(item *aiproductimage.AIProductImageItem) string {
	if item == nil {
		return ""
	}
	switch strings.TrimSpace(item.Status) {
	case aiproductimage.ItemConflict:
		return aiproductimage.ConflictUserMessage
	case aiproductimage.ItemFailed:
		msg := strings.TrimSpace(item.ErrorMessage)
		if msg != "" {
			return truncateRunes(msg, maxErrorMessageLen)
		}
		return "AI 图片处理失败，请重试或查看复核页详情。"
	case aiproductimage.ItemPendingReview, aiproductimage.ItemSuccess:
		if hasQualityWarningsJSON(item.QualityWarnings) {
			var warnings []struct {
				Message string `json:"message"`
			}
			_ = json.Unmarshal(item.QualityWarnings, &warnings)
			if len(warnings) > 0 && strings.TrimSpace(warnings[0].Message) != "" {
				return truncateRunes(warnings[0].Message, maxErrorMessageLen)
			}
			return "AI 图片结果需要人工复核，请查看质量提醒。"
		}
	}
	return truncateRunes(item.ErrorMessage, maxErrorMessageLen)
}

func aiImageNormalizedStatus(item *aiproductimage.AIProductImageItem) string {
	if item == nil {
		return NormFailed
	}
	switch strings.TrimSpace(item.Status) {
	case aiproductimage.ItemConflict, aiproductimage.ItemFailed:
		return NormFailed
	case aiproductimage.ItemPendingReview, aiproductimage.ItemSuccess:
		if hasQualityWarningsJSON(item.QualityWarnings) {
			return NormFailed
		}
		return NormSuccess
	default:
		return NormPending
	}
}

func aiImageFailureRowFilter(db *gorm.DB, includeResolved bool) *gorm.DB {
	if includeResolved {
		return db
	}
	return db.Where(`(
		status IN ?
		OR (status IN ? AND quality_warnings IS NOT NULL AND TRIM(quality_warnings::text) NOT IN ('null', '[]', ''))
	)`, []string{aiproductimage.ItemFailed, aiproductimage.ItemConflict},
		[]string{aiproductimage.ItemPendingReview, aiproductimage.ItemSuccess})
}

func mapAIProductImageItem(row *aiproductimage.AIProductImageItem, productTitles map[uuid.UUID]string, marks markSet, now time.Time) UnifiedTaskDTO {
	if row == nil {
		return UnifiedTaskDTO{}
	}
	cat := aiImageFailureCategory(row)
	norm := aiImageNormalizedStatus(row)
	ptitle := productTitles[row.ProductID]
	opLabel := aiproductimage.OperationTypeLabel(row.OperationType)
	title := "AI 图片 · " + opLabel
	if ptitle != "" {
		title = truncateRunes("AI 图片 · "+opLabel+" · "+ptitle, 240)
	}
	errMsg := aiImageFailureUserMessage(row)
	dto := UnifiedTaskDTO{
		ID:                   row.ID.String(),
		TaskType:             TaskTypeAIImage,
		SourceTable:          SourceTableAIProductImageItems,
		SourceID:             row.ID.String(),
		Title:                title,
		RelatedResourceType:  "product",
		RelatedResourceID:    row.ProductID.String(),
		RelatedResourceTitle: truncateRunes(ptitle, 255),
		Status:               row.Status,
		NormalizedStatus:     norm,
		Retryable:            strings.TrimSpace(row.Status) == aiproductimage.ItemFailed,
		ErrorMessage:         errMsg,
		ErrorCode:            strings.TrimSpace(row.ErrorCode),
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
		DetailURL:            aiImageDetailURL(row.BatchID.String(), row.ID.String()),
		RetryAction:          "POST /api/v1/products/ai-images/items/:id/regenerate",
		RawSummary:           truncateRunes("batchId="+row.BatchID.String()+" op="+row.OperationType, maxRawSummaryLen),
		SortKey:              row.UpdatedAt,
		FailureCategory:      cat,
	}
	applyClassification(&dto)
	applyMarks(&dto, TaskTypeAIImage, row.ID.String(), marks)
	_ = now
	return dto
}
