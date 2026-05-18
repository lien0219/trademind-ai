package product

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/aiprompt"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
)

type titleOptimizeOutput struct {
	OptimizedTitle string   `json:"optimizedTitle"`
	Keywords       []string `json:"keywords"`
	Reason         string   `json:"reason"`
}

func (s *Service) providerNameFromSettings(ctx *gin.Context) string {
	if s == nil || s.Settings == nil {
		return "openai_compatible"
	}
	m, err := s.Settings.PlainByGroup(ctx.Request.Context(), 0, "ai")
	if err != nil {
		return "openai_compatible"
	}
	v := strings.ToLower(strings.TrimSpace(m["provider"]))
	v = strings.ReplaceAll(v, "-", "_")
	if v == "" {
		return "openai_compatible"
	}
	return v
}

func productPromptTitle(p *Product) string {
	t := strings.TrimSpace(p.Title)
	if t == "" {
		t = strings.TrimSpace(p.OriginalTitle)
	}
	return t
}

func productCategoryFromRaw(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	for _, key := range []string{"category", "catName", "categoryName", "leafCategory"} {
		if v, ok := m[key]; ok {
			var s string
			if json.Unmarshal(v, &s) == nil && strings.TrimSpace(s) != "" {
				return s
			}
		}
	}
	return ""
}

func productAttributesSummary(p *Product) string {
	var b strings.Builder
	for _, sku := range p.SKUs {
		if len(sku.Attrs) > 0 {
			if b.Len() > 0 {
				b.WriteString("; ")
			}
			b.WriteString(string(sku.Attrs))
		}
	}
	if b.Len() == 0 && len(p.RawData) > 0 {
		s := string(p.RawData)
		return truncateRunes(s, 800)
	}
	return truncateRunes(b.String(), 800)
}

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimLeft(s, "\n")
	s = strings.TrimPrefix(s, "json")
	s = strings.TrimLeft(s, "\n")
	if idx := strings.LastIndex(s, "```"); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

func parseTitleOptimizeJSON(content string) (titleOptimizeOutput, error) {
	content = stripCodeFences(content)
	var out titleOptimizeOutput
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return titleOptimizeOutput{}, err
	}
	out.OptimizedTitle = strings.TrimSpace(out.OptimizedTitle)
	for i := range out.Keywords {
		out.Keywords[i] = strings.TrimSpace(out.Keywords[i])
	}
	return out, nil
}

// OptimizeTitle runs the product_title_optimize prompt via AI gateway.
func (s *Service) OptimizeTitle(c *gin.Context, productID uuid.UUID, body OptimizeTitleBody, adminID *uuid.UUID) (*OptimizeTitleResult, error) {
	return s.optimizeTitleWithExtra(c, productID, body, adminID, nil)
}

// OptimizeTitleWithBatch runs title optimization with optional batch linkage (bulk AI).
func (s *Service) OptimizeTitleWithBatch(c *gin.Context, productID uuid.UUID, body OptimizeTitleBody, adminID *uuid.UUID, extra *AITitleRunExtra) (*OptimizeTitleResult, error) {
	return s.optimizeTitleWithExtra(c, productID, body, adminID, extra)
}

func (s *Service) optimizeTitleWithExtra(c *gin.Context, productID uuid.UUID, body OptimizeTitleBody, adminID *uuid.UUID, extra *AITitleRunExtra) (*OptimizeTitleResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	if s.Prompts == nil || s.AITasks == nil || s.AIGateway == nil {
		return nil, fmt.Errorf("product: ai not configured")
	}

	lang := strings.TrimSpace(body.Language)
	if lang == "" {
		lang = "en"
	}
	platform := strings.TrimSpace(body.Platform)
	if platform == "" {
		platform = "TikTok Shop"
	}
	maxLen := body.MaxLength
	if maxLen <= 0 {
		maxLen = 120
	}
	tone := strings.TrimSpace(body.Tone)
	if tone == "" {
		tone = "professional"
	}

	var p Product
	if err := s.DB.WithContext(c.Request.Context()).
		Preload("SKUs", func(db *gorm.DB) *gorm.DB { return db.Order("created_at ASC") }).
		First(&p, "id = ?", productID).Error; err != nil {
		return nil, err
	}

	promptRow, err := s.Prompts.GetEnabledByCode(c.Request.Context(), aiprompt.CodeProductTitleOptimize)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("prompt %s not found or disabled", aiprompt.CodeProductTitleOptimize)
		}
		return nil, err
	}

	vars := map[string]string{
		"title":      productPromptTitle(&p),
		"category":   productCategoryFromRaw(json.RawMessage(p.RawData)),
		"attributes": productAttributesSummary(&p),
		"language":   lang,
		"maxLength":  fmt.Sprintf("%d", maxLen),
		"platform":   platform,
		"tone":       tone,
	}
	if vars["category"] == "" {
		vars["category"] = "unknown"
	}
	sys := aiprompt.ReplaceVariables(promptRow.SystemPrompt, vars)
	user := aiprompt.ReplaceVariables(promptRow.UserPrompt, vars)

	msgs := []aigate.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: user},
	}
	model := strings.TrimSpace(promptRow.Model)
	temp := promptRow.Temperature
	maxTok := promptRow.MaxTokens
	req := aigate.ChatRequest{
		Model:       model,
		Messages:    msgs,
		Temperature: temp,
		MaxTokens:   maxTok,
		ResponseFormat: &aigate.ResponseFormat{
			Type: "json_object",
		},
	}

	inputPayload := map[string]any{
		"promptCode": aiprompt.CodeProductTitleOptimize,
		"productId":  p.ID.String(),
		"language":   lang,
		"platform":   platform,
		"maxLength":  maxLen,
	}
	inputJSON, _ := json.Marshal(inputPayload)

	task := &aitask.AITask{
		TaskType:    "title_optimize",
		Provider:    s.providerNameFromSettings(c),
		Model:       model,
		PromptCode:  aiprompt.CodeProductTitleOptimize,
		Input:       datatypes.JSON(inputJSON),
		ProductID:   &p.ID,
		CreatedBy:   adminID,
		TokenInput:  0,
		TokenOutput: 0,
		CostAmount:  0,
	}
	if extra != nil && extra.BatchID != nil {
		task.BatchID = extra.BatchID
		task.BatchNo = strings.TrimSpace(extra.BatchNo)
	}
	if err := s.AITasks.Create(c.Request.Context(), task); err != nil {
		return nil, err
	}
	taskID := task.ID

	resp, err := s.AIGateway.Chat(c.Request.Context(), req)
	if err != nil {
		_ = s.AITasks.MarkFailed(c.Request.Context(), taskID, err.Error())
		if s.OpLog != nil && (extra == nil || !extra.SkipSingleOpLog) {
			_ = s.OpLog.Write(c, operationlog.WriteOpts{
				AdminUserID: adminID,
				Action:      "ai.title_optimize.failed",
				Resource:    "product",
				ResourceID:  p.ID.String(),
				Status:      "failed",
				Message:     fmt.Sprintf("taskId=%s err=%s", taskID.String(), truncateRunes(err.Error(), 400)),
			})
		}
		return nil, err
	}

	parsed, perr := parseTitleOptimizeJSON(resp.Content)
	if perr != nil {
		msg := fmt.Sprintf("parse ai json: %v", perr)
		_ = s.AITasks.MarkFailed(c.Request.Context(), taskID, msg)
		if s.OpLog != nil && (extra == nil || !extra.SkipSingleOpLog) {
			_ = s.OpLog.Write(c, operationlog.WriteOpts{
				AdminUserID: adminID,
				Action:      "ai.title_optimize.failed",
				Resource:    "product",
				ResourceID:  p.ID.String(),
				Status:      "failed",
				Message:     fmt.Sprintf("taskId=%s err=invalid_model_output", taskID.String()),
			})
		}
		return nil, fmt.Errorf("invalid model output")
	}

	outJSON, _ := json.Marshal(parsed)
	raw := resp.Raw
	if raw == nil {
		raw = []byte("{}")
	}
	usedModel := strings.TrimSpace(resp.Model)
	if usedModel == "" {
		usedModel = strings.TrimSpace(model)
	}
	_ = s.AITasks.MarkSuccess(c.Request.Context(), taskID, outJSON, raw, resp.InputTokens, resp.OutputTokens, usedModel)

	if extra != nil && extra.SaveAIField && strings.TrimSpace(parsed.OptimizedTitle) != "" {
		_ = s.DB.WithContext(c.Request.Context()).Model(&Product{}).Where("id = ?", p.ID).Update("ai_title", strings.TrimSpace(parsed.OptimizedTitle)).Error
	}

	if s.OpLog != nil && (extra == nil || !extra.SkipSingleOpLog) {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "ai.title_optimize.success",
			Resource:    "product",
			ResourceID:  p.ID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("taskId=%s optimizedLen=%d", taskID.String(), len([]rune(parsed.OptimizedTitle))),
		})
	}

	return &OptimizeTitleResult{
		OptimizedTitle: parsed.OptimizedTitle,
		Keywords:       parsed.Keywords,
		Reason:         parsed.Reason,
		TaskID:         taskID.String(),
	}, nil
}

// ApplyAITitle stores the chosen AI title on the product (ai_title only).
func (s *Service) ApplyAITitle(c *gin.Context, productID uuid.UUID, body ApplyAITitleBody, adminID *uuid.UUID) (*DetailDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	title := strings.TrimSpace(body.AITitle)
	if title == "" {
		return nil, fmt.Errorf("aiTitle is required")
	}
	taskIDStr := strings.TrimSpace(body.TaskID)
	if taskIDStr == "" {
		return nil, fmt.Errorf("taskId is required")
	}
	taskUUID, err := uuid.Parse(taskIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid taskId")
	}
	if s.AITasks != nil {
		tk, err := s.AITasks.GetByID(c.Request.Context(), taskUUID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				return nil, fmt.Errorf("task not found")
			}
			return nil, err
		}
		if tk.ProductID == nil || *tk.ProductID != productID {
			return nil, fmt.Errorf("task does not belong to this product")
		}
	}

	var p Product
	if err := s.DB.WithContext(c.Request.Context()).First(&p, "id = ?", productID).Error; err != nil {
		return nil, err
	}
	p.AITitle = title
	if err := s.DB.WithContext(c.Request.Context()).Save(&p).Error; err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "ai.title.apply",
			Resource:    "product",
			ResourceID:  p.ID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("taskId=%s titleLen=%d", taskUUID.String(), len([]rune(title))),
		})
	}
	return s.Get(c, p.ID)
}

// ListRecentAITasks returns recent AI tasks for a product (detail page).
func (s *Service) ListRecentAITasks(c *gin.Context, productID uuid.UUID, limit int) ([]aitask.AITask, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	var p Product
	if err := s.DB.WithContext(c.Request.Context()).Select("id").First(&p, "id = ?", productID).Error; err != nil {
		return nil, err
	}
	if s.AITasks == nil {
		return []aitask.AITask{}, nil
	}
	return s.AITasks.ListRecentForProduct(c.Request.Context(), productID, limit)
}
