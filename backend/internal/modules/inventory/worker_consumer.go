package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
)

// StartWorker runs Redis BRPOP consumers until ctx cancelled.
func StartWorker(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service, queueName string, concurrency int, reg *worker.Registry) {
	if svc == nil || svc.Redis == nil || svc.Redis.Client == nil {
		return
	}
	if queueName == "" {
		queueName = "inventory:sync:tasks"
	}
	concurrency = normalizeConcurrency(concurrency)
	SetInventorySyncWorkersRunning(true)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			var wid string
			if reg != nil {
				inst := reg.Register(ctx, worker.TypeInventorySync, fmt.Sprintf("inventory-sync-%d", slot), map[string]any{"queue": queueName})
				if inst != nil {
					defer inst.Stop(context.Background())
					wid = inst.WorkerID()
				}
			}
			if wid == "" {
				wid = worker.GenerateWorkerID(worker.TypeInventorySync)
			}
			runInventorySyncWorker(ctx, log, svc, queueName, slot, wid)
		}(i + 1)
	}
}

func runInventorySyncWorker(ctx context.Context, log *slog.Logger, svc *Service, queueName string, slot int, workerLeaseID string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		res, err := svc.Redis.BRPop(ctx, 5*time.Second, queueName).Result()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		if len(res) < 2 {
			continue
		}
		payload := res[1]

		var msg QueueMessage
		if err := json.Unmarshal([]byte(payload), &msg); err != nil {
			if log != nil {
				log.Warn("inventory_sync_worker_bad_message", "worker", slot, "error", err)
			}
			continue
		}
		tid, err := uuid.Parse(strings.TrimSpace(msg.TaskID))
		if err != nil {
			if log != nil {
				log.Warn("inventory_sync_worker_bad_task_id", "worker", slot, "error", err)
			}
			continue
		}

		jobCtx := context.Background()
		plat := ""
		if svc.DB != nil {
			var probe InventorySyncTask
			if err := svc.DB.WithContext(jobCtx).Select("platform").First(&probe, "id = ?", tid).Error; err == nil {
				plat = strings.TrimSpace(strings.ToLower(probe.Platform))
			}
		}
		deferRate, rerr := svc.InventoryRateDefer(jobCtx, plat)
		if rerr == nil && deferRate && svc.Redis != nil && svc.Redis.Client != nil {
			_ = svc.Redis.RPush(ctx, queueName, payload).Err()
			if log != nil {
				log.Warn("inventory_sync_rate_limit_deferred", "worker", slot, "taskId", tid.String(), "platform", plat)
			}
			continue
		}

		if err := svc.ProcessQueuedTask(jobCtx, tid, workerLeaseID); err != nil && log != nil {
			log.Warn("inventory_sync_worker_task_error", "worker", slot, "taskId", tid.String(), "error", err)
		}
	}
}

func normalizeConcurrency(n int) int {
	if n < 1 {
		return 1
	}
	if n > 32 {
		return 32
	}
	return n
}
