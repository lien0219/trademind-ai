package shop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	DouyinCategorySyncFailed       = "DOUYIN_CATEGORY_SYNC_FAILED"
	DouyinCategoryEmpty            = "DOUYIN_CATEGORY_EMPTY"
	DouyinCategoryNotSelected      = "DOUYIN_CATEGORY_NOT_SELECTED"
	DouyinCategoryNotLeaf          = "DOUYIN_CATEGORY_NOT_LEAF"
	DouyinCategoryAttrSyncFailed   = "DOUYIN_CATEGORY_ATTR_SYNC_FAILED"
	DouyinRequiredAttrMissing      = "DOUYIN_REQUIRED_ATTR_MISSING"
	DouyinCategoryCacheStale       = "DOUYIN_CATEGORY_CACHE_STALE"
	DouyinCategoryPermissionDenied = "DOUYIN_CATEGORY_PERMISSION_DENIED"

	douyinPlatform = "douyin_shop"
)

type DouyinCategoryError struct {
	Code    string
	Message string
	Err     error
}

func (e *DouyinCategoryError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

func (e *DouyinCategoryError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func douyinCategoryErr(code string, err error) *DouyinCategoryError {
	return &DouyinCategoryError{Code: code, Message: douyinFriendlyMessage(code), Err: err}
}

type DouyinCategoryListQuery struct {
	Keyword  string
	ParentID string
	OnlyLeaf bool
	Refresh  bool
	ShopID   *uuid.UUID
}

type DouyinCategoryNodeDTO struct {
	ID         uuid.UUID               `json:"id"`
	CategoryID string                  `json:"categoryId"`
	ParentID   string                  `json:"parentId"`
	Name       string                  `json:"name"`
	Level      int                     `json:"level"`
	IsLeaf     bool                    `json:"isLeaf"`
	Status     string                  `json:"status,omitempty"`
	Path       string                  `json:"path"`
	SyncedAt   *time.Time              `json:"syncedAt,omitempty"`
	Children   []DouyinCategoryNodeDTO `json:"children,omitempty"`
}

type DouyinCategoryListResult struct {
	List         []DouyinCategoryNodeDTO `json:"list"`
	Flat         []DouyinCategoryNodeDTO `json:"flat"`
	Total        int                     `json:"total"`
	LeafCount    int                     `json:"leafCount"`
	LastSyncedAt *time.Time              `json:"lastSyncedAt,omitempty"`
}

type DouyinAttributeDTO struct {
	ID          uuid.UUID       `json:"id"`
	CategoryID  string          `json:"categoryId"`
	AttrID      string          `json:"attrId"`
	Name        string          `json:"name"`
	Required    bool            `json:"required"`
	ValueType   string          `json:"valueType,omitempty"`
	Options     json.RawMessage `json:"options,omitempty"`
	UnitOptions json.RawMessage `json:"unitOptions,omitempty"`
	SyncedAt    *time.Time      `json:"syncedAt,omitempty"`
}

type DouyinCategoryStats struct {
	Count        int64      `json:"count"`
	LeafCount    int64      `json:"leafCount"`
	LastSyncedAt *time.Time `json:"lastSyncedAt,omitempty"`
}

func (s *Service) ListDouyinCategories(c *gin.Context, q DouyinCategoryListQuery, adminID *uuid.UUID) (*DouyinCategoryListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop service unavailable")
	}
	if q.Refresh {
		sid := q.ShopID
		if sid == nil || *sid == uuid.Nil {
			return nil, douyinCategoryErr(DouyinCategorySyncFailed, fmt.Errorf("shopId required for refresh"))
		}
		if _, err := s.SyncDouyinCategories(c, *sid, adminID); err != nil {
			return nil, err
		}
	}
	ctx := c.Request.Context()
	var rows []PlatformCategory
	tx := s.DB.WithContext(ctx).Where("platform = ?", douyinPlatform)
	if v := strings.TrimSpace(q.Keyword); v != "" {
		tx = tx.Where("LOWER(name) LIKE ?", "%"+strings.ToLower(v)+"%")
	}
	if v := strings.TrimSpace(q.ParentID); v != "" {
		tx = tx.Where("parent_id = ?", v)
	}
	if q.OnlyLeaf {
		tx = tx.Where("is_leaf = ?", true)
	}
	if err := tx.Order("level ASC, name ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return buildDouyinCategoryTree(rows), nil
}

func (s *Service) SyncDouyinCategories(c *gin.Context, shopID uuid.UUID, adminID *uuid.UUID) (*DouyinCategoryStats, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop service unavailable")
	}
	ctx := c.Request.Context()
	s.douyinLog(c, adminID, &shopID, "douyin.category.sync.start", "success", "", "category sync requested")
	client, _, _, err := s.douyinClientForShop(c, ctx, shopID, adminID)
	if err != nil {
		ce := douyinCategoryErr(DouyinCategorySyncFailed, err)
		s.douyinLog(c, adminID, &shopID, "douyin.category.sync.failed", "failed", ce.Code, ce.Message)
		return nil, ce
	}
	now := time.Now().UTC()
	seen := map[string]struct{}{}
	rows, err := s.fetchDouyinCategoryRecursive(ctx, client, platformdouyin.DefaultRootCategoryID, seen, 0)
	if err != nil {
		code := DouyinCategorySyncFailed
		var pe *platformdouyin.Error
		if errors.As(err, &pe) && pe.Code == platformdouyin.CodeDouyinPermissionDenied {
			code = DouyinCategoryPermissionDenied
		}
		ce := douyinCategoryErr(code, err)
		s.douyinLog(c, adminID, &shopID, "douyin.category.sync.failed", "failed", ce.Code, ce.Message)
		return nil, ce
	}
	if len(rows) == 0 {
		ce := douyinCategoryErr(DouyinCategoryEmpty, nil)
		s.douyinLog(c, adminID, &shopID, "douyin.category.sync.failed", "failed", ce.Code, ce.Message)
		return nil, ce
	}
	var leafCount int64
	for i := range rows {
		rows[i].SyncedAt = &now
		if rows[i].IsLeaf {
			leafCount++
		}
	}
	if err := s.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "platform"}, {Name: "category_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"parent_id", "name", "level", "is_leaf", "status", "raw", "synced_at", "updated_at",
		}),
	}).CreateInBatches(rows, 200).Error; err != nil {
		ce := douyinCategoryErr(DouyinCategorySyncFailed, err)
		s.douyinLog(c, adminID, &shopID, "douyin.category.sync.failed", "failed", ce.Code, ce.Message)
		return nil, ce
	}
	stat := &DouyinCategoryStats{Count: int64(len(rows)), LeafCount: leafCount, LastSyncedAt: &now}
	s.douyinLog(c, adminID, &shopID, "douyin.category.sync.success", "success", "", fmt.Sprintf("count=%d leaf=%d", stat.Count, stat.LeafCount))
	return stat, nil
}

func (s *Service) fetchDouyinCategoryRecursive(ctx context.Context, client *platformdouyin.Client, parentID string, seen map[string]struct{}, depth int) ([]PlatformCategory, error) {
	if depth > 12 {
		return nil, fmt.Errorf("douyin category tree depth exceeded")
	}
	items, err := client.GetCategories(ctx, platformdouyin.CategoryRequest{ParentID: parentID})
	if err != nil {
		return nil, err
	}
	rows := make([]PlatformCategory, 0, len(items))
	for _, item := range items {
		cid := strings.TrimSpace(item.CategoryID)
		if cid == "" {
			continue
		}
		if _, ok := seen[cid]; ok {
			continue
		}
		seen[cid] = struct{}{}
		raw, _ := json.Marshal(item.Raw)
		row := PlatformCategory{
			Platform:   douyinPlatform,
			CategoryID: cid,
			ParentID:   strings.TrimSpace(item.ParentID),
			Name:       strings.TrimSpace(item.Name),
			Level:      item.Level,
			IsLeaf:     item.IsLeaf,
			Status:     strings.TrimSpace(item.Status),
			Raw:        datatypes.JSON(raw),
		}
		rows = append(rows, row)
		if !item.IsLeaf {
			children, err := s.fetchDouyinCategoryRecursive(ctx, client, cid, seen, depth+1)
			if err != nil {
				return nil, err
			}
			rows = append(rows, children...)
		}
	}
	return rows, nil
}

func (s *Service) ListDouyinCategoryAttributes(ctx context.Context, categoryID string) ([]DouyinAttributeDTO, error) {
	var rows []PlatformCategoryAttribute
	if err := s.DB.WithContext(ctx).
		Where("platform = ? AND category_id = ?", douyinPlatform, strings.TrimSpace(categoryID)).
		Order("required DESC, name ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]DouyinAttributeDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, DouyinAttributeDTO{
			ID:          r.ID,
			CategoryID:  r.CategoryID,
			AttrID:      r.AttrID,
			Name:        r.Name,
			Required:    r.Required,
			ValueType:   r.ValueType,
			Options:     json.RawMessage(r.Options),
			UnitOptions: json.RawMessage(r.UnitOptions),
			SyncedAt:    r.SyncedAt,
		})
	}
	return out, nil
}

func (s *Service) SyncDouyinCategoryAttributes(c *gin.Context, shopID uuid.UUID, categoryID string, adminID *uuid.UUID) ([]DouyinAttributeDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("shop service unavailable")
	}
	ctx := c.Request.Context()
	cid := strings.TrimSpace(categoryID)
	if cid == "" {
		return nil, douyinCategoryErr(DouyinCategoryNotSelected, nil)
	}
	s.douyinLog(c, adminID, &shopID, "douyin.category.attr.sync.start", "success", "", "attribute sync requested")
	client, _, _, err := s.douyinClientForShop(c, ctx, shopID, adminID)
	if err != nil {
		ce := douyinCategoryErr(DouyinCategoryAttrSyncFailed, err)
		s.douyinLog(c, adminID, &shopID, "douyin.category.attr.sync.failed", "failed", ce.Code, ce.Message)
		return nil, ce
	}
	attrs, err := client.GetCategoryAttributes(ctx, cid)
	if err != nil {
		code := DouyinCategoryAttrSyncFailed
		var pe *platformdouyin.Error
		if errors.As(err, &pe) && pe.Code == platformdouyin.CodeDouyinPermissionDenied {
			code = DouyinCategoryPermissionDenied
		}
		ce := douyinCategoryErr(code, err)
		s.douyinLog(c, adminID, &shopID, "douyin.category.attr.sync.failed", "failed", ce.Code, ce.Message)
		return nil, ce
	}
	now := time.Now().UTC()
	rows := make([]PlatformCategoryAttribute, 0, len(attrs))
	for _, attr := range attrs {
		opts, _ := json.Marshal(attr.Options)
		units, _ := json.Marshal(attr.UnitOptions)
		raw, _ := json.Marshal(attr.Raw)
		rows = append(rows, PlatformCategoryAttribute{
			Platform:    douyinPlatform,
			CategoryID:  cid,
			AttrID:      strings.TrimSpace(attr.AttrID),
			Name:        strings.TrimSpace(attr.Name),
			Required:    attr.Required,
			ValueType:   strings.TrimSpace(attr.ValueType),
			Options:     datatypes.JSON(opts),
			UnitOptions: datatypes.JSON(units),
			Raw:         datatypes.JSON(raw),
			SyncedAt:    &now,
		})
	}
	if len(rows) > 0 {
		if err := s.DB.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "platform"}, {Name: "category_id"}, {Name: "attr_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"name", "required", "value_type", "options", "unit_options", "raw", "synced_at", "updated_at",
			}),
		}).CreateInBatches(rows, 200).Error; err != nil {
			ce := douyinCategoryErr(DouyinCategoryAttrSyncFailed, err)
			s.douyinLog(c, adminID, &shopID, "douyin.category.attr.sync.failed", "failed", ce.Code, ce.Message)
			return nil, ce
		}
	}
	s.douyinLog(c, adminID, &shopID, "douyin.category.attr.sync.success", "success", "", fmt.Sprintf("categoryId=%s count=%d", cid, len(rows)))
	return s.ListDouyinCategoryAttributes(ctx, cid)
}

func (s *Service) DouyinCategoryStats(ctx context.Context) (*DouyinCategoryStats, error) {
	var out DouyinCategoryStats
	if err := s.DB.WithContext(ctx).Model(&PlatformCategory{}).Where("platform = ?", douyinPlatform).Count(&out.Count).Error; err != nil {
		return nil, err
	}
	if err := s.DB.WithContext(ctx).Model(&PlatformCategory{}).Where("platform = ? AND is_leaf = ?", douyinPlatform, true).Count(&out.LeafCount).Error; err != nil {
		return nil, err
	}
	var row PlatformCategory
	if err := s.DB.WithContext(ctx).Where("platform = ? AND synced_at IS NOT NULL", douyinPlatform).Order("synced_at DESC").First(&row).Error; err == nil {
		out.LastSyncedAt = row.SyncedAt
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &out, nil
}

func buildDouyinCategoryTree(rows []PlatformCategory) *DouyinCategoryListResult {
	idToRow := map[string]PlatformCategory{}
	children := map[string][]string{}
	for _, r := range rows {
		idToRow[r.CategoryID] = r
		children[r.ParentID] = append(children[r.ParentID], r.CategoryID)
	}
	var last *time.Time
	for _, r := range rows {
		if r.SyncedAt != nil && (last == nil || r.SyncedAt.After(*last)) {
			t := *r.SyncedAt
			last = &t
		}
	}
	var mk func(id string, ancestors []string) DouyinCategoryNodeDTO
	mk = func(id string, ancestors []string) DouyinCategoryNodeDTO {
		r := idToRow[id]
		pathParts := append(append([]string{}, ancestors...), strings.TrimSpace(r.Name))
		node := DouyinCategoryNodeDTO{
			ID:         r.ID,
			CategoryID: r.CategoryID,
			ParentID:   r.ParentID,
			Name:       r.Name,
			Level:      r.Level,
			IsLeaf:     r.IsLeaf,
			Status:     r.Status,
			Path:       strings.Join(compactStrings(pathParts), " / "),
			SyncedAt:   r.SyncedAt,
		}
		for _, childID := range children[id] {
			node.Children = append(node.Children, mk(childID, pathParts))
		}
		return node
	}
	roots := children[platformdouyin.DefaultRootCategoryID]
	if len(roots) == 0 {
		for _, r := range rows {
			if _, ok := idToRow[r.ParentID]; !ok {
				roots = append(roots, r.CategoryID)
			}
		}
	}
	seenRoot := map[string]struct{}{}
	tree := make([]DouyinCategoryNodeDTO, 0, len(roots))
	for _, id := range roots {
		if _, ok := seenRoot[id]; ok {
			continue
		}
		seenRoot[id] = struct{}{}
		tree = append(tree, mk(id, nil))
	}
	flat := make([]DouyinCategoryNodeDTO, 0)
	var walk func([]DouyinCategoryNodeDTO)
	walk = func(nodes []DouyinCategoryNodeDTO) {
		for _, n := range nodes {
			cp := n
			cp.Children = nil
			flat = append(flat, cp)
			walk(n.Children)
		}
	}
	walk(tree)
	leaf := 0
	for _, n := range flat {
		if n.IsLeaf {
			leaf++
		}
	}
	return &DouyinCategoryListResult{List: tree, Flat: flat, Total: len(flat), LeafCount: leaf, LastSyncedAt: last}
}

func compactStrings(xs []string) []string {
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if s := strings.TrimSpace(x); s != "" {
			out = append(out, s)
		}
	}
	return out
}
