package aiproducttext

// TextGenerationOptions user-facing generation preferences (no full prompts).
type TextGenerationOptions struct {
	Language           string   `json:"language"`
	Platform           string   `json:"platform"`
	Tone               string   `json:"tone"`
	MaxLength          int      `json:"maxLength"`
	TitleStyle         string   `json:"titleStyle"`
	HighlightSelling   bool     `json:"highlightSelling"`
	KeepBrandWords     bool     `json:"keepBrandWords"`
	KeepSpecWords      bool     `json:"keepSpecWords"`
	RemoveCollectNoise bool     `json:"removeCollectNoise"`
	DescStyle          string   `json:"descStyle"`
	DescStructure      string   `json:"descStructure"`
	HighlightScenarios bool     `json:"highlightScenarios"`
	GenerateBullets    bool     `json:"generateBullets"`
	KeepOriginalParams bool     `json:"keepOriginalParams"`
	CrossBorderReady   bool     `json:"crossBorderReady"`
	Keywords           []string `json:"keywords"`
	ForbiddenWords     []string `json:"forbiddenWords"`
	Remark             string   `json:"remark"`
}

// CheckBatchRequest POST /products/ai-text/batches/check
type CheckBatchRequest struct {
	ProductIDs     []string              `json:"productIds"`
	OperationTypes []string              `json:"operationTypes"`
	Options        TextGenerationOptions `json:"options"`
}

// CheckBatchSummary aggregates pre-flight check.
type CheckBatchSummary struct {
	ProductCount int `json:"productCount"`
	ItemCount    int `json:"itemCount"`
	ReadyCount   int `json:"readyCount"`
	WarningCount int `json:"warningCount"`
	BlockedCount int `json:"blockedCount"`
}

// CheckBatchItem is one product × operationType check cell.
type CheckBatchItem struct {
	ProductID      string   `json:"productId"`
	ProductTitle   string   `json:"productTitle"`
	OperationType  string   `json:"operationType"`
	OperationLabel string   `json:"operationLabel"`
	Status         string   `json:"status"`
	StatusLabel    string   `json:"statusLabel"`
	CurrentContent string   `json:"currentContent,omitempty"`
	Issues         []string `json:"issues"`
}

// CheckBatchResponse POST /products/ai-text/batches/check response.
type CheckBatchResponse struct {
	Summary CheckBatchSummary `json:"summary"`
	Items   []CheckBatchItem  `json:"items"`
}

// CreateBatchRequest POST /products/ai-text/batches
type CreateBatchRequest struct {
	ProductIDs     []string              `json:"productIds"`
	OperationTypes []string              `json:"operationTypes"`
	Options        TextGenerationOptions `json:"options"`
	IdempotencyKey string                `json:"idempotencyKey,omitempty"`
}

// BatchListItem is one row in batch list.
type BatchListItem struct {
	ID             string   `json:"id"`
	BatchNo        string   `json:"batchNo"`
	Status         string   `json:"status"`
	StatusLabel    string   `json:"statusLabel"`
	ProductCount   int      `json:"productCount"`
	ItemCount      int      `json:"itemCount"`
	SuccessCount   int      `json:"successCount"`
	FailedCount    int      `json:"failedCount"`
	AppliedCount   int      `json:"appliedCount"`
	OperationTypes []string `json:"operationTypes"`
	CreatedAt      string   `json:"createdAt"`
	FinishedAt     *string  `json:"finishedAt,omitempty"`
}

// ItemDetailDTO is one review row.
type ItemDetailDTO struct {
	ID                 string           `json:"id"`
	ProductID          string           `json:"productId"`
	ProductTitle       string           `json:"productTitle"`
	OperationType      string           `json:"operationType"`
	OperationLabel     string           `json:"operationLabel"`
	Status             string           `json:"status"`
	StatusLabel        string           `json:"statusLabel"`
	CurrentContent     string           `json:"currentContent"`
	GeneratedText      string           `json:"generatedText"`
	EditedText         string           `json:"editedText"`
	PrepareApplyText   string           `json:"prepareApplyText"`
	QualityWarnings    []QualityWarning `json:"qualityWarnings"`
	ErrorMessage       string           `json:"errorMessage,omitempty"`
	AITaskID           string           `json:"aiTaskId,omitempty"`
	SourceSnapshotHash string           `json:"sourceSnapshotHash,omitempty"`
	ProductUpdatedAt   string           `json:"productUpdatedAt,omitempty"`
	AppliedAt          *string          `json:"appliedAt,omitempty"`
	ApplicationID      string           `json:"applicationId,omitempty"`
}

// BatchDetailDTO GET batches/:id response.
type BatchDetailDTO struct {
	BatchListItem
	Items  []ItemDetailDTO `json:"items"`
	Input  map[string]any  `json:"input,omitempty"`
	Output map[string]any  `json:"output,omitempty"`
}

// UpdateEditedTextBody POST items/:id/update-edited-text
type UpdateEditedTextBody struct {
	EditedText string `json:"editedText"`
}

// ApplyItemBody POST items/:id/apply
type ApplyItemBody struct {
	Text string `json:"text,omitempty"`
}

// ApplySelectedBody POST batches/:id/apply-selected
type ApplySelectedBody struct {
	ItemIDs []string `json:"itemIds"`
}

// ApplyResultSummary batch apply outcome.
type ApplyResultSummary struct {
	SuccessCount  int               `json:"successCount"`
	ConflictCount int               `json:"conflictCount"`
	FailedCount   int               `json:"failedCount"`
	Items         []ApplyItemResult `json:"items"`
}

// ApplyItemResult per-item apply outcome.
type ApplyItemResult struct {
	ItemID       string `json:"itemId"`
	ProductID    string `json:"productId"`
	Status       string `json:"status"`
	StatusLabel  string `json:"statusLabel"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// UndoAppliedSummary batch undo outcome.
type UndoAppliedSummary struct {
	SuccessCount  int               `json:"successCount"`
	ConflictCount int               `json:"conflictCount"`
	FailedCount   int               `json:"failedCount"`
	Items         []ApplyItemResult `json:"items"`
}
