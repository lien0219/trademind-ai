package collectbrowserprofile

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

const StatusActive = "active"
const StatusDisabled = "disabled"

const LastCheckLoggedIn = "logged_in"
const LastCheckLoginRequired = "login_required"
const LastCheckVerifyRequired = "verify_required"
const LastCheckUnknown = "unknown"
const LastCheckFailed = "failed"

// CollectBrowserProfile maps to collect_browser_profiles (login state lives in Collector FS only).
type CollectBrowserProfile struct {
	model.Base
	Name            string     `gorm:"size:255;not null" json:"name"`
	Domain          string     `gorm:"size:512;not null;index" json:"domain"`
	ProfileKey      string     `gorm:"size:128;not null;uniqueIndex" json:"profileKey"`
	Provider        string     `gorm:"size:64;index" json:"provider,omitempty"`
	Status          string     `gorm:"size:32;not null;index" json:"status"`
	LastCheckStatus string     `gorm:"size:64" json:"lastCheckStatus,omitempty"`
	LastCheckURL    string     `gorm:"type:text" json:"lastCheckUrl,omitempty"`
	LastCheckAt     *time.Time `json:"lastCheckAt,omitempty"`
	LastErrorCode   string     `gorm:"size:128" json:"lastErrorCode,omitempty"`
	Remark          string     `gorm:"type:text" json:"remark,omitempty"`
	CreatedBy       *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (CollectBrowserProfile) TableName() string { return "collect_browser_profiles" }
