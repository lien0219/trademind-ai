package aiproductimage

// ImageGenerationOptions user-facing processing preferences.
type ImageGenerationOptions struct {
	Language         string `json:"language"`
	BackgroundStyle  string `json:"backgroundStyle"`
	KeepSubject      bool   `json:"keepSubject"`
	KeepBrandLogo    bool   `json:"keepBrandLogo"`
	SkipFailedImages bool   `json:"skipFailedImages"`
	OutputFormat     string `json:"outputFormat"`
	Remark           string `json:"remark"`
}

// CheckBatchRequest POST /products/ai-images/batches/check
type CheckBatchRequest struct {
	ProductIDs     []string               `json:"productIds"`
	ImageIDs       []string               `json:"imageIds,omitempty"`
	OperationTypes []string               `json:"operationTypes"`
	Options        ImageGenerationOptions `json:"options"`
}

// CheckBatchSummary aggregates pre-flight check.
type CheckBatchSummary struct {
	ProductCount int `json:"productCount"`
	ImageCount   int `json:"imageCount"`
	ItemCount    int `json:"itemCount"`
	ReadyCount   int `json:"readyCount"`
	WarningCount int `json:"warningCount"`
	BlockedCount int `json:"blockedCount"`
}

// CheckBatchItem is one image × operation check cell.
type CheckBatchItem struct {
	ProductID      string   `json:"productId"`
	ProductTitle   string   `json:"productTitle,omitempty"`
	ImageID        string   `json:"imageId"`
	ImageType      string   `json:"imageType"`
	ImageTypeLabel string   `json:"imageTypeLabel"`
	SourceImageURL string   `json:"sourceImageUrl,omitempty"`
	OperationType  string   `json:"operationType"`
	OperationLabel string   `json:"operationLabel"`
	Status         string   `json:"status"`
	StatusLabel    string   `json:"statusLabel"`
	Issues         []string `json:"issues"`
}

// CheckBatchResponse POST /products/ai-images/batches/check response.
type CheckBatchResponse struct {
	Summary CheckBatchSummary `json:"summary"`
	Items   []CheckBatchItem  `json:"items"`
}

// CreateBatchRequest POST /products/ai-images/batches
type CreateBatchRequest struct {
	ProductIDs     []string               `json:"productIds"`
	ImageIDs       []string               `json:"imageIds"`
	OperationTypes []string               `json:"operationTypes"`
	Options        ImageGenerationOptions `json:"options"`
	IdempotencyKey string                 `json:"idempotencyKey,omitempty"`
}

// BatchListItem is one row in batch list.
type BatchListItem struct {
	ID             string   `json:"id"`
	BatchNo        string   `json:"batchNo"`
	Status         string   `json:"status"`
	StatusLabel    string   `json:"statusLabel"`
	ProductCount   int      `json:"productCount"`
	ImageCount     int      `json:"imageCount"`
	ItemCount      int      `json:"itemCount"`
	SuccessCount   int      `json:"successCount"`
	FailedCount    int      `json:"failedCount"`
	AppliedCount   int      `json:"appliedCount"`
	OperationTypes []string `json:"operationTypes"`
	CreatedAt      string   `json:"createdAt"`
	FinishedAt     *string  `json:"finishedAt,omitempty"`
}

// QualityWarning is a user-facing quality hint.
type QualityWarning struct {
	Code    string `json:"code"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

// ItemDetailDTO is one review row.
type ItemDetailDTO struct {
	ID                 string           `json:"id"`
	ProductID          string           `json:"productId"`
	ProductTitle       string           `json:"productTitle"`
	ImageID            string           `json:"imageId,omitempty"`
	ImageType          string           `json:"imageType"`
	ImageTypeLabel     string           `json:"imageTypeLabel"`
	OperationType      string           `json:"operationType"`
	OperationLabel     string           `json:"operationLabel"`
	Status             string           `json:"status"`
	StatusLabel        string           `json:"statusLabel"`
	SourceImageURL     string           `json:"sourceImageUrl"`
	ResultImageURL     string           `json:"resultImageUrl,omitempty"`
	QualityWarnings    []QualityWarning `json:"qualityWarnings"`
	ErrorMessage       string           `json:"errorMessage,omitempty"`
	ImageTaskID        string           `json:"imageTaskId,omitempty"`
	SourceSnapshotHash string           `json:"sourceSnapshotHash,omitempty"`
	ApplyMode          string           `json:"applyMode,omitempty"`
	ApplyModeLabel     string           `json:"applyModeLabel,omitempty"`
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

// ApplyItemBody POST items/:id/apply
type ApplyItemBody struct {
	ApplyMode string `json:"applyMode"`
}

// ApplySelectedBody POST batches/:id/apply-selected
type ApplySelectedBody struct {
	ItemIDs   []string `json:"itemIds"`
	ApplyMode string   `json:"applyMode,omitempty"`
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
