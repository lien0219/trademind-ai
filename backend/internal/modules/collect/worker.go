package collect

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

func normalizeCollectConcurrency(n int) int {
	if n < 1 {
		return 1
	}
	if n > 32 {
		return 32
	}
	return n
}

// StartWorker runs BRPOP consumers until ctx is cancelled.
func StartWorker(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service, queueName string, concurrency int) {
	if svc == nil || svc.Redis == nil || svc.Redis.Client == nil {
		return
	}
	if queueName == "" {
		queueName = "collect:tasks"
	}
	concurrency = normalizeCollectConcurrency(concurrency)

	SetCollectWorkersRunning(true)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			runCollectWorker(ctx, log, svc, queueName, workerID)
		}(i + 1)
	}
}

func runCollectWorker(ctx context.Context, log *slog.Logger, svc *Service, queueName string, workerID int) {
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
				log.Warn("collect_worker_bad_message", "worker", workerID, "error", err)
			}
			continue
		}
		tid, err := uuid.Parse(strings.TrimSpace(msg.TaskID))
		if err != nil {
			if log != nil {
				log.Warn("collect_worker_bad_task_id", "worker", workerID, "error", err)
			}
			continue
		}

		// Use a detached context so in-flight Collector calls are not cut off by worker shutdown.
		jobCtx := context.Background()
		svc.RunCollectJob(jobCtx, tid)
	}
}
