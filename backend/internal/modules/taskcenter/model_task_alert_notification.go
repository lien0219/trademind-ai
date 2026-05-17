package taskcenter

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Notification row statuses for external channel delivery audit.
const (
	TaskAlertNotifStatusPending = "pending"
	TaskAlertNotifStatusSuccess = "success"
	TaskAlertNotifStatusFailed  = "failed"
	TaskAlertNotifStatusSkipped = "skipped"
)

// TaskAlertNotification records one outbound notify attempt for a task alert (dedup / audit; no secrets).
type TaskAlertNotification struct {
	ID           uuid.UUID      `gorm:"type:char(36);primaryKey" json:"id"`
	AlertID      uuid.UUID      `gorm:"type:char(36);index:idx_task_alert_notif_alert;not null" json:"alertId"`
	Channel      string         `gorm:"size:32;index:idx_task_alert_notif_alert;not null" json:"channel"`
	Status       string         `gorm:"size:16;index;not null" json:"status"`
	Target       string         `gorm:"size:512" json:"target,omitempty"`
	SentAt       *time.Time     `json:"sentAt,omitempty"`
	ErrorMessage string         `gorm:"type:text" json:"errorMessage,omitempty"`
	RetryCount   int            `gorm:"default:0" json:"retryCount"`
	RawSummary   datatypes.JSON `gorm:"type:jsonb" json:"rawSummary,omitempty"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
}

func (TaskAlertNotification) TableName() string { return "task_alert_notifications" }
