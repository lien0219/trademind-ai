package aiprompt

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Service manages ai_prompts rows.
type Service struct {
	DB *gorm.DB
}

// List returns all prompts (admin).
func (s *Service) List(ctx context.Context) ([]AIPrompt, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("aiprompt: no db")
	}
	var rows []AIPrompt
	if err := s.DB.WithContext(ctx).Order("code ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetByID returns one prompt.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*AIPrompt, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("aiprompt: no db")
	}
	var row AIPrompt
	if err := s.DB.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// GetEnabledByCode returns an enabled prompt by unique code.
func (s *Service) GetEnabledByCode(ctx context.Context, code string) (*AIPrompt, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("aiprompt: no db")
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, fmt.Errorf("aiprompt: code required")
	}
	var row AIPrompt
	if err := s.DB.WithContext(ctx).Where("code = ? AND enabled = ?", code, true).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// CreateBody binds POST /ai/prompts.
type CreateBody struct {
	Code         string          `json:"code"`
	Name         string          `json:"name"`
	Scene        string          `json:"scene"`
	Provider     string          `json:"provider"`
	Model        string          `json:"model"`
	SystemPrompt string          `json:"systemPrompt"`
	UserPrompt   string          `json:"userPrompt"`
	OutputSchema json.RawMessage `json:"outputSchema"`
	Temperature  *float64        `json:"temperature"`
	MaxTokens    *int            `json:"maxTokens"`
	Enabled      *bool           `json:"enabled"`
}

// UpdateBody binds PUT /ai/prompts/:id.
type UpdateBody struct {
	Name         *string         `json:"name"`
	Scene        *string         `json:"scene"`
	Provider     *string         `json:"provider"`
	Model        *string         `json:"model"`
	SystemPrompt *string         `json:"systemPrompt"`
	UserPrompt   *string         `json:"userPrompt"`
	OutputSchema json.RawMessage `json:"outputSchema"`
	Temperature  *float64        `json:"temperature"`
	MaxTokens    *int            `json:"maxTokens"`
	Enabled      *bool           `json:"enabled"`
}

func schemaJSON(raw json.RawMessage) (datatypes.JSON, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var tmp any
	if err := json.Unmarshal(raw, &tmp); err != nil {
		return nil, fmt.Errorf("outputSchema: invalid json")
	}
	return datatypes.JSON(raw), nil
}

// Create inserts a new prompt.
func (s *Service) Create(ctx context.Context, body CreateBody) (*AIPrompt, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("aiprompt: no db")
	}
	code := strings.TrimSpace(body.Code)
	if code == "" {
		return nil, fmt.Errorf("code required")
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}
	schema, err := schemaJSON(body.OutputSchema)
	if err != nil {
		return nil, err
	}
	row := &AIPrompt{
		Code:         code,
		Name:         name,
		Scene:        strings.TrimSpace(body.Scene),
		Provider:     strings.TrimSpace(body.Provider),
		Model:        strings.TrimSpace(body.Model),
		SystemPrompt: body.SystemPrompt,
		UserPrompt:   strings.TrimSpace(body.UserPrompt),
		OutputSchema: schema,
		Temperature:  0.7,
		MaxTokens:    512,
		Enabled:      true,
	}
	if body.Temperature != nil {
		row.Temperature = *body.Temperature
	}
	if body.MaxTokens != nil {
		row.MaxTokens = *body.MaxTokens
	}
	if body.Enabled != nil {
		row.Enabled = *body.Enabled
	}
	if err := s.DB.WithContext(ctx).Create(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

// Update patches a prompt (code immutable).
func (s *Service) Update(ctx context.Context, id uuid.UUID, body UpdateBody) (*AIPrompt, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("aiprompt: no db")
	}
	var row AIPrompt
	if err := s.DB.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if body.Name != nil {
		row.Name = strings.TrimSpace(*body.Name)
	}
	if body.Scene != nil {
		row.Scene = strings.TrimSpace(*body.Scene)
	}
	if body.Provider != nil {
		row.Provider = strings.TrimSpace(*body.Provider)
	}
	if body.Model != nil {
		row.Model = strings.TrimSpace(*body.Model)
	}
	if body.SystemPrompt != nil {
		row.SystemPrompt = *body.SystemPrompt
	}
	if body.UserPrompt != nil {
		row.UserPrompt = strings.TrimSpace(*body.UserPrompt)
	}
	if len(body.OutputSchema) > 0 {
		schema, err := schemaJSON(body.OutputSchema)
		if err != nil {
			return nil, err
		}
		row.OutputSchema = schema
	}
	if body.Temperature != nil {
		row.Temperature = *body.Temperature
	}
	if body.MaxTokens != nil {
		row.MaxTokens = *body.MaxTokens
	}
	if body.Enabled != nil {
		row.Enabled = *body.Enabled
	}
	if err := s.DB.WithContext(ctx).Save(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// Delete soft-deletes a prompt.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("aiprompt: no db")
	}
	res := s.DB.WithContext(ctx).Delete(&AIPrompt{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// SetEnabled updates the enabled flag.
func (s *Service) SetEnabled(ctx context.Context, id uuid.UUID, enabled bool) (*AIPrompt, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("aiprompt: no db")
	}
	var row AIPrompt
	if err := s.DB.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	row.Enabled = enabled
	if err := s.DB.WithContext(ctx).Save(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// ReplaceVariables substitutes {{key}} placeholders in s using vars.
func ReplaceVariables(s string, vars map[string]string) string {
	out := s
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out
}
