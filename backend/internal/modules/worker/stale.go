package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/config"
	"gorm.io/gorm"
)

// StartStaleMarker periodically marks worker instances as stale in DB.
func StartStaleMarker(ctx context.Context, wg *sync.WaitGroup, db *gorm.DB, cfg *config.Config, log *slog.Logger) {
	if wg == nil || db == nil || cfg == nil || !cfg.WorkerHeartbeatEnabled {
		return
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		iv := time.Duration(cfg.WorkerStaleAfterSeconds) * time.Second
		if iv < 10*time.Second {
			iv = 30 * time.Second
		}
		tick := time.NewTicker(iv)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				sa := time.Duration(cfg.WorkerStaleAfterSeconds) * time.Second
				if sa <= 0 {
					sa = 30 * time.Second
				}
				if err := MarkStaleWorkers(context.Background(), db, sa); err != nil && log != nil {
					log.Warn("worker_stale_marker_failed", "error", err)
				}
			}
		}
	}()
}
