package imagetask

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// StartWorker runs BRPOP consumers until ctx is cancelled.
func StartWorker(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service, queueName string, concurrency int) {
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
		go func(workerID int) {
			defer wg.Done()
			runImageWorker(ctx, log, svc, queueName, workerID)
		}(i + 1)
	}
}

func runImageWorker(ctx context.Context, log *slog.Logger, svc *Service, queueName string, workerID int) {
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
				log.Warn("image_worker_bad_message", "worker", workerID, "error", err)
			}
			continue
		}
		tid, err := uuid.Parse(strings.TrimSpace(msg.TaskID))
		if err != nil {
			if log != nil {
				log.Warn("image_worker_bad_task_id", "worker", workerID, "error", err)
			}
			continue
		}

		jobCtx := context.Background()
		if err := svc.ProcessQueuedTask(jobCtx, tid); err != nil && log != nil {
			log.Warn("image_worker_task_error", "worker", workerID, "taskId", tid.String(), "error", err)
		}
	}
}
