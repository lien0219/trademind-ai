package aiopsworkbench

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproductimage"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproducttext"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productcheck"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
	"github.com/trademind-ai/trademind/backend/internal/pkg/opslabels"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Service aggregates AI operation workbench todos (read-only; no side effects).
type Service struct {
	DB           *gorm.DB
	ProductCheck *productcheck.Service
	TaskCenter   *taskcenter.Service
}

type todoCollector struct {
	items map[string]TodoItem
}

func newCollector() *todoCollector {
	return &todoCollector{items: make(map[string]TodoItem)}
}

func (c *todoCollector) add(item TodoItem) {
	key := item.ID
	if key == "" {
		key = fmt.Sprintf("todo:%s:%s:%s", item.SourceType, item.SourceID, item.IssueCode)
		item.ID = key
	}
	if _, exists := c.items[key]; exists {
		return
	}
	c.items[key] = item
}

func (c *todoCollector) list() []TodoItem {
	out := make([]TodoItem, 0, len(c.items))
	for _, v := range c.items {
		out = append(out, v)
	}
	sortTodos(out)
	return out
}

func sortTodos(items []TodoItem) {
	sort.Slice(items, func(i, j int) bool {
		pi := priorityRank[items[i].Priority]
		pj := priorityRank[items[j].Priority]
		if pi != pj {
			return pi < pj
		}
		if !items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].UpdatedAt.After(items[j].UpdatedAt)
		}
		return items[i].ID < items[j].ID
	})
}

func clampPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

func todayStartUTC() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func hasQualityWarnings(raw datatypes.JSON) bool {
	if len(raw) == 0 {
		return false
	}
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" || s == "[]" {
		return false
	}
	var warnings []json.RawMessage
	if err := json.Unmarshal(raw, &warnings); err != nil {
		return false
	}
	return len(warnings) > 0
}

func aiTextDetailURL(batchID, itemID string) string {
	if batchID == "" {
		return ""
	}
	path := "/product/ai-text-batches/" + url.PathEscape(batchID)
	if itemID != "" {
		q := url.Values{}
		q.Set("itemId", itemID)
		path += "?" + q.Encode()
	}
	return path
}

func aiImageDetailURL(batchID, itemID string) string {
	if batchID == "" {
		return ""
	}
	path := "/product/ai-image-batches/" + url.PathEscape(batchID)
	if itemID != "" {
		q := url.Values{}
		q.Set("itemId", itemID)
		path += "?" + q.Encode()
	}
	return path
}

func productDraftURL(productID, section string) string {
	base := "/product/drafts/" + url.PathEscape(productID)
	if section != "" {
		return base + "?tab=readiness&section=" + url.QueryEscape(section)
	}
	return base
}

func publishBatchURL(batchID string) string {
	return "/product/publish-batches/" + url.PathEscape(batchID)
}

func taskCenterURL(detailURL string) string {
	if strings.TrimSpace(detailURL) != "" {
		return detailURL
	}
	return "/ops/task-center/failures"
}

func (s *Service) batchProductTitles(ctx context.Context, ids []uuid.UUID) map[uuid.UUID]string {
	out := make(map[uuid.UUID]string, len(ids))
	if s == nil || s.DB == nil || len(ids) == 0 {
		return out
	}
	var rows []struct {
		ID    uuid.UUID `gorm:"column:id"`
		Title string    `gorm:"column:title"`
	}
	_ = s.DB.WithContext(ctx).Model(&product.Product{}).
		Select("id, COALESCE(NULLIF(TRIM(title), ''), NULLIF(TRIM(original_title), ''), '未命名商品') AS title").
		Where("id IN ?", ids).Find(&rows).Error
	for _, r := range rows {
		out[r.ID] = strings.TrimSpace(r.Title)
	}
	return out
}

func (s *Service) batchShopNames(ctx context.Context, ids []uuid.UUID) map[uuid.UUID]string {
	out := make(map[uuid.UUID]string, len(ids))
	if s == nil || s.DB == nil || len(ids) == 0 {
		return out
	}
	type row struct {
		ID   uuid.UUID `gorm:"column:id"`
		Name string    `gorm:"column:name"`
	}
	var rows []row
	_ = s.DB.WithContext(ctx).Table("shops").
		Select("id, name").Where("id IN ?", ids).Find(&rows).Error
	for _, r := range rows {
		out[r.ID] = strings.TrimSpace(r.Name)
	}
	return out
}

func passesKeyword(keyword, productTitle, message, sourceID string) bool {
	kw := strings.TrimSpace(strings.ToLower(keyword))
	if kw == "" {
		return true
	}
	hay := strings.ToLower(productTitle + " " + message + " " + sourceID)
	return strings.Contains(hay, kw)
}

func passesTypeFilter(filter, todoType string) bool {
	f := strings.TrimSpace(filter)
	if f == "" {
		return true
	}
	return strings.EqualFold(f, todoType)
}

func passesPriorityFilter(filter, priority string) bool {
	f := strings.TrimSpace(filter)
	if f == "" {
		return true
	}
	return strings.EqualFold(f, priority)
}

func passesPlatformFilter(filter, platform string) bool {
	f := strings.TrimSpace(strings.ToLower(filter))
	if f == "" {
		return true
	}
	return strings.EqualFold(f, strings.TrimSpace(platform))
}

func passesShopFilter(filter, shopID string) bool {
	f := strings.TrimSpace(filter)
	if f == "" {
		return true
	}
	return strings.EqualFold(f, strings.TrimSpace(shopID))
}

func passesTimeRange(t time.Time, start, end *time.Time) bool {
	if start != nil && t.Before(*start) {
		return false
	}
	if end != nil && t.After(*end) {
		return false
	}
	return true
}

func filterTodos(all []TodoItem, q Query) []TodoItem {
	out := make([]TodoItem, 0, len(all))
	for _, item := range all {
		if !passesTypeFilter(q.Type, item.Type) {
			continue
		}
		if !passesPriorityFilter(q.Priority, item.Priority) {
			continue
		}
		if !passesPlatformFilter(q.Platform, item.Platform) {
			continue
		}
		if !passesShopFilter(q.ShopID, item.ShopID) {
			continue
		}
		if !passesKeyword(q.Keyword, item.ProductTitle, item.Message, item.SourceID) {
			continue
		}
		if !passesTimeRange(item.UpdatedAt, q.Start, q.End) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func isHighPriority(p string) bool {
	return p == PriorityP0 || p == PriorityP1
}

func buildSummaryFromTodos(all []TodoItem) SummaryDTO {
	today := todayStartUTC()
	sum := SummaryDTO{}
	for _, item := range all {
		if isHighPriority(item.Priority) {
			sum.HighPriorityCount++
		}
		switch item.Type {
		case TodoTypeAITextReview, TodoTypeAITextConflict:
			sum.AITextReviewCount++
			if isHighPriority(item.Priority) {
				sum.AITextReviewHigh++
			}
			if !item.CreatedAt.Before(today) {
				sum.AITextReviewTodayNew++
			}
		case TodoTypeAIImageReview, TodoTypeAIImageConflict:
			sum.AIImageReviewCount++
			if isHighPriority(item.Priority) {
				sum.AIImageReviewHigh++
			}
			if !item.CreatedAt.Before(today) {
				sum.AIImageReviewTodayNew++
			}
		case TodoTypePublishCheckFailed, TodoTypePublishCheckWarning:
			sum.PublishCheckIssueCount++
			if isHighPriority(item.Priority) {
				sum.PublishCheckHigh++
			}
			if !item.CreatedAt.Before(today) {
				sum.PublishCheckTodayNew++
			}
		case TodoTypePublishBatchFailed, TodoTypePublishBatchPartial:
			sum.PublishTaskIssueCount++
			if isHighPriority(item.Priority) {
				sum.PublishTaskHigh++
			}
			if !item.CreatedAt.Before(today) {
				sum.PublishTaskTodayNew++
			}
		}
	}
	return sum
}

// GetSummary returns headline counts.
func (s *Service) GetSummary(ctx context.Context, q Query) (SummaryDTO, error) {
	if s == nil || s.DB == nil {
		return SummaryDTO{}, fmt.Errorf("aiopsworkbench: no db")
	}
	all, err := s.collectAllTodos(ctx, q)
	if err != nil {
		return SummaryDTO{}, err
	}
	filtered := filterTodos(all, Query{
		Platform: q.Platform,
		ShopID:   q.ShopID,
		Start:    q.Start,
		End:      q.End,
	})
	sum := buildSummaryFromTodos(filtered)
	resolved, err := s.countTodayResolved(ctx)
	if err != nil {
		return SummaryDTO{}, err
	}
	sum.TodayResolvedCount = resolved
	return sum, nil
}

// ListTodos returns paginated todos.
func (s *Service) ListTodos(ctx context.Context, q Query) (TodosResponse, error) {
	if s == nil || s.DB == nil {
		return TodosResponse{}, fmt.Errorf("aiopsworkbench: no db")
	}
	page, pageSize := clampPage(q.Page, q.PageSize)
	all, err := s.collectAllTodos(ctx, q)
	if err != nil {
		return TodosResponse{}, err
	}
	filtered := filterTodos(all, q)
	total := int64(len(filtered))
	start := (page - 1) * pageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	return TodosResponse{
		Items: filtered[start:end],
		Pagination: Pagination{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	}, nil
}

// GetTodo returns one todo by id.
func (s *Service) GetTodo(ctx context.Context, id string, q Query) (*TodoItem, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("todo id required")
	}
	all, err := s.collectAllTodos(ctx, q)
	if err != nil {
		return nil, err
	}
	for _, item := range all {
		if item.ID == id {
			copy := item
			return &copy, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

// RefreshTodos re-aggregates and returns summary (no side effects).
func (s *Service) RefreshTodos(ctx context.Context, q Query) (RefreshResponse, error) {
	sum, err := s.GetSummary(ctx, q)
	if err != nil {
		return RefreshResponse{}, err
	}
	return RefreshResponse{
		RefreshedAt: time.Now().UTC(),
		Summary:     sum,
	}, nil
}

func (s *Service) collectAllTodos(ctx context.Context, q Query) ([]TodoItem, error) {
	col := newCollector()
	if err := s.collectAITextTodos(ctx, col, q); err != nil {
		return nil, err
	}
	if err := s.collectAIImageTodos(ctx, col, q); err != nil {
		return nil, err
	}
	if err := s.collectPublishCheckTodos(ctx, col, q); err != nil {
		return nil, err
	}
	if err := s.collectPublishBatchTodos(ctx, col, q); err != nil {
		return nil, err
	}
	if err := s.collectTaskCenterTodos(ctx, col, q); err != nil {
		return nil, err
	}
	return col.list(), nil
}

func (s *Service) collectAITextTodos(ctx context.Context, col *todoCollector, q Query) error {
	var rows []aiproducttext.AIProductTextItem
	tx := s.DB.WithContext(ctx).Model(&aiproducttext.AIProductTextItem{}).
		Where(`status IN ? OR status = ?`,
			[]string{aiproducttext.ItemPendingReview, aiproducttext.ItemSuccess},
			aiproducttext.ItemConflict).
		Where("status NOT IN ?", []string{aiproducttext.ItemApplied, aiproducttext.ItemRejected, aiproducttext.ItemCancelled})
	if q.Start != nil {
		tx = tx.Where("updated_at >= ?", *q.Start)
	}
	if q.End != nil {
		tx = tx.Where("updated_at <= ?", *q.End)
	}
	if err := tx.Order("updated_at DESC").Limit(maxMergePerSource).Find(&rows).Error; err != nil {
		return err
	}
	prodIDs := make([]uuid.UUID, 0, len(rows))
	for i := range rows {
		prodIDs = append(prodIDs, rows[i].ProductID)
	}
	titles := s.batchProductTitles(ctx, prodIDs)

	for i := range rows {
		row := rows[i]
		ptitle := titles[row.ProductID]
		itemID := row.ID.String()
		batchID := row.BatchID.String()

		switch row.Status {
		case aiproducttext.ItemConflict:
			col.add(TodoItem{
				ID:            fmt.Sprintf("todo:%s:%s:%s", SourceAIText, itemID, TodoIssueConflict),
				Type:          TodoTypeAITextConflict,
				TypeLabel:     TypeLabel(TodoTypeAITextConflict),
				Priority:      PriorityP1,
				PriorityLabel: PriorityLabel(PriorityP1),
				ProductID:     row.ProductID.String(),
				ProductTitle:  ptitle,
				Title:         "AI 文案应用时发现内容冲突",
				Message:       aiproducttext.ConflictUserMessage,
				ActionLabel:   "去复核",
				ActionURL:     aiTextDetailURL(batchID, itemID),
				SourceType:    SourceAIText,
				SourceID:      itemID,
				IssueCode:     TodoIssueConflict,
				CreatedAt:     row.CreatedAt,
				UpdatedAt:     row.UpdatedAt,
				Technical: map[string]any{
					"batchId":       batchID,
					"operationType": row.OperationType,
					"status":        row.Status,
				},
			})
		default:
			priority := PriorityP3
			msg := "AI 已生成新的商品文案，建议确认后再应用。"
			title := "商品文案有 AI 建议待复核"
			issueCode := TodoIssuePendingReview
			if row.OperationType == aiproducttext.OpTitle {
				title = "商品标题有 AI 建议待复核"
				msg = "AI 已生成新的商品标题，建议确认后再应用。"
			} else if row.OperationType == aiproducttext.OpDescription {
				title = "商品描述有 AI 建议待复核"
				msg = "AI 已生成新的商品描述，建议确认后再应用。"
			}
			if hasQualityWarnings(row.QualityWarnings) {
				priority = PriorityP2
				issueCode = TodoIssueQualityWarning
				msg = "AI 文案建议存在质量提醒，建议编辑后再应用。"
			}
			col.add(TodoItem{
				ID:            fmt.Sprintf("todo:%s:%s:%s", SourceAIText, itemID, issueCode),
				Type:          TodoTypeAITextReview,
				TypeLabel:     TypeLabel(TodoTypeAITextReview),
				Priority:      priority,
				PriorityLabel: PriorityLabel(priority),
				ProductID:     row.ProductID.String(),
				ProductTitle:  ptitle,
				Title:         title,
				Message:       msg,
				ActionLabel:   "去复核",
				ActionURL:     aiTextDetailURL(batchID, itemID),
				SourceType:    SourceAIText,
				SourceID:      itemID,
				IssueCode:     issueCode,
				CreatedAt:     row.CreatedAt,
				UpdatedAt:     row.UpdatedAt,
				Technical: map[string]any{
					"batchId":       batchID,
					"operationType": row.OperationType,
					"status":        row.Status,
				},
			})
		}
	}
	return nil
}

func (s *Service) collectAIImageTodos(ctx context.Context, col *todoCollector, q Query) error {
	var rows []aiproductimage.AIProductImageItem
	tx := s.DB.WithContext(ctx).Model(&aiproductimage.AIProductImageItem{}).
		Where(`status IN ? OR status = ?`,
			[]string{aiproductimage.ItemPendingReview, aiproductimage.ItemSuccess},
			aiproductimage.ItemConflict).
		Where("status NOT IN ?", []string{aiproductimage.ItemApplied, aiproductimage.ItemRejected, aiproductimage.ItemCancelled})
	if q.Start != nil {
		tx = tx.Where("updated_at >= ?", *q.Start)
	}
	if q.End != nil {
		tx = tx.Where("updated_at <= ?", *q.End)
	}
	if err := tx.Order("updated_at DESC").Limit(maxMergePerSource).Find(&rows).Error; err != nil {
		return err
	}
	prodIDs := make([]uuid.UUID, 0, len(rows))
	for i := range rows {
		prodIDs = append(prodIDs, rows[i].ProductID)
	}
	titles := s.batchProductTitles(ctx, prodIDs)

	for i := range rows {
		row := rows[i]
		ptitle := titles[row.ProductID]
		itemID := row.ID.String()
		batchID := row.BatchID.String()

		switch row.Status {
		case aiproductimage.ItemConflict:
			col.add(TodoItem{
				ID:            fmt.Sprintf("todo:%s:%s:%s", SourceAIImage, itemID, TodoIssueConflict),
				Type:          TodoTypeAIImageConflict,
				TypeLabel:     TypeLabel(TodoTypeAIImageConflict),
				Priority:      PriorityP1,
				PriorityLabel: PriorityLabel(PriorityP1),
				ProductID:     row.ProductID.String(),
				ProductTitle:  ptitle,
				Title:         "AI 图片应用时发现内容冲突",
				Message:       aiproductimage.ConflictUserMessage,
				ActionLabel:   "去复核",
				ActionURL:     aiImageDetailURL(batchID, itemID),
				SourceType:    SourceAIImage,
				SourceID:      itemID,
				IssueCode:     TodoIssueConflict,
				CreatedAt:     row.CreatedAt,
				UpdatedAt:     row.UpdatedAt,
				Technical: map[string]any{
					"batchId":       batchID,
					"operationType": row.OperationType,
					"status":        row.Status,
				},
			})
		default:
			priority := PriorityP3
			issueCode := TodoIssuePendingReview
			msg := "AI 已生成图片处理结果，建议确认后再应用。"
			if hasQualityWarnings(row.QualityWarnings) {
				priority = PriorityP2
				issueCode = TodoIssueQualityWarning
				msg = "AI 图片处理结果存在质量提醒，建议复核后再应用。"
			}
			col.add(TodoItem{
				ID:            fmt.Sprintf("todo:%s:%s:%s", SourceAIImage, itemID, issueCode),
				Type:          TodoTypeAIImageReview,
				TypeLabel:     TypeLabel(TodoTypeAIImageReview),
				Priority:      priority,
				PriorityLabel: PriorityLabel(priority),
				ProductID:     row.ProductID.String(),
				ProductTitle:  ptitle,
				Title:         "商品图片有 AI 处理结果待复核",
				Message:       msg,
				ActionLabel:   "去复核",
				ActionURL:     aiImageDetailURL(batchID, itemID),
				SourceType:    SourceAIImage,
				SourceID:      itemID,
				IssueCode:     issueCode,
				CreatedAt:     row.CreatedAt,
				UpdatedAt:     row.UpdatedAt,
				Technical: map[string]any{
					"batchId":       batchID,
					"operationType": row.OperationType,
					"status":        row.Status,
				},
			})
		}
	}
	return nil
}

func (s *Service) collectPublishCheckTodos(ctx context.Context, col *todoCollector, q Query) error {
	if s.ProductCheck == nil {
		return nil
	}
	tx := s.DB.WithContext(ctx).Model(&product.Product{}).
		Select("id").
		Where("status NOT IN ?", []string{product.StatusArchived})
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		like := "%" + kw + "%"
		tx = tx.Where("(title ILIKE ? OR original_title ILIKE ? OR CAST(id AS TEXT) ILIKE ?)", like, like, like)
	}
	var prodRows []struct {
		ID uuid.UUID `gorm:"column:id"`
	}
	if err := tx.Order("updated_at DESC").Limit(maxPublishCheckScan).Find(&prodRows).Error; err != nil {
		return err
	}
	var shopPtr *uuid.UUID
	if sid := strings.TrimSpace(q.ShopID); sid != "" {
		if u, err := uuid.Parse(sid); err == nil {
			shopPtr = &u
		}
	}
	platform := strings.TrimSpace(q.Platform)
	titles := s.batchProductTitles(ctx, func() []uuid.UUID {
		ids := make([]uuid.UUID, len(prodRows))
		for i, r := range prodRows {
			ids[i] = r.ID
		}
		return ids
	}())

	for _, pr := range prodRows {
		res, err := s.ProductCheck.CheckProductReadiness(ctx, productcheck.CheckProductReadinessRequest{
			ProductID: pr.ID,
			Mode:      "draft",
			Platform:  platform,
			ShopID:    shopPtr,
		})
		if err != nil || res == nil {
			continue
		}
		localized := productcheck.LocalizeReadinessResult(res)
		ptitle := titles[pr.ID]
		for _, chk := range localized.Checks {
			if chk.Level != "error" && chk.Level != "warning" {
				continue
			}
			todoType := TodoTypePublishCheckWarning
			priority := PriorityP2
			actionLabel := "去处理"
			if chk.Level == "error" {
				todoType = TodoTypePublishCheckFailed
				priority = PriorityP1
			}
			code := strings.TrimSpace(chk.Code)
			if code == "" {
				code = strings.TrimSpace(chk.Group)
			}
			msg := strings.TrimSpace(chk.Message)
			if msg == "" {
				msg = strings.TrimSpace(chk.Title)
			}
			suggestion := strings.TrimSpace(chk.Suggestion)
			title := strings.TrimSpace(chk.Title)
			if title == "" {
				title = msg
			}
			plat := strings.TrimSpace(localized.Platform)
			shopID := ""
			shopName := ""
			if localized.ShopID != nil {
				shopID = localized.ShopID.String()
			}
			col.add(TodoItem{
				ID:            fmt.Sprintf("todo:%s:%s:%s", SourcePublishCheck, pr.ID.String(), code),
				Type:          todoType,
				TypeLabel:     TypeLabel(todoType),
				Priority:      priority,
				PriorityLabel: PriorityLabel(priority),
				ProductID:     pr.ID.String(),
				ProductTitle:  ptitle,
				Platform:      plat,
				PlatformLabel: opslabels.PlatformLabel(plat),
				ShopID:        shopID,
				ShopName:      shopName,
				Title:         title,
				Message:       msg,
				ActionLabel:   actionLabel,
				ActionURL:     productDraftURL(pr.ID.String(), "publish-check"),
				SourceType:    SourcePublishCheck,
				SourceID:      pr.ID.String(),
				IssueCode:     code,
				CreatedAt:     time.Now().UTC(),
				UpdatedAt:     time.Now().UTC(),
				Technical: map[string]any{
					"checkGroup": chk.Group,
					"level":      chk.Level,
					"suggestion": suggestion,
					"rawCode":    code,
				},
			})
		}
	}
	return nil
}

func (s *Service) collectPublishBatchTodos(ctx context.Context, col *todoCollector, q Query) error {
	tx := s.DB.WithContext(ctx).Model(&productpublish.ProductPublishBatch{}).
		Where("status IN ?", []string{productpublish.BatchFailed, productpublish.BatchPartialSuccess})
	if q.Start != nil {
		tx = tx.Where("updated_at >= ?", *q.Start)
	}
	if q.End != nil {
		tx = tx.Where("updated_at <= ?", *q.End)
	}
	var rows []productpublish.ProductPublishBatch
	if err := tx.Order("updated_at DESC").Limit(maxMergePerSource).Find(&rows).Error; err != nil {
		return err
	}
	prodIDs := make([]uuid.UUID, 0, len(rows))
	for i := range rows {
		if rows[i].ProductID != nil {
			prodIDs = append(prodIDs, *rows[i].ProductID)
		}
	}
	titles := s.batchProductTitles(ctx, prodIDs)

	for i := range rows {
		row := rows[i]
		batchID := row.ID.String()
		todoType := TodoTypePublishBatchFailed
		priority := PriorityP1
		issueCode := TodoIssueBatchFailed
		title := "刊登批次创建失败"
		msg := "批量创建刊登草稿未全部成功，请查看失败项并重试。"
		actionLabel := "查看批次"
		if row.Status == productpublish.BatchPartialSuccess {
			todoType = TodoTypePublishBatchPartial
			priority = PriorityP2
			issueCode = TodoIssuePartialSuccess
			title = "刊登批次部分成功"
			msg = "部分目标已创建草稿，仍有失败项需要处理。"
		}
		productID := ""
		productTitle := ""
		if row.ProductID != nil {
			productID = row.ProductID.String()
			productTitle = titles[*row.ProductID]
		}
		if row.BatchType == productpublish.BatchTypeMultiProduct {
			title = "多商品刊登批次异常"
			if row.ProductCount > 1 {
				productTitle = fmt.Sprintf("%d 个商品", row.ProductCount)
			}
		}
		col.add(TodoItem{
			ID:            fmt.Sprintf("todo:%s:%s:%s", SourcePublishBatch, batchID, issueCode),
			Type:          todoType,
			TypeLabel:     TypeLabel(todoType),
			Priority:      priority,
			PriorityLabel: PriorityLabel(priority),
			ProductID:     productID,
			ProductTitle:  productTitle,
			Title:         title,
			Message:       msg,
			ActionLabel:   actionLabel,
			ActionURL:     publishBatchURL(batchID),
			SourceType:    SourcePublishBatch,
			SourceID:      batchID,
			IssueCode:     issueCode,
			CreatedAt:     row.CreatedAt,
			UpdatedAt:     row.UpdatedAt,
			Technical: map[string]any{
				"batchType":    row.BatchType,
				"status":       row.Status,
				"failedCount":  row.FailedCount,
				"successCount": row.SuccessCount,
				"taskCount":    row.TaskCount,
			},
		})
	}
	return nil
}

func (s *Service) collectTaskCenterTodos(ctx context.Context, col *todoCollector, q Query) error {
	if s.TaskCenter == nil {
		return nil
	}
	p := taskcenter.ListFailureParams{
		Platform:        strings.TrimSpace(q.Platform),
		ShopID:          strings.TrimSpace(q.ShopID),
		Keyword:         strings.TrimSpace(q.Keyword),
		IncludeResolved: false,
		IncludeMarked:   false,
		Start:           q.Start,
		End:             q.End,
		Page:            1,
		PageSize:        maxMergePerSource,
	}
	res, err := s.TaskCenter.ListFailures(ctx, p)
	if err != nil {
		return err
	}
	seenAI := make(map[string]struct{})
	for k := range col.items {
		if strings.HasPrefix(k, "todo:ai_text:") || strings.HasPrefix(k, "todo:ai_image:") {
			parts := strings.Split(k, ":")
			if len(parts) >= 3 {
				seenAI[parts[2]] = struct{}{}
			}
		}
	}

	for _, d := range res.List {
		if d.NormalizedStatus != taskcenter.NormFailed {
			continue
		}
		// Dedup: ai_text/ai_image failed items already surfaced via review/conflict paths
		if (d.TaskType == taskcenter.TaskTypeAIText || d.TaskType == taskcenter.TaskTypeAIImage) && d.SourceID != "" {
			if _, ok := seenAI[d.SourceID]; ok {
				continue
			}
			// Skip pending_review quality-only rows handled as review todos
			if d.TaskType == taskcenter.TaskTypeAIText &&
				(d.Status == aiproducttext.ItemPendingReview || d.Status == aiproducttext.ItemSuccess) {
				continue
			}
			if d.TaskType == taskcenter.TaskTypeAIImage &&
				(d.Status == aiproductimage.ItemPendingReview || d.Status == aiproductimage.ItemSuccess) {
				continue
			}
		}
		issueCode := strings.TrimSpace(d.FailureCategory)
		if issueCode == "" {
			issueCode = TodoIssueFailed
		}
		priority := PriorityP1
		if d.Severity == "low" {
			priority = PriorityP2
		}
		msg := strings.TrimSpace(d.ErrorMessage)
		if msg == "" {
			msg = strings.TrimSpace(d.ClassificationReason)
		}
		actionURL := taskCenterURL(d.DetailURL)
		actionLabel := "查看失败任务"
		if strings.Contains(actionURL, "ai-text-batches") || strings.Contains(actionURL, "ai-image-batches") {
			actionLabel = "去复核"
		} else if strings.Contains(actionURL, "publish-batches") {
			actionLabel = "查看批次"
		}
		col.add(TodoItem{
			ID:            fmt.Sprintf("todo:%s:%s:%s", SourceTaskCenter, d.SourceID, issueCode),
			Type:          TodoTypeTaskCenterFailure,
			TypeLabel:     TypeLabel(TodoTypeTaskCenterFailure),
			Priority:      priority,
			PriorityLabel: PriorityLabel(priority),
			ProductID:     d.RelatedResourceID,
			ProductTitle:  d.RelatedResourceTitle,
			Platform:      d.Platform,
			PlatformLabel: opslabels.PlatformLabel(d.Platform),
			ShopID:        d.ShopID,
			ShopName:      d.ShopName,
			Title:         d.Title,
			Message:       msg,
			ActionLabel:   actionLabel,
			ActionURL:     actionURL,
			SourceType:    SourceTaskCenter,
			SourceID:      d.SourceID,
			IssueCode:     issueCode,
			CreatedAt:     d.CreatedAt,
			UpdatedAt:     d.UpdatedAt,
			Technical: map[string]any{
				"taskType":        d.TaskType,
				"failureCategory": d.FailureCategory,
				"severity":        d.Severity,
				"suggestedAction": d.SuggestedAction,
			},
		})
	}
	return nil
}

func (s *Service) countTodayResolved(ctx context.Context) (int64, error) {
	start := todayStartUTC()
	var total int64

	var textApplied int64
	_ = s.DB.WithContext(ctx).Model(&aiproducttext.AIProductTextItem{}).
		Where("status IN ? AND applied_at >= ?", []string{aiproducttext.ItemApplied, aiproducttext.ItemRejected}, start).
		Count(&textApplied).Error
	total += textApplied

	var imageApplied int64
	_ = s.DB.WithContext(ctx).Model(&aiproductimage.AIProductImageItem{}).
		Where("status IN ? AND applied_at >= ?", []string{aiproductimage.ItemApplied, aiproductimage.ItemRejected}, start).
		Count(&imageApplied).Error
	total += imageApplied

	var handled int64
	_ = s.DB.WithContext(ctx).Model(&taskcenter.TaskFailureMark{}).
		Where("mark_type = ? AND created_at >= ?", taskcenter.MarkHandled, start).
		Count(&handled).Error
	total += handled

	var batchFixed int64
	_ = s.DB.WithContext(ctx).Model(&productpublish.ProductPublishBatch{}).
		Where("status = ? AND updated_at >= ?", productpublish.BatchSuccess, start).
		Count(&batchFixed).Error
	total += batchFixed

	return total, nil
}
