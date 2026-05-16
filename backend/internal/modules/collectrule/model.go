package collectrule

import (
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

const SourceCustom = "custom"

const StatusEnabled = "enabled"
const StatusDisabled = "disabled"

// CollectRule stores declarative scraping rules for the custom collector Provider.
type CollectRule struct {
	model.Base
	Name         string         `gorm:"size:255;not null" json:"name"`
	Source       string         `gorm:"size:64;not null;default:custom;index" json:"source"`
	Domain       string         `gorm:"size:512;not null;index" json:"domain"`
	MatchPattern string         `gorm:"type:text" json:"matchPattern,omitempty"`
	Status       string         `gorm:"size:32;not null;index" json:"status"`
	Priority     int            `gorm:"not null;default:100;index" json:"priority"`
	Rule         datatypes.JSON `gorm:"type:jsonb;not null" json:"rule"`
	Remark       string         `gorm:"type:text" json:"remark,omitempty"`
	CreatedBy    *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (CollectRule) TableName() string { return "collect_rules" }
