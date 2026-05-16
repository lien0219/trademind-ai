package worker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"gorm.io/gorm"
)

const monitorLeasedLimit = 100

// MonitorSummary for API envelope.
type MonitorSummary struct {
	Running int `json:"running"`
	Stale   int `json:"stale"`
	Stopped int `json:"stopped"`
}

// MonitorInstanceDTO is one worker_instances row (no secrets).
type MonitorInstanceDTO struct {
	WorkerID         string         `json:"workerId"`
	WorkerType       string         `json:"workerType"`
	InstanceName     string         `json:"instanceName,omitempty"`
	Hostname         string         `json:"hostname,omitempty"`
	PID              int            `json:"pid"`
	Status           string         `json:"status"`
	LastHeartbeatAt  *time.Time     `json:"lastHeartbeatAt,omitempty"`
	StartedAt        time.Time      `json:"startedAt"`
	StoppedAt        *time.Time     `json:"stoppedAt,omitempty"`
	Meta             map[string]any `json:"meta,omitempty"`
	EffectiveStatus  string         `json:"effectiveStatus,omitempty"`
	WorkerInstanceID uuid.UUID      `json:"workerInstanceId,omitempty"`
}

// LeasedTaskDTO minimal lease holder row.
type LeasedTaskDTO struct {
	ID          uuid.UUID  `json:"id"`
	Status      string     `json:"status"`
	LockedBy    *string    `json:"lockedBy,omitempty"`
	LockedUntil *time.Time `json:"lockedUntil,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// MonitorResponse is GET /api/v1/workers/monitor.
type MonitorResponse struct {
	Summary     MonitorSummary             `json:"summary"`
	ByType      map[string]MonitorSummary  `json:"byType"`
	Instances   []MonitorInstanceDTO       `json:"instances"`
	LeasedTasks map[string][]LeasedTaskDTO `json:"leasedTasks"`
}

type leasedCollectTask struct {
	ID          uuid.UUID  `gorm:"column:id"`
	Status      string     `gorm:"column:status"`
	LockedBy    *string    `gorm:"column:locked_by"`
	LockedUntil *time.Time `gorm:"column:locked_until"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`
}

func (leasedCollectTask) TableName() string { return "collect_tasks" }

type leasedImageTask struct {
	ID          uuid.UUID  `gorm:"column:id"`
	Status      string     `gorm:"column:status"`
	LockedBy    *string    `gorm:"column:locked_by"`
	LockedUntil *time.Time `gorm:"column:locked_until"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`
}

func (leasedImageTask) TableName() string { return "image_tasks" }

type leasedOrderSyncTask struct {
	ID          uuid.UUID  `gorm:"column:id"`
	Status      string     `gorm:"column:status"`
	LockedBy    *string    `gorm:"column:locked_by"`
	LockedUntil *time.Time `gorm:"column:locked_until"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`
}

func (leasedOrderSyncTask) TableName() string { return "order_sync_tasks" }

// BuildMonitorResponse loads instances and leased tasks (capped).
func BuildMonitorResponse(ctx context.Context, db *gorm.DB, cfg *config.Config) (*MonitorResponse, error) {
	if db == nil {
		return nil, gorm.ErrInvalidDB
	}
	staleAfter := 30 * time.Second
	if cfg != nil && cfg.WorkerStaleAfterSeconds > 0 {
		staleAfter = time.Duration(cfg.WorkerStaleAfterSeconds) * time.Second
	}
	cut := time.Now().UTC().Add(-staleAfter)

	var rows []Instance
	if err := db.WithContext(ctx).Model(&Instance{}).
		Order("started_at DESC").
		Limit(100).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	effective := func(r Instance) string {
		st := r.Status
		if st == StatusRunning && r.LastHeartbeatAt != nil && r.LastHeartbeatAt.Before(cut) {
			return StatusStale
		}
		if st == StatusRunning && r.LastHeartbeatAt == nil && r.StartedAt.Before(cut) {
			return StatusStale
		}
		return st
	}

	out := &MonitorResponse{
		ByType: map[string]MonitorSummary{
			TypeCollect:   {},
			TypeImage:     {},
			TypeOrderSync: {},
		},
		LeasedTasks: map[string][]LeasedTaskDTO{
			"collect":   {},
			"image":     {},
			"orderSync": {},
		},
	}

	byEff := map[string]MonitorSummary{}

	for _, r := range rows {
		eff := effective(r)
		var meta map[string]any
		if len(r.Meta) > 0 {
			_ = json.Unmarshal(r.Meta, &meta)
		}
		out.Instances = append(out.Instances, MonitorInstanceDTO{
			WorkerInstanceID: r.ID,
			WorkerID:         r.WorkerID,
			WorkerType:       r.WorkerType,
			InstanceName:     r.InstanceName,
			Hostname:         r.Hostname,
			PID:              r.PID,
			Status:           r.Status,
			EffectiveStatus:  eff,
			LastHeartbeatAt:  r.LastHeartbeatAt,
			StartedAt:        r.StartedAt,
			StoppedAt:        r.StoppedAt,
			Meta:             meta,
		})
		switch eff {
		case StatusRunning:
			out.Summary.Running++
		case StatusStale:
			out.Summary.Stale++
		case StatusStopped:
			out.Summary.Stopped++
		}
		ts := byEff[r.WorkerType]
		switch eff {
		case StatusRunning:
			ts.Running++
		case StatusStale:
			ts.Stale++
		case StatusStopped:
			ts.Stopped++
		}
		byEff[r.WorkerType] = ts
	}
	out.ByType = byEff

	var ctasks []leasedCollectTask
	_ = db.WithContext(ctx).Model(&leasedCollectTask{}).
		Where("status = ? AND locked_by IS NOT NULL", "running").
		Order("updated_at DESC").
		Limit(monitorLeasedLimit).
		Find(&ctasks).Error
	for i := range ctasks {
		t := ctasks[i]
		out.LeasedTasks["collect"] = append(out.LeasedTasks["collect"], LeasedTaskDTO{
			ID:          t.ID,
			Status:      t.Status,
			LockedBy:    t.LockedBy,
			LockedUntil: t.LockedUntil,
			CreatedAt:   t.CreatedAt,
			UpdatedAt:   t.UpdatedAt,
		})
	}

	var itasks []leasedImageTask
	_ = db.WithContext(ctx).Model(&leasedImageTask{}).
		Where("status = ? AND locked_by IS NOT NULL", "running").
		Order("updated_at DESC").
		Limit(monitorLeasedLimit).
		Find(&itasks).Error
	for i := range itasks {
		t := itasks[i]
		out.LeasedTasks["image"] = append(out.LeasedTasks["image"], LeasedTaskDTO{
			ID:          t.ID,
			Status:      t.Status,
			LockedBy:    t.LockedBy,
			LockedUntil: t.LockedUntil,
			CreatedAt:   t.CreatedAt,
			UpdatedAt:   t.UpdatedAt,
		})
	}

	var otasks []leasedOrderSyncTask
	_ = db.WithContext(ctx).Model(&leasedOrderSyncTask{}).
		Where("status = ? AND locked_by IS NOT NULL", "running").
		Order("updated_at DESC").
		Limit(monitorLeasedLimit).
		Find(&otasks).Error
	for i := range otasks {
		t := otasks[i]
		out.LeasedTasks["orderSync"] = append(out.LeasedTasks["orderSync"], LeasedTaskDTO{
			ID:          t.ID,
			Status:      t.Status,
			LockedBy:    t.LockedBy,
			LockedUntil: t.LockedUntil,
			CreatedAt:   t.CreatedAt,
			UpdatedAt:   t.UpdatedAt,
		})
	}

	return out, nil
}
