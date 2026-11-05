package operationtask

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type APIService struct {
	DB              *gorm.DB
	Authorizer      *RBACAuthorizer
	Drafts          *DraftVersionService
	Approvals       *ApprovalService
	Executor        *ExecutionOrchestrator
	Retry           *ManualRetryService
	Transitions     *TaskTransitionService
	SnapshotBuilder ProductionSnapshotBuilder
	Production      *ProductionExecutionService
}

func NewAPIService(db *gorm.DB) *APIService {
	authorizer := NewRBACAuthorizer(db)
	registry := NewSafePlatformDraftAdapterRegistry()
	executor := NewExecutionOrchestrator(db, authorizer, registry)
	return &APIService{
		DB:          db,
		Authorizer:  authorizer,
		Drafts:      NewDraftVersionService(db),
		Approvals:   NewApprovalService(db, authorizer),
		Executor:    executor,
		Retry:       NewManualRetryService(db, executor, DefaultMaxManualRetryAttempts),
		Transitions: NewTaskTransitionService(db),
	}
}

func (s *APIService) CreateTask(ctx context.Context, actor APIActor, req CreateTaskRequest, requestID, idemKey string) (*OperationTaskDetailResponse, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if actor.TenantID < 0 || actor.ActorID == uuid.Nil || strings.TrimSpace(requestID) == "" || strings.TrimSpace(idemKey) == "" {
		return nil, ErrValidation
	}
	if err := s.Authorizer.CanCreate(ctx, actor.TenantID, actor.ActorID); err != nil {
		return nil, ErrPermissionDenied
	}
	payload, err := apiValidateCreateTaskRequest(req)
	if err != nil {
		return nil, err
	}
	payloadHash, err := ComputePayloadHash([]byte(payload))
	if err != nil {
		return nil, ErrValidation
	}
	if existing, lookupErr := NewOperationTaskRepository(s.DB).GetByIdempotencyKey(ctx, actor.TenantID, idemKey); lookupErr == nil {
		existingHash, hashErr := ComputePayloadHash([]byte(existing.Payload))
		if hashErr != nil || existingHash != payloadHash || existing.Title != strings.TrimSpace(req.Title) {
			return nil, ErrIdemPayloadConflict
		}
		return s.detail(ctx, actor, existing.ID)
	} else if !errors.Is(lookupErr, ErrNotFound) {
		return nil, lookupErr
	}
	productionIntent, productionTask := DouyinDraftIntent{}, req.TaskType == OperationTaskTypeProductPublish && req.Platform == PlatformDouyin
	var frozenPayload datatypes.JSON
	var frozenHash string
	if productionTask {
		if actor.TenantID <= 0 || s.SnapshotBuilder == nil {
			return nil, ErrPermissionDenied
		}
		productionIntent, err = ParseDouyinDraftIntent(payload)
		if err != nil {
			return nil, err
		}
		frozenPayload, err = s.SnapshotBuilder.BuildDouyinDraftSnapshot(ctx, actor.TenantID, actor.ActorID, productionIntent)
		if err != nil {
			return nil, err
		}
		frozenHash, err = ComputePayloadHash(frozenPayload)
		if err != nil {
			return nil, ErrValidation
		}
	}
	var task OperationTask
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if existing, err := NewOperationTaskRepository(tx).GetByIdempotencyKey(ctx, actor.TenantID, idemKey); err == nil {
			if existing.Payload == nil {
				return ErrDuplicateRequest
			}
			existingHash, hashErr := ComputePayloadHash([]byte(existing.Payload))
			if hashErr != nil || existingHash != payloadHash || existing.Title != strings.TrimSpace(req.Title) {
				return ErrIdemPayloadConflict
			}
			task = *existing
			return nil
		} else if !errors.Is(err, ErrNotFound) {
			return err
		}
		key := strings.TrimSpace(idemKey)
		initialStatus := OperationTaskStatusSuggested
		if productionTask {
			initialStatus = OperationTaskStatusDraftPreparing
		}
		task = OperationTask{
			TenantID:        actor.TenantID,
			SourceType:      req.SourceType,
			SourceReference: req.SourceReference,
			TaskType:        req.TaskType,
			Platform:        req.Platform,
			Title:           req.Title,
			Summary:         req.Summary,
			Payload:         payload,
			Status:          initialStatus,
			Priority:        req.Priority,
			IdempotencyKey:  &key,
			Revision:        1,
			CreatedBy:       &actor.ActorID,
			UpdatedBy:       &actor.ActorID,
		}
		if task.Priority == "" {
			task.Priority = OperationTaskPriorityNormal
		}
		if err := validateOperationTask(&task); err != nil {
			return err
		}
		if err := tx.Create(&task).Error; err != nil {
			if isUniqueViolation(err) {
				return ErrDuplicateRequest
			}
			return stableError(err, ErrConflict)
		}
		if err := appendAuditEventTx(tx, OperationTaskEvent{
			TenantID:        actor.TenantID,
			OperationTaskID: task.ID,
			EventType:       OperationTaskEventTypeTaskCreated,
			ActorType:       OperationTaskEventActorUser,
			ActorID:         &actor.ActorID,
			AfterState:      initialStatus,
			RequestID:       strings.TrimSpace(requestID),
			Reason:          "operation task created",
			Metadata:        safeMetadataJSON(map[string]any{"idempotencyKeyHash": safeKeyHash(idemKey), "payloadHash": payloadHash}),
		}); err != nil {
			return err
		}
		if !productionTask {
			return nil
		}
		draftKey := key + ":draft:v1"
		draft := PlatformDraft{TenantID: actor.TenantID, OperationTaskID: task.ID, Platform: PlatformDouyin, AdapterMode: AdapterModeProductionDraft, DraftVersion: 1, Payload: frozenPayload, PayloadHash: frozenHash, Status: PlatformDraftStatusPendingReview, ChangeReason: "production snapshot generated", IdempotencyKey: &draftKey, CreatedBy: &actor.ActorID, UpdatedBy: &actor.ActorID}
		if err := validatePlatformDraft(&draft); err != nil {
			return err
		}
		if err := tx.Create(&draft).Error; err != nil {
			return stableError(err, ErrDuplicateDraftVersion)
		}
		if err := updateTaskStatusRevisionTx(tx, &task, OperationTaskStatusPendingReview, &actor.ActorID); err != nil {
			return err
		}
		if err := appendAuditEventTx(tx, OperationTaskEvent{TenantID: actor.TenantID, OperationTaskID: task.ID, EventType: OperationTaskEventTypeDraftGenerated, ActorType: OperationTaskEventActorSystem, BeforeState: OperationTaskStatusDraftPreparing, AfterState: OperationTaskStatusPendingReview, PlatformDraftID: &draft.ID, DraftVersion: draft.DraftVersion, RequestID: strings.TrimSpace(requestID), Reason: "immutable platform request snapshot generated", Metadata: safeMetadataJSON(map[string]any{"payloadHash": frozenHash, "schemaVersion": productionIntent.SchemaVersion})}); err != nil {
			return err
		}
		if err := appendAuditEventTx(tx, OperationTaskEvent{TenantID: actor.TenantID, OperationTaskID: task.ID, EventType: OperationTaskEventTypeReviewRequested, ActorType: OperationTaskEventActorSystem, BeforeState: OperationTaskStatusPendingReview, AfterState: OperationTaskStatusPendingReview, PlatformDraftID: &draft.ID, DraftVersion: draft.DraftVersion, RequestID: strings.TrimSpace(requestID), Reason: "human review required before platform write", Metadata: datatypes.JSON([]byte(`{}`))}); err != nil {
			return err
		}
		task.Status = OperationTaskStatusPendingReview
		task.Revision++
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.detail(ctx, actor, task.ID)
}

func (s *APIService) ListTasks(ctx context.Context, actor APIActor, p OperationTaskListParams) (TaskListResponse, error) {
	var zero TaskListResponse
	if err := s.ready(); err != nil {
		return zero, err
	}
	if err := s.Authorizer.CanRead(ctx, actor.TenantID, actor.ActorID); err != nil {
		return zero, ErrPermissionDenied
	}
	p.TenantID = actor.TenantID
	result, err := NewOperationTaskRepository(s.DB).ListWithLatestSummaries(ctx, p)
	if err != nil {
		return zero, err
	}
	items := make([]OperationTaskSummaryResponse, 0, len(result.Items))
	for _, summary := range result.Items {
		item := taskSummary(summary.Task, nil, nil)
		item.LatestDraftVersion = summary.LatestDraftVersion
		item.LatestExecutionStatus = summary.LatestExecutionStatus
		items = append(items, item)
	}
	return TaskListResponse{Items: items, NextCursor: result.NextCursor, HasMore: result.HasMore, Limit: result.Limit}, nil
}

func (s *APIService) GetTask(ctx context.Context, actor APIActor, taskID uuid.UUID) (*OperationTaskDetailResponse, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if err := s.Authorizer.CanRead(ctx, actor.TenantID, actor.ActorID); err != nil {
		return nil, ErrPermissionDenied
	}
	return s.detail(ctx, actor, taskID)
}

func (s *APIService) CreateInitialDraft(ctx context.Context, actor APIActor, taskID uuid.UUID, req CreateDraftRequest, requestID, idemKey string) (*DraftSummaryResponse, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if err := s.Authorizer.CanEdit(ctx, actor.TenantID, actor.ActorID, taskID); err != nil {
		return nil, ErrPermissionDenied
	}
	if task, loadErr := NewOperationTaskRepository(s.DB).GetByID(ctx, actor.TenantID, taskID); loadErr == nil && task.TaskType == OperationTaskTypeProductPublish && task.Platform == PlatformDouyin {
		return nil, ErrExecutionModeForbidden
	}
	payload, err := apiJSONPayload(req.Payload)
	if err != nil {
		return nil, err
	}
	if err := apiValidateReason(req.ChangeReason, false); err != nil {
		return nil, err
	}
	draft, err := s.Drafts.CreateInitialDraft(ctx, DraftVersionInput{TenantID: actor.TenantID, OperationTaskID: taskID, ExpectedRevision: req.ExpectedTaskRevision, Payload: payload, ActorID: &actor.ActorID, RequestID: requestID, IdempotencyKey: idemKey, ChangeReason: req.ChangeReason})
	if err != nil {
		return nil, err
	}
	out := draftSummary(*draft)
	return &out, nil
}

func (s *APIService) EditLatestDraft(ctx context.Context, actor APIActor, taskID uuid.UUID, req EditDraftRequest, requestID, idemKey string) (*DraftSummaryResponse, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if err := s.Authorizer.CanEdit(ctx, actor.TenantID, actor.ActorID, taskID); err != nil {
		return nil, ErrPermissionDenied
	}
	if task, loadErr := NewOperationTaskRepository(s.DB).GetByID(ctx, actor.TenantID, taskID); loadErr == nil && task.TaskType == OperationTaskTypeProductPublish && task.Platform == PlatformDouyin {
		return nil, ErrExecutionModeForbidden
	}
	payload, err := apiJSONPayload(req.Payload)
	if err != nil {
		return nil, err
	}
	if err := apiValidateReason(req.ChangeReason, false); err != nil {
		return nil, err
	}
	latest, err := s.Drafts.GetLatestDraft(ctx, actor.TenantID, taskID)
	if err != nil {
		return nil, err
	}
	if req.ExpectedDraftVersion > 0 && latest.DraftVersion != req.ExpectedDraftVersion {
		return nil, ErrDraftVersionMismatch
	}
	draft, err := s.Drafts.EditDraft(ctx, DraftVersionInput{TenantID: actor.TenantID, OperationTaskID: taskID, ExpectedRevision: req.ExpectedTaskRevision, Payload: payload, ActorID: &actor.ActorID, RequestID: requestID, IdempotencyKey: idemKey, ChangeReason: req.ChangeReason})
	if err != nil {
		return nil, err
	}
	out := draftSummary(*draft)
	return &out, nil
}

func (s *APIService) ListDrafts(ctx context.Context, actor APIActor, taskID uuid.UUID, limit int) (DraftListResponse, error) {
	var zero DraftListResponse
	if err := s.ready(); err != nil {
		return zero, err
	}
	if err := s.Authorizer.CanRead(ctx, actor.TenantID, actor.ActorID); err != nil {
		return zero, ErrPermissionDenied
	}
	if _, err := NewOperationTaskRepository(s.DB).GetByID(ctx, actor.TenantID, taskID); err != nil {
		return zero, err
	}
	drafts, err := NewPlatformDraftRepository(s.DB).ListVersions(ctx, actor.TenantID, taskID)
	if err != nil {
		return zero, err
	}
	if limit <= 0 || limit > apiMaxLimit {
		limit = apiMaxLimit
	}
	if len(drafts) > limit {
		drafts = drafts[:limit]
	}
	items := make([]DraftSummaryResponse, 0, len(drafts))
	for _, draft := range drafts {
		items = append(items, draftSummary(draft))
	}
	return DraftListResponse{Items: items, Limit: limit}, nil
}

func (s *APIService) Approve(ctx context.Context, actor APIActor, taskID uuid.UUID, req ApprovalRequest, requestID, idemKey string) (*ApprovalResponse, error) {
	return s.decide(ctx, actor, taskID, req, requestID, idemKey, true)
}

func (s *APIService) Reject(ctx context.Context, actor APIActor, taskID uuid.UUID, req ApprovalRequest, requestID, idemKey string) (*ApprovalResponse, error) {
	return s.decide(ctx, actor, taskID, req, requestID, idemKey, false)
}

func (s *APIService) decide(ctx context.Context, actor APIActor, taskID uuid.UUID, req ApprovalRequest, requestID, idemKey string, approve bool) (*ApprovalResponse, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if err := apiValidateReason(req.Reason, !approve); err != nil {
		return nil, err
	}
	if err := apiValidateComment(req.Comment); err != nil {
		return nil, err
	}
	input := ApprovalInput{TenantID: actor.TenantID, OperationTaskID: taskID, ExpectedRevision: req.ExpectedTaskRevision, DraftVersion: req.DraftVersion, DraftPayloadHash: req.DraftPayloadHash, ReviewerID: actor.ActorID, ReviewerRole: reviewerRoleForActor(actor.Role), Reason: req.Reason, Comment: req.Comment, RequestID: requestID, IdempotencyKey: idemKey}
	var record *ApprovalRecord
	var err error
	if approve {
		record, err = s.Approvals.Approve(ctx, input)
	} else {
		record, err = s.Approvals.Reject(ctx, input)
	}
	if err != nil {
		return nil, err
	}
	out := approvalResponse(*record)
	return &out, nil
}

func (s *APIService) Execute(ctx context.Context, actor APIActor, taskID uuid.UUID, req ExecuteRequest, requestID, idemKey string) (*ExecutionResponse, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if req.AdapterMode != "" && forbiddenExecutionMode(req.AdapterMode) {
		return nil, ErrExecutionModeForbidden
	}
	if err := s.Authorizer.CanExecute(ctx, actor.TenantID, actor.ActorID, taskID); err != nil {
		return nil, ErrPermissionDenied
	}
	if req.ExpectedTaskRevision > 0 {
		task, err := NewOperationTaskRepository(s.DB).GetByID(ctx, actor.TenantID, taskID)
		if err != nil {
			return nil, err
		}
		if task.Revision != req.ExpectedTaskRevision {
			return nil, ErrRevisionConflict
		}
	}
	if s.Production != nil {
		task, loadErr := NewOperationTaskRepository(s.DB).GetByID(ctx, actor.TenantID, taskID)
		if loadErr != nil {
			return nil, loadErr
		}
		draft, loadErr := NewPlatformDraftRepository(s.DB).GetLatest(ctx, actor.TenantID, taskID)
		if loadErr != nil {
			return nil, loadErr
		}
		if IsProductionDouyinTask(task, draft) {
			mode := strings.TrimSpace(strings.ToLower(req.AdapterMode))
			if mode != "" && mode != AdapterModeProductionDraft {
				return nil, ErrExecutionModeForbidden
			}
			out, queueErr := s.Production.Queue(ctx, ExecutionInput{TenantID: actor.TenantID, OperationTaskID: taskID, ActorID: actor.ActorID, RequestID: requestID, IdempotencyKey: idemKey, AdapterMode: AdapterModeProductionDraft}, false)
			if queueErr != nil {
				return nil, queueErr
			}
			return s.executionResponse(ctx, actor.TenantID, out), nil
		}
	}
	out, err := s.Executor.Execute(ctx, ExecutionInput{TenantID: actor.TenantID, OperationTaskID: taskID, ActorID: actor.ActorID, RequestID: requestID, IdempotencyKey: idemKey, AdapterMode: req.AdapterMode})
	if err != nil && out == nil {
		return nil, err
	}
	return s.executionResponse(ctx, actor.TenantID, out), err
}

func (s *APIService) RetryExecution(ctx context.Context, actor APIActor, taskID uuid.UUID, req RetryRequest, requestID, idemKey string) (*ExecutionResponse, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if err := s.Authorizer.CanRetry(ctx, actor.TenantID, actor.ActorID, taskID); err != nil {
		return nil, ErrPermissionDenied
	}
	if err := apiValidateReason(req.Reason, true); err != nil {
		return nil, err
	}
	if req.ExpectedTaskRevision > 0 {
		task, err := NewOperationTaskRepository(s.DB).GetByID(ctx, actor.TenantID, taskID)
		if err != nil {
			return nil, err
		}
		if task.Revision != req.ExpectedTaskRevision {
			return nil, ErrRevisionConflict
		}
	}
	if req.FailedAttemptID != nil {
		attempt, err := NewExecutionAttemptRepository(s.DB).GetByID(ctx, actor.TenantID, *req.FailedAttemptID)
		if err != nil {
			return nil, err
		}
		if attempt.OperationTaskID != taskID || attempt.Status != ExecutionAttemptStatusFailed {
			return nil, ErrStateConflict
		}
	}
	if s.Production != nil {
		task, loadErr := NewOperationTaskRepository(s.DB).GetByID(ctx, actor.TenantID, taskID)
		if loadErr != nil {
			return nil, loadErr
		}
		draft, loadErr := NewPlatformDraftRepository(s.DB).GetLatest(ctx, actor.TenantID, taskID)
		if loadErr != nil {
			return nil, loadErr
		}
		if IsProductionDouyinTask(task, draft) {
			out, queueErr := s.Production.Queue(ctx, ExecutionInput{TenantID: actor.TenantID, OperationTaskID: taskID, ActorID: actor.ActorID, RequestID: requestID, IdempotencyKey: idemKey, AdapterMode: AdapterModeProductionDraft}, true)
			if queueErr != nil {
				return nil, queueErr
			}
			return s.executionResponse(ctx, actor.TenantID, out), nil
		}
	}
	out, err := s.Retry.Retry(ctx, ExecutionInput{TenantID: actor.TenantID, OperationTaskID: taskID, ActorID: actor.ActorID, RequestID: requestID, IdempotencyKey: idemKey})
	if err != nil && out == nil {
		return nil, err
	}
	return s.executionResponse(ctx, actor.TenantID, out), err
}

func (s *APIService) Cancel(ctx context.Context, actor APIActor, taskID uuid.UUID, req CancelTaskRequest, requestID, idemKey string) (*OperationTaskDetailResponse, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if err := s.Authorizer.CanCancel(ctx, actor.TenantID, actor.ActorID, taskID); err != nil {
		return nil, ErrPermissionDenied
	}
	if err := apiValidateReason(req.Reason, true); err != nil {
		return nil, err
	}
	keyHash := safeKeyHash(idemKey)
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing OperationTaskEvent
		err := tx.Where("tenant_id = ? AND operation_task_id = ? AND event_type = ?", actor.TenantID, taskID, OperationTaskEventTypeCancelled).
			Order("sequence DESC, id DESC").
			First(&existing).Error
		if err == nil && strings.Contains(string(existing.Metadata), keyHash) {
			return nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return stableError(err, ErrConflict)
		}
		task, err := lockTask(tx, actor.TenantID, taskID)
		if err != nil {
			return err
		}
		if task.Revision != req.ExpectedTaskRevision {
			return ErrRevisionConflict
		}
		if err := NewTaskStateMachine().ValidateTransition(task.Status, OperationTaskStatusCancelled); err != nil {
			return err
		}
		if err := updateTaskStatusRevisionTx(tx, task, OperationTaskStatusCancelled, &actor.ActorID); err != nil {
			return err
		}
		return appendAuditEventTx(tx, OperationTaskEvent{
			TenantID:        actor.TenantID,
			OperationTaskID: taskID,
			EventType:       OperationTaskEventTypeCancelled,
			ActorType:       OperationTaskEventActorUser,
			ActorID:         &actor.ActorID,
			BeforeState:     task.Status,
			AfterState:      OperationTaskStatusCancelled,
			RequestID:       strings.TrimSpace(requestID),
			Reason:          strings.TrimSpace(req.Reason),
			Metadata:        safeMetadataJSON(map[string]any{"idempotencyKeyHash": keyHash}),
		})
	})
	if err != nil {
		return nil, err
	}
	return s.detail(ctx, actor, taskID)
}

func (s *APIService) ListAttempts(ctx context.Context, actor APIActor, taskID uuid.UUID, limit int, cursor string) (AttemptListResponse, error) {
	var zero AttemptListResponse
	if err := s.ready(); err != nil {
		return zero, err
	}
	if err := s.Authorizer.CanRead(ctx, actor.TenantID, actor.ActorID); err != nil {
		return zero, ErrPermissionDenied
	}
	if _, err := NewOperationTaskRepository(s.DB).GetByID(ctx, actor.TenantID, taskID); err != nil {
		return zero, err
	}
	attempts, err := NewExecutionAttemptRepository(s.DB).ListByTask(ctx, actor.TenantID, taskID)
	if err != nil {
		return zero, err
	}
	start := 0
	if cursor != "" {
		start, err = strconv.Atoi(strings.TrimSpace(cursor))
		if err != nil || start < 0 {
			return zero, ErrValidation
		}
	}
	limit, err = apiValidateLimitCursor(limit, cursor)
	if err != nil {
		return zero, err
	}
	items := make([]AttemptSummaryResponse, 0, limit)
	for _, attempt := range attempts {
		if attempt.AttemptNumber <= start {
			continue
		}
		items = append(items, attemptSummary(attempt))
		if len(items) == limit {
			break
		}
	}
	hasMore := false
	next := ""
	if len(items) > 0 {
		last := items[len(items)-1].AttemptNumber
		for _, attempt := range attempts {
			if attempt.AttemptNumber > last {
				hasMore = true
				next = strconv.Itoa(last)
				break
			}
		}
	}
	return AttemptListResponse{Items: items, NextCursor: next, HasMore: hasMore, Limit: limit}, nil
}

func (s *APIService) ListEvents(ctx context.Context, actor APIActor, taskID uuid.UUID, limit int, afterSequence int) (EventListResponse, error) {
	var zero EventListResponse
	if err := s.ready(); err != nil {
		return zero, err
	}
	if err := s.Authorizer.CanRead(ctx, actor.TenantID, actor.ActorID); err != nil {
		return zero, ErrPermissionDenied
	}
	if _, err := NewOperationTaskRepository(s.DB).GetByID(ctx, actor.TenantID, taskID); err != nil {
		return zero, err
	}
	result, err := NewOperationTaskEventRepository(s.DB).ListByTask(ctx, OperationTaskEventListParams{TenantID: actor.TenantID, OperationTaskID: taskID, AfterSequence: afterSequence, Limit: limit})
	if err != nil {
		return zero, err
	}
	items := make([]EventResponse, 0, len(result.Items))
	for _, event := range result.Items {
		items = append(items, eventResponse(event))
	}
	return EventListResponse{Items: items, NextSequence: result.NextSequence, HasMore: result.HasMore, Limit: result.Limit}, nil
}

func (s *APIService) ready() error {
	if s == nil || s.DB == nil || s.Authorizer == nil {
		return fmt.Errorf("operation task api unavailable")
	}
	return nil
}

func (s *APIService) detail(ctx context.Context, actor APIActor, taskID uuid.UUID) (*OperationTaskDetailResponse, error) {
	task, err := NewOperationTaskRepository(s.DB).GetByID(ctx, actor.TenantID, taskID)
	if err != nil {
		return nil, err
	}
	draft, err := NewPlatformDraftRepository(s.DB).GetLatest(ctx, actor.TenantID, taskID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	approval, err := NewApprovalRecordRepository(s.DB).GetLatestByTask(ctx, actor.TenantID, taskID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	attempt, err := latestAttempt(s.DB, ctx, actor.TenantID, taskID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	summary := taskSummary(*task, draft, attempt)
	detail := OperationTaskDetailResponse{OperationTaskSummaryResponse: summary, Payload: decodeSafeJSON(task.Payload), AllowedActions: s.allowedActions(ctx, actor, *task)}
	if draft != nil {
		v := draftSummary(*draft)
		detail.LatestDraft = &v
	}
	if approval != nil {
		v := approvalResponse(*approval)
		detail.LatestApproval = &v
	}
	if attempt != nil {
		v := attemptSummary(*attempt)
		detail.LatestAttempt = &v
	}
	return &detail, nil
}

func taskSummary(task OperationTask, draft *PlatformDraft, attempt *ExecutionAttempt) OperationTaskSummaryResponse {
	out := OperationTaskSummaryResponse{ID: task.ID, SourceType: task.SourceType, SourceReference: task.SourceReference, TaskType: task.TaskType, Platform: task.Platform, Title: task.Title, Summary: task.Summary, Status: task.Status, Priority: task.Priority, Revision: task.Revision, CreatedBy: task.CreatedBy, CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt}
	if draft != nil {
		out.LatestDraftVersion = draft.DraftVersion
	}
	if attempt != nil {
		out.LatestExecutionStatus = attempt.Status
	}
	return out
}

func (s *APIService) allowedActions(ctx context.Context, actor APIActor, task OperationTask) AllowedTaskActions {
	actions := AllowedTaskActions{}
	sm := NewTaskStateMachine()
	canEdit := s.Authorizer.CanEdit(ctx, actor.TenantID, actor.ActorID, task.ID) == nil
	canReview := s.Authorizer.CanReview(ctx, actor.TenantID, actor.ActorID) == nil
	canExecute := s.Authorizer.CanExecute(ctx, actor.TenantID, actor.ActorID, task.ID) == nil
	canRetry := s.Authorizer.CanRetry(ctx, actor.TenantID, actor.ActorID, task.ID) == nil
	actions.CanEditDraft = canEdit && !sm.IsTerminal(task.Status)
	actions.CanApprove = canReview && task.Status == OperationTaskStatusPendingReview
	actions.CanReject = actions.CanApprove
	actions.CanExecute = canExecute && sm.CanExecute(task.Status)
	actions.CanRetry = canRetry && task.Status == OperationTaskStatusExecutionFailed
	actions.CanCancel = canEdit && sm.CanTransition(task.Status, OperationTaskStatusCancelled)
	return actions
}

func (s *APIService) executionResponse(ctx context.Context, tenantID int64, out *ExecutionOutput) *ExecutionResponse {
	if out == nil {
		return nil
	}
	attempt := attemptSummary(out.Attempt)
	resp := &ExecutionResponse{Status: out.Status, Attempt: attempt, ResultType: out.Result.ResultType, RequestID: out.Attempt.RequestID}
	if task, err := NewOperationTaskRepository(s.DB).GetByID(ctx, tenantID, out.Attempt.OperationTaskID); err == nil {
		resp.TaskStatus = task.Status
	}
	if out.Failure != nil {
		resp.Failure = &ExecutionFailureDTO{Category: out.Failure.Category, Code: out.Failure.Code, SafeMessage: out.Failure.SafeMessage, Retryable: out.Failure.Retryable}
	}
	return resp
}

func draftSummary(draft PlatformDraft) DraftSummaryResponse {
	return DraftSummaryResponse{ID: draft.ID, DraftVersion: draft.DraftVersion, AdapterMode: draft.AdapterMode, PayloadHash: draft.PayloadHash, Status: draft.Status, ChangeReason: draft.ChangeReason, CreatedBy: draft.CreatedBy, CreatedAt: draft.CreatedAt, UpdatedAt: draft.UpdatedAt, Payload: draftPayloadForAPI(draft)}
}

func draftPayloadForAPI(draft PlatformDraft) any {
	if draft.AdapterMode != AdapterModeProductionDraft {
		return decodeSafeJSON(draft.Payload)
	}
	frozen, err := ParseFrozenDouyinDraft(draft.Payload)
	if err != nil {
		return nil
	}
	return map[string]any{
		"schemaVersion":   frozen.SchemaVersion,
		"productId":       frozen.ProductID,
		"shopId":          frozen.ShopID,
		"publishMode":     frozen.PublishMode,
		"skuCount":        frozen.SKUCount,
		"review":          productionReviewForAPI(frozen.Review),
		"mappingSnapshot": productionMappingForAPI(frozen.MappingSnapshot),
		"mappingHash":     frozen.MappingHash,
	}
}

func productionReviewForAPI(raw json.RawMessage) any {
	var review struct {
		Name           string         `json:"name"`
		CategoryLeafID string         `json:"categoryLeafId"`
		Pic            string         `json:"pic"`
		Description    string         `json:"description"`
		ProductFormat  map[string]any `json:"productFormat"`
		SpecInfo       map[string]any `json:"specInfo"`
		SpecPricesV2   []any          `json:"specPricesV2"`
		Commit         bool           `json:"commit"`
		StartSaleType  int            `json:"startSaleType"`
	}
	if json.Unmarshal(raw, &review) != nil {
		return map[string]any{}
	}
	return productionSafeValue(review)
}

func productionMappingForAPI(raw json.RawMessage) any {
	type image struct {
		PlatformImageURL string `json:"platformImageUrl,omitempty"`
		URL              string `json:"url,omitempty"`
	}
	type sku struct {
		LocalSKUID string         `json:"localSkuId,omitempty"`
		Name       string         `json:"name,omitempty"`
		Attrs      map[string]any `json:"attrs,omitempty"`
		Price      float64        `json:"price"`
		Stock      *int           `json:"stock,omitempty"`
	}
	var mapping struct {
		CategoryID   string  `json:"categoryId,omitempty"`
		CategoryPath string  `json:"categoryPath,omitempty"`
		Title        string  `json:"title,omitempty"`
		Description  string  `json:"description,omitempty"`
		MainImages   []image `json:"mainImages,omitempty"`
		DetailImages []image `json:"detailImages,omitempty"`
		SKUs         []sku   `json:"skus,omitempty"`
		Price        struct {
			Currency string   `json:"currency,omitempty"`
			Min      *float64 `json:"min,omitempty"`
			Max      *float64 `json:"max,omitempty"`
		} `json:"price,omitempty"`
		Stock struct {
			Total       *int `json:"total,omitempty"`
			Min         *int `json:"min,omitempty"`
			Unconfirmed bool `json:"unconfirmed"`
		} `json:"stock,omitempty"`
	}
	if json.Unmarshal(raw, &mapping) != nil {
		return map[string]any{}
	}
	if len(mapping.MainImages) > 20 {
		mapping.MainImages = mapping.MainImages[:20]
	}
	if len(mapping.DetailImages) > 50 {
		mapping.DetailImages = mapping.DetailImages[:50]
	}
	if len(mapping.SKUs) > 100 {
		mapping.SKUs = mapping.SKUs[:100]
	}
	return productionSafeValue(mapping)
}

func productionSafeValue(value any) any {
	raw, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	var decoded any
	if json.Unmarshal(raw, &decoded) != nil {
		return map[string]any{}
	}
	return redactSafeValue(decoded)
}

func approvalResponse(record ApprovalRecord) ApprovalResponse {
	return ApprovalResponse{ID: record.ID, Decision: record.Decision, DraftVersion: record.DraftVersion, DraftPayloadHash: record.DraftPayloadHash, ReviewerID: record.ReviewerID, Reason: record.Reason, Comment: record.Comment, RequestID: record.RequestID, CreatedAt: record.CreatedAt}
}

func attemptSummary(attempt ExecutionAttempt) AttemptSummaryResponse {
	return AttemptSummaryResponse{ID: attempt.ID, AttemptNumber: attempt.AttemptNumber, Status: attempt.Status, AdapterMode: attempt.AdapterMode, Platform: attempt.Platform, ApprovedDraftVersion: attempt.ApprovedDraftVersion, ApprovedDraftPayloadHash: attempt.ApprovedDraftPayloadHash, ExecutedDraftVersion: attempt.ExecutedDraftVersion, ExecutedDraftPayloadHash: attempt.ExecutedDraftPayloadHash, ResultType: attempt.ResultType, RequestID: attempt.RequestID, StartedAt: attempt.StartedAt, FinishedAt: attempt.FinishedAt, CreatedAt: attempt.CreatedAt}
}

func eventResponse(event OperationTaskEvent) EventResponse {
	return EventResponse{ID: event.ID, Sequence: event.Sequence, EventType: event.EventType, ActorType: event.ActorType, ActorID: event.ActorID, BeforeState: event.BeforeState, AfterState: event.AfterState, PlatformDraftID: event.PlatformDraftID, DraftVersion: event.DraftVersion, RequestID: event.RequestID, Reason: event.Reason, Metadata: decodeSafeJSON(event.Metadata), OccurredAt: event.OccurredAt}
}

func latestAttempt(db *gorm.DB, ctx context.Context, tenantID int64, taskID uuid.UUID) (*ExecutionAttempt, error) {
	var attempt ExecutionAttempt
	err := db.WithContext(ctx).Where("tenant_id = ? AND operation_task_id = ?", tenantID, taskID).Order("attempt_number DESC, id DESC").First(&attempt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, stableError(err, ErrConflict)
	}
	return &attempt, nil
}

func reviewerRoleForActor(role string) string {
	switch strings.TrimSpace(strings.ToLower(role)) {
	case "admin":
		return ReviewerRoleAdmin
	default:
		return ReviewerRoleReviewer
	}
}

func safeMetadataJSON(value map[string]any) datatypes.JSON {
	data, err := json.Marshal(value)
	if err != nil {
		return datatypes.JSON([]byte(`{}`))
	}
	return redactSafeJSON(datatypes.JSON(data))
}

func safeKeyHash(key string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(key)))
	return hex.EncodeToString(sum[:])
}
