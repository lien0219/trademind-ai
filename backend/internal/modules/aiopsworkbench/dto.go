package aiopsworkbench

import "time"

// Query binds list/summary filters.
type Query struct {
	Type     string
	Priority string
	Platform string
	ShopID   string
	Keyword  string
	Status   string // open | resolved (open default)
	Start    *time.Time
	End      *time.Time
	Page     int
	PageSize int
}

// SummaryDTO is the workbench headline counts.
type SummaryDTO struct {
	AITextReviewCount      int64 `json:"aiTextReviewCount"`
	AIImageReviewCount     int64 `json:"aiImageReviewCount"`
	PublishCheckIssueCount int64 `json:"publishCheckIssueCount"`
	PublishTaskIssueCount  int64 `json:"publishTaskIssueCount"`
	TodayResolvedCount     int64 `json:"todayResolvedCount"`
	HighPriorityCount      int64 `json:"highPriorityCount"`
	AITextReviewHigh       int64 `json:"aiTextReviewHighPriority,omitempty"`
	AIImageReviewHigh      int64 `json:"aiImageReviewHighPriority,omitempty"`
	PublishCheckHigh       int64 `json:"publishCheckHighPriority,omitempty"`
	PublishTaskHigh        int64 `json:"publishTaskIssueHighPriority,omitempty"`
	AITextReviewTodayNew   int64 `json:"aiTextReviewTodayNew,omitempty"`
	AIImageReviewTodayNew  int64 `json:"aiImageReviewTodayNew,omitempty"`
	PublishCheckTodayNew   int64 `json:"publishCheckTodayNew,omitempty"`
	PublishTaskTodayNew    int64 `json:"publishTaskIssueTodayNew,omitempty"`
}

// SummaryResponse wraps summary for API.
type SummaryResponse struct {
	Summary SummaryDTO `json:"summary"`
}

// TodoItem is one actionable workbench row.
type TodoItem struct {
	ID            string         `json:"id"`
	Type          string         `json:"type"`
	TypeLabel     string         `json:"typeLabel"`
	Priority      string         `json:"priority"`
	PriorityLabel string         `json:"priorityLabel"`
	ProductID     string         `json:"productId,omitempty"`
	ProductTitle  string         `json:"productTitle,omitempty"`
	Platform      string         `json:"platform,omitempty"`
	PlatformLabel string         `json:"platformLabel,omitempty"`
	ShopID        string         `json:"shopId,omitempty"`
	ShopName      string         `json:"shopName,omitempty"`
	Title         string         `json:"title"`
	Message       string         `json:"message"`
	ActionLabel   string         `json:"actionLabel"`
	ActionURL     string         `json:"actionUrl"`
	SourceType    string         `json:"sourceType"`
	SourceID      string         `json:"sourceId"`
	IssueCode     string         `json:"issueCode,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	Technical     map[string]any `json:"technicalDetails,omitempty"`
}

// Pagination is list paging metadata.
type Pagination struct {
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

// TodosResponse is paginated todo list.
type TodosResponse struct {
	Items      []TodoItem `json:"items"`
	Pagination Pagination `json:"pagination"`
}

// RefreshResponse acknowledges refresh.
type RefreshResponse struct {
	RefreshedAt time.Time  `json:"refreshedAt"`
	Summary     SummaryDTO `json:"summary"`
}
