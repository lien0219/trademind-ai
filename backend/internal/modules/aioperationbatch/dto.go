package aioperationbatch

// ProductFilters narrows drafts for bulk AI (camelCase JSON from admin).
type ProductFilters struct {
	Keyword                  string `json:"keyword"`
	Status                   string `json:"status"`
	Source                   string `json:"source"`
	OnlyMissingAiTitle       bool   `json:"onlyMissingAiTitle"`
	OnlyMissingAiDescription bool   `json:"onlyMissingAiDescription"`
	OnlyHasMainImage         bool   `json:"onlyHasMainImage"`
}

// ProductTextOptions AI parameters summary (no full prompts).
type ProductTextOptions struct {
	Language  string `json:"language"`
	Platform  string `json:"platform"`
	MaxLength int    `json:"maxLength"`
	Tone      string `json:"tone"`
}

// CreateProductTextBatchBody POST /ai/batches/product-text
type CreateProductTextBatchBody struct {
	OperationType string             `json:"operationType" binding:"required"`
	ProductIDs    []string           `json:"productIds"`
	Filters       ProductFilters     `json:"filters"`
	Options       ProductTextOptions `json:"options"`
	ApplyMode     string             `json:"applyMode"` // task_only | save_ai_field
	ConfirmAll    bool               `json:"confirmAll"`
}

// ProductImageOptions user options for image batch (trimmed when stored on batch.input).
type ProductImageOptions struct {
	Provider         string `json:"provider"`
	Prompt           string `json:"prompt"`
	BackgroundPrompt string `json:"backgroundPrompt"`
	Style            string `json:"style"`
}

// CreateProductImagesBatchBody POST /ai/batches/product-images
type CreateProductImagesBatchBody struct {
	OperationType string              `json:"operationType" binding:"required"`
	ProductIDs    []string            `json:"productIds"`
	Filters       ProductFilters      `json:"filters"`
	Options       ProductImageOptions `json:"options"`
	ConfirmAll    bool                `json:"confirmAll"`
}

// ListBatchesQuery GET /ai/batches
type ListBatchesQuery struct {
	Page          int
	PageSize      int
	OperationType string
	Status        string
	CreatedBy     *string
	Start         *string
	End           *string
}

// BatchTasksQuery GET /ai/batches/:id/tasks
type BatchTasksQuery struct {
	Page     int
	PageSize int
}

// ApplyBatchResultsBody POST /ai/batches/:id/apply-results
type ApplyBatchResultsBody struct {
	Target     string   `json:"target" binding:"required"` // ai_field
	ProductIDs []string `json:"productIds"`
}
