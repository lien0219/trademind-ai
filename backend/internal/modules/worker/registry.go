package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Registry manages DB-backed worker instance rows and heartbeats.
type Registry struct {
	DB                 *gorm.DB
	OpLog              *operationlog.Service
	HeartbeatEnabled   bool
	HeartbeatInterval  time.Duration
	StaleAfter         time.Duration
	Log                *slog.Logger
}

// RunningInstance is a registered consumer identity; call Stop on shutdown.
type RunningInstance struct {
	reg            *Registry
	rowID          uuid.UUID
	workerID       string
	ephemeral      bool
	stopMu         sync.Mutex
	stopCh         chan struct{}
	heartbeatGroup sync.WaitGroup
}

// WorkerID returns the id used for task lease rows.
func (r *RunningInstance) WorkerID() string {
	if r == nil {
		return ""
	}
	return r.workerID
}

// Stop marks the instance stopped and ends heartbeat.
func (r *RunningInstance) Stop(ctx context.Context) {
	if r == nil {
		return
	}
	r.stopMu.Lock()
	defer r.stopMu.Unlock()
	select {
	case <-r.stopCh:
		return
	default:
		close(r.stopCh)
	}
	r.heartbeatGroup.Wait()
	if r.ephemeral || r.reg == nil || r.reg.DB == nil || r.rowID == uuid.Nil {
		return
	}
	now := time.Now().UTC()
	_ = r.reg.DB.WithContext(ctx).Model(&Instance{}).
		Where("id = ?", r.rowID).
		Updates(map[string]any{
			"status":      StatusStopped,
			"stopped_at":  &now,
			"updated_at":  now,
		}).Error
	if r.reg.OpLog != nil {
		_ = r.reg.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			Action:     ActionStop,
			Resource:   "worker_instance",
			ResourceID: r.rowID.String(),
			Status:     "success",
			Message:    "workerId=" + r.workerID,
		})
	}
}

// NewRegistryFromConfig builds options from application config.
func NewRegistryFromConfig(db *gorm.DB, op *operationlog.Service, cfg *config.Config, log *slog.Logger) *Registry {
	if db == nil || cfg == nil {
		return nil
	}
	r := &Registry{
		DB:                db,
		OpLog:             op,
		HeartbeatEnabled:  cfg.WorkerHeartbeatEnabled,
		HeartbeatInterval: time.Duration(cfg.WorkerHeartbeatIntervalSeconds) * time.Second,
		StaleAfter:        time.Duration(cfg.WorkerStaleAfterSeconds) * time.Second,
		Log:               log,
	}
	if r.HeartbeatInterval <= 0 {
		r.HeartbeatInterval = 10 * time.Second
	}
	if r.StaleAfter <= 0 {
		r.StaleAfter = 30 * time.Second
	}
	return r
}

// Register creates a DB row (when heartbeat enabled) and starts heartbeat.
func (r *Registry) Register(ctx context.Context, workerType, instanceName string, meta map[string]any) *RunningInstance {
	if r == nil {
		return &RunningInstance{workerID: GenerateWorkerID(workerType), ephemeral: true, stopCh: make(chan struct{})}
	}
	wid := GenerateWorkerID(workerType)
	out := &RunningInstance{
		reg:       r,
		workerID:  wid,
		ephemeral: !r.HeartbeatEnabled || r.DB == nil,
		stopCh:    make(chan struct{}),
	}
	if out.ephemeral {
		return out
	}
	host, _ := os.Hostname()
	var metaJSON datatypes.JSON
	if len(meta) > 0 {
		if b, err := json.Marshal(meta); err == nil {
			metaJSON = b
		}
	}
	now := time.Now().UTC()
	row := Instance{
		WorkerID:        wid,
		WorkerType:      workerType,
		InstanceName:    instanceName,
		Hostname:        host,
		PID:             os.Getpid(),
		Status:          StatusRunning,
		LastHeartbeatAt: &now,
		StartedAt:       now,
		Meta:            metaJSON,
	}
	if err := r.DB.WithContext(ctx).Create(&row).Error; err != nil {
		if r.Log != nil {
			r.Log.Warn("worker_instance_register_failed", "workerId", wid, "error", err)
		}
		out.ephemeral = true
		out.rowID = uuid.Nil
		return out
	}
	out.rowID = row.ID
	if r.OpLog != nil {
		_ = r.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			Action:     ActionStart,
			Resource:   "worker_instance",
			ResourceID: row.ID.String(),
			Status:     "success",
			Message:    "workerId=" + wid + " type=" + workerType,
		})
	}
	r.startHeartbeat(ctx, out, row.ID)
	return out
}

func (r *Registry) startHeartbeat(ctx context.Context, inst *RunningInstance, rowID uuid.UUID) {
	if r == nil || inst == nil || r.DB == nil {
		return
	}
	interval := r.HeartbeatInterval
	if interval <= 0 {
		interval = 10 * time.Second
	}
	inst.heartbeatGroup.Add(1)
	go func() {
		defer inst.heartbeatGroup.Done()
		tick := time.NewTicker(interval)
		defer tick.Stop()
		for {
			select {
			case <-inst.stopCh:
				return
			case <-ctx.Done():
				return
			case <-tick.C:
				now := time.Now().UTC()
				res := r.DB.WithContext(context.Background()).Model(&Instance{}).
					Where("id = ? AND status IN ?", rowID, []string{StatusRunning, StatusStale}).
					Updates(map[string]any{
						"last_heartbeat_at": &now,
						"status":            StatusRunning,
						"updated_at":        now,
					})
				if res.Error != nil && r.Log != nil {
					r.Log.Warn("worker_heartbeat_failed", "workerInstanceId", rowID.String(), "error", res.Error)
				}
			}
		}
	}()
}

// MarkStaleWorkers updates DB status for instances that missed heartbeats.
func MarkStaleWorkers(ctx context.Context, db *gorm.DB, staleAfter time.Duration) error {
	if db == nil || staleAfter <= 0 {
		return nil
	}
	cut := time.Now().UTC().Add(-staleAfter)
	return db.WithContext(ctx).Model(&Instance{}).
		Where("status = ? AND (last_heartbeat_at IS NULL OR last_heartbeat_at < ?)", StatusRunning, cut).
		Updates(map[string]any{
			"status":     StatusStale,
			"updated_at": time.Now().UTC(),
		}).Error
}
