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

type descriptionGenerateOutput struct {
	Description     string   `json:"description"`
	Highlights      []string `json:"highlights"`
	Specifications  []string `json:"specifications"`
	PackageIncludes []string `json:"packageIncludes"`
	Notes           string   `json:"notes"`
	Reason          string   `json:"reason"`
}

func productSKUSummary(p *Product) string {
	if p == nil || len(p.SKUs) == 0 {
		return "none"
	}
	var b strings.Builder
	for _, sku := range p.SKUs {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		line := strings.TrimSpace(sku.SKUCode) + " | " + strings.TrimSpace(sku.SKUName)
		if len(sku.Attrs) > 0 {
			line += " | attrs: " + truncateRunes(string(sku.Attrs), 200)
		}
		if sku.Price != nil {
			line += fmt.Sprintf(" | price: %v", *sku.Price)
		}
		if sku.Stock != nil {
			line += fmt.Sprintf(" | stock: %d", *sku.Stock)
		}
		b.WriteString(line)
	}
	return truncateRunes(b.String(), 4000)
}

func parseDescriptionGenerateJSON(content string) (descriptionGenerateOutput, error) {
	content = stripCodeFences(content)
	var out descriptionGenerateOutput
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return descriptionGenerateOutput{}, err
	}
	out.Description = strings.TrimSpace(out.Description)
	out.Notes = strings.TrimSpace(out.Notes)
	out.Reason = strings.TrimSpace(out.Reason)
	trimLines := func(s []string) []string {
		for i := range s {
			s[i] = strings.TrimSpace(s[i])
		}
		return s
	}
	out.Highlights = trimLines(out.Highlights)
	out.Specifications = trimLines(out.Specifications)
	out.PackageIncludes = trimLines(out.PackageIncludes)
	return out, nil
}

// GenerateDescription runs product_description_generate via AI gateway.
func (s *Service) GenerateDescription(c *gin.Context, productID uuid.UUID, body GenerateDescriptionBody, adminID *uuid.UUID) (*GenerateDescriptionResult, error) {
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
	tone := strings.TrimSpace(body.Tone)
	if tone == "" {
		tone = "professional"
	}

	var p Product
	if err := s.DB.WithContext(c.Request.Context()).
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order ASC, created_at ASC") }).
		Preload("SKUs", func(db *gorm.DB) *gorm.DB { return db.Order("created_at ASC") }).
		First(&p, "id = ?", productID).Error; err != nil {
		return nil, err
	}

	promptRow, err := s.Prompts.GetEnabledByCode(c.Request.Context(), aiprompt.CodeProductDescriptionGenerate)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("prompt %s not found or disabled", aiprompt.CodeProductDescriptionGenerate)
		}
		return nil, err
	}

	aiTitleVal := strings.TrimSpace(p.AITitle)
	if aiTitleVal == "" {
		aiTitleVal = "(none)"
	}

	vars := map[string]string{
		"title":         productPromptTitle(&p),
		"originalTitle": strings.TrimSpace(p.OriginalTitle),
		"aiTitle":       aiTitleVal,
		"attributes":    productAttributesSummary(&p),
		"skus":          productSKUSummary(&p),
		"language":      lang,
		"platform":      platform,
		"tone":          tone,
	}
	if vars["originalTitle"] == "" {
		vars["originalTitle"] = "(none)"
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
		"promptCode": aiprompt.CodeProductDescriptionGenerate,
		"productId":  p.ID.String(),
		"language":   lang,
		"platform":   platform,
		"tone":       tone,
	}
	inputJSON, _ := json.Marshal(inputPayload)

	task := &aitask.AITask{
		TaskType:    "product_description_generate",
		Provider:    s.providerNameFromSettings(c),
		Model:       model,
		PromptCode:  aiprompt.CodeProductDescriptionGenerate,
		Input:       datatypes.JSON(inputJSON),
		ProductID:   &p.ID,
		CreatedBy:   adminID,
		TokenInput:  0,
		TokenOutput: 0,
		CostAmount:  0,
	}
	if err := s.AITasks.Create(c.Request.Context(), task); err != nil {
		return nil, err
	}
	taskID := task.ID

	resp, err := s.AIGateway.Chat(c.Request.Context(), req)
	if err != nil {
		_ = s.AITasks.MarkFailed(c.Request.Context(), taskID, err.Error())
		if s.OpLog != nil {
			_ = s.OpLog.Write(c, operationlog.WriteOpts{
				AdminUserID: adminID,
				Action:      "ai.description_generate.failed",
				Resource:    "product",
				ResourceID:  p.ID.String(),
				Status:      "failed",
				Message:     fmt.Sprintf("taskId=%s err=%s", taskID.String(), truncateRunes(err.Error(), 400)),
			})
		}
		return nil, err
	}

	parsed, perr := parseDescriptionGenerateJSON(resp.Content)
	if perr != nil {
		msg := fmt.Sprintf("parse ai json: %v", perr)
		_ = s.AITasks.MarkFailed(c.Request.Context(), taskID, msg)
		if s.OpLog != nil {
			_ = s.OpLog.Write(c, operationlog.WriteOpts{
				AdminUserID: adminID,
				Action:      "ai.description_generate.failed",
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

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "ai.description_generate.success",
			Resource:    "product",
			ResourceID:  p.ID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("taskId=%s descLen=%d", taskID.String(), len([]rune(parsed.Description))),
		})
	}

	return &GenerateDescriptionResult{
		Description:     parsed.Description,
		Highlights:      parsed.Highlights,
		Specifications:  parsed.Specifications,
		PackageIncludes: parsed.PackageIncludes,
		Notes:           parsed.Notes,
		Reason:          parsed.Reason,
		TaskID:          taskID.String(),
	}, nil
}

// ApplyAIDescription stores generated copy on products.ai_description only.
func (s *Service) ApplyAIDescription(c *gin.Context, productID uuid.UUID, body ApplyAIDescriptionBody, adminID *uuid.UUID) (*DetailDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	text := strings.TrimSpace(body.AIDescription)
	if text == "" {
		return nil, fmt.Errorf("aiDescription is required")
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
	p.AIDescription = text
	if err := s.DB.WithContext(c.Request.Context()).Save(&p).Error; err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "ai.description.apply",
			Resource:    "product",
			ResourceID:  p.ID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("taskId=%s aiDescLen=%d", taskUUID.String(), len([]rune(text))),
		})
	}
	return s.Get(c, p.ID)
}
