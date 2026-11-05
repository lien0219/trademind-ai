package customersync

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

// StartWorker runs BRPOP consumers until ctx is cancelled.
func StartWorker(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service, queueName string, concurrency int, reg *worker.Registry) {
	if svc == nil || svc.Redis == nil || svc.Redis.Client == nil || svc.DB == nil {
		if log != nil {
			log.Warn("customer_message_sync_worker_skipped", "reason", "worker dependencies unavailable")
		}
		return
	}
	if queueName == "" {
		queueName = "customer:message:sync:tasks"
	}
	concurrency = normalizeConcurrency(concurrency)
	SetCustomerMessageSyncWorkersRunning(true)

	group, err := taskqueue.RegisterConsumerGroup(ctx, svc.Redis.Client, queueName)
	if err != nil {
		if log != nil {
			log.Warn("customer_message_sync_consumer_group_register_failed", "error", err)
		}
		return
	}
	if _, err := group.RecoverIfIdle(ctx); err != nil && log != nil {
		log.Warn("customer_message_sync_queue_recovery_failed", "error", err)
	}
	var slots sync.WaitGroup
	slots.Add(1) // registration barrier; prevents cleanup from racing the loop below
	wg.Add(1)
	go func() {
		defer wg.Done()
		slots.Wait()
		if err := group.Stop(context.Background()); err != nil && log != nil {
			log.Warn("customer_message_sync_consumer_group_stop_failed", "error", err)
		}
	}()

	for i := 0; i < concurrency; i++ {
		consumer, err := group.RegisterSlot(ctx)
		if err != nil {
			if log != nil {
				log.Warn("customer_message_sync_consumer_slot_register_failed", "worker", i+1, "error", err)
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
					log.Warn("customer_message_sync_consumer_slot_stop_failed", "worker", slot, "error", err)
				}
			}()
			var wid string
			if reg != nil {
				inst := reg.Register(ctx, worker.TypeCustomerMessageSync, fmt.Sprintf("customer-msg-sync-%d", slot), map[string]any{"queue": queueName})
				if inst != nil {
					defer inst.Stop(context.Background())
					wid = inst.WorkerID()
				}
			}
			if wid == "" {
				wid = worker.GenerateWorkerID(worker.TypeCustomerMessageSync)
			}
			runWorker(ctx, log, svc, queueName, slot, wid, consumer.ID())
		}(i+1, consumer)
	}
	slots.Done()
}

func runWorker(ctx context.Context, log *slog.Logger, svc *Service, queueName string, slot int, workerLeaseID, consumerID string) {
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
				log.Warn("customer_message_sync_worker_ack_failed", "worker", slot, "error", err)
			}
		}
		requeue := func(cause error) {
			if err := taskqueue.Requeue(ctx, svc.Redis.Client, queueName, consumerID, payload); err != nil && log != nil {
				log.Error("customer_message_sync_worker_requeue_failed", "worker", slot, "cause", cause, "error", err)
			} else if log != nil {
				log.Warn("customer_message_sync_worker_requeued", "worker", slot, "cause", cause)
			}
		}

		var msg QueueMessage
		if err := json.Unmarshal([]byte(payload), &msg); err != nil {
			if log != nil {
				log.Warn("customer_message_sync_worker_bad_message", "worker", slot, "error", err)
			}
			ack()
			continue
		}
		tid, err := uuid.Parse(strings.TrimSpace(msg.TaskID))
		if err != nil {
			if log != nil {
				log.Warn("customer_message_sync_worker_bad_task_id", "worker", slot, "error", err)
			}
			ack()
			continue
		}

		jobCtx := ctx
		if svc.DB == nil {
			if log != nil {
				log.Warn("customer_sync_worker_tenant_missing", "worker", slot, "taskId", tid.String(), "error", "database unavailable")
			}
			requeue(errors.New("database unavailable"))
			continue
		}
		{
			var probe CustomerMessageSyncTask
			if err := svc.DB.WithContext(jobCtx).Select("shop_id, tenant_id").First(&probe, "id = ?", tid).Error; err != nil {
				if log != nil {
					log.Warn("customer_sync_worker_tenant_missing", "worker", slot, "taskId", tid.String(), "error", err)
				}
				if errors.Is(err, gorm.ErrRecordNotFound) {
					ack()
				} else {
					requeue(err)
				}
				continue
			}
			wctx, _, terr := tasktenant.BeginWorkerWithLegacyZero(jobCtx, svc.DB, probe.TenantID, probe.ShopID, "customer_message_sync", svc.AllowLegacyTenantZero)
			if terr != nil {
				if tasktenant.IsPermanentWorkerScopeError(terr) {
					if rejectionErr := tasktenant.RejectUnstartedTask(jobCtx, svc.DB.Model(&CustomerMessageSyncTask{}).Where("id = ? AND tenant_id = ?", tid, probe.TenantID), terr, nil); rejectionErr != nil {
						if log != nil {
							log.Error("customer_message_worker_rejection_failed", "taskId", tid.String(), "error", rejectionErr)
						}
						requeue(rejectionErr)
					} else {
						ack()
					}
				} else {
					requeue(terr)
				}
				if log != nil {
					log.Warn("customer_sync_worker_tenant_missing", "worker", slot, "taskId", tid.String(), "error", tasktenant.WrapError(terr))
				}
				continue
			}
			jobCtx = wctx
		}
		processErr := svc.ProcessQueuedTask(jobCtx, tid, workerLeaseID)
		if processErr != nil {
			if log != nil {
				log.Warn("customer_message_sync_worker_task_error", "worker", slot, "taskId", tid.String(), "error", processErr)
			}
		}
		var state CustomerMessageSyncTask
		if queryErr := svc.DB.WithContext(jobCtx).Select("status").First(&state, "id = ?", tid).Error; queryErr != nil {
			requeue(queryErr)
			continue
		}
		if state.Status == StatusPending || state.Status == StatusRunning {
			if processErr != nil {
				requeue(processErr)
			} else if log != nil {
				log.Warn("customer_message_sync_worker_task_nonterminal", "worker", slot, "taskId", tid.String(), "status", state.Status)
			}
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
