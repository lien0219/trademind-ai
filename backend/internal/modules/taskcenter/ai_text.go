package taskcenter

import (
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproducttext"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter/failureclassifier"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func aiTextDetailURL(batchID, itemID string) string {
	if batchID == "" {
		return ""
	}
	path := "/product/ai-text-batches/" + url.PathEscape(batchID)
	if itemID != "" {
		q := url.Values{}
		q.Set("itemId", itemID)
		path += "?" + q.Encode()
	}
	return path
}

func aiTextFailureCategory(item *aiproducttext.AIProductTextItem) string {
	if item == nil {
		return CategoryAITextGenerationFailed
	}
	switch strings.TrimSpace(item.Status) {
	case aiproducttext.ItemConflict:
		return CategoryAITextApplyConflict
	case aiproducttext.ItemFailed:
		code := strings.TrimSpace(strings.ToLower(item.ErrorCode))
		if code == "apply_failed" {
			return CategoryAITextApplyFailed
		}
		if code == "undo_failed" {
			return CategoryAITextUndoFailed
		}
		return CategoryAITextGenerationFailed
	case aiproducttext.ItemPendingReview, aiproducttext.ItemSuccess:
		if hasQualityWarnings(item.QualityWarnings) {
			return CategoryAITextQualityWarning
		}
	}
	return CategoryAITextGenerationFailed
}

func hasQualityWarnings(raw datatypes.JSON) bool {
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

func aiTextFailureUserMessage(item *aiproducttext.AIProductTextItem) string {
	if item == nil {
		return ""
	}
	switch strings.TrimSpace(item.Status) {
	case aiproducttext.ItemConflict:
		return aiproducttext.ConflictUserMessage
	case aiproducttext.ItemFailed:
		msg := strings.TrimSpace(item.ErrorMessage)
		if msg != "" {
			return truncateRunes(msg, maxErrorMessageLen)
		}
		return "AI 文案生成失败，请重试或查看复核页详情。"
	case aiproducttext.ItemPendingReview, aiproducttext.ItemSuccess:
		if hasQualityWarnings(item.QualityWarnings) {
			var warnings []struct {
				Message string `json:"message"`
			}
			_ = json.Unmarshal(item.QualityWarnings, &warnings)
			if len(warnings) > 0 && strings.TrimSpace(warnings[0].Message) != "" {
				return truncateRunes(warnings[0].Message, maxErrorMessageLen)
			}
			return "AI 文案建议需要人工复核，请查看质量提醒。"
		}
	}
	return truncateRunes(item.ErrorMessage, maxErrorMessageLen)
}

func aiTextNormalizedStatus(item *aiproducttext.AIProductTextItem) string {
	if item == nil {
		return NormFailed
	}
	switch strings.TrimSpace(item.Status) {
	case aiproducttext.ItemConflict, aiproducttext.ItemFailed:
		return NormFailed
	case aiproducttext.ItemPendingReview, aiproducttext.ItemSuccess:
		if hasQualityWarnings(item.QualityWarnings) {
			return NormFailed
		}
		return NormSuccess
	default:
		return NormSuccess
	}
}

func aiTextFailureSeverity(category string) string {
	switch category {
	case CategoryAITextQualityWarning:
		return failureclassifier.SeverityLow
	case CategoryAITextApplyConflict:
		return failureclassifier.SeverityMedium
	case CategoryAITextGenerationFailed, CategoryAITextApplyFailed, CategoryAITextUndoFailed:
		return failureclassifier.SeverityMedium
	default:
		return failureclassifier.SeverityLow
	}
}

func aiTextFailureReason(category string) string {
	switch category {
	case CategoryAITextGenerationFailed:
		return "AI 文案生成失败。"
	case CategoryAITextApplyConflict:
		return "AI 文案应用时发现内容冲突。"
	case CategoryAITextApplyFailed:
		return "AI 文案应用失败。"
	case CategoryAITextUndoFailed:
		return "AI 文案撤销失败。"
	case CategoryAITextQualityWarning:
		return "AI 文案建议需要复核。"
	default:
		return "AI 文案任务异常。"
	}
}

func aiTextSuggestedAction(category string) string {
	switch category {
	case CategoryAITextGenerationFailed:
		return "请在批量文案复核页重试失败项或单独重新生成。"
	case CategoryAITextApplyConflict:
		return "请打开复核页对比当前内容与 AI 建议，确认无人工修改后再应用或重新生成。"
	case CategoryAITextApplyFailed:
		return "请检查商品状态与 AI 任务关联后，在复核页重试应用。"
	case CategoryAITextUndoFailed:
		return "若商品已被人工修改，撤销会被阻止；请在复核页查看详情。"
	case CategoryAITextQualityWarning:
		return "质量提醒不阻断应用，但建议编辑 AI 文案后再应用。"
	default:
		return "请打开批量文案复核页处理。"
	}
}

func applyAITextClassification(d *UnifiedTaskDTO) failureclassifier.Result {
	if d == nil {
		return failureclassifier.Result{}
	}
	cat := strings.TrimSpace(d.FailureCategory)
	if cat == "" {
		cat = CategoryAITextGenerationFailed
	}
	sev := aiTextFailureSeverity(cat)
	r := failureclassifier.Result{
		Category:        cat,
		Severity:        sev,
		Reason:          aiTextFailureReason(cat),
		MatchedRule:     "ai_text:status",
		SuggestedAction: aiTextSuggestedAction(cat),
	}
	d.FailureCategory = r.Category
	d.Severity = r.Severity
	d.ClassificationReason = r.Reason
	d.MatchedRule = r.MatchedRule
	d.SuggestedAction = r.SuggestedAction
	return r
}

func mapAIProductTextItem(row *aiproducttext.AIProductTextItem, productTitles map[uuid.UUID]string, marks markSet, now time.Time) UnifiedTaskDTO {
	if row == nil {
		return UnifiedTaskDTO{}
	}
	cat := aiTextFailureCategory(row)
	norm := aiTextNormalizedStatus(row)
	ptitle := productTitles[row.ProductID]
	opLabel := strings.TrimSpace(row.OperationType)
	switch opLabel {
	case aiproducttext.OpTitle:
		opLabel = "标题"
	case aiproducttext.OpDescription:
		opLabel = "描述"
	}
	title := "AI 文案 · " + opLabel
	if ptitle != "" {
		title = truncateRunes("AI 文案 · "+opLabel+" · "+ptitle, 240)
	}
	errMsg := aiTextFailureUserMessage(row)
	dto := UnifiedTaskDTO{
		ID:                   row.ID.String(),
		TaskType:             TaskTypeAIText,
		SourceTable:          SourceTableAIProductTextItems,
		SourceID:             row.ID.String(),
		Title:                title,
		RelatedResourceType:  "product",
		RelatedResourceID:    row.ProductID.String(),
		RelatedResourceTitle: truncateRunes(ptitle, 255),
		Status:               row.Status,
		NormalizedStatus:     norm,
		Retryable:            strings.TrimSpace(row.Status) == aiproducttext.ItemFailed,
		ErrorMessage:         errMsg,
		ErrorCode:            strings.TrimSpace(row.ErrorCode),
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
		DetailURL:            aiTextDetailURL(row.BatchID.String(), row.ID.String()),
		RetryAction:          "POST /api/v1/products/ai-text/items/:id/regenerate",
		RawSummary:           truncateRunes("batchId="+row.BatchID.String()+" op="+row.OperationType, maxRawSummaryLen),
		SortKey:              row.UpdatedAt,
		FailureCategory:      cat,
	}
	applyAITextClassification(&dto)
	applyMarks(&dto, TaskTypeAIText, row.ID.String(), marks)
	_ = now
	return dto
}

func aiTextFailureRowFilter(db *gorm.DB, includeResolved bool) *gorm.DB {
	if includeResolved {
		return db
	}
	return db.Where(`
		status IN ?
		OR (
			status IN ?
			AND quality_warnings IS NOT NULL
			AND TRIM(quality_warnings::text) NOT IN ('null', '[]', '')
		)`, []string{aiproducttext.ItemFailed, aiproducttext.ItemConflict},
		[]string{aiproducttext.ItemPendingReview, aiproducttext.ItemSuccess})
}
