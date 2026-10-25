package inventory

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
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
	"github.com/trademind-ai/trademind/backend/internal/pkg/taskqueue"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasktenant"
	"gorm.io/gorm"
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

	group, err := taskqueue.RegisterConsumerGroup(ctx, svc.Redis.Client, queueName)
	if err != nil {
		if log != nil {
			log.Warn("inventory_sync_consumer_group_register_failed", "error", err)
		}
		return
	}
	if _, err := group.RecoverIfIdle(ctx); err != nil && log != nil {
		log.Warn("inventory_sync_queue_recovery_failed", "error", err)
	}
	var slots sync.WaitGroup
	slots.Add(1) // registration barrier; prevents cleanup from racing the loop below
	wg.Add(1)
	go func() {
		defer wg.Done()
		slots.Wait()
		if err := group.Stop(context.Background()); err != nil && log != nil {
			log.Warn("inventory_sync_consumer_group_stop_failed", "error", err)
		}
	}()

	for i := 0; i < concurrency; i++ {
		consumer, err := group.RegisterSlot(ctx)
		if err != nil {
			if log != nil {
				log.Warn("inventory_sync_consumer_slot_register_failed", "worker", i+1, "error", err)
			}
			continue
		}
		slots.Add(1)
		wg.Add(1)
		go func(slot int, consumer *taskqueue.ConsumerSlot) {
			defer wg.Done()
			defer slots.Done()
			defer func() {
				if err := consumer.Stop(context.Background()); err != nil && log != nil {
					log.Warn("inventory_sync_consumer_slot_stop_failed", "worker", slot, "error", err)
				}
			}()
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
			runInventorySyncWorker(ctx, log, svc, queueName, slot, wid, consumer.ID())
		}(i+1, consumer)
	}
	slots.Done()
}

func runInventorySyncWorker(ctx context.Context, log *slog.Logger, svc *Service, queueName string, slot int, workerLeaseID, consumerID string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		payload, err := taskqueue.Reserve(ctx, svc.Redis.Client, queueName, consumerID, 5*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		ack := func() {
			if err := taskqueue.Ack(ctx, svc.Redis.Client, queueName, consumerID, payload); err != nil && log != nil {
				log.Warn("inventory_sync_worker_ack_failed", "worker", slot, "error", err)
			}
		}
		requeue := func(cause error) {
			if err := taskqueue.Requeue(ctx, svc.Redis.Client, queueName, consumerID, payload); err != nil && log != nil {
				log.Error("inventory_sync_worker_requeue_failed", "worker", slot, "cause", cause, "error", err)
			} else if log != nil {
				log.Warn("inventory_sync_worker_requeued", "worker", slot, "cause", cause)
			}
		}

		var msg QueueMessage
		if err := json.Unmarshal([]byte(payload), &msg); err != nil {
			if log != nil {
				log.Warn("inventory_sync_worker_bad_message", "worker", slot, "error", err)
			}
			ack()
			continue
		}
		tid, err := uuid.Parse(strings.TrimSpace(msg.TaskID))
		if err != nil {
			if log != nil {
				log.Warn("inventory_sync_worker_bad_task_id", "worker", slot, "error", err)
			}
			ack()
			continue
		}

		jobCtx := ctx
		plat := ""
		var shopID uuid.UUID
		if svc.DB == nil {
			if log != nil {
				log.Warn("inventory_sync_worker_tenant_missing", "worker", slot, "taskId", tid.String(), "error", "database unavailable")
			}
			requeue(errors.New("database unavailable"))
			continue
		}
		{
			var probe InventorySyncTask
			if err := svc.DB.WithContext(jobCtx).Select("platform, shop_id, tenant_id").First(&probe, "id = ?", tid).Error; err != nil {
				if log != nil {
					log.Warn("inventory_sync_worker_tenant_missing", "worker", slot, "taskId", tid.String(), "error", err)
				}
				if errors.Is(err, gorm.ErrRecordNotFound) {
					ack()
				} else {
					requeue(err)
				}
				continue
			}
			plat = strings.TrimSpace(strings.ToLower(probe.Platform))
			shopID = probe.ShopID
			wctx, _, terr := tasktenant.BeginWorker(jobCtx, svc.DB, probe.TenantID, shopID, "inventory_sync")
			if terr != nil {
				if tasktenant.IsPermanentWorkerScopeError(terr) {
					if err := tasktenant.RejectUnstartedTask(jobCtx, svc.DB.Model(&InventorySyncTask{}).Where("id = ? AND tenant_id = ?", tid, probe.TenantID), terr, nil); err != nil {
						if log != nil {
							log.Error("inventory_sync_worker_rejection_failed", "taskId", tid.String(), "error", err)
						}
						requeue(err)
					} else {
						ack()
					}
				} else {
					requeue(terr)
				}
				if log != nil {
					log.Warn("inventory_sync_worker_tenant_missing", "worker", slot, "taskId", tid.String(), "error", tasktenant.WrapError(terr))
				}
				continue
			}
			jobCtx = wctx
		}
		deferRate, rerr := svc.InventoryRateDefer(jobCtx, plat)
		if rerr == nil && deferRate && svc.Redis != nil && svc.Redis.Client != nil {
			deferCount := svc.inventoryRateDeferCount(jobCtx, tid)
			if deferCount >= 5 {
				if log != nil {
					log.Warn("inventory_sync_rate_limit_give_up", "worker", slot, "taskId", tid.String(), "platform", plat)
				}
				if err := svc.markInventoryRateDeferFailed(jobCtx, tid, "inventory sync deferred too many times due to rate limit"); err != nil {
					requeue(err)
				} else {
					ack()
				}
				continue
			}
			svc.inventoryRateDeferIncr(jobCtx, tid)
			timer := time.NewTimer(2 * time.Second)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return
			case <-timer.C:
			}
			requeue(errors.New("inventory sync deferred by rate limit"))
			if log != nil {
				log.Warn("inventory_sync_rate_limit_deferred", "worker", slot, "taskId", tid.String(), "platform", plat)
			}
			continue
		}

		processErr := svc.ProcessQueuedTask(jobCtx, tid, workerLeaseID)
		if processErr != nil {
			if log != nil {
				log.Warn("inventory_sync_worker_task_error", "worker", slot, "taskId", tid.String(), "error", processErr)
			}
		}
		var state InventorySyncTask
		if qerr := svc.DB.WithContext(jobCtx).Select("status").First(&state, "id = ?", tid).Error; qerr != nil {
			requeue(qerr)
			continue
		}
		if state.Status == StatusPending || state.Status == StatusRunning {
			if processErr == nil {
				processErr = errors.New("inventory sync task claim is still active")
			}
			requeue(processErr)
			continue
		}
		ack()
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
