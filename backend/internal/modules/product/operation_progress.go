package product

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/opslabels"
	"gorm.io/gorm"
)

type operationImageTaskProbe struct {
	ProductID *uuid.UUID `gorm:"column:product_id"`
	Status    string     `gorm:"column:status"`
}

func (operationImageTaskProbe) TableName() string { return "image_tasks" }

type operationFacts struct {
	Product        *Product
	Readiness      *OperationReadinessResult
	ImageTaskStats map[string]int
}

var operationStepOrder = []ProductOperationStep{
	OperationStepCollectReview,
	OperationStepTitle,
	OperationStepDescription,
	OperationStepImages,
	OperationStepPricing,
	OperationStepAttributes,
	OperationStepPublishCheck,
	OperationStepReady,
}

var operationStepLabels = map[ProductOperationStep]string{
	OperationStepCollectReview: "检查采集结果",
	OperationStepTitle:         "优化商品标题",
	OperationStepDescription:   "完善商品描述",
	OperationStepImages:        "检查商品图片",
	OperationStepPricing:       "设置销售价格",
	OperationStepAttributes:    "补充商品参数",
	OperationStepPublishCheck:  "完成发布检查",
	OperationStepReady:         "可以生成刊登草稿",
}

type operationAction struct {
	Key   string
	Label string
	URL   string
}

func operationActionFor(productID uuid.UUID, step ProductOperationStep) operationAction {
	base := "/product/drafts/" + productID.String()
	switch step {
	case OperationStepCollectReview:
		return operationAction{"review_collect", "检查采集结果", base + "?tab=basic&section=collect-review"}
	case OperationStepTitle:
		return operationAction{"optimize_title", "去优化标题", base + "?tab=basic&section=title"}
	case OperationStepDescription:
		return operationAction{"generate_description", "去生成描述", base + "?tab=basic&section=description"}
	case OperationStepImages:
		return operationAction{"process_images", "去处理图片", base + "?tab=images"}
	case OperationStepPricing:
		return operationAction{"set_price", "去设置价格", base + "?tab=skus&section=pricing"}
	case OperationStepAttributes:
		return operationAction{"edit_attributes", "去补充参数", base + "?tab=basic&section=attributes"}
	case OperationStepPublishCheck:
		return operationAction{"fix_publish_check", "去发布检查", base + "?tab=readiness&section=publish-check"}
	case OperationStepReady:
		return operationAction{"create_listing_draft", "生成刊登草稿", base + "?tab=publish"}
	default:
		return operationAction{"continue_product", "继续完善", base}
	}
}

// GetOperationProgress returns the detailed product operation progress for one product.
func (s *Service) GetOperationProgress(ctx context.Context, productID uuid.UUID) (*ProductOperationProgress, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	var p Product
	if err := s.DB.WithContext(ctx).
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order ASC, created_at ASC") }).
		Preload("SKUs", func(db *gorm.DB) *gorm.DB { return db.Order("created_at ASC") }).
		First(&p, "id = ?", productID).Error; err != nil {
		return nil, err
	}
	imgStats, err := s.loadImageTaskStats(ctx, []uuid.UUID{productID})
	if err != nil {
		return nil, err
	}
	var ready *OperationReadinessResult
	if s.Readiness != nil {
		ready, _ = s.Readiness(ctx, OperationReadinessRequest{
			ProductID: productID,
			Mode:      "draft",
		})
	}
	return buildOperationProgress(operationFacts{
		Product:        &p,
		Readiness:      ready,
		ImageTaskStats: imgStats[productID],
	}), nil
}

func (s *Service) attachOperationProgressSummaries(ctx context.Context, rows []Product, items []ListItem) ([]ListItem, error) {
	if len(rows) == 0 || len(items) == 0 {
		return items, nil
	}
	ids := make([]uuid.UUID, 0, len(rows))
	byID := make(map[uuid.UUID]*Product, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].ID)
		row := rows[i]
		byID[row.ID] = &row
	}
	var imgs []ProductImage
	if err := s.DB.WithContext(ctx).Where("product_id IN ?", ids).Order("sort_order ASC, created_at ASC").Find(&imgs).Error; err != nil {
		return nil, err
	}
	for _, img := range imgs {
		if p := byID[img.ProductID]; p != nil {
			p.Images = append(p.Images, img)
		}
	}
	var skus []ProductSKU
	if err := s.DB.WithContext(ctx).Where("product_id IN ?", ids).Order("created_at ASC").Find(&skus).Error; err != nil {
		return nil, err
	}
	for _, sku := range skus {
		if p := byID[sku.ProductID]; p != nil {
			p.SKUs = append(p.SKUs, sku)
		}
	}
	imgStats, err := s.loadImageTaskStats(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range items {
		p := byID[items[i].ID]
		if p == nil {
			continue
		}
		full := buildOperationProgress(operationFacts{Product: p, ImageTaskStats: imgStats[p.ID]})
		items[i].OperationProgress = full.Summary()
	}
	return items, nil
}

func (s *Service) loadImageTaskStats(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]map[string]int, error) {
	out := make(map[uuid.UUID]map[string]int, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []operationImageTaskProbe
	if err := s.DB.WithContext(ctx).
		Model(&operationImageTaskProbe{}).
		Select("product_id", "status").
		Where("product_id IN ?", ids).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.ProductID == nil {
			continue
		}
		if _, ok := out[*row.ProductID]; !ok {
			out[*row.ProductID] = map[string]int{}
		}
		out[*row.ProductID][strings.TrimSpace(strings.ToLower(row.Status))]++
	}
	return out, nil
}

func buildOperationProgress(f operationFacts) *ProductOperationProgress {
	p := f.Product
	if p == nil {
		return nil
	}
	stepStatus := map[ProductOperationStep]string{}
	blockers := make([]ProductOperationIssue, 0)
	warnings := make([]ProductOperationWarning, 0)
	addBlocker := func(step ProductOperationStep, code, title, msg string) {
		act := operationActionFor(p.ID, step)
		blockers = append(blockers, ProductOperationIssue{
			Code: code, Title: title, Message: msg, Severity: "failed",
			ActionLabel: act.Label, ActionKey: act.Key, ActionURL: act.URL,
		})
		stepStatus[step] = "failed"
	}
	addWarning := func(step ProductOperationStep, code, title, msg string) {
		warnings = append(warnings, ProductOperationWarning{Code: code, Title: title, Message: msg})
		if stepStatus[step] == "" {
			stepStatus[step] = "warning"
		}
	}

	titleOK := validProductTitle(p)
	sourceOK := strings.TrimSpace(p.Source) != ""
	mainOK := hasMainImage(p)
	priceInfoOK := hasAnyPriceOrCost(p)
	collectWarnings := collectWarningsFromRaw(json.RawMessage(p.RawData))
	if !titleOK {
		addBlocker(OperationStepCollectReview, "collect.title_missing", "商品标题缺失", "采集结果没有可用标题，请先补齐标题。")
	}
	if !sourceOK {
		addBlocker(OperationStepCollectReview, "collect.source_missing", "商品来源缺失", "商品来源为空，请确认采集结果或手动补充。")
	}
	if !mainOK {
		addBlocker(OperationStepCollectReview, "collect.main_image_missing", "主图缺失", "采集结果没有可用主图，请上传或处理图片。")
	}
	if !priceInfoOK {
		addBlocker(OperationStepCollectReview, "collect.price_missing", "价格信息缺失", "采集结果没有价格或成本信息，请设置销售价格。")
	}
	for _, w := range collectWarnings {
		title, msg := opslabels.LocalizeCollectWarning(w)
		addWarning(OperationStepCollectReview, "collect.warning_requires_confirmation", title, msg)
	}
	if titleOK && sourceOK && mainOK && priceInfoOK && stepStatus[OperationStepCollectReview] == "" {
		stepStatus[OperationStepCollectReview] = "done"
	}

	if validProductTitle(p) {
		stepStatus[OperationStepTitle] = "done"
	} else {
		addBlocker(OperationStepTitle, "product.title_invalid", "商品标题不可用", "请填写清晰的商品标题，可以使用 AI 建议但不强制。")
	}

	if validProductDescription(p) {
		stepStatus[OperationStepDescription] = "done"
	} else {
		addBlocker(OperationStepDescription, "product.description_missing", "商品描述待完善", "请填写商品描述，或生成 AI 描述后人工确认。")
	}

	if !mainOK {
		addBlocker(OperationStepImages, "image.main_missing", "主图缺失", "至少需要一张有效主图。")
	} else {
		stepStatus[OperationStepImages] = "done"
	}
	if n := f.ImageTaskStats["running"] + f.ImageTaskStats["pending"] + f.ImageTaskStats["retrying"]; n > 0 {
		addWarning(OperationStepImages, "image.task_processing", "图片任务处理中", fmt.Sprintf("有 %d 个图片任务仍在处理，请稍后刷新。", n))
	}
	if n := f.ImageTaskStats["failed"] + f.ImageTaskStats["failed_render_validation"]; n > 0 {
		addWarning(OperationStepImages, "image.task_failed", "图片任务失败", fmt.Sprintf("有 %d 个图片任务失败，可到图片管理中重试或重新处理。", n))
	}
	if hasLowQualityImageTask(f.ImageTaskStats) {
		addWarning(OperationStepImages, "image.low_quality", "图片建议复核", "存在低质量或需人工复核的图片处理结果。")
	}

	if pricingOK(p) {
		stepStatus[OperationStepPricing] = "done"
	} else {
		addBlocker(OperationStepPricing, "pricing.price_invalid", "销售价格待设置", "请为每个 SKU 填写有效销售价，并确认币种。")
	}

	if attributesOK(p) {
		stepStatus[OperationStepAttributes] = "done"
	} else {
		addWarning(OperationStepAttributes, "product.attributes_missing", "商品参数建议补充", "通用商品参数不足，平台专属必填项会在刊登草稿时继续校验。")
	}

	if f.Readiness != nil {
		appendReadinessIssues(p.ID, f.Readiness, &blockers, &warnings, stepStatus)
		if f.Readiness.Result == "failed" || f.Readiness.Status == "blocked" || f.Readiness.ErrorCount > 0 {
			stepStatus[OperationStepPublishCheck] = "failed"
		} else {
			stepStatus[OperationStepPublishCheck] = "done"
		}
	}

	publishReady := stepStatus[OperationStepCollectReview] == "done" &&
		stepStatus[OperationStepTitle] == "done" &&
		stepStatus[OperationStepDescription] == "done" &&
		stepStatus[OperationStepImages] == "done" &&
		stepStatus[OperationStepPricing] == "done"
	if f.Readiness != nil {
		publishReady = publishReady && f.Readiness.CanPublish
	}
	if publishReady {
		stepStatus[OperationStepReady] = "done"
	} else if stepStatus[OperationStepReady] == "" {
		stepStatus[OperationStepReady] = "pending"
	}

	completed := make([]ProductOperationStep, 0, len(operationStepOrder))
	pending := make([]ProductOperationStep, 0, len(operationStepOrder))
	for _, st := range operationStepOrder {
		if stepStatus[st] == "done" {
			completed = append(completed, st)
		} else {
			pending = append(pending, st)
		}
	}
	current := OperationStepReady
	if len(pending) > 0 {
		current = pending[0]
	}
	act := operationActionFor(p.ID, current)
	return &ProductOperationProgress{
		ProductID:         p.ID,
		CompletionPercent: int(float64(len(completed))*100/float64(len(operationStepOrder)) + 0.5),
		CurrentStep:       current,
		CurrentStepLabel:  operationStepLabels[current],
		NextActionLabel:   act.Label,
		NextActionKey:     act.Key,
		NextActionURL:     act.URL,
		CompletedSteps:    completed,
		PendingSteps:      pending,
		Blockers:          dedupeOperationBlockers(blockers),
		Warnings:          dedupeOperationWarnings(warnings),
		PublishReady:      publishReady,
		UpdatedAt:         p.UpdatedAt,
		StepStatus:        stepStatus,
	}
}

func (p *ProductOperationProgress) Summary() *ProductOperationProgressSummary {
	if p == nil {
		return nil
	}
	return &ProductOperationProgressSummary{
		CompletionPercent: p.CompletionPercent,
		CurrentStep:       p.CurrentStep,
		CurrentStepLabel:  p.CurrentStepLabel,
		NextActionLabel:   p.NextActionLabel,
		NextActionKey:     p.NextActionKey,
		NextActionURL:     p.NextActionURL,
		BlockerCount:      len(p.Blockers),
		WarningCount:      len(p.Warnings),
		PublishReady:      p.PublishReady,
	}
}

func validProductTitle(p *Product) bool {
	title := strings.TrimSpace(p.Title)
	if title == "" {
		title = strings.TrimSpace(p.OriginalTitle)
	}
	rn := len([]rune(title))
	return rn >= 4 && rn <= 180 && !strings.Contains(title, "未命名")
}

func validProductDescription(p *Product) bool {
	desc := strings.TrimSpace(p.Description)
	if desc == "" {
		desc = strings.TrimSpace(p.AIDescription)
	}
	return len([]rune(desc)) >= 20
}

func hasMainImage(p *Product) bool {
	if p == nil {
		return false
	}
	for _, img := range p.Images {
		t := strings.TrimSpace(strings.ToLower(img.ImageType))
		if t == ImageTypeDescription {
			t = ImageTypeDetail
		}
		if t == ImageTypeMain && bestProgressImageURL(img) != "" {
			return true
		}
	}
	for _, img := range p.Images {
		t := strings.TrimSpace(strings.ToLower(img.ImageType))
		if t != ImageTypeSKU && bestProgressImageURL(img) != "" {
			return true
		}
	}
	return false
}

func bestProgressImageURL(img ProductImage) string {
	for _, v := range []string{img.PublicURL, img.OriginURL, img.ObjectKey, img.StorageKey} {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func hasAnyPriceOrCost(p *Product) bool {
	for _, sku := range p.SKUs {
		if sku.Price != nil && *sku.Price > 0 {
			return true
		}
		if sku.CostPrice != nil && *sku.CostPrice > 0 {
			return true
		}
	}
	return false
}

func pricingOK(p *Product) bool {
	if strings.TrimSpace(p.Currency) == "" || len(p.SKUs) == 0 {
		return false
	}
	for _, sku := range p.SKUs {
		if sku.Price == nil || *sku.Price <= 0 {
			return false
		}
	}
	return true
}

func attributesOK(p *Product) bool {
	for _, sku := range p.SKUs {
		if len(sku.Attrs) > 2 && string(sku.Attrs) != "null" && string(sku.Attrs) != "{}" {
			return true
		}
	}
	attrs, _ := rawDraftDebugFields(json.RawMessage(p.RawData))
	return len(attrs) > 2 && string(attrs) != "null" && string(attrs) != "{}"
}

func hasLowQualityImageTask(stats map[string]int) bool {
	for _, st := range []string{"low_quality", "need_manual_review", "success_with_review", "success_with_warnings"} {
		if stats[st] > 0 {
			return true
		}
	}
	return false
}

func appendReadinessIssues(productID uuid.UUID, r *OperationReadinessResult, blockers *[]ProductOperationIssue, warnings *[]ProductOperationWarning, steps map[ProductOperationStep]string) {
	for _, c := range r.Checks {
		step := readinessStep(c)
		act := operationActionFor(productID, step)
		title := strings.TrimSpace(c.Message)
		if title == "" {
			title = c.Code
		}
		msg := strings.TrimSpace(c.Suggestion)
		if msg == "" {
			msg = title
		}
		if strings.EqualFold(c.Level, "error") || strings.EqualFold(c.Level, "failed") {
			*blockers = append(*blockers, ProductOperationIssue{
				Code: c.Code, Title: title, Message: msg, Severity: "failed",
				ActionLabel: act.Label, ActionKey: act.Key, ActionURL: act.URL,
			})
			steps[step] = "failed"
		} else {
			*warnings = append(*warnings, ProductOperationWarning{Code: c.Code, Title: title, Message: msg})
			if steps[step] == "" {
				steps[step] = "warning"
			}
		}
	}
}

func readinessStep(c OperationReadinessCheck) ProductOperationStep {
	code := strings.ToLower(c.Code)
	group := strings.ToLower(c.Group)
	switch {
	case strings.Contains(code, "title"):
		return OperationStepTitle
	case strings.Contains(code, "description"):
		return OperationStepDescription
	case group == "image" || strings.Contains(code, "image"):
		return OperationStepImages
	case group == "pricing" || strings.Contains(code, "price"):
		return OperationStepPricing
	case group == "sku":
		return OperationStepPricing
	case group == "collect":
		return OperationStepCollectReview
	case group == "platform":
		return OperationStepPublishCheck
	default:
		return OperationStepPublishCheck
	}
}

func dedupeOperationBlockers(items []ProductOperationIssue) []ProductOperationIssue {
	seen := map[string]struct{}{}
	out := make([]ProductOperationIssue, 0, len(items))
	for _, item := range items {
		key := item.Code + "|" + item.ActionKey
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func dedupeOperationWarnings(items []ProductOperationWarning) []ProductOperationWarning {
	seen := map[string]struct{}{}
	out := make([]ProductOperationWarning, 0, len(items))
	for _, item := range items {
		key := item.Code + "|" + item.Title + "|" + item.Message
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func productContentHash(s string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(s)))
	return hex.EncodeToString(sum[:])
}

func parseExpectedUpdatedAt(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return nil, fmt.Errorf("invalid expectedUpdatedAt")
	}
	u := t.UTC()
	return &u, nil
}
