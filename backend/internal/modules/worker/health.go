package worker

import (
	"context"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/config"
	"gorm.io/gorm"
)

// HealthWorkersBlock is returned under data.workers on /health.
type HealthWorkersBlock struct {
	HeartbeatEnabled bool              `json:"heartbeatEnabled"`
	ReaperEnabled    bool              `json:"reaperEnabled"`
	Running          int               `json:"running"`
	Stale            int               `json:"stale"`
	Stopped          int               `json:"stopped,omitempty"`
	ByType           map[string]TypeStat `json:"byType"`
	Degraded         bool              `json:"degraded,omitempty"`
	Error            string            `json:"error,omitempty"`
}

// TypeStat counts per worker_type using effective status (heartbeat age).
type TypeStat struct {
	Running int `json:"running"`
	Stale   int `json:"stale"`
	Stopped int `json:"stopped,omitempty"`
}

// BuildHealthWorkersBlock queries worker_instances; never panics.
func BuildHealthWorkersBlock(ctx context.Context, db *gorm.DB, cfg *config.Config) HealthWorkersBlock {
	out := HealthWorkersBlock{
		ByType: map[string]TypeStat{
			TypeCollect:   {},
			TypeImage:     {},
			TypeOrderSync: {},
		},
	}
	if cfg != nil {
		out.HeartbeatEnabled = cfg.WorkerHeartbeatEnabled
		out.ReaperEnabled = cfg.WorkerReaperEnabled
	}
	if db == nil {
		out.Degraded = true
		out.Error = "database_unavailable"
		return out
	}
	staleAfter := 30 * time.Second
	if cfg != nil && cfg.WorkerStaleAfterSeconds > 0 {
		staleAfter = time.Duration(cfg.WorkerStaleAfterSeconds) * time.Second
	}
	cut := time.Now().UTC().Add(-staleAfter)

	type row struct {
		WorkerType string
		Status     string
	}
	var rows []Instance
	if err := db.WithContext(ctx).Model(&Instance{}).Find(&rows).Error; err != nil {
		out.Degraded = true
		out.Error = "worker_query_failed"
		return out
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

	countType := func(t, eff string, delta int) {
		ts := out.ByType[t]
		switch eff {
		case StatusRunning:
			ts.Running += delta
		case StatusStale:
			ts.Stale += delta
		case StatusStopped:
			ts.Stopped += delta
		}
		out.ByType[t] = ts
	}

	for _, r := range rows {
		eff := effective(r)
		switch eff {
		case StatusRunning:
			out.Running++
		case StatusStale:
			out.Stale++
		case StatusStopped:
			out.Stopped++
		}
		countType(r.WorkerType, eff, 1)
	}
	return out
}
