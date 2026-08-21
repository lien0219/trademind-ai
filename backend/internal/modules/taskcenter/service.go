package taskcenter

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproducttext"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/customersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/pagination"
	"gorm.io/gorm"
)

// Service aggregates read models across task tables and dispatches retries.
type Service struct {
	DB             *gorm.DB
	Cfg            *config.Config
	OpLog          *operationlog.Service
	Settings       *settings.Service
	Collect        *collect.Service
	Image          *imagetask.Service
	OrderSync      *ordersync.Service
	CustomerSync   *customersync.Service
	ProductPublish *productpublish.Service
	Inventory      *inventory.Service
	AIProductText  *aiproducttext.Service
}

// ListFailureParams binds list query options.
type ListFailureParams struct {
	TaskType         string // single-type filter (empty=all)
	Status           string // raw status on source row
	NormalizedStatus string // filter mapped norm
	Platform         string
	ShopID           string
	Keyword          string
	IncludeResolved  bool
	IncludeMarked    bool // when false (default), hide rows with ignored OR handled marks
	RequireIgnored   bool
	RequireHandled   bool
	Start            *time.Time
	End              *time.Time
	Page             int
	PageSize         int
	Cursor           string
	Limit            int
	UseCursor        bool

	FailureCategory string
	Severity        string
	RecoveryStatus  string
	AllowedShopIDs  []uuid.UUID
	TenantID        int64
}

func taskCursorScope(p ListFailureParams) (filterFingerprint string, shopScopeHash string) {
	allowed := make([]string, 0, len(p.AllowedShopIDs))
	for _, id := range p.AllowedShopIDs {
		allowed = append(allowed, id.String())
	}
	sort.Strings(allowed)
	shopScope := strings.TrimSpace(p.ShopID)
	shopScopeHash = pagination.Fingerprint(map[string]any{
		"shopId":         shopScope,
		"allowedShopIds": allowed,
	})
	filterFingerprint = pagination.Fingerprint(map[string]any{
		"tenantId":         p.TenantID,
		"taskType":         p.TaskType,
		"status":           p.Status,
		"normalizedStatus": p.NormalizedStatus,
		"platform":         p.Platform,
		"shopId":           shopScope,
		"keyword":          p.Keyword,
		"includeResolved":  p.IncludeResolved,
		"includeMarked":    p.IncludeMarked,
		"requireIgnored":   p.RequireIgnored,
		"requireHandled":   p.RequireHandled,
		"start":            p.Start,
		"end":              p.End,
		"failureCategory":  p.FailureCategory,
		"severity":         p.Severity,
		"recoveryStatus":   p.RecoveryStatus,
		"allowedShopIds":   allowed,
		"sort":             "sort_key_desc_id_desc",
	})
	return filterFingerprint, shopScopeHash
}

func (s *Service) applyTenantListFilter(q *gorm.DB, p ListFailureParams) *gorm.DB {
	if p.TenantID > 0 {
		return q.Where("tenant_id = ?", p.TenantID)
	}
	return q
}

func (s *Service) summarizeGlobalMarks(ctx context.Context, tenantID int64) (ignored int64, handled int64, err error) {
	if s == nil || s.DB == nil {
		return 0, 0, fmt.Errorf("taskcenter: no db")
	}
	if err = s.DB.WithContext(ctx).Model(&TaskFailureMark{}).Where("tenant_id = ? AND mark_type = ?", tenantID, MarkIgnored).Count(&ignored).Error; err != nil {
		return 0, 0, err
	}
	if err = s.DB.WithContext(ctx).Model(&TaskFailureMark{}).Where("tenant_id = ? AND mark_type = ?", tenantID, MarkHandled).Count(&handled).Error; err != nil {
		return 0, 0, err
	}
	return ignored, handled, nil
}

func (s *Service) Summary(ctx context.Context, p ListFailureParams) (FailuresSummary, error) {
	merged, err := s.collectMerged(ctx, p, maxMergeFetchPerTbl)
	if err != nil {
		return FailuresSummary{}, err
	}
	su := FailuresSummary{
		ByType:     map[string]int64{},
		ByPlatform: map[string]int64{},
	}
	su.fillFromMerged(merged)
	ig, hd, err := s.summarizeGlobalMarks(ctx, p.TenantID)
	if err != nil {
		return FailuresSummary{}, err
	}
	su.IgnoredCount = ig
	su.HandledCount = hd
	return su, nil
}

func passesUnifiedFilters(d UnifiedTaskDTO, p ListFailureParams) bool {
	if !p.IncludeResolved && isResolvedFamily(d.NormalizedStatus) {
		return false
	}
	if !normMatchesFilter(d, p.NormalizedStatus) {
		return false
	}
	if !statusMatchesFilter(d, p.Status) {
		return false
	}
	if p.Platform != "" && !strings.EqualFold(strings.TrimSpace(p.Platform), strings.TrimSpace(d.Platform)) {
		return false
	}
	if p.ShopID != "" && !strings.EqualFold(strings.TrimSpace(p.ShopID), strings.TrimSpace(d.ShopID)) {
		return false
	}
	if len(p.AllowedShopIDs) > 0 {
		shopRaw := strings.TrimSpace(d.ShopID)
		if shopRaw == "" {
			return false
		}
		sid, err := uuid.Parse(shopRaw)
		if err != nil {
			return false
		}
		allowed := false
		for _, id := range p.AllowedShopIDs {
			if id == sid {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}
	if wf := strings.TrimSpace(p.FailureCategory); wf != "" && !strings.EqualFold(wf, strings.TrimSpace(d.FailureCategory)) {
		return false
	}
	if ws := strings.TrimSpace(p.Severity); ws != "" && !strings.EqualFold(ws, strings.TrimSpace(d.Severity)) {
		return false
	}
	if wr := strings.TrimSpace(p.RecoveryStatus); wr != "" && !strings.EqualFold(wr, strings.TrimSpace(d.RecoveryStatus)) {
		return false
	}
	if !keywordMatches(p.Keyword, d.Title, d.ErrorMessage, d.ID, d.Platform, d.ShopName, d.RelatedResourceTitle, d.FailureCategory) {
		return false
	}
	return true
}

func (s *Service) collectMerged(ctx context.Context, p ListFailureParams, perTypeLimit int) ([]UnifiedTaskDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("taskcenter: no db")
	}
	now := time.Now().UTC()
	types := taskTypesFor(p)
	if len(types) == 0 {
		return nil, fmt.Errorf("invalid taskType")
	}
	var merged []UnifiedTaskDTO
	for _, tt := range types {
		part, err := s.listOneType(ctx, tt, p, now, perTypeLimit, TaskSourceCursor{SourceID: tt})
		if err != nil {
			return nil, err
		}
		for i := range part {
			applyClassification(&part[i])
			d := part[i]
			if !passesUnifiedFilters(d, p) {
				continue
			}
			merged = append(merged, d)
		}
	}
	sortUnifiedDesc(merged)
	return merged, nil
}

func (s *Service) marksSubquery(tx *gorm.DB, taskType string, idSQL string, tenantID int64) *gorm.DB {
	return tx.Session(&gorm.Session{NewDB: true}).Table("task_failure_marks AS m").Select("1").
		Where("m.task_type = ?", taskType).
		Where("m.tenant_id = ?", tenantID).
		Where("m.source_id = "+idSQL).
		Where("m.mark_type IN ?", []string{MarkIgnored, MarkHandled})
}

func (s *Service) applyMarkFilters(db *gorm.DB, taskType string, idSQL string, p ListFailureParams) *gorm.DB {
	switch {
	case p.RequireIgnored:
		sub := db.Session(&gorm.Session{NewDB: true}).Table("task_failure_marks AS m").Select("1").
			Where("m.task_type = ?", taskType).
			Where("m.tenant_id = ?", p.TenantID).
			Where("m.source_id = "+idSQL).
			Where("m.mark_type = ?", MarkIgnored)
		db = db.Where("EXISTS (?)", sub)
	case p.RequireHandled:
		sub := db.Session(&gorm.Session{NewDB: true}).Table("task_failure_marks AS m").Select("1").
			Where("m.task_type = ?", taskType).
			Where("m.tenant_id = ?", p.TenantID).
			Where("m.source_id = "+idSQL).
			Where("m.mark_type = ?", MarkHandled)
		db = db.Where("EXISTS (?)", sub)
	default:
		if !p.IncludeMarked {
			db = db.Where("NOT EXISTS (?)", s.marksSubquery(s.DB, taskType, idSQL, p.TenantID))
		}
	}
	return db
}

func (s *Service) applyTimeRange(db *gorm.DB, p ListFailureParams) *gorm.DB {
	if p.Start != nil {
		db = db.Where("updated_at >= ?", *p.Start)
	}
	if p.End != nil {
		db = db.Where("updated_at <= ?", *p.End)
	}
	return db
}

func failureRowFilter(db *gorm.DB, now time.Time, includeResolved bool, trackRetryStale bool) *gorm.DB {
	if includeResolved {
		return db
	}
	q := `
		status IN ?
		OR (
			status = 'running'
			AND locked_until IS NOT NULL
			AND locked_by IS NOT NULL
			AND TRIM(locked_by) <> ''
			AND locked_until < ?
		)`
	args := []any{[]string{"failed", "retrying", "partial_success"}, now}
	if trackRetryStale {
		staleCut := now.Add(-staleRetryAfterDrift * time.Minute)
		q += `
		OR (
			status = 'retrying'
			AND next_retry_at IS NOT NULL
			AND next_retry_at < ?
		)`
		args = append(args, staleCut)
	}
	return db.Where("("+strings.TrimSpace(q)+")", args...)
}

func (s *Service) fetchMarks(ctx context.Context, tenantID int64, taskType string, ids []string) (markSet, error) {
	out := markSet{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []TaskFailureMark
	if err := s.DB.WithContext(ctx).
		Where("tenant_id = ? AND task_type = ? AND source_id IN ? AND mark_type IN ?", tenantID, taskType, ids, []string{MarkIgnored, MarkHandled}).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		k := markKey(rows[i].TaskType, rows[i].SourceID)
		mf := out[k]
		switch rows[i].MarkType {
		case MarkIgnored:
			mf.Ignored = true
		case MarkHandled:
			mf.Handled = true
		}
		out[k] = mf
	}
	return out, nil
}

func (s *Service) batchShopNames(ctx context.Context, ids []uuid.UUID) map[uuid.UUID]string {
	out := map[uuid.UUID]string{}
	if s == nil || s.DB == nil || len(ids) == 0 {
		return out
	}
	var rows []shop.Shop
	if err := s.DB.WithContext(ctx).
		Model(&shop.Shop{}).
		Select("id", "shop_name").
		Where("id IN ?", ids).
		Find(&rows).Error; err != nil {
		return out
	}
	for i := range rows {
		out[rows[i].ID] = rows[i].ShopName
	}
	return out
}

func (s *Service) batchProductTitles(ctx context.Context, ids []uuid.UUID) map[uuid.UUID]string {
	out := map[uuid.UUID]string{}
	if len(ids) == 0 {
		return out
	}
	var rows []product.Product
	if err := s.DB.WithContext(ctx).
		Model(&product.Product{}).Unscoped().
		Select("id", "title").
		Where("id IN ?", ids).
		Find(&rows).Error; err != nil {
		return out
	}
	for i := range rows {
		out[rows[i].ID] = rows[i].Title
	}
	return out
}

func clampPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func mergeFetchLimit(page, pageSize int) int {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	n := page * pageSize * 5
	if n < pageSize*15 {
		n = pageSize * 15
	}
	if n > maxMergeFetchPerTbl {
		n = maxMergeFetchPerTbl
	}
	return n
}

func normMatchesFilter(dto UnifiedTaskDTO, want string) bool {
	want = strings.TrimSpace(strings.ToLower(want))
	if want == "" {
		return true
	}
	return strings.EqualFold(dto.NormalizedStatus, want)
}

func statusMatchesFilter(dto UnifiedTaskDTO, want string) bool {
	want = strings.TrimSpace(strings.ToLower(want))
	if want == "" {
		return true
	}
	return strings.EqualFold(dto.Status, want)
}

func keywordMatches(kw string, fields ...string) bool {
	kw = strings.TrimSpace(strings.ToLower(kw))
	if kw == "" {
		return true
	}
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), kw) {
			return true
		}
	}
	return false
}

func taskTypesFor(p ListFailureParams) []string {
	all := []string{
		TaskTypeCollect, TaskTypeImage, TaskTypeOrderSync,
		TaskTypeCustomerMessageSync, TaskTypeProductPublish, TaskTypeInventorySync,
		TaskTypeAIText, TaskTypeAIImage, TaskTypeCustomerFailure,
	}
	tt := strings.TrimSpace(p.TaskType)
	if tt == "" {
		return all
	}
	for _, a := range all {
		if strings.EqualFold(a, tt) {
			return []string{a}
		}
	}
	return nil
}

func parseTaskType(s string) (string, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case TaskTypeCollect, TaskTypeImage, TaskTypeOrderSync,
		TaskTypeCustomerMessageSync, TaskTypeProductPublish, TaskTypeInventorySync, TaskTypeAIText, TaskTypeAIImage, TaskTypeCustomerFailure:
		return s, nil
	default:
		return "", fmt.Errorf("unknown taskType")
	}
}

func (s *Service) ListFailures(ctx context.Context, p ListFailureParams) (ListFailuresResult, error) {
	var zero ListFailuresResult
	if s == nil || s.DB == nil {
		return zero, fmt.Errorf("taskcenter: no db")
	}
	if p.UseCursor && p.Limit > 0 {
		p.PageSize = p.Limit
	}
	page, pageSize := clampPage(p.Page, p.PageSize)
	p.Page, p.PageSize = page, pageSize
	types := taskTypesFor(p)
	if len(types) == 0 {
		return zero, fmt.Errorf("invalid taskType")
	}

	filterFingerprint, shopScopeHash := taskCursorScope(p)
	var mergeCur *TaskMergeCursorPayload
	if p.UseCursor && strings.TrimSpace(p.Cursor) != "" {
		decoded, err := decodeTaskMergeCursor(p.Cursor, p.TenantID, shopScopeHash, filterFingerprint)
		if err != nil {
			return zero, err
		}
		mergeCur = &decoded
	}

	limit := pageSize
	if p.UseCursor {
		merged, sourceCursors, hasMore, err := s.mergeTaskSources(ctx, p, mergeCur, limit)
		if err != nil {
			return zero, err
		}
		if err := s.attachAlertStatuses(ctx, merged); err != nil {
			return zero, err
		}
		summary := FailuresSummary{ByType: map[string]int64{}, ByPlatform: map[string]int64{}}
		summary.fillFromMerged(merged)
		nextCursor, err := buildNextMergeCursor(hasMore, p.TenantID, shopScopeHash, filterFingerprint, sourceCursors, types)
		if err != nil {
			return zero, err
		}
		return ListFailuresResult{
			List:       merged,
			Total:      int64(len(merged)),
			Summary:    summary,
			Limit:      pageSize,
			NextCursor: nextCursor,
			HasMore:    hasMore,
		}, nil
	}

	if _, err := pagination.NormalizePage(page, pageSize); err != nil {
		return zero, err
	}
	merged, err := s.collectMerged(ctx, p, mergeFetchLimit(page, pageSize))
	if err != nil {
		return zero, err
	}
	if err := s.attachAlertStatuses(ctx, merged); err != nil {
		return zero, err
	}
	summary := FailuresSummary{ByType: map[string]int64{}, ByPlatform: map[string]int64{}}
	summary.fillFromMerged(merged)

	start := (page - 1) * pageSize
	end := start + pageSize
	if start > len(merged) {
		start = len(merged)
	}
	if end > len(merged) {
		end = len(merged)
	}
	pageRows := merged[start:end]

	return ListFailuresResult{
		List:    pageRows,
		Total:   int64(len(merged)),
		Summary: summary,
		Limit:   pageSize,
		HasMore: end < len(merged),
	}, nil
}

func (su *FailuresSummary) fillFromMerged(rows []UnifiedTaskDTO) {
	if su == nil {
		return
	}
	if su.ByType == nil {
		su.ByType = map[string]int64{}
	}
	if su.ByPlatform == nil {
		su.ByPlatform = map[string]int64{}
	}
	var latest *time.Time
	for i := range rows {
		d := rows[i]
		su.ByType[d.TaskType]++
		if strings.TrimSpace(d.Platform) != "" {
			su.ByPlatform[d.Platform]++
		}
		if d.Retryable {
			su.RetryableCount++
		}
		switch d.NormalizedStatus {
		case NormFailed:
			su.TotalFailed++
			if d.FinishedAt != nil {
				if latest == nil || d.FinishedAt.After(*latest) {
					t := *d.FinishedAt
					latest = &t
				}
			} else if latest == nil || d.UpdatedAt.After(*latest) {
				t := d.UpdatedAt
				latest = &t
			}
		case NormRetrying:
			su.RetryingTotal++
		case NormStale:
			su.StaleTotal++
		case NormLeaseExpired:
			su.LeaseExpiredTotal++
		}
	}
	su.LatestFailedAt = latest
}
