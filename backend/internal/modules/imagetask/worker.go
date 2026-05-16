package imagetask

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

// StartWorker runs BRPOP consumers until ctx is cancelled.
func StartWorker(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service, queueName string, concurrency int, reg *worker.Registry) {
	if svc == nil || svc.Redis == nil || svc.Redis.Client == nil {
		return
	}
	if queueName == "" {
		queueName = "image:tasks"
	}
	concurrency = normalizeImageWorkerConcurrency(concurrency)

	SetImageWorkersRunning(true)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			var wid string
			if reg != nil {
				inst := reg.Register(ctx, worker.TypeImage, fmt.Sprintf("image-%d", slot), map[string]any{"queue": queueName})
				if inst != nil {
					defer inst.Stop(context.Background())
					wid = inst.WorkerID()
				}
			}
			if wid == "" {
				wid = worker.GenerateWorkerID(worker.TypeImage)
			}
			runImageWorker(ctx, log, svc, queueName, slot, wid)
		}(i + 1)
	}
}

func runImageWorker(ctx context.Context, log *slog.Logger, svc *Service, queueName string, slot int, workerLeaseID string) {
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

		var msg ImageQueueMessage
		if err := json.Unmarshal([]byte(payload), &msg); err != nil {
			if log != nil {
				log.Warn("image_worker_bad_message", "worker", slot, "error", err)
			}
			continue
		}
		tid, err := uuid.Parse(strings.TrimSpace(msg.TaskID))
		if err != nil {
			if log != nil {
				log.Warn("image_worker_bad_task_id", "worker", slot, "error", err)
			}
			continue
		}

		jobCtx := context.Background()
		if err := svc.ProcessQueuedTask(jobCtx, tid, workerLeaseID); err != nil && log != nil {
			log.Warn("image_worker_task_error", "worker", slot, "taskId", tid.String(), "error", err)
		}
	}
}
