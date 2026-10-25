package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/pkg/runtimediag"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasktenant"
	"gorm.io/gorm"
)

// ProcessEventByRowID processes one durable webhook event with tenant + shop scope.
func (s *Service) ProcessEventByRowID(ctx context.Context, tenantID int64, webhookEventRowID uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("webhook: unavailable")
	}
	if err := tasktenant.RequireTaskTenant(tenantID); err != nil {
		return err
	}
	if webhookEventRowID == uuid.Nil {
		return fmt.Errorf("webhook event id required")
	}
	var ev Event
	if err := s.DB.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", webhookEventRowID, tenantID).
		First(&ev).Error; err != nil {
		return err
	}
	wctx, err := s.beginEventWorker(ctx, &ev)
	if err != nil {
		return err
	}
	return s.processEventRow(wctx, &ev)
}

// ProcessEvent runs async business handling for one durable webhook event (tenant-aware when possible).
func (s *Service) ProcessEvent(ctx context.Context, tenantID int64, internalShopID uuid.UUID, platform, platformShopID, eventID string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("webhook: unavailable")
	}
	platform = strings.TrimSpace(platform)
	eventID = strings.TrimSpace(eventID)
	if platform == "" || eventID == "" {
		return fmt.Errorf("platform and eventId required")
	}
	if err := tasktenant.RequireTaskTenant(tenantID); err != nil {
		return err
	}

	var ev Event
	q := s.DB.WithContext(ctx).Where("platform = ? AND event_id = ? AND tenant_id = ?", platform, eventID, tenantID)
	if internalShopID != uuid.Nil {
		q = q.Where("internal_shop_id = ?", internalShopID)
	}
	if platformShopID != "" {
		q = q.Where("platform_shop_id = ?", platformShopID)
	}
	if err := q.First(&ev).Error; err != nil {
		return err
	}
	wctx, err := s.beginEventWorker(ctx, &ev)
	if err != nil {
		return err
	}
	return s.processEventRow(wctx, &ev)
}

// ProcessEventByID processes one already selected event row. Workers use this
// path so identical eventId values from different shops cannot resolve to the
// wrong row.
func (s *Service) ProcessEventByID(ctx context.Context, id uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("webhook: unavailable")
	}
	if id == uuid.Nil {
		return fmt.Errorf("webhook event id required")
	}
	var ev Event
	if err := s.DB.WithContext(ctx).First(&ev, "id = ?", id).Error; err != nil {
		return err
	}
	wctx, err := s.beginEventWorker(ctx, &ev)
	if err != nil {
		return err
	}
	return s.processEventRow(wctx, &ev)
}

func (s *Service) beginEventWorker(ctx context.Context, ev *Event) (context.Context, error) {
	if ev == nil {
		return ctx, fmt.Errorf("webhook event required")
	}
	shopID := uuid.Nil
	if ev.InternalShopID != nil {
		shopID = *ev.InternalShopID
	}
	wctx, _, err := tasktenant.BeginWorker(ctx, s.DB, ev.TenantID, shopID, "webhook_process")
	return wctx, err
}

func (s *Service) processEventRow(ctx context.Context, ev *Event) error {
	if ev == nil {
		return nil
	}
	start := time.Now()
	if ev.Status == StatusProcessed || ev.Status == StatusIgnored || ev.Status == StatusDuplicate {
		return nil
	}

	key := webhookProcessKey(ev)
	owner := "webhook-process"
	var idemJob *webhookAcquire
	if s.Idempotency != nil {
		reqHash := idempotency.HashRequest([]byte(ev.PayloadHash))
		res, acqErr := s.Idempotency.Acquire(ctx, idempotency.ScopeWebhook, key, reqHash, owner, idempotency.DefaultLease)
		decision, rec, _ := idempotency.Classify(res, acqErr)
		switch decision {
		case idempotency.DecisionAlreadySucceeded:
			return nil
		case idempotency.DecisionInProgress:
			return nil
		case idempotency.DecisionKeyConflict, idempotency.DecisionPermanentFailure:
			return newCodeError(CodeKeyConflict, 409, CodeKeyConflict)
		case idempotency.DecisionAcquired, idempotency.DecisionRetryAllowed:
			if rec == nil && res != nil {
				rec = res.Record
			}
			if rec != nil {
				idemJob = &webhookAcquire{RecordID: rec.ID, Owner: owner}
			}
		default:
			if acqErr != nil {
				return acqErr
			}
		}
	}

	now := s.now()
	// Ingest ACK path has no enveloping business transaction; processor claim/update are discrete statements.
	// Emit transaction_* / inventory_update / task_enqueue only when real code paths exist (not forged).
	claim := s.DB.WithContext(ctx).Model(&Event{}).
		Where("id = ? AND status IN ?", ev.ID, []string{StatusQueued, StatusReceived, StatusFailedRetryable}).
		Updates(map[string]any{
			"status":     StatusProcessing,
			"updated_at": now,
		})
	if claim.Error != nil {
		s.failProcess(ctx, idemJob, "WEBHOOK_CLAIM_FAILED", true)
		return claim.Error
	}
	if claim.RowsAffected == 0 {
		if idemJob != nil && s.Idempotency != nil {
			_ = s.Idempotency.Complete(ctx, idemJob.RecordID, idemJob.Owner, idempotency.CompleteResult{
				ResponseCode: "WEBHOOK_ALREADY_CLAIMED",
				ResourceType: "webhook_event",
				ResourceID:   ev.EventID,
			})
		}
		return nil
	}

	businessStart := time.Now()
	businessApplicable := ev.Platform == "douyin_shop" || ev.Platform == "douyin"
	if err := s.handlePlatformEvent(ctx, ev); err != nil {
		if businessApplicable {
			runtimediag.ObserveStage(runtimediag.RouteWebhookIngestion, "business_upsert", runtimediag.OutcomeError, businessStart)
			runtimediag.ObserveStage(runtimediag.RouteWebhookIngestion, "order_or_entity_upsert", runtimediag.OutcomeError, businessStart)
		}
		_ = s.markFailed(ctx, ev.ID, StatusFailedRetryable, "WEBHOOK_PROCESS_FAILED", err.Error())
		s.failProcess(ctx, idemJob, "WEBHOOK_PROCESS_FAILED", true)
		s.ObserveWebhook(ev.Platform, ev.EventType, "processed", "failure", "process_failed", time.Since(start))
		return err
	}
	if businessApplicable {
		runtimediag.ObserveStage(runtimediag.RouteWebhookIngestion, "business_upsert", runtimediag.OutcomeSuccess, businessStart)
		runtimediag.ObserveStage(runtimediag.RouteWebhookIngestion, "order_or_entity_upsert", runtimediag.OutcomeSuccess, businessStart)
		runtimediag.Path(runtimediag.RouteWebhookIngestion, "business_upsert")
		runtimediag.Count(runtimediag.RouteWebhookIngestion, "businessUpsertCount", 1)
	}

	processedAt := s.now()
	if err := s.DB.WithContext(ctx).Model(&Event{}).Where("id = ?", ev.ID).Updates(map[string]any{
		"status":        StatusProcessed,
		"processed_at":  processedAt,
		"error_code":    "",
		"error_message": "",
		"updated_at":    processedAt,
	}).Error; err != nil {
		s.failProcess(ctx, idemJob, "WEBHOOK_MARK_PROCESSED_FAILED", true)
		s.ObserveWebhook(ev.Platform, ev.EventType, "processed", "failure", "mark_processed_failed", time.Since(start))
		return err
	}

	if idemJob != nil && s.Idempotency != nil {
		summary, _ := json.Marshal(map[string]string{"eventId": ev.EventID, "status": StatusProcessed})
		_ = s.Idempotency.Complete(ctx, idemJob.RecordID, idemJob.Owner, idempotency.CompleteResult{
			ResponseCode:    "WEBHOOK_PROCESSED",
			ResponseSummary: string(summary),
			ResourceType:    "webhook_event",
			ResourceID:      ev.EventID,
		})
	}
	lag := time.Duration(0)
	if !ev.CreatedAt.IsZero() {
		lag = processedAt.Sub(ev.CreatedAt)
	}
	if s != nil && s.Metrics != nil {
		s.Metrics.ObserveWebhookProcessed(safeWebhookPlatform(ev.Platform), eventGroup(ev.EventType), "success", "", time.Since(start), lag)
	}
	return nil
}

// ProcessQueuedEvents claims and processes up to limit queued webhook events (DB poll worker).
func (s *Service) ProcessQueuedEvents(ctx context.Context, limit int) (int, error) {
	if s == nil || s.DB == nil {
		return 0, fmt.Errorf("webhook: unavailable")
	}
	if limit <= 0 {
		limit = 20
	}
	var rows []Event
	query := s.DB.WithContext(ctx).
		Where("status = ?", StatusQueued).
		Order("created_at ASC").
		Limit(limit)
	if s.DB.Migrator().HasTable("tenants") {
		var tenantCount int64
		if err := s.DB.WithContext(ctx).Table("tenants").Count(&tenantCount).Error; err != nil {
			return 0, err
		}
		if tenantCount > 0 {
			query = query.Where("EXISTS (SELECT 1 FROM tenants WHERE tenants.id = webhook_events.tenant_id AND tenants.status = ?)", "active")
		}
	}
	err := query.Find(&rows).Error
	if err != nil {
		return 0, err
	}
	done := 0
	for i := range rows {
		if err := tasktenant.RequireTaskTenant(rows[i].TenantID); err != nil {
			continue
		}
		shopID := uuid.Nil
		if rows[i].InternalShopID != nil {
			shopID = *rows[i].InternalShopID
		}
		wctx, _, err := tasktenant.BeginWorker(ctx, s.DB, rows[i].TenantID, shopID, "webhook_process")
		if err != nil {
			continue
		}
		if err := s.ProcessEventByRowID(wctx, rows[i].TenantID, rows[i].ID); err != nil {
			continue
		}
		done++
	}
	return done, nil
}

func (s *Service) handlePlatformEvent(ctx context.Context, ev *Event) error {
	if ev == nil {
		return nil
	}
	switch ev.Platform {
	case "douyin_shop", "douyin":
		return s.HandleDouyinPlatformEvent(ctx, ev)
	default:
		// Other platforms: noop marks processed
		return nil
	}
}

func (s *Service) markFailed(ctx context.Context, id uuid.UUID, status, code, msg string) error {
	return s.DB.WithContext(ctx).Model(&Event{}).Where("id = ?", id).Updates(map[string]any{
		"status":        status,
		"error_code":    code,
		"error_message": truncateSummary(msg),
		"updated_at":    s.now(),
	}).Error
}

func (s *Service) failProcess(ctx context.Context, job *webhookAcquire, code string, retryable bool) {
	if s == nil || s.Idempotency == nil || job == nil {
		return
	}
	_ = s.Idempotency.Fail(ctx, job.RecordID, job.Owner, code, retryable)
}

// LoadEventByPlatformEventID loads a durable event row.
func (s *Service) LoadEventByPlatformEventID(ctx context.Context, platform, eventID string) (*Event, error) {
	var ev Event
	if err := s.DB.WithContext(ctx).Where("platform = ? AND event_id = ?", platform, eventID).First(&ev).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, err
	}
	return &ev, nil
}
