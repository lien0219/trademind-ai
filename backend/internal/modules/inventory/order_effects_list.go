package inventory

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

// OrderEffectsQuery paginates inventory effects scoped to orders.
type OrderEffectsQuery struct {
	Page         int
	PageSize     int
	OrderID      *uuid.UUID
	ProductSKUID *uuid.UUID
	EffectType   string
	Status       string
	Start        *time.Time
	End          *time.Time
}

type OrderInventoryEffectDTO struct {
	ID                   uuid.UUID  `json:"id"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
	OrderID              uuid.UUID  `json:"orderId"`
	OrderNo              string     `json:"orderNo,omitempty"`
	OrderItemID          uuid.UUID  `json:"orderItemId"`
	ProductID            *uuid.UUID `json:"productId,omitempty"`
	ProductSKUID         uuid.UUID  `json:"productSkuId"`
	EffectType           string     `json:"effectType"`
	Quantity             int        `json:"quantity"`
	Status               string     `json:"status"`
	BeforeStock          *int       `json:"beforeStock,omitempty"`
	AfterStock           *int       `json:"afterStock,omitempty"`
	Reason               string     `json:"reason,omitempty"`
	ErrorMessage         string     `json:"errorMessage,omitempty"`
	InventoryChangeLogID *uuid.UUID `json:"inventoryChangeLogId,omitempty"`
}

type PaginatedOrderEffects struct {
	Items      []OrderInventoryEffectDTO `json:"list"`
	Total      int64                     `json:"total"`
	Page       int                       `json:"page"`
	PageSize   int                       `json:"pageSize"`
	TotalPages int                       `json:"totalPages"`
}

func (s *Service) ListOrderEffectsByOrder(ctx context.Context, orderID uuid.UUID, page, ps int) (*PaginatedOrderEffects, error) {
	if page < 1 {
		page = 1
	}
	if ps < 1 || ps > 200 {
		ps = 50
	}
	tx := s.DB.WithContext(ctx).Model(&OrderInventoryEffect{}).Where("order_id = ?", orderID)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	offset := (page - 1) * ps
	var rows []OrderInventoryEffect
	if err := tx.Order("created_at DESC, id DESC").Offset(offset).Limit(ps).Find(&rows).Error; err != nil {
		return nil, err
	}
	return s.effectsPage(rows, total, page, ps)
}

func (s *Service) ListOrderEffectsGlobal(ctx context.Context, q OrderEffectsQuery) (*PaginatedOrderEffects, error) {
	page := q.Page
	if page < 1 {
		page = 1
	}
	ps := q.PageSize
	if ps < 1 || ps > 200 {
		ps = 20
	}
	tx := s.DB.WithContext(ctx).Model(&OrderInventoryEffect{})
	if q.OrderID != nil && *q.OrderID != uuid.Nil {
		tx = tx.Where("order_id = ?", *q.OrderID)
	}
	if q.ProductSKUID != nil && *q.ProductSKUID != uuid.Nil {
		tx = tx.Where("product_sku_id = ?", *q.ProductSKUID)
	}
	if strings.TrimSpace(q.EffectType) != "" {
		tx = tx.Where("effect_type = ?", strings.TrimSpace(q.EffectType))
	}
	if strings.TrimSpace(q.Status) != "" {
		tx = tx.Where("status = ?", strings.TrimSpace(q.Status))
	}
	if q.Start != nil {
		tx = tx.Where("created_at >= ?", *q.Start)
	}
	if q.End != nil {
		tx = tx.Where("created_at <= ?", *q.End)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	offset := (page - 1) * ps
	var rows []OrderInventoryEffect
	if err := tx.Order("created_at DESC, id DESC").Offset(offset).Limit(ps).Find(&rows).Error; err != nil {
		return nil, err
	}
	return s.effectsPage(rows, total, page, ps)
}

func (s *Service) effectsPage(rows []OrderInventoryEffect, total int64, page, ps int) (*PaginatedOrderEffects, error) {
	orderIDs := make([]uuid.UUID, 0)
	seen := map[uuid.UUID]struct{}{}
	for _, r := range rows {
		if _, ok := seen[r.OrderID]; ok {
			continue
		}
		seen[r.OrderID] = struct{}{}
		orderIDs = append(orderIDs, r.OrderID)
	}
	no := map[uuid.UUID]string{}
	if len(orderIDs) > 0 {
		type mini struct {
			ID      uuid.UUID `gorm:"column:id"`
			OrderNo string    `gorm:"column:order_no"`
		}
		var mm []mini
		_ = s.DB.Table("orders").Where("id IN ?", orderIDs).Scan(&mm).Error
		for _, m := range mm {
			no[m.ID] = m.OrderNo
		}
	}
	out := make([]OrderInventoryEffectDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, OrderInventoryEffectDTO{
			ID:                   r.ID,
			CreatedAt:            r.CreatedAt,
			UpdatedAt:            r.UpdatedAt,
			OrderID:              r.OrderID,
			OrderNo:              no[r.OrderID],
			OrderItemID:          r.OrderItemID,
			ProductID:            r.ProductID,
			ProductSKUID:         r.ProductSKUID,
			EffectType:           r.EffectType,
			Quantity:             r.Quantity,
			Status:               r.Status,
			BeforeStock:          r.BeforeStock,
			AfterStock:           r.AfterStock,
			Reason:               r.Reason,
			ErrorMessage:         r.ErrorMessage,
			InventoryChangeLogID: r.LogID,
		})
	}
	return &PaginatedOrderEffects{
		Items:      out,
		Total:      total,
		Page:       page,
		PageSize:   ps,
		TotalPages: pagesOf(total, ps),
	}, nil
}
