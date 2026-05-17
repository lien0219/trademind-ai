package worker

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// Worker instance table: worker_instances

const (
	TypeCollect             = "collect"
	TypeImage               = "image"
	TypeOrderSync           = "order_sync"
	TypeCustomerMessageSync = "customer_message_sync"
	TypeProductPublish      = "product_publish"
	StatusRunning           = "running"
	StatusStale             = "stale"
	StatusStopped           = "stopped"
	ActionStart             = "worker.instance.start"
	ActionStop              = "worker.instance.stop"
	ActionLeaseExp          = "worker.task.lease_expired"
)

// Instance is one registered worker process goroutine (or logical consumer).
type Instance struct {
	model.HardDeleteBase
	WorkerID        string         `gorm:"size:220;uniqueIndex;not null" json:"workerId"`
	WorkerType      string         `gorm:"size:32;index;not null" json:"workerType"`
	InstanceName    string         `gorm:"size:220" json:"instanceName,omitempty"`
	Hostname        string         `gorm:"size:255" json:"hostname,omitempty"`
	PID             int            `gorm:"not null" json:"pid"`
	Status          string         `gorm:"size:32;index;not null" json:"status"`
	LastHeartbeatAt *time.Time     `json:"lastHeartbeatAt,omitempty"`
	StartedAt       time.Time      `json:"startedAt"`
	StoppedAt       *time.Time     `json:"stoppedAt,omitempty"`
	Meta            datatypes.JSON `gorm:"type:jsonb" json:"meta,omitempty"`
}

func (Instance) TableName() string { return "worker_instances" }

// RowID returns primary key for audits.
func (i *Instance) RowID() uuid.UUID {
	if i == nil {
		return uuid.Nil
	}
	return i.ID
}
