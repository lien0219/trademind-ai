package taskcenter

import (
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// TaskFailureMark records operator intent in the failure center without mutating source task rows.
type TaskFailureMark struct {
	model.HardDeleteBase
	TaskType    string     `gorm:"size:32;uniqueIndex:uniq_task_failure_mark;not null" json:"taskType"`
	SourceID    string     `gorm:"size:64;uniqueIndex:uniq_task_failure_mark;not null" json:"sourceId"`
	SourceTable string     `gorm:"size:64;not null" json:"sourceTable"`
	MarkType    string     `gorm:"size:32;uniqueIndex:uniq_task_failure_mark;not null" json:"markType"`
	Remark      string     `gorm:"type:text" json:"remark,omitempty"`
	CreatedBy   *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (TaskFailureMark) TableName() string { return "task_failure_marks" }
