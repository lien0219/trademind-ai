package operationtask

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/pagination"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OperationTaskRepository struct {
	DB *gorm.DB
}

// OperationTaskListSummary contains an operation task and its latest list
// metadata loaded under the same tenant-scoped query.
type OperationTaskListSummary struct {
	Task                  OperationTask
	LatestDraftVersion    int
	LatestExecutionStatus string
}

// OperationTaskListSummaryResult is the paginated result for list responses
// that include latest draft and execution state.
type OperationTaskListSummaryResult struct {
	Items      []OperationTaskListSummary
	Limit      int
	HasMore    bool
	NextCursor string
}

type operationTaskListSummaryRow struct {
	OperationTask        `gorm:"embedded"`
	LatestDraftVersion   *int    `gorm:"column:latest_draft_version"`
	LatestExecutionState *string `gorm:"column:latest_execution_status"`
}

func NewOperationTaskRepository(db *gorm.DB) *OperationTaskRepository {
	return &OperationTaskRepository{DB: db}
}

func (r *OperationTaskRepository) Create(ctx context.Context, task *OperationTask) error {
	if r == nil || r.DB == nil {
		return fmt.Errorf("operation task repository: db is nil")
	}
	if err := validateOperationTask(task); err != nil {
		return err
	}
	if err := r.DB.WithContext(ctx).Create(task).Error; err != nil {
		if isUniqueViolation(err) {
			return stableError(err, ErrDuplicateIdempotencyKey)
		}
		return stableError(err, ErrConflict)
	}
	return nil
}

func (r *OperationTaskRepository) GetByID(ctx context.Context, tenantID int64, id uuid.UUID) (*OperationTask, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("operation task repository: db is nil")
	}
	var task OperationTask
	if err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrConflict)
	}
	return &task, nil
}

func (r *OperationTaskRepository) GetByIdempotencyKey(ctx context.Context, tenantID int64, key string) (*OperationTask, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("operation task repository: db is nil")
	}
	key = strings.TrimSpace(key)
	if tenantID < 0 || key == "" {
		return nil, ErrValidation
	}
	var task OperationTask
	if err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND idempotency_key = ?", tenantID, key).
		First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrConflict)
	}
	return &task, nil
}

func (r *OperationTaskRepository) List(ctx context.Context, p OperationTaskListParams) (OperationTaskListResult, error) {
	var zero OperationTaskListResult
	q, limit, scopeHash, err := r.listQuery(ctx, p)
	if err != nil {
		return zero, err
	}

	var rows []OperationTask
	if err := q.Order("updated_at DESC, id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return zero, stableError(err, ErrConflict)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	next, err := nextOperationTaskCursor(p.TenantID, scopeHash, hasMore, rows)
	if err != nil {
		return zero, err
	}
	return OperationTaskListResult{Items: rows, Limit: limit, HasMore: hasMore, NextCursor: next}, nil
}

func (r *OperationTaskRepository) ListWithLatestSummaries(ctx context.Context, p OperationTaskListParams) (OperationTaskListSummaryResult, error) {
	var zero OperationTaskListSummaryResult
	q, limit, scopeHash, err := r.listQuery(ctx, p)
	if err != nil {
		return zero, err
	}
	q = q.Select(`operation_tasks.*,
		(SELECT pd.draft_version FROM platform_drafts AS pd
		 WHERE pd.tenant_id = operation_tasks.tenant_id
		   AND pd.operation_task_id = operation_tasks.id
		 ORDER BY pd.draft_version DESC, pd.id DESC LIMIT 1) AS latest_draft_version,
		(SELECT ea.status FROM execution_attempts AS ea
		 WHERE ea.tenant_id = operation_tasks.tenant_id
		   AND ea.operation_task_id = operation_tasks.id
		 ORDER BY ea.attempt_number DESC, ea.id DESC LIMIT 1) AS latest_execution_status`)
	var rows []operationTaskListSummaryRow
	if err := q.Order("updated_at DESC, id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return zero, stableError(err, ErrConflict)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	tasks := make([]OperationTask, 0, len(rows))
	items := make([]OperationTaskListSummary, 0, len(rows))
	for _, row := range rows {
		tasks = append(tasks, row.OperationTask)
		item := OperationTaskListSummary{Task: row.OperationTask}
		if row.LatestDraftVersion != nil {
			item.LatestDraftVersion = *row.LatestDraftVersion
		}
		if row.LatestExecutionState != nil {
			item.LatestExecutionStatus = *row.LatestExecutionState
		}
		items = append(items, item)
	}
	next, err := nextOperationTaskCursor(p.TenantID, scopeHash, hasMore, tasks)
	if err != nil {
		return zero, err
	}
	return OperationTaskListSummaryResult{Items: items, Limit: limit, HasMore: hasMore, NextCursor: next}, nil
}

func (r *OperationTaskRepository) listQuery(ctx context.Context, p OperationTaskListParams) (*gorm.DB, int, string, error) {
	if r == nil || r.DB == nil {
		return nil, 0, "", fmt.Errorf("operation task repository: db is nil")
	}
	if p.TenantID < 0 {
		return nil, 0, "", ErrValidation
	}
	limit := p.Limit
	if limit <= 0 {
		limit = pagination.DefaultLimit
	}
	if limit > pagination.MaxLimit {
		limit = pagination.MaxLimit
	}
	scopeHash := operationTaskListScopeHash(p)
	var cur pagination.CursorPayload
	if strings.TrimSpace(p.Cursor) != "" {
		decoded, err := pagination.DecodeCursor(p.Cursor, p.TenantID, "", scopeHash)
		if err != nil {
			return nil, 0, "", err
		}
		cur = decoded
	}
	q := r.DB.WithContext(ctx).Model(&OperationTask{}).Where("tenant_id = ?", p.TenantID)
	if p.Status = strings.TrimSpace(strings.ToLower(p.Status)); p.Status != "" {
		q = q.Where("status = ?", p.Status)
	}
	if p.Platform = strings.TrimSpace(strings.ToLower(p.Platform)); p.Platform != "" {
		q = q.Where("platform = ?", p.Platform)
	}
	if p.TaskType = strings.TrimSpace(strings.ToLower(p.TaskType)); p.TaskType != "" {
		q = q.Where("task_type = ?", p.TaskType)
	}
	q, err := pagination.ApplyDescKeyset(q, "updated_at", "id", cur)
	if err != nil {
		return nil, 0, "", err
	}
	return q, limit, scopeHash, nil
}

func nextOperationTaskCursor(tenantID int64, scopeHash string, hasMore bool, rows []OperationTask) (string, error) {
	if !hasMore || len(rows) == 0 {
		return "", nil
	}
	last := rows[len(rows)-1]
	return pagination.BuildNextCursor(true, tenantID, "", scopeHash, "updated_at", last.UpdatedAt, last.ID.String())
}

func (r *OperationTaskRepository) UpdateRevision(ctx context.Context, tenantID int64, id uuid.UUID, expectedRevision int, patch OperationTaskPatch) (*OperationTask, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("operation task repository: db is nil")
	}
	if tenantID < 0 || expectedRevision < 1 {
		return nil, ErrValidation
	}
	now := utcNow()
	updates := map[string]any{
		"revision":   gorm.Expr("revision + 1"),
		"updated_at": now,
	}
	if patch.Title != nil {
		title := strings.TrimSpace(*patch.Title)
		if title == "" {
			return nil, ErrValidation
		}
		updates["title"] = title
	}
	if patch.Summary != nil {
		updates["summary"] = strings.TrimSpace(*patch.Summary)
	}
	if patch.Payload != nil {
		if !isValidJSON(*patch.Payload) || payloadHasSecret(*patch.Payload) {
			return nil, ErrValidation
		}
		updates["payload"] = *patch.Payload
	}
	if patch.Status != nil {
		status := strings.TrimSpace(strings.ToLower(*patch.Status))
		if !allowedOperationTaskStatuses[status] {
			return nil, ErrValidation
		}
		updates["status"] = status
	}
	if patch.Priority != nil {
		priority := strings.TrimSpace(strings.ToLower(*patch.Priority))
		if !allowedPriorities[priority] {
			return nil, ErrValidation
		}
		updates["priority"] = priority
	}
	if patch.UpdatedBy != nil {
		updates["updated_by"] = patch.UpdatedBy
	}

	res := r.DB.WithContext(ctx).Model(&OperationTask{}).
		Where("tenant_id = ? AND id = ? AND revision = ?", tenantID, id, expectedRevision).
		Updates(updates)
	if res.Error != nil {
		return nil, stableError(res.Error, ErrConflict)
	}
	if res.RowsAffected == 0 {
		var exists int64
		if err := r.DB.WithContext(ctx).Model(&OperationTask{}).Where("tenant_id = ? AND id = ?", tenantID, id).Count(&exists).Error; err != nil {
			return nil, stableError(err, ErrConflict)
		}
		if exists == 0 {
			return nil, ErrNotFound
		}
		return nil, ErrRevisionConflict
	}
	return r.GetByID(ctx, tenantID, id)
}

func operationTaskListScopeHash(p OperationTaskListParams) string {
	return pagination.Fingerprint(map[string]any{
		"tenantId": p.TenantID,
		"status":   p.Status,
		"platform": p.Platform,
		"taskType": p.TaskType,
		"sort":     "updated_at_desc_id_desc",
	})
}

type PlatformDraftRepository struct {
	DB *gorm.DB
}

func NewPlatformDraftRepository(db *gorm.DB) *PlatformDraftRepository {
	return &PlatformDraftRepository{DB: db}
}

func (r *PlatformDraftRepository) CreateVersion(ctx context.Context, draft *PlatformDraft) error {
	if r == nil || r.DB == nil {
		return fmt.Errorf("platform draft repository: db is nil")
	}
	if err := validatePlatformDraft(draft); err != nil {
		return err
	}
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task OperationTask
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", draft.OperationTaskID).
			First(&task).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return stableError(err, ErrConflict)
		}
		if task.TenantID != draft.TenantID {
			return ErrTenantMismatch
		}
		if !strings.EqualFold(task.Platform, draft.Platform) {
			return ErrValidation
		}
		if err := tx.Create(draft).Error; err != nil {
			if isUniqueViolation(err) {
				return stableError(err, ErrDuplicateDraftVersion)
			}
			return stableError(err, ErrConflict)
		}
		return nil
	})
}

func (r *PlatformDraftRepository) GetByID(ctx context.Context, tenantID int64, id uuid.UUID) (*PlatformDraft, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("platform draft repository: db is nil")
	}
	var draft PlatformDraft
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&draft).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrConflict)
	}
	return &draft, nil
}

func (r *PlatformDraftRepository) GetVersion(ctx context.Context, tenantID int64, taskID uuid.UUID, version int) (*PlatformDraft, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("platform draft repository: db is nil")
	}
	var draft PlatformDraft
	if err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND operation_task_id = ? AND draft_version = ?", tenantID, taskID, version).
		First(&draft).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrConflict)
	}
	return &draft, nil
}

func (r *PlatformDraftRepository) GetLatest(ctx context.Context, tenantID int64, taskID uuid.UUID) (*PlatformDraft, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("platform draft repository: db is nil")
	}
	var draft PlatformDraft
	if err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND operation_task_id = ?", tenantID, taskID).
		Order("draft_version DESC").
		First(&draft).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrConflict)
	}
	return &draft, nil
}

func (r *PlatformDraftRepository) ListVersions(ctx context.Context, tenantID int64, taskID uuid.UUID) ([]PlatformDraft, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("platform draft repository: db is nil")
	}
	var drafts []PlatformDraft
	if err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND operation_task_id = ?", tenantID, taskID).
		Order("draft_version DESC, id DESC").
		Find(&drafts).Error; err != nil {
		return nil, stableError(err, ErrConflict)
	}
	return drafts, nil
}

func (r *PlatformDraftRepository) ListLatestByTasks(ctx context.Context, tenantID int64, taskIDs []uuid.UUID) (map[uuid.UUID]PlatformDraft, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("platform draft repository: db is nil")
	}
	out := make(map[uuid.UUID]PlatformDraft, len(taskIDs))
	if tenantID < 0 {
		return nil, ErrValidation
	}
	if len(taskIDs) == 0 {
		return out, nil
	}
	latest := r.DB.WithContext(ctx).Model(&PlatformDraft{}).
		Select("operation_task_id, MAX(draft_version) AS max_version").
		Where("tenant_id = ? AND operation_task_id IN ?", tenantID, taskIDs).
		Group("operation_task_id")
	var drafts []PlatformDraft
	err := r.DB.WithContext(ctx).Model(&PlatformDraft{}).
		Joins("JOIN (?) AS latest ON latest.operation_task_id = platform_drafts.operation_task_id AND latest.max_version = platform_drafts.draft_version", latest).
		Where("platform_drafts.tenant_id = ?", tenantID).
		Find(&drafts).Error
	if err != nil {
		return nil, stableError(err, ErrConflict)
	}
	for _, draft := range drafts {
		out[draft.OperationTaskID] = draft
	}
	return out, nil
}

type ApprovalRecordRepository struct {
	DB *gorm.DB
}

func NewApprovalRecordRepository(db *gorm.DB) *ApprovalRecordRepository {
	return &ApprovalRecordRepository{DB: db}
}

func (r *ApprovalRecordRepository) CreateDecision(ctx context.Context, record *ApprovalRecord) error {
	if r == nil || r.DB == nil {
		return fmt.Errorf("approval record repository: db is nil")
	}
	if err := validateApprovalRecord(record); err != nil {
		return err
	}
	if existing, err := r.getExistingIdempotency(ctx, record); err == nil {
		*record = *existing
		return nil
	} else if !errors.Is(err, ErrNotFound) {
		return err
	}
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateTaskDraftReference(tx, record.TenantID, record.OperationTaskID, record.PlatformDraftID, record.DraftVersion, record.DraftPayloadHash); err != nil {
			return err
		}
		if err := tx.Create(record).Error; err != nil {
			if isUniqueViolation(err) && record.IdempotencyKey != nil {
				existing, lookupErr := NewApprovalRecordRepository(tx).getExistingIdempotency(ctx, record)
				if lookupErr == nil {
					*record = *existing
					return nil
				}
				return stableError(lookupErr, ErrDuplicateApprovalIdem)
			}
			return stableError(err, ErrConflict)
		}
		return nil
	})
}

func (r *ApprovalRecordRepository) getExistingIdempotency(ctx context.Context, record *ApprovalRecord) (*ApprovalRecord, error) {
	if record == nil || record.IdempotencyKey == nil {
		return nil, ErrNotFound
	}
	return r.GetByIdempotencyKey(ctx, record.TenantID, record.OperationTaskID, *record.IdempotencyKey)
}

func (r *ApprovalRecordRepository) GetByID(ctx context.Context, tenantID int64, id uuid.UUID) (*ApprovalRecord, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("approval record repository: db is nil")
	}
	var record ApprovalRecord
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrConflict)
	}
	return &record, nil
}

func (r *ApprovalRecordRepository) GetByIdempotencyKey(ctx context.Context, tenantID int64, taskID uuid.UUID, key string) (*ApprovalRecord, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("approval record repository: db is nil")
	}
	key = strings.TrimSpace(key)
	if tenantID < 0 || taskID == uuid.Nil || key == "" {
		return nil, ErrValidation
	}
	var record ApprovalRecord
	if err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND operation_task_id = ? AND idempotency_key = ?", tenantID, taskID, key).
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrConflict)
	}
	return &record, nil
}

func (r *ApprovalRecordRepository) GetLatestByTask(ctx context.Context, tenantID int64, taskID uuid.UUID) (*ApprovalRecord, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("approval record repository: db is nil")
	}
	var record ApprovalRecord
	if err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND operation_task_id = ?", tenantID, taskID).
		Order("created_at DESC, id DESC").
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrConflict)
	}
	return &record, nil
}

func (r *ApprovalRecordRepository) ListByTask(ctx context.Context, tenantID int64, taskID uuid.UUID) ([]ApprovalRecord, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("approval record repository: db is nil")
	}
	var records []ApprovalRecord
	if err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND operation_task_id = ?", tenantID, taskID).
		Order("created_at DESC, id DESC").
		Find(&records).Error; err != nil {
		return nil, stableError(err, ErrConflict)
	}
	return records, nil
}

type ExecutionAttemptRepository struct {
	DB *gorm.DB
}

func NewExecutionAttemptRepository(db *gorm.DB) *ExecutionAttemptRepository {
	return &ExecutionAttemptRepository{DB: db}
}

func (r *ExecutionAttemptRepository) CreateAttempt(ctx context.Context, attempt *ExecutionAttempt) error {
	if r == nil || r.DB == nil {
		return fmt.Errorf("execution attempt repository: db is nil")
	}
	if err := validateExecutionAttempt(attempt); err != nil {
		return err
	}
	if existing, err := r.getExistingIdempotency(ctx, attempt); err == nil {
		*attempt = *existing
		return nil
	} else if !errors.Is(err, ErrNotFound) {
		return err
	}
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateAttemptReferences(tx, attempt); err != nil {
			return err
		}
		if err := tx.Create(attempt).Error; err != nil {
			if isUniqueViolation(err) {
				if attempt.IdempotencyKey != nil {
					existing, lookupErr := NewExecutionAttemptRepository(tx).getExistingIdempotency(ctx, attempt)
					if lookupErr == nil {
						*attempt = *existing
						return nil
					}
				}
				return ErrDuplicateAttemptNumber
			}
			return stableError(err, ErrConflict)
		}
		return nil
	})
}

func (r *ExecutionAttemptRepository) getExistingIdempotency(ctx context.Context, attempt *ExecutionAttempt) (*ExecutionAttempt, error) {
	if attempt == nil || attempt.IdempotencyKey == nil {
		return nil, ErrNotFound
	}
	return r.GetByIdempotencyKey(ctx, attempt.TenantID, attempt.OperationTaskID, *attempt.IdempotencyKey)
}

func (r *ExecutionAttemptRepository) GetByID(ctx context.Context, tenantID int64, id uuid.UUID) (*ExecutionAttempt, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("execution attempt repository: db is nil")
	}
	var attempt ExecutionAttempt
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&attempt).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrConflict)
	}
	return &attempt, nil
}

func (r *ExecutionAttemptRepository) GetByIdempotencyKey(ctx context.Context, tenantID int64, taskID uuid.UUID, key string) (*ExecutionAttempt, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("execution attempt repository: db is nil")
	}
	key = strings.TrimSpace(key)
	if tenantID < 0 || taskID == uuid.Nil || key == "" {
		return nil, ErrValidation
	}
	var attempt ExecutionAttempt
	if err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND operation_task_id = ? AND idempotency_key = ?", tenantID, taskID, key).
		First(&attempt).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrConflict)
	}
	return &attempt, nil
}

func (r *ExecutionAttemptRepository) GetByAttemptNumber(ctx context.Context, tenantID int64, taskID uuid.UUID, attemptNumber int) (*ExecutionAttempt, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("execution attempt repository: db is nil")
	}
	var attempt ExecutionAttempt
	if err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND operation_task_id = ? AND attempt_number = ?", tenantID, taskID, attemptNumber).
		First(&attempt).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrConflict)
	}
	return &attempt, nil
}

func (r *ExecutionAttemptRepository) ListByTask(ctx context.Context, tenantID int64, taskID uuid.UUID) ([]ExecutionAttempt, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("execution attempt repository: db is nil")
	}
	var attempts []ExecutionAttempt
	if err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND operation_task_id = ?", tenantID, taskID).
		Order("attempt_number ASC, id ASC").
		Find(&attempts).Error; err != nil {
		return nil, stableError(err, ErrConflict)
	}
	return attempts, nil
}

func (r *ExecutionAttemptRepository) ListLatestByTasks(ctx context.Context, tenantID int64, taskIDs []uuid.UUID) (map[uuid.UUID]ExecutionAttempt, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("execution attempt repository: db is nil")
	}
	out := make(map[uuid.UUID]ExecutionAttempt, len(taskIDs))
	if tenantID < 0 {
		return nil, ErrValidation
	}
	if len(taskIDs) == 0 {
		return out, nil
	}
	latest := r.DB.WithContext(ctx).Model(&ExecutionAttempt{}).
		Select("operation_task_id, MAX(attempt_number) AS max_attempt_number").
		Where("tenant_id = ? AND operation_task_id IN ?", tenantID, taskIDs).
		Group("operation_task_id")
	var attempts []ExecutionAttempt
	err := r.DB.WithContext(ctx).Model(&ExecutionAttempt{}).
		Joins("JOIN (?) AS latest ON latest.operation_task_id = execution_attempts.operation_task_id AND latest.max_attempt_number = execution_attempts.attempt_number", latest).
		Where("execution_attempts.tenant_id = ?", tenantID).
		Find(&attempts).Error
	if err != nil {
		return nil, stableError(err, ErrConflict)
	}
	for _, attempt := range attempts {
		out[attempt.OperationTaskID] = attempt
	}
	return out, nil
}

func (r *ExecutionAttemptRepository) UpdateLifecycle(ctx context.Context, tenantID int64, id uuid.UUID, expectedRevision int, patch ExecutionAttemptLifecyclePatch) (*ExecutionAttempt, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("execution attempt repository: db is nil")
	}
	if tenantID < 0 || id == uuid.Nil || expectedRevision < 1 {
		return nil, ErrValidation
	}
	updates := map[string]any{
		"revision":   gorm.Expr("revision + 1"),
		"updated_at": utcNow(),
	}
	if patch.Status != nil {
		status := strings.TrimSpace(strings.ToLower(*patch.Status))
		if !allowedExecutionAttemptStatuses[status] {
			return nil, ErrValidation
		}
		updates["status"] = status
	}
	if patch.ExecutedDraftVersion != nil {
		if *patch.ExecutedDraftVersion < 1 {
			return nil, ErrValidation
		}
		updates["executed_draft_version"] = *patch.ExecutedDraftVersion
	}
	if patch.ExecutedDraftPayloadHash != nil {
		hash := strings.TrimSpace(strings.ToLower(*patch.ExecutedDraftPayloadHash))
		if !sha256LowerHex.MatchString(hash) {
			return nil, ErrValidation
		}
		updates["executed_draft_payload_hash"] = hash
	}
	if patch.ResultType != nil {
		resultType := strings.TrimSpace(strings.ToLower(*patch.ResultType))
		if !allowedExecutionResultTypes[resultType] {
			return nil, ErrValidation
		}
		updates["result_type"] = resultType
	}
	if patch.ExternalReference != nil {
		updates["external_reference"] = strings.TrimSpace(*patch.ExternalReference)
	}
	if patch.SafeMetadata != nil {
		metadata := redactSafeJSON(*patch.SafeMetadata)
		if !isValidJSON(metadata) || payloadHasSecret(metadata) {
			return nil, ErrValidation
		}
		updates["safe_metadata"] = metadata
	}
	if patch.StartedAt != nil {
		updates["started_at"] = patch.StartedAt.UTC()
	}
	if patch.FinishedAt != nil {
		updates["finished_at"] = patch.FinishedAt.UTC()
	}
	res := r.DB.WithContext(ctx).Model(&ExecutionAttempt{}).
		Where("tenant_id = ? AND id = ? AND revision = ?", tenantID, id, expectedRevision).
		Updates(updates)
	if res.Error != nil {
		return nil, stableError(res.Error, ErrConflict)
	}
	if res.RowsAffected == 0 {
		var exists int64
		if err := r.DB.WithContext(ctx).Model(&ExecutionAttempt{}).Where("tenant_id = ? AND id = ?", tenantID, id).Count(&exists).Error; err != nil {
			return nil, stableError(err, ErrConflict)
		}
		if exists == 0 {
			return nil, ErrNotFound
		}
		return nil, ErrRevisionConflict
	}
	return r.GetByID(ctx, tenantID, id)
}

type ExecutionErrorRepository struct {
	DB *gorm.DB
}

func NewExecutionErrorRepository(db *gorm.DB) *ExecutionErrorRepository {
	return &ExecutionErrorRepository{DB: db}
}

func (r *ExecutionErrorRepository) AppendError(ctx context.Context, executionError *ExecutionError) error {
	if r == nil || r.DB == nil {
		return fmt.Errorf("execution error repository: db is nil")
	}
	executionError.Details = redactSafeJSON(executionError.Details)
	if err := validateExecutionError(executionError); err != nil {
		return err
	}
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var attempt ExecutionAttempt
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND id = ?", executionError.TenantID, executionError.ExecutionAttemptID).
			First(&attempt).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return stableError(err, ErrConflict)
		}
		if err := tx.Create(executionError).Error; err != nil {
			if isUniqueViolation(err) {
				return ErrDuplicateErrorSequence
			}
			return stableError(err, ErrConflict)
		}
		return nil
	})
}

func (r *ExecutionErrorRepository) GetByID(ctx context.Context, tenantID int64, id uuid.UUID) (*ExecutionError, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("execution error repository: db is nil")
	}
	var executionError ExecutionError
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&executionError).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrConflict)
	}
	return &executionError, nil
}

func (r *ExecutionErrorRepository) ListByAttempt(ctx context.Context, tenantID int64, attemptID uuid.UUID) ([]ExecutionError, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("execution error repository: db is nil")
	}
	var errorsList []ExecutionError
	if err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND execution_attempt_id = ?", tenantID, attemptID).
		Order("sequence ASC, id ASC").
		Find(&errorsList).Error; err != nil {
		return nil, stableError(err, ErrConflict)
	}
	return errorsList, nil
}

func (r *ExecutionErrorRepository) GetLatestByAttempt(ctx context.Context, tenantID int64, attemptID uuid.UUID) (*ExecutionError, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("execution error repository: db is nil")
	}
	var executionError ExecutionError
	if err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND execution_attempt_id = ?", tenantID, attemptID).
		Order("sequence DESC, id DESC").
		First(&executionError).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrConflict)
	}
	return &executionError, nil
}

type OperationTaskEventRepository struct {
	DB *gorm.DB
}

func NewOperationTaskEventRepository(db *gorm.DB) *OperationTaskEventRepository {
	return &OperationTaskEventRepository{DB: db}
}

func (r *OperationTaskEventRepository) AppendEvent(ctx context.Context, event *OperationTaskEvent) error {
	if r == nil || r.DB == nil {
		return fmt.Errorf("operation task event repository: db is nil")
	}
	event.Metadata = redactSafeJSON(event.Metadata)
	if err := validateOperationTaskEvent(event); err != nil {
		return err
	}
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task OperationTask
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND id = ?", event.TenantID, event.OperationTaskID).
			First(&task).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return stableError(err, ErrConflict)
		}
		if event.PlatformDraftID != nil {
			var draft PlatformDraft
			err := tx.Where("tenant_id = ? AND id = ?", event.TenantID, *event.PlatformDraftID).First(&draft).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			if err != nil {
				return stableError(err, ErrConflict)
			}
			if draft.OperationTaskID != event.OperationTaskID || draft.DraftVersion != event.DraftVersion {
				return ErrReferenceMismatch
			}
		}
		if err := tx.Create(event).Error; err != nil {
			if isUniqueViolation(err) {
				return ErrDuplicateEventSequence
			}
			return stableError(err, ErrConflict)
		}
		return nil
	})
}

func (r *OperationTaskEventRepository) GetByID(ctx context.Context, tenantID int64, id uuid.UUID) (*OperationTaskEvent, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("operation task event repository: db is nil")
	}
	var event OperationTaskEvent
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&event).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrConflict)
	}
	return &event, nil
}

func (r *OperationTaskEventRepository) GetBySequence(ctx context.Context, tenantID int64, taskID uuid.UUID, sequence int) (*OperationTaskEvent, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("operation task event repository: db is nil")
	}
	var event OperationTaskEvent
	if err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND operation_task_id = ? AND sequence = ?", tenantID, taskID, sequence).
		First(&event).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrConflict)
	}
	return &event, nil
}

func (r *OperationTaskEventRepository) ListByTask(ctx context.Context, p OperationTaskEventListParams) (OperationTaskEventListResult, error) {
	var zero OperationTaskEventListResult
	if r == nil || r.DB == nil {
		return zero, fmt.Errorf("operation task event repository: db is nil")
	}
	if p.TenantID < 0 || p.OperationTaskID == uuid.Nil || p.AfterSequence < 0 {
		return zero, ErrValidation
	}
	limit := p.Limit
	if limit <= 0 {
		limit = pagination.DefaultLimit
	}
	if limit > pagination.MaxLimit {
		limit = pagination.MaxLimit
	}
	var events []OperationTaskEvent
	err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND operation_task_id = ? AND sequence > ?", p.TenantID, p.OperationTaskID, p.AfterSequence).
		Order("sequence ASC, id ASC").
		Limit(limit + 1).
		Find(&events).Error
	if err != nil {
		return zero, stableError(err, ErrConflict)
	}
	hasMore := len(events) > limit
	if hasMore {
		events = events[:limit]
	}
	nextSequence := 0
	if hasMore && len(events) > 0 {
		nextSequence = events[len(events)-1].Sequence
	}
	return OperationTaskEventListResult{Items: events, Limit: limit, HasMore: hasMore, NextSequence: nextSequence}, nil
}

func (r *OperationTaskEventRepository) GetLatestByTask(ctx context.Context, tenantID int64, taskID uuid.UUID) (*OperationTaskEvent, error) {
	if r == nil || r.DB == nil {
		return nil, fmt.Errorf("operation task event repository: db is nil")
	}
	var event OperationTaskEvent
	if err := r.DB.WithContext(ctx).
		Where("tenant_id = ? AND operation_task_id = ?", tenantID, taskID).
		Order("sequence DESC, id DESC").
		First(&event).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, stableError(err, ErrConflict)
	}
	return &event, nil
}

func validateTaskDraftReference(tx *gorm.DB, tenantID int64, taskID uuid.UUID, draftID uuid.UUID, draftVersion int, payloadHash string) error {
	var task OperationTask
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, taskID).First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return stableError(err, ErrConflict)
	}
	var draft PlatformDraft
	err = tx.Where("tenant_id = ? AND id = ?", tenantID, draftID).First(&draft).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return stableError(err, ErrConflict)
	}
	if draft.OperationTaskID != taskID || draft.DraftVersion != draftVersion || draft.PayloadHash != payloadHash {
		return ErrReferenceMismatch
	}
	return nil
}

func validateAttemptReferences(tx *gorm.DB, attempt *ExecutionAttempt) error {
	if err := validateTaskDraftReference(tx, attempt.TenantID, attempt.OperationTaskID, attempt.PlatformDraftID, attempt.ExecutedDraftVersion, attempt.ExecutedDraftPayloadHash); err != nil {
		return err
	}
	var approval ApprovalRecord
	err := tx.Where("tenant_id = ? AND id = ?", attempt.TenantID, attempt.ApprovalRecordID).First(&approval).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return stableError(err, ErrConflict)
	}
	if approval.OperationTaskID != attempt.OperationTaskID ||
		approval.PlatformDraftID != attempt.PlatformDraftID ||
		approval.DraftVersion != attempt.ApprovedDraftVersion ||
		approval.DraftPayloadHash != attempt.ApprovedDraftPayloadHash {
		return ErrReferenceMismatch
	}
	return nil
}
