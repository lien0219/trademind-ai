package taskreaper

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"gorm.io/gorm"
)

// Deps wires module services for reclaiming expired leases.
type Deps struct {
	Log    *slog.Logger
	DB     *gorm.DB
	Config *config.Config

	Collect *collect.Service
	Image   *imagetask.Service
	Order   *ordersync.Service
}

// Start launches the periodic reaper until ctx is cancelled.
func Start(ctx context.Context, wg *sync.WaitGroup, d Deps) {
	if d.Config != nil && !d.Config.WorkerReaperEnabled {
		return
	}
	if wg != nil {
		wg.Add(1)
	}
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		interval := 15 * time.Second
		if d.Config != nil && d.Config.WorkerReaperIntervalSeconds > 0 {
			interval = time.Duration(d.Config.WorkerReaperIntervalSeconds) * time.Second
		}
		legacy := 30 * time.Minute
		if d.Config != nil && d.Config.WorkerLegacyRunningTimeoutSeconds > 0 {
			legacy = time.Duration(d.Config.WorkerLegacyRunningTimeoutSeconds) * time.Second
		}
		tick := time.NewTicker(interval)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				runOnce(context.Background(), d, legacy)
			}
		}
	}()
}

func runOnce(ctx context.Context, d Deps, legacyTimeout time.Duration) {
	if d.DB == nil {
		return
	}
	now := time.Now().UTC()
	legacyCut := now.Add(-legacyTimeout)

	if d.Collect != nil {
		var ids []string
		_ = d.DB.WithContext(ctx).Model(&collect.CollectTask{}).
			Where("status = ? AND locked_until IS NOT NULL AND locked_until < ?", collect.StatusRunning, now).
			Limit(50).
			Pluck("id", &ids).Error
		for _, sid := range ids {
			if err := d.Collect.RecoverLeaseExpired(ctx, parseUUID(sid)); err != nil && d.Log != nil {
				d.Log.Warn("taskreaper_collect_lease", "taskId", sid, "error", err)
			}
		}
	}

	if d.Image != nil {
		var ids []string
		_ = d.DB.WithContext(ctx).Model(&imagetask.ImageTask{}).
			Where("status = ? AND locked_until IS NOT NULL AND locked_until < ?", imagetask.StatusRunning, now).
			Limit(50).
			Pluck("id", &ids).Error
		for _, sid := range ids {
			if err := d.Image.RecoverLeaseExpired(ctx, parseUUID(sid)); err != nil && d.Log != nil {
				d.Log.Warn("taskreaper_image_lease", "taskId", sid, "error", err)
			}
		}
	}

	if d.Order != nil {
		var ids []string
		_ = d.DB.WithContext(ctx).Model(&ordersync.OrderSyncTask{}).
			Where("status = ? AND locked_until IS NOT NULL AND locked_until < ?", ordersync.StatusRunning, now).
			Limit(50).
			Pluck("id", &ids).Error
		for _, sid := range ids {
			if err := d.Order.RecoverLeaseExpired(ctx, parseUUID(sid)); err != nil && d.Log != nil {
				d.Log.Warn("taskreaper_order_sync_lease", "taskId", sid, "error", err)
			}
		}
	}

	if d.Collect != nil {
		var ids []string
		_ = d.DB.WithContext(ctx).Model(&collect.CollectTask{}).
			Where("status = ? AND locked_by IS NULL AND updated_at < ?", collect.StatusRunning, legacyCut).
			Limit(50).
			Pluck("id", &ids).Error
		for _, sid := range ids {
			if err := d.Collect.RecoverLegacyRunning(ctx, parseUUID(sid), legacyCut); err != nil && d.Log != nil {
				d.Log.Warn("taskreaper_collect_legacy", "taskId", sid, "error", err)
			}
		}
	}

	if d.Image != nil {
		var ids []string
		_ = d.DB.WithContext(ctx).Model(&imagetask.ImageTask{}).
			Where("status = ? AND locked_by IS NULL AND updated_at < ?", imagetask.StatusRunning, legacyCut).
			Limit(50).
			Pluck("id", &ids).Error
		for _, sid := range ids {
			if err := d.Image.RecoverLegacyRunning(ctx, parseUUID(sid), legacyCut); err != nil && d.Log != nil {
				d.Log.Warn("taskreaper_image_legacy", "taskId", sid, "error", err)
			}
		}
	}

	if d.Order != nil {
		var ids []string
		_ = d.DB.WithContext(ctx).Model(&ordersync.OrderSyncTask{}).
			Where("status = ? AND locked_by IS NULL AND updated_at < ?", ordersync.StatusRunning, legacyCut).
			Limit(50).
			Pluck("id", &ids).Error
		for _, sid := range ids {
			if err := d.Order.RecoverLegacyRunning(ctx, parseUUID(sid), legacyCut); err != nil && d.Log != nil {
				d.Log.Warn("taskreaper_order_sync_legacy", "taskId", sid, "error", err)
			}
		}
	}
}

func parseUUID(s string) uuid.UUID {
	u, _ := uuid.Parse(s)
	return u
}
