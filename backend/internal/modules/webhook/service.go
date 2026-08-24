package webhook

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
	"github.com/trademind-ai/trademind/backend/internal/pkg/pagination"
	"github.com/trademind-ai/trademind/backend/internal/pkg/runtimediag"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Service handles webhook idempotency, ingest, and async processing.
type Service struct {
	DB               *gorm.DB
	Idempotency      *idempotency.Service
	Verifiers        *Registry
	ShopResolver     WebhookShopResolver
	OrderHandler     OrderEventHandler
	AfterSaleHandler AfterSaleEventHandler
	Metrics          *metrics.Catalog
	MaxPayloadBytes  int64
	MaxClockSkew     time.Duration
	AppEnv           string
	Now              func() time.Time
}

// IngestRequest is normalized webhook input.
type IngestRequest struct {
	Platform     string
	EventType    string
	EventID      string
	Payload      json.RawMessage
	Timestamp    time.Time
	ResolvedShop *ResolvedWebhookShop
}

// IngestResult describes ACK outcome.
type IngestResult struct {
	EventID   string `json:"eventId"`
	Status    string `json:"status"`
	Duplicate bool   `json:"duplicate"`
}

type EventListQuery struct {
	TenantID       int64
	Platform       string
	Status         string
	EventType      string
	InternalShopID *uuid.UUID
	Start          *time.Time
	End            *time.Time
	Page           int
	PageSize       int
	Cursor         string
	Limit          int
	UseCursor      bool
}

type EventListResult struct {
	Items      []Event `json:"list"`
	Total      int64   `json:"total"`
	Page       int     `json:"page"`
	PageSize   int     `json:"pageSize"`
	TotalPages int     `json:"totalPages"`
	Limit      int     `json:"limit"`
	NextCursor string  `json:"nextCursor,omitempty"`
	HasMore    bool    `json:"hasMore"`
}

func eventListCursorScope(q EventListQuery) (string, string) {
	shopScope := ""
	if q.InternalShopID != nil && *q.InternalShopID != uuid.Nil {
		shopScope = q.InternalShopID.String()
	}
	return pagination.Fingerprint(map[string]any{
		"tenantId":  q.TenantID,
		"shopId":    shopScope,
		"platform":  q.Platform,
		"status":    q.Status,
		"eventType": q.EventType,
		"start":     q.Start,
		"end":       q.End,
		"sort":      "created_at_desc_id_desc",
	}), shopScope
}

func eventPages(total int64, ps int) int {
	if ps < 1 {
		ps = pagination.DefaultLimit
	}
	pages := int(total) / ps
	if int(total)%ps != 0 {
		pages++
	}
	if pages == 0 && total > 0 {
		pages = 1
	}
	return pages
}

func (s *Service) ListEvents(ctx context.Context, q EventListQuery) (*EventListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("webhook: no db")
	}
	if q.UseCursor && q.Limit > 0 {
		q.PageSize = q.Limit
	}
	paged, err := pagination.NormalizePage(q.Page, q.PageSize)
	if err != nil {
		return nil, err
	}
	page, ps := paged.Page, paged.Limit
	tx := s.DB.WithContext(ctx).Model(&Event{}).Where("tenant_id = ?", q.TenantID)
	if v := strings.TrimSpace(q.Platform); v != "" {
		tx = tx.Where("platform = ?", v)
	}
	if v := strings.TrimSpace(q.Status); v != "" {
		tx = tx.Where("status = ?", v)
	}
	if v := strings.TrimSpace(q.EventType); v != "" {
		tx = tx.Where("event_type = ?", v)
	}
	if q.InternalShopID != nil && *q.InternalShopID != uuid.Nil {
		tx = tx.Where("internal_shop_id = ?", *q.InternalShopID)
	}
	if q.Start != nil {
		tx = tx.Where("created_at >= ?", *q.Start)
	}
	if q.End != nil {
		tx = tx.Where("created_at <= ?", *q.End)
	}
	scopeHash, cursorShopID := eventListCursorScope(q)
	if q.UseCursor && strings.TrimSpace(q.Cursor) != "" {
		cur, err := pagination.DecodeCursor(q.Cursor, q.TenantID, cursorShopID, scopeHash)
		if err != nil {
			return nil, err
		}
		next, err := pagination.ApplyDescKeyset(tx, "created_at", "id", cur)
		if err != nil {
			return nil, err
		}
		tx = next
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	query := tx.Order("created_at DESC, id DESC")
	limit := ps
	if q.UseCursor {
		limit = ps + 1
	} else {
		query = query.Offset(paged.Offset)
	}
	var rows []Event
	if err := query.Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	hasMore := q.UseCursor && len(rows) > ps
	if hasMore {
		rows = rows[:ps]
	}
	nextCursor := ""
	if q.UseCursor && hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		nextCursor, err = pagination.BuildNextCursor(true, q.TenantID, cursorShopID, scopeHash, "created_at", last.CreatedAt, last.ID.String())
		if err != nil {
			return nil, err
		}
	}
	return &EventListResult{
		Items:      rows,
		Total:      total,
		Page:       page,
		PageSize:   ps,
		TotalPages: eventPages(total, ps),
		Limit:      ps,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

type webhookAcquire struct {
	RecordID uuid.UUID
	Owner    string
}

func (s *Service) now() time.Time {
	if s != nil && s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

func (s *Service) maxPayload() int64 {
	if s != nil && s.MaxPayloadBytes > 0 {
		return s.MaxPayloadBytes
	}
	return 512 * 1024
}

func (s *Service) maxSkew() time.Duration {
	if s != nil && s.MaxClockSkew > 0 {
		return s.MaxClockSkew
	}
	return 300 * time.Second
}

// ValidateTimestamp rejects missing (when required) and out-of-window timestamps.
func (s *Service) ValidateTimestamp(ts time.Time, required bool) error {
	if ts.IsZero() {
		if required {
			return newCodeError(CodeTimestampMissing, http.StatusUnauthorized, CodeTimestampMissing)
		}
		return nil
	}
	now := s.now()
	delta := now.Sub(ts)
	if delta < 0 {
		delta = -delta
	}
	if delta > s.maxSkew() {
		return newCodeError(CodeTimestampExpired, http.StatusUnauthorized, CodeTimestampExpired)
	}
	return nil
}

// Ingest stores webhook event idempotently and returns fast ACK metadata.
func (s *Service) Ingest(ctx context.Context, req IngestRequest) (*IngestResult, error) {
	start := time.Now()
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("webhook: unavailable")
	}
	platform := strings.TrimSpace(req.Platform)
	if platform == "" {
		return nil, fmt.Errorf("platform is required")
	}
	if len(req.Payload) > int(s.maxPayload()) {
		s.ObserveWebhook(platform, req.EventType, "payload_rejected", "failure", CodePayloadTooLarge, 0)
		return nil, newCodeError(CodePayloadTooLarge, http.StatusRequestEntityTooLarge, CodePayloadTooLarge)
	}
	if err := s.ValidateTimestamp(req.Timestamp, !req.Timestamp.IsZero()); err != nil {
		s.ObserveWebhook(platform, req.EventType, "replay_rejected", "failure", CodeTimestampExpired, 0)
		return nil, err
	}
	eventID := strings.TrimSpace(req.EventID)
	if eventID == "" {
		eventID = hashPayload(req.Payload)
	}
	hash := hashPayload(req.Payload)
	key := webhookIngestKey(platform, eventID, req.ResolvedShop)
	reqHash := idempotency.HashRequest(req.Payload)
	owner := "webhook-ingest"

	var existing Event
	dbStart := time.Now()
	lookupTiming, err := runtimediag.TimedGorm(s.DB, func() error {
		return s.eventScopeQuery(ctx, platform, eventID, req.ResolvedShop).First(&existing).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		runtimediag.ObserveDBOperation(runtimediag.RouteWebhookIngestion, "observedDbEntryLatency", runtimediag.OutcomeExpectedRejection, dbStart)
	} else {
		runtimediag.ObserveDBOperation(runtimediag.RouteWebhookIngestion, "observedDbEntryLatency", outcomeFromError(err), dbStart)
	}
	runtimediag.ObserveSQL(runtimediag.RouteWebhookIngestion, "webhook", "webhook.idempotency_lookup", "select", "webhook_events", outcomeFromError(err), false, lookupTiming)
	runtimediag.SnapshotGormDB(s.DB)
	if err == nil {
		runtimediag.Path(runtimediag.RouteWebhookIngestion, "duplicate_conflict")
		s.ObserveWebhook(platform, req.EventType, "duplicate", "duplicate", "", 0)
		s.ObserveWebhook(platform, req.EventType, "request", "success", "", time.Since(start))
		return &IngestResult{EventID: eventID, Status: existing.Status, Duplicate: true}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var idemJob *webhookAcquire
	if s.Idempotency != nil {
		stageStart := time.Now()
		var res *idempotency.AcquireResult
		var acqErr error
		idemTiming, _ := runtimediag.TimedGorm(s.DB, func() error {
			res, acqErr = s.Idempotency.Acquire(ctx, idempotency.ScopeWebhook, key, reqHash, owner, idempotency.DefaultLease)
			return acqErr
		})
		runtimediag.ObserveStage(runtimediag.RouteWebhookIngestion, "idempotency_check", outcomeFromError(acqErr), stageStart)
		runtimediag.ObserveDBOperation(runtimediag.RouteWebhookIngestion, "idempotency_check", outcomeFromError(acqErr), stageStart)
		runtimediag.ObserveSQL(runtimediag.RouteWebhookIngestion, "webhook", "webhook.idempotency_lookup", "select", "idempotency_keys", outcomeFromError(acqErr), false, idemTiming)
		runtimediag.SnapshotGormDB(s.DB)
		decision, rec, _ := idempotency.Classify(res, acqErr)
		switch decision {
		case idempotency.DecisionAlreadySucceeded:
			if row, loadErr := s.loadExistingEvent(ctx, platform, eventID, req.ResolvedShop); loadErr == nil {
				return &IngestResult{EventID: eventID, Status: row.Status, Duplicate: true}, nil
			}
			return &IngestResult{EventID: eventID, Status: StatusDuplicate, Duplicate: true}, nil
		case idempotency.DecisionInProgress:
			return nil, newCodeError(CodeInProgress, http.StatusConflict, CodeInProgress)
		case idempotency.DecisionKeyConflict, idempotency.DecisionPermanentFailure:
			return nil, newCodeError(CodeKeyConflict, http.StatusConflict, CodeKeyConflict)
		case idempotency.DecisionAcquired, idempotency.DecisionRetryAllowed:
			if rec == nil && res != nil {
				rec = res.Record
			}
			if rec != nil {
				idemJob = &webhookAcquire{RecordID: rec.ID, Owner: owner}
			}
		default:
			if acqErr != nil {
				return nil, acqErr
			}
		}
	}

	meta := eventMetadata(req.EventType, req.ResolvedShop)
	ev := Event{
		Platform:    platform,
		EventID:     eventID,
		EventType:   strings.TrimSpace(req.EventType),
		PayloadHash: hash,
		PayloadBody: string(req.Payload),
		Status:      StatusQueued,
		RawSummary:  truncateSummary(string(req.Payload)),
		Metadata:    datatypes.JSON(meta),
	}
	applyResolvedShopToEvent(&ev, req.ResolvedShop)
	if ev.TenantID <= 0 {
		ev.TenantID = devTestWebhookTenant(platform, s.AppEnv)
	}
	stageStart := time.Now()
	var createRes *gorm.DB
	insertTiming, _ := runtimediag.TimedGormRows(s.DB, func() (int64, error) {
		createRes = s.DB.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "platform"}, {Name: "tenant_id"}, {Name: "platform_shop_id"}, {Name: "event_id"}},
			DoNothing: true,
		}).Create(&ev)
		return createRes.RowsAffected, createRes.Error
	})
	insertOutcome := outcomeFromError(createRes.Error)
	if createRes.Error == nil && createRes.RowsAffected == 0 {
		insertOutcome = runtimediag.OutcomeExpectedConflict
		insertTiming.QueryError = "expected_conflict"
	}
	runtimediag.ObserveStage(runtimediag.RouteWebhookIngestion, "event_insert", insertOutcome, stageStart)
	runtimediag.ObserveDBOperation(runtimediag.RouteWebhookIngestion, "event_insert", insertOutcome, stageStart)
	runtimediag.ObserveSQL(runtimediag.RouteWebhookIngestion, "webhook", "webhook.event_insert", "upsert", "webhook_events", insertOutcome, false, insertTiming)
	runtimediag.SnapshotGormDB(s.DB)
	if createRes.Error != nil {
		s.failWebhookIngest(ctx, idemJob, CodeStoreFailed, true)
		s.ObserveWebhook(platform, req.EventType, "request", "failure", CodeStoreFailed, time.Since(start))
		return nil, createRes.Error
	}

	// Concurrent insert: another writer won ON CONFLICT DoNothing.
	if createRes.RowsAffected == 0 {
		var duplicate Event
		runtimediag.Count(runtimediag.RouteWebhookIngestion, "duplicateConflictCount", 1)
		runtimediag.Path(runtimediag.RouteWebhookIngestion, "duplicate_conflict")
		stageStart = time.Now()
		reloadTiming, reloadErr := runtimediag.TimedGorm(s.DB, func() error {
			return s.eventScopeQuery(ctx, platform, eventID, req.ResolvedShop).First(&duplicate).Error
		})
		if reloadErr != nil {
			runtimediag.ObserveStage(runtimediag.RouteWebhookIngestion, "duplicate_event_reload", outcomeFromError(reloadErr), stageStart)
			runtimediag.ObserveDBOperation(runtimediag.RouteWebhookIngestion, "duplicate_event_reload", outcomeFromError(reloadErr), stageStart)
			runtimediag.ObserveSQL(runtimediag.RouteWebhookIngestion, "webhook", "webhook.event_conflict_reload", "select", "webhook_events", outcomeFromError(reloadErr), false, reloadTiming)
			runtimediag.SnapshotGormDB(s.DB)
			s.failWebhookIngest(ctx, idemJob, CodeStoreFailed, true)
			s.ObserveWebhook(platform, req.EventType, "request", "failure", CodeStoreFailed, time.Since(start))
			if errors.Is(reloadErr, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("webhook event conflict reload consistency error: platform=%s event_id=%s: %w", platform, eventID, reloadErr)
			}
			return nil, fmt.Errorf("webhook event conflict reload failed: platform=%s event_id=%s: %w", platform, eventID, reloadErr)
		}
		runtimediag.ObserveStage(runtimediag.RouteWebhookIngestion, "duplicate_event_reload", runtimediag.OutcomeSuccess, stageStart)
		runtimediag.ObserveDBOperation(runtimediag.RouteWebhookIngestion, "duplicate_event_reload", runtimediag.OutcomeSuccess, stageStart)
		runtimediag.ObserveSQL(runtimediag.RouteWebhookIngestion, "webhook", "webhook.event_conflict_reload", "select", "webhook_events", runtimediag.OutcomeSuccess, false, reloadTiming)
		runtimediag.SnapshotGormDB(s.DB)
		runtimediag.Count(runtimediag.RouteWebhookIngestion, "duplicateReloadCount", 1)
		ev = duplicate
		if idemJob != nil && s.Idempotency != nil {
			stageStart = time.Now()
			completeTiming, _ := runtimediag.TimedGorm(s.DB, func() error {
				return s.Idempotency.Complete(ctx, idemJob.RecordID, idemJob.Owner, idempotency.CompleteResult{
					ResponseCode:    "WEBHOOK_DUPLICATE",
					ResponseSummary: `{"duplicate":true}`,
					ResourceType:    "webhook_event",
					ResourceID:      eventID,
				})
			})
			runtimediag.ObserveStage(runtimediag.RouteWebhookIngestion, "idempotency_check", runtimediag.OutcomeSuccess, stageStart)
			runtimediag.ObserveSQL(runtimediag.RouteWebhookIngestion, "webhook", "webhook.idempotency_complete", "update", "idempotency_keys", runtimediag.OutcomeSuccess, false, completeTiming)
		}
		s.ObserveWebhook(platform, req.EventType, "duplicate", "duplicate", "", 0)
		s.ObserveWebhook(platform, req.EventType, "request", "success", "", time.Since(start))
		return &IngestResult{EventID: eventID, Status: ev.Status, Duplicate: true}, nil
	}

	runtimediag.Count(runtimediag.RouteWebhookIngestion, "normalInsertCount", 1)
	runtimediag.Path(runtimediag.RouteWebhookIngestion, "normal_insert")
	if idemJob != nil && s.Idempotency != nil {
		summary, _ := json.Marshal(map[string]string{"eventId": eventID, "status": ev.Status})
		stageStart = time.Now()
		var completeErr error
		completeTiming, _ := runtimediag.TimedGorm(s.DB, func() error {
			completeErr = s.Idempotency.Complete(ctx, idemJob.RecordID, idemJob.Owner, idempotency.CompleteResult{
				ResponseCode:    "WEBHOOK_RECEIVED",
				ResponseSummary: string(summary),
				ResourceType:    "webhook_event",
				ResourceID:      eventID,
			})
			return completeErr
		})
		if completeErr != nil {
			runtimediag.ObserveStage(runtimediag.RouteWebhookIngestion, "idempotency_check", outcomeFromError(completeErr), stageStart)
			runtimediag.ObserveSQL(runtimediag.RouteWebhookIngestion, "webhook", "webhook.idempotency_complete", "update", "idempotency_keys", outcomeFromError(completeErr), false, completeTiming)
			return nil, completeErr
		}
		runtimediag.ObserveStage(runtimediag.RouteWebhookIngestion, "idempotency_check", runtimediag.OutcomeSuccess, stageStart)
		runtimediag.ObserveSQL(runtimediag.RouteWebhookIngestion, "webhook", "webhook.idempotency_complete", "update", "idempotency_keys", runtimediag.OutcomeSuccess, false, completeTiming)
	}

	s.ObserveWebhook(platform, req.EventType, "persisted", "success", "", 0)
	s.ObserveWebhook(platform, req.EventType, "request", "success", "", time.Since(start))
	return &IngestResult{EventID: eventID, Status: ev.Status, Duplicate: false}, nil
}

func outcomeFromError(err error) string {
	if err == nil {
		return runtimediag.OutcomeSuccess
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return runtimediag.OutcomeExpectedRejection
	}
	return runtimediag.OutcomeError
}

func (s *Service) loadExistingEvent(ctx context.Context, platform, eventID string, resolved *ResolvedWebhookShop) (*Event, error) {
	var row Event
	if err := s.eventScopeQuery(ctx, platform, eventID, resolved).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Service) eventScopeQuery(ctx context.Context, platform, eventID string, resolved *ResolvedWebhookShop) *gorm.DB {
	q := s.DB.WithContext(ctx).Where("platform = ? AND event_id = ?", platform, eventID)
	if resolved != nil && strings.TrimSpace(resolved.PlatformShopID) != "" {
		q = q.Where("tenant_id = ? AND platform_shop_id = ?", resolved.TenantID, strings.TrimSpace(resolved.PlatformShopID))
	} else {
		q = q.Where("tenant_id = ? AND platform_shop_id = ?", devTestWebhookTenant(platform, s.AppEnv), "")
	}
	return q
}

// devTestWebhookTenant assigns a positive tenant for dev-only internal-test webhooks without shop binding.
func devTestWebhookTenant(platform, appEnv string) int64 {
	if platform == PlatformInternalTest && !config.IsProduction(appEnv) {
		return 1
	}
	return 0
}

func webhookIngestKey(platform, eventID string, resolved *ResolvedWebhookShop) string {
	if resolved != nil && strings.TrimSpace(resolved.PlatformShopID) != "" {
		return idempotency.WebhookScoped(platform, resolved.TenantID, resolved.PlatformShopID, eventID)
	}
	return idempotency.Webhook(platform, eventID)
}

func webhookProcessKey(ev *Event) string {
	if ev != nil && strings.TrimSpace(ev.PlatformShopID) != "" {
		return idempotency.WebhookProcessScoped(ev.Platform, ev.TenantID, ev.PlatformShopID, ev.EventID)
	}
	if ev == nil {
		return idempotency.WebhookProcess("", "")
	}
	return idempotency.WebhookProcess(ev.Platform, ev.EventID)
}

func applyResolvedShopToEvent(ev *Event, resolved *ResolvedWebhookShop) {
	if ev == nil || resolved == nil {
		return
	}
	ev.TenantID = resolved.TenantID
	if resolved.InternalShopID != uuid.Nil {
		id := resolved.InternalShopID
		ev.InternalShopID = &id
	}
	ev.PlatformShopID = strings.TrimSpace(resolved.PlatformShopID)
	ev.AppID = strings.TrimSpace(resolved.AppID)
	if resolved.BindingID != uuid.Nil {
		id := resolved.BindingID
		ev.BindingID = &id
	}
}

func eventMetadata(eventType string, resolved *ResolvedWebhookShop) []byte {
	m := map[string]any{"eventType": strings.TrimSpace(eventType)}
	if resolved != nil {
		m["tenantId"] = resolved.TenantID
		m["internalShopId"] = resolved.InternalShopID.String()
		m["platformShopId"] = resolved.PlatformShopID
		m["appId"] = resolved.AppID
		m["bindingId"] = resolved.BindingID.String()
		m["authorizationStatus"] = resolved.AuthorizationStatus
		m["contractStatus"] = resolved.ContractStatus
		if resolved.TestFallback {
			m["test_fallback"] = true
		}
	}
	b, _ := json.Marshal(m)
	return b
}

func (s *Service) failWebhookIngest(ctx context.Context, job *webhookAcquire, code string, retryable bool) {
	if s == nil || s.Idempotency == nil || job == nil {
		return
	}
	_ = s.Idempotency.Fail(ctx, job.RecordID, job.Owner, code, retryable)
}

func hashPayload(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func truncateSummary(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
