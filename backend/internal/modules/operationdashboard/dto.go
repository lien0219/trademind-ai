package operationdashboard

import "time"

// Query binds GET /dashboard/product-operations.
type Query struct {
	Start    *time.Time
	End      *time.Time
	Platform string
	ShopID   string // raw UUID string
	Source   string
	Scope    Scope
}

// Summary aggregates MVP operational KPIs (read-only; local DB only).
type Summary struct {
	TotalProducts                int64 `json:"totalProducts"`
	DraftProducts                int64 `json:"draftProducts"`
	ReadyProducts                int64 `json:"readyProducts"`
	PublishedProducts            int64 `json:"publishedProducts"`
	ArchivedProducts             int64 `json:"archivedProducts"`
	AiPendingProducts            int64 `json:"aiPendingProducts"`
	ReadinessBlocked             int64 `json:"readinessBlockedProducts"`
	PublishFailedTasks           int64 `json:"publishFailedTasks"`
	LowStockSkus                 int64 `json:"lowStockSkus"`
	CustomerPendingConversations int64 `json:"customerPendingConversations"`
	FailedTasks                  int64 `json:"failedTasks"`

	MissingAiTitleCount           int64 `json:"missingAiTitleCount"`
	MissingAiDescriptionCount     int64 `json:"missingAiDescriptionCount"`
	AiTaskFailedCount             int64 `json:"aiTaskFailedCount"`
	AiBatchRunningCount           int64 `json:"aiBatchRunningCount"`
	AiBatchFailedCount            int64 `json:"aiBatchFailedCount"`
	ReadinessWarningProducts      int64 `json:"readinessWarningProducts"`
	ReadinessReadyProducts        int64 `json:"readinessReadyProducts"`
	PublishPendingTasks           int64 `json:"publishPendingTasks"`
	PublishRunningTasks           int64 `json:"publishRunningTasks"`
	PublishedPublicationCount     int64 `json:"publishedPublicationCount"`
	OutOfStockSkus                int64 `json:"outOfStockSkus"`
	PlatformStockMismatchCount    int64 `json:"platformStockMismatchCount"`
	InventorySyncFailedCount      int64 `json:"inventorySyncFailedCount"`
	CustomerOpenConversations     int64 `json:"customerOpenConversations"`
	CustomerPendingReplyCount     int64 `json:"customerPendingReplyCount"`
	AiReplySuggestionPendingCount int64 `json:"aiReplySuggestionPendingCount"`
	FailedTaskTotal               int64 `json:"failedTaskTotal"`
	CriticalAlertCount            int64 `json:"criticalAlertCount"`
	OpenAlertCount                int64 `json:"openAlertCount"`
	OrderExceptionTotal           int64 `json:"orderExceptionTotal"`
	SKUUnmatchedOrderItems        int64 `json:"skuUnmatchedOrderItems"`
	InventoryDeductFailedOrders   int64 `json:"inventoryDeductFailedOrders"`

	// Compact KPI aliases for the workbench overview cards.
	DraftTotal           int64 `json:"draftTotal"`
	TodayNewProducts     int64 `json:"todayNewProducts"`
	MissingAiTitle       int64 `json:"missingAiTitle"`
	MissingAiDescription int64 `json:"missingAiDescription"`
	ImageTaskPending     int64 `json:"imageTaskPending"`
	ImageTaskFailed      int64 `json:"imageTaskFailed"`
	ReadinessBlockedKPI  int64 `json:"readinessBlocked"`
	Publishable          int64 `json:"publishable"`
	Published            int64 `json:"published"`
	ImageProcessedCount  int64 `json:"imageProcessedProducts"`
	InventoryAlerts      int64 `json:"inventoryAlerts"`
	OrderExceptions      int64 `json:"orderExceptions"`
	CollectFailedCount   int64 `json:"collectFailedCount"`
	ConfigRiskCount      int64 `json:"configRiskCount"`
	AiTitleCompleted     int64 `json:"aiTitleCompletedCount"`
	AiDescCompleted      int64 `json:"aiDescriptionCompletedCount"`
	CollectedProducts    int64 `json:"collectedProductsCount"`
	AiTextCompleted      int64 `json:"aiTextCompletedCount"`
	ReadinessPassed      int64 `json:"readinessPassedCount"`
}

// TodoCard is a single actionable backlog bucket.
type TodoCard struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Title       string `json:"title"`
	Count       int64  `json:"count"`
	Severity    string `json:"severity"`
	Level       string `json:"level"`
	Description string `json:"description"`
	Link        string `json:"link"`
}

// QuickLink is a curated navigation chip.
type QuickLink struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
}

// FunnelStep is one stage in the product operations funnel.
type FunnelStep struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Count       int64  `json:"count"`
	Link        string `json:"link"`
	Description string `json:"description,omitempty"`
}

// ExceptionItem summarizes one failure category for the workbench.
type ExceptionItem struct {
	Key          string     `json:"key"`
	Title        string     `json:"title"`
	Count        int64      `json:"count"`
	LastOccurred *time.Time `json:"lastOccurredAt,omitempty"`
	Link         string     `json:"link"`
	Description  string     `json:"description"`
}

// RecentItem is a lightweight activity row (no large JSON / no secrets / no message bodies).
type RecentItem struct {
	Type       string    `json:"type"`
	Title      string    `json:"title"`
	Subtitle   string    `json:"subtitle,omitempty"`
	Status     string    `json:"status,omitempty"`
	OccurredAt time.Time `json:"occurredAt"`
	Link       string    `json:"link"`
}

// RecentBuckets groups last activity lists (each ≤10).
type RecentBuckets struct {
	Products              []RecentItem `json:"products,omitempty"`
	CollectedProducts     []RecentItem `json:"collectedProducts,omitempty"`
	AiTasks               []RecentItem `json:"aiTasks,omitempty"`
	AiBatches             []RecentItem `json:"aiBatches,omitempty"`
	ImageTasks            []RecentItem `json:"imageTasks,omitempty"`
	PublishTasks          []RecentItem `json:"publishTasks,omitempty"`
	InventoryAlerts       []RecentItem `json:"inventoryAlerts,omitempty"`
	CustomerConversations []RecentItem `json:"customerConversations,omitempty"`
	FailedTasks           []RecentItem `json:"failedTasks,omitempty"`
	Alerts                []RecentItem `json:"alerts,omitempty"`
}

// ProductOperationsDTO is the HTTP envelope for the admin board.
type ProductOperationsDTO struct {
	Summary     Summary         `json:"summary"`
	Todos       []TodoCard      `json:"todos"`
	Funnel      []FunnelStep    `json:"funnel"`
	Exceptions  []ExceptionItem `json:"exceptions"`
	Charts      map[string]any  `json:"charts"`
	QuickLinks  []QuickLink     `json:"quickLinks"`
	Recent      RecentBuckets   `json:"recent"`
	FiltersEcho map[string]any  `json:"filters,omitempty"`
}
