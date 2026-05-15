package aiprompt

import (
	"context"
	"encoding/json"
	"strings"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const CodeProductTitleOptimize = "product_title_optimize"

// EnsureDefaults creates built-in prompts when missing.
func EnsureDefaults(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return nil
	}
	schema, _ := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"optimizedTitle": map[string]string{"type": "string"},
			"keywords": map[string]any{
				"type":  "array",
				"items": map[string]string{"type": "string"},
			},
			"reason": map[string]string{"type": "string"},
		},
		"required": []string{"optimizedTitle", "keywords", "reason"},
	})
	defaultSys := strings.TrimSpace(`You are an expert cross-border e-commerce copywriter.
Return ONLY valid JSON (no markdown fences) with keys: optimizedTitle (string), keywords (string array), reason (short string in the same language as the user's requested listing language).
The optimizedTitle must respect max length and platform style hints from the user message.`)
	defaultUser := strings.TrimSpace(`Optimize this product listing title.

Context:
- Current title: {{title}}
- Category: {{category}}
- Attributes / specs: {{attributes}}
- Target language: {{language}}
- Target platform: {{platform}}
- Max title length (characters): {{maxLength}}

Reply with JSON only.`)

	var count int64
	if err := db.WithContext(ctx).Model(&AIPrompt{}).Where("code = ?", CodeProductTitleOptimize).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	row := &AIPrompt{
		Code:         CodeProductTitleOptimize,
		Name:         "商品标题优化",
		Scene:        "product",
		Provider:     "",
		Model:        "",
		SystemPrompt: defaultSys,
		UserPrompt:   defaultUser,
		OutputSchema: datatypes.JSON(schema),
		Temperature:  0.4,
		MaxTokens:    800,
		Enabled:      true,
	}
	return db.WithContext(ctx).Create(row).Error
}
