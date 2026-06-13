package taskcenter

import (
	"time"

	"github.com/google/uuid"
)

// Alert row statuses (distinct from UnifiedTaskDTO.alertStatus view).
const (
	TaskAlertStatusOpen     = "open"
	TaskAlertStatusHandled  = "handled"
	TaskAlertStatusIgnored  = "ignored"
	TaskAlertStatusResolved = "resolved"
)

// AlertStatusProjection for failure list rows linking to TaskAlert rows.
const (
	AlertStatusNone      = "none"
	AlertStatusGenerated = "generated"
	AlertStatusIgnored   = "ignored"
	AlertStatusHandled   = "handled"
)

// TaskAlert is an in-site alert record (deduped per task+category).
type TaskAlert struct {
	ID              uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	TaskType        string     `gorm:"size:48;uniqueIndex:uq_task_alert_type_src_cat;not null" json:"taskType"`
	SourceID        string     `gorm:"size:64;uniqueIndex:uq_task_alert_type_src_cat;not null" json:"sourceId"`
	SourceTable     string     `gorm:"size:64" json:"sourceTable,omitempty"`
	FailureCategory string     `gorm:"size:48;uniqueIndex:uq_task_alert_type_src_cat;not null" json:"failureCategory"`
	Severity        string     `gorm:"size:16;index" json:"severity"`
	Platform        string     `gorm:"size:48;index" json:"platform,omitempty"`
	Title           string     `gorm:"size:255" json:"title"`
	Message         string     `gorm:"type:text" json:"message,omitempty"`
	SuggestedAction string     `gorm:"type:text" json:"suggestedAction,omitempty"`
	Status          string     `gorm:"size:16;index;not null" json:"status"`
	AlertCount      int        `gorm:"default:1" json:"alertCount"`
	FirstSeenAt     time.Time  `gorm:"index" json:"firstSeenAt"`
	LastSeenAt      time.Time  `gorm:"index" json:"lastSeenAt"`
	HandledAt       *time.Time `json:"handledAt,omitempty"`
	HandledBy       *uuid.UUID `gorm:"type:char(36)" json:"handledBy,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

func (TaskAlert) TableName() string { return "task_alerts" }
