package taskcenter

import "time"

// TaskAlertDTO is returned by alerts list/detail APIs (no nested task rows).
type TaskAlertDTO struct {
	ID                 string     `json:"id"`
	TaskType           string     `json:"taskType"`
	SourceID           string     `json:"sourceId"`
	SourceTable        string     `json:"sourceTable,omitempty"`
	Platform           string     `json:"platform,omitempty"`
	FailureCategory    string     `json:"failureCategory"`
	Severity           string     `json:"severity"`
	Title              string     `json:"title"`
	Message            string     `json:"message,omitempty"`
	SuggestedAction    string     `json:"suggestedAction,omitempty"`
	Status             string     `json:"status"`
	AlertCount         int        `json:"alertCount"`
	FirstSeenAt        time.Time  `json:"firstSeenAt"`
	LastSeenAt         time.Time  `json:"lastSeenAt"`
	HandledAt          *time.Time `json:"handledAt,omitempty"`
	NotificationStatus string     `json:"notificationStatus,omitempty"`
}

// ListAlertsResult is paged alerts.
type ListAlertsResult struct {
	List  []TaskAlertDTO `json:"list"`
	Total int64          `json:"total"`
}

// ScanAlertsSummary is POST .../alerts/scan response.
type ScanAlertsSummary struct {
	ScannedCount   int `json:"scannedCount"`
	GeneratedCount int `json:"generatedCount"`
	UpdatedCount   int `json:"updatedCount"`
	IgnoredCount   int `json:"ignoredCount"`
}
