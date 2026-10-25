package collect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasktenant"
	"gorm.io/gorm"
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

// StartWorker runs reliable ready-to-processing consumers until ctx is cancelled.
func StartWorker(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service, queueName string, concurrency int, reg *worker.Registry) {
	if svc == nil || svc.Redis == nil || svc.Redis.Client == nil || svc.DB == nil || svc.Client == nil || svc.Products == nil {
		if log != nil {
			log.Warn("collect_worker_skipped", "reason", "worker dependencies unavailable")
		}
		return
	}
	queueName = collectQueueName(queueName)
	concurrency = normalizeCollectConcurrency(concurrency)

	if err := svc.recoverCollectQueue(ctx, queueName, log); err != nil && log != nil {
		log.Warn("collect_queue_recovery_failed", "error", err)
	}
	startCollectQueueRecovery(ctx, wg, log, svc, queueName, time.Minute)
	SetCollectWorkersRunning(true)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			var wid string
			if reg != nil {
				inst := reg.Register(ctx, worker.TypeCollect, fmt.Sprintf("collect-%d", slot), map[string]any{"queue": queueName})
				if inst != nil {
					defer inst.Stop(context.Background())
					wid = inst.WorkerID()
				}
			}
			if wid == "" {
				wid = worker.GenerateWorkerID(worker.TypeCollect)
			}
			runCollectWorker(ctx, log, svc, queueName, slot, wid)
		}(i + 1)
	}
}

func startCollectQueueRecovery(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service, queueName string, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := svc.recoverCollectQueue(ctx, queueName, log); err != nil && log != nil {
					log.Warn("collect_queue_recovery_failed", "error", err)
				}
			}
		}
	}()
}

func runCollectWorker(ctx context.Context, log *slog.Logger, svc *Service, queueName string, slot int, workerLeaseID string) {
	var lastReserveErrorLog time.Time
	for {
		payload, err := svc.reserveTask(ctx, queueName, 5*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if !errors.Is(err, goredis.Nil) {
				if log != nil && (lastReserveErrorLog.IsZero() || time.Since(lastReserveErrorLog) >= 30*time.Second) {
					log.Warn("collect_worker_reserve_failed", "worker", slot, "queue", queueName, "error", err)
					lastReserveErrorLog = time.Now()
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Second):
				}
			}
			continue
		}
		processReservedCollectTask(ctx, log, svc, queueName, payload, slot, workerLeaseID)
	}
}

func processReservedCollectTask(ctx context.Context, log *slog.Logger, svc *Service, queueName, payload string, slot int, workerLeaseID string) {
	jobCtx := ctx
	ack := func() {
		if err := svc.ackReservedTask(jobCtx, queueName, payload); err != nil && log != nil {
			log.Warn("collect_worker_ack_failed", "worker", slot, "error", err)
		}
	}
	requeue := func(cause error) {
		if err := svc.requeueReservedTask(jobCtx, queueName, payload); err != nil {
			if log != nil {
				log.Error("collect_worker_requeue_failed", "worker", slot, "cause", cause, "error", err)
			}
			return
		}
		if log != nil {
			log.Warn("collect_worker_requeued", "worker", slot, "cause", cause)
		}
	}
	requeueClaimMiss := func(cause error) {
		timer := time.NewTimer(time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			requeue(cause)
		}
	}

	var msg QueueMessage
	if err := json.Unmarshal([]byte(payload), &msg); err != nil {
		if log != nil {
			log.Warn("collect_worker_bad_message", "worker", slot, "error", err)
		}
		ack()
		return
	}
	tid, err := uuid.Parse(strings.TrimSpace(msg.TaskID))
	if err != nil {
		if log != nil {
			log.Warn("collect_worker_bad_task_id", "worker", slot, "error", err)
		}
		ack()
		return
	}

	var task CollectTask
	if err := svc.DB.WithContext(jobCtx).First(&task, "id = ?", tid).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if log != nil {
				log.Warn("collect_worker_task_not_found", "worker", slot, "taskId", tid.String())
			}
			ack()
			return
		}
		requeue(err)
		return
	}
	if task.Status != StatusPending && !(task.Status == StatusRetrying && task.NextRetryAt == nil) {
		if task.Status == StatusRunning {
			requeueClaimMiss(errors.New("collect task claim is still active"))
		} else {
			ack()
		}
		return
	}
	if !svc.acceptsTenantID(task.TenantID) {
		if err := svc.failTaskMissingTenant(jobCtx, &task); err != nil {
			requeue(err)
			return
		}
		if log != nil {
			log.Warn("collect_worker_tenant_missing", "worker", slot, "taskId", tid.String(), "error", tasktenant.WrapError(tasktenant.RequireTaskTenant(task.TenantID)))
		}
		ack()
		return
	}

	wctx, _, err := tasktenant.BeginWorkerWithLegacyZero(jobCtx, svc.DB, task.TenantID, uuid.Nil, "collect", svc.AllowLegacyTenantZero)
	if err != nil {
		requeue(err)
		return
	}
	claimed, err := svc.RunCollectJob(wctx, tid, workerLeaseID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ack()
			return
		}
		requeue(err)
		return
	}
	if !claimed {
		var current CollectTask
		if qerr := svc.DB.WithContext(jobCtx).Select("status", "next_retry_at").First(&current, "id = ?", tid).Error; qerr != nil {
			requeue(qerr)
			return
		}
		if current.Status == StatusPending || current.Status == StatusRunning || (current.Status == StatusRetrying && current.NextRetryAt == nil) {
			requeueClaimMiss(errors.New("collect task claim is still active"))
			return
		}
	}
	ack()
}
