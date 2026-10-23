package operationlog

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/authutil"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/pagination"
	"github.com/trademind-ai/trademind/backend/internal/pkg/runtimediag"
	"gorm.io/gorm"
)

// WriteOpts is a single audit row to append.
type WriteOpts struct {
	TenantID    int64
	AdminUserID *uuid.UUID
	SessionID   *uuid.UUID
	AdminRole   string
	Username    string
	Action      string
	Resource    string
	ResourceID  string
	ShopID      *uuid.UUID
	Platform    string
	Permission  string
	RequestID   string
	Status      string
	Message     string
}

// Service persists operation logs.
type Service struct {
	DB *gorm.DB
}

// Write inserts one log row from the HTTP context plus overrides in opts.
func (s *Service) Write(c *gin.Context, opts WriteOpts) error {
	if s == nil || s.DB == nil || c == nil {
		return nil
	}
	row := s.newRow(c, opts)
	return s.writeWithDiagnostics(c.Request.Context(), row, opts)
}

// WriteTx persists an audit record as part of the caller's business
// transaction. Sensitive mutations can therefore commit only with their
// corresponding immutable audit evidence.
func (s *Service) WriteTx(c *gin.Context, tx *gorm.DB, opts WriteOpts) error {
	if s == nil || s.DB == nil || c == nil || tx == nil {
		return fmt.Errorf("operationlog: audit transaction unavailable")
	}
	row := s.newRowBase(c, opts)
	// Transactional permission evidence contains structured before/after
	// snapshots. The TEXT column must retain it in full; truncating JSON here
	// would silently discard evidence while committing the sensitive mutation.
	row.Message = opts.Message
	if row.AdminRole == "" && row.AdminUserID != nil {
		var actor struct{ Role string }
		if err := tx.Table("admin_users").Select("role").Where("id = ?", *row.AdminUserID).Take(&actor).Error; err != nil {
			return fmt.Errorf("operationlog: resolve audit actor: %w", err)
		}
		row.AdminRole = strings.TrimSpace(actor.Role)
	}
	var releaseChainLocks func()
	if err := func() error {
		var err error
		releaseChainLocks, err = s.appendHashChain(tx, row)
		return err
	}(); err != nil {
		return fmt.Errorf("operationlog: append audit chain: %w", err)
	}
	defer func() {
		if releaseChainLocks != nil {
			releaseChainLocks()
		}
	}()
	if err := tx.Create(row).Error; err != nil {
		return fmt.Errorf("operationlog: insert audit record: %w", err)
	}
	return nil
}

func (s *Service) newRow(c *gin.Context, opts WriteOpts) *OperationLog {
	row := s.newRowBase(c, opts)
	if row.AdminRole == "" && row.AdminUserID != nil && s.DB != nil {
		if p, err := adminperm.LoadPrincipal(c, s.DB); err == nil && p != nil {
			row.AdminRole = p.Role
		}
	}
	return row
}

// newRowBase builds request evidence without opening another DB connection.
// Callers inside a business transaction must resolve DB-backed fields through
// that transaction to avoid observing stale data or deadlocking SQLite tests.
func (s *Service) newRowBase(c *gin.Context, opts WriteOpts) *OperationLog {
	reqID, _ := c.Get(ctxkey.TraceID)
	rid, _ := reqID.(string)

	adminID := opts.AdminUserID
	if adminID == nil {
		if idStr, ok := c.Get(ctxkey.AdminID); ok {
			if sub, ok := idStr.(string); ok {
				if u, err := uuid.Parse(sub); err == nil {
					adminID = &u
				}
			}
		}
	}
	username := strings.TrimSpace(opts.Username)
	if username == "" {
		if u, ok := c.Get(ctxkey.AdminUsername); ok {
			username, _ = u.(string)
			username = strings.TrimSpace(username)
		}
	}

	path := c.Request.URL.Path
	if fp := c.FullPath(); fp != "" {
		path = fp
	}

	row := &OperationLog{
		TenantID:         opts.TenantID,
		AdminUserID:      adminID,
		SessionID:        opts.SessionID,
		AdminRole:        strings.TrimSpace(opts.AdminRole),
		Username:         username,
		Action:           strings.TrimSpace(opts.Action),
		Resource:         strings.TrimSpace(opts.Resource),
		ResourceID:       strings.TrimSpace(opts.ResourceID),
		ShopID:           opts.ShopID,
		Platform:         strings.TrimSpace(opts.Platform),
		Permission:       strings.TrimSpace(opts.Permission),
		Method:           c.Request.Method,
		Path:             path,
		IPHash:           authutil.HashIP(c.ClientIP()),
		UserAgentSummary: authutil.SummarizeUserAgent(c.Request.UserAgent()),
		RequestID:        rid,
		Status:           strings.TrimSpace(opts.Status),
		Message:          truncateRunes(opts.Message, 2000),
		CreatedAt:        time.Now().UTC(),
	}
	if row.TenantID == 0 {
		if tid, ok := c.Get(ctxkey.TenantID); ok {
			if v, ok := tid.(int64); ok {
				row.TenantID = v
			}
		}
	}
	if row.SessionID == nil {
		if sid, ok := c.Get(ctxkey.SessionID); ok {
			if s, ok := sid.(string); ok {
				if u, err := uuid.Parse(s); err == nil {
					row.SessionID = &u
				}
			}
		}
	}
	row.CreatedAt = row.CreatedAt.UTC().Truncate(time.Microsecond)
	row.ChainPartition = chainPartition(row.TenantID, row.CreatedAt)
	return row
}

// WriteBackground inserts one log row without an HTTP request (workers, cron).
func (s *Service) WriteBackground(ctx context.Context, opts WriteOpts) error {
	if s == nil || s.DB == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	adminID := opts.AdminUserID
	username := strings.TrimSpace(opts.Username)

	row := &OperationLog{
		AdminUserID: adminID,
		SessionID:   opts.SessionID,
		AdminRole:   strings.TrimSpace(opts.AdminRole),
		Username:    username,
		TenantID:    opts.TenantID,
		Action:      strings.TrimSpace(opts.Action),
		Resource:    strings.TrimSpace(opts.Resource),
		ResourceID:  strings.TrimSpace(opts.ResourceID),
		ShopID:      opts.ShopID,
		Platform:    strings.TrimSpace(opts.Platform),
		Permission:  strings.TrimSpace(opts.Permission),
		Method:      "INTERNAL",
		Path:        "/internal/worker",
		RequestID:   strings.TrimSpace(opts.RequestID),
		Status:      strings.TrimSpace(opts.Status),
		Message:     truncateRunes(opts.Message, 2000),
		CreatedAt:   time.Now().UTC(),
	}
	return s.writeWithDiagnostics(ctx, row, opts)
}

func (s *Service) writeWithDiagnostics(ctx context.Context, row *OperationLog, opts WriteOpts) error {
	authDiag := isAuthLoginDiagnostic(opts)
	if row != nil {
		if row.CreatedAt.IsZero() {
			row.CreatedAt = time.Now().UTC()
		}
		row.CreatedAt = row.CreatedAt.UTC().Truncate(time.Microsecond)
		row.ChainPartition = chainPartition(row.TenantID, row.CreatedAt)
	}
	if row != nil {
		row.ChainPartition = chainPartition(row.TenantID, row.CreatedAt)
	}
	txStart := time.Now()
	if authDiag {
		runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "transaction_begin", runtimediag.OutcomeSuccess, txStart)
	}
	var commitStart time.Time
	var releaseChainLocks func()
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		chainStart := time.Now()
		chainTiming, chainErr := runtimediag.TimedGorm(tx, func() error {
			var err error
			releaseChainLocks, err = s.appendHashChain(tx, row)
			return err
		})
		chainTiming.TransactionState = "open"
		if authDiag {
			outcome := runtimediag.OutcomeSuccess
			if chainErr != nil {
				outcome = runtimediag.OutcomeError
			}
			runtimediag.ObserveSQL(runtimediag.RouteAuthInvalidLogin, "auth", "auth.operation_log_chain_lookup", "select", "operation_logs", outcome, true, chainTiming)
			runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "security_audit", outcome, chainStart)
		}
		if chainErr != nil {
			return chainErr
		}
		insertStart := time.Now()
		insertTiming, insertErr := runtimediag.TimedGormRows(tx, func() (int64, error) {
			res := tx.Create(row)
			return res.RowsAffected, res.Error
		})
		insertTiming.TransactionState = "open"
		if authDiag {
			outcome := runtimediag.OutcomeSuccess
			if insertErr != nil {
				outcome = runtimediag.OutcomeError
			}
			runtimediag.ObserveSQL(runtimediag.RouteAuthInvalidLogin, "auth", "auth.operation_log_insert", "insert", "operation_logs", outcome, true, insertTiming)
			runtimediag.ObserveSQL(runtimediag.RouteAuthInvalidLogin, "auth", "auth.security_audit_insert", "insert", "operation_logs", outcome, true, insertTiming)
			runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "operation_log", outcome, insertStart)
		}
		commitStart = time.Now()
		return insertErr
	})
	if releaseChainLocks != nil {
		releaseChainLocks()
	}
	if !authDiag {
		return err
	}
	txOutcome := runtimediag.OutcomeSuccess
	state := "committed"
	if err != nil {
		txOutcome = runtimediag.OutcomeError
		state = "rolled_back"
	}
	commitMs := 0.0
	if !commitStart.IsZero() {
		commitMs = float64(time.Since(commitStart).Nanoseconds()) / float64(time.Millisecond)
	}
	runtimediag.ObserveStage(runtimediag.RouteAuthInvalidLogin, "transaction_commit", txOutcome, commitStart)
	// Emit commit/transaction envelope without a second fingerprint call count.
	if commitMs > 0 || !txStart.IsZero() {
		runtimediag.ObserveSQL(runtimediag.RouteAuthInvalidLogin, "auth", "auth.operation_log_insert", "insert", "operation_logs", txOutcome, true, runtimediag.SQLTiming{
			TransactionMs:    float64(time.Since(txStart).Nanoseconds()) / float64(time.Millisecond),
			CommitMs:         commitMs,
			TransactionState: state,
		})
	}
	return err
}

func isAuthLoginDiagnostic(opts WriteOpts) bool {
	return strings.EqualFold(strings.TrimSpace(opts.Action), "login") &&
		strings.EqualFold(strings.TrimSpace(opts.Resource), "auth")
}

// ListQuery binds query params for listing operation logs.
type ListQuery struct {
	Page      int
	PageSize  int
	Cursor    string
	Limit     int
	UseCursor bool
	Action    string
	Username  string
	Resource  string
	ShopID    *uuid.UUID
	Start     *time.Time
	End       *time.Time
}

// ListResult is a paginated slice of logs.
type ListResult struct {
	Items      []OperationLog
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
	Limit      int
	NextCursor string
	HasMore    bool
}

func operationLogCursorScope(c *gin.Context, db *gorm.DB, q ListQuery, tenantID int64) (string, string) {
	shopScope := ""
	if q.ShopID != nil && *q.ShopID != uuid.Nil {
		shopScope = q.ShopID.String()
	}
	allowed := []string{}
	if p, err := adminperm.LoadPrincipal(c, db); err == nil && p != nil {
		for _, id := range p.AllowedStoreIDs() {
			allowed = append(allowed, id.String())
		}
		sort.Strings(allowed)
	}
	return pagination.Fingerprint(map[string]any{
		"tenantId":       tenantID,
		"shopId":         shopScope,
		"allowedShopIds": allowed,
		"action":         q.Action,
		"username":       q.Username,
		"resource":       q.Resource,
		"start":          q.Start,
		"end":            q.End,
		"sort":           "created_at_desc_id_desc",
	}), shopScope
}

// List returns a paginated list with optional filters.
func (s *Service) List(c *gin.Context, q ListQuery) (*ListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("operationlog: no db")
	}
	if q.UseCursor && q.Limit > 0 {
		q.PageSize = q.Limit
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	ps := q.PageSize
	if ps < 1 {
		ps = 20
	}
	if ps > 100 {
		ps = 100
	}

	tx := s.DB.WithContext(c.Request.Context()).Model(&OperationLog{})
	var tenantID int64
	if scoped, tid, err := adminperm.ApplyTenantScope(c, tx); err != nil {
		return nil, err
	} else {
		tx = scoped
		tenantID = tid
	}
	if scoped, err := adminperm.ApplyStoreScope(c, s.DB, tx, "shop_id"); err != nil {
		return nil, err
	} else {
		tx = scoped
	}
	if v := strings.TrimSpace(q.Action); v != "" {
		tx = tx.Where("action = ?", v)
	}
	if v := strings.TrimSpace(q.Username); v != "" {
		pat := "%" + strings.ToLower(v) + "%"
		tx = tx.Where("LOWER(username) LIKE ?", pat)
	}
	if v := strings.TrimSpace(q.Resource); v != "" {
		tx = tx.Where("resource = ?", v)
	}
	if q.ShopID != nil && *q.ShopID != uuid.Nil {
		tx = tx.Where("shop_id = ?", *q.ShopID)
	}
	if q.Start != nil {
		tx = tx.Where("created_at >= ?", *q.Start)
	}
	if q.End != nil {
		tx = tx.Where("created_at <= ?", *q.End)
	}
	scopeHash, cursorShopID := operationLogCursorScope(c, s.DB, q, tenantID)
	if q.UseCursor && strings.TrimSpace(q.Cursor) != "" {
		cur, err := pagination.DecodeCursor(q.Cursor, tenantID, cursorShopID, scopeHash)
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

	var items []OperationLog
	query := tx.Order("created_at DESC, id DESC")
	limit := ps
	if q.UseCursor {
		limit = ps + 1
	} else {
		offset := (page - 1) * ps
		query = query.Offset(offset)
	}
	if err := query.Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	hasMore := q.UseCursor && len(items) > ps
	if hasMore {
		items = items[:ps]
	}
	nextCursor := ""
	if q.UseCursor && hasMore && len(items) > 0 {
		last := items[len(items)-1]
		var err error
		nextCursor, err = pagination.BuildNextCursor(true, tenantID, cursorShopID, scopeHash, "created_at", last.CreatedAt, last.ID.String())
		if err != nil {
			return nil, err
		}
	}

	pages := int(total) / ps
	if int(total)%ps != 0 {
		pages++
	}
	if pages == 0 && total > 0 {
		pages = 1
	}

	return &ListResult{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   ps,
		TotalPages: pages,
		Limit:      ps,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	if len(runes) > max {
		return string(runes[:max])
	}
	return s
}
