package order

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// WarehouseFeeScope is the explicit store boundary used by the warehouse fee
// domain when reading immutable fulfillment facts from the order domain.
type WarehouseFeeScope struct {
	RestrictStoreScope bool
	AllowedShopIDs     []uuid.UUID
}

type WarehouseFeeFactQuery struct {
	Page        int
	PageSize    int
	OrderNo     string
	WarehouseID *uuid.UUID
}

type WarehouseFeeFact struct {
	OrderID            uuid.UUID  `json:"orderId"`
	OrderNo            string     `json:"orderNo"`
	ShopID             *uuid.UUID `json:"shopId,omitempty"`
	ShopName           string     `json:"shopName,omitempty"`
	Platform           string     `json:"platform"`
	Currency           string     `json:"currency"`
	WarehouseID        uuid.UUID  `json:"warehouseId"`
	WarehouseCode      string     `json:"warehouseCode"`
	WarehouseName      string     `json:"warehouseName"`
	WaveID             uuid.UUID  `json:"waveId"`
	WaveNo             string     `json:"waveNo"`
	WaveRevision       int        `json:"waveRevision"`
	WaveStatus         string     `json:"waveStatus"`
	WaveOrderID        uuid.UUID  `json:"waveOrderId"`
	WaveOrderStatus    string     `json:"waveOrderStatus"`
	ShipmentID         *uuid.UUID `json:"shipmentId,omitempty"`
	PackVerificationID *uuid.UUID `json:"packVerificationId,omitempty"`
	PackageCode        string     `json:"packageCode"`
	ItemQuantity       int        `json:"itemQuantity"`
	PackageQuantity    int        `json:"packageQuantity"`
	FulfilledAt        *time.Time `json:"fulfilledAt,omitempty"`
	VerifiedAt         *time.Time `json:"verifiedAt,omitempty"`
}

type WarehouseFeeFactList struct {
	List       []WarehouseFeeFact
	Page       int
	PageSize   int
	Total      int64
	TotalPages int
}

func warehouseFeeFactQuery(db *gorm.DB, tenantID int64, scope WarehouseFeeScope) *gorm.DB {
	q := db.Table("fulfillment_wave_orders AS wave_order").
		Joins("JOIN fulfillment_waves AS wave ON wave.id = wave_order.wave_id AND wave.tenant_id = wave_order.tenant_id AND wave.deleted_at IS NULL").
		Joins("JOIN orders AS orders ON orders.id = wave_order.order_id AND orders.tenant_id = wave_order.tenant_id AND orders.deleted_at IS NULL").
		Joins("JOIN warehouses AS warehouse ON warehouse.id = wave.warehouse_id AND warehouse.tenant_id = wave.tenant_id AND warehouse.deleted_at IS NULL").
		Joins("LEFT JOIN shops AS shop ON shop.id = orders.shop_id AND shop.tenant_id = orders.tenant_id AND shop.deleted_at IS NULL").
		Joins("LEFT JOIN fulfillment_wave_pack_verifications AS verification ON verification.tenant_id = wave_order.tenant_id AND verification.wave_order_id = wave_order.id").
		Where("wave_order.tenant_id = ? AND wave_order.status = ? AND orders.fulfillment_status = ?", tenantID, FulfillmentWaveOrderFulfilled, "fulfilled").
		Where(`wave_order.id = (
			SELECT newer.id FROM fulfillment_wave_orders AS newer
			JOIN fulfillment_waves AS newer_wave ON newer_wave.id = newer.wave_id AND newer_wave.tenant_id = newer.tenant_id AND newer_wave.deleted_at IS NULL
			WHERE newer.tenant_id = wave_order.tenant_id AND newer.order_id = wave_order.order_id AND newer.status = ?
			ORDER BY newer.fulfilled_at DESC, newer.created_at DESC, newer.id DESC LIMIT 1
		)`, FulfillmentWaveOrderFulfilled)
	if scope.RestrictStoreScope {
		if len(scope.AllowedShopIDs) == 0 {
			q = q.Where("1 = 0")
		} else {
			q = q.Where("orders.shop_id IN ?", scope.AllowedShopIDs)
		}
	}
	return q
}

func selectWarehouseFeeFacts(q *gorm.DB) *gorm.DB {
	return q.Select(`orders.id AS order_id, orders.order_no, orders.shop_id,
		COALESCE(shop.shop_name, '') AS shop_name, orders.platform, orders.currency,
		warehouse.id AS warehouse_id, warehouse.code AS warehouse_code, warehouse.name AS warehouse_name,
		wave.id AS wave_id, wave.wave_no, wave.revision AS wave_revision, wave.status AS wave_status,
		wave_order.id AS wave_order_id, wave_order.status AS wave_order_status, wave_order.shipment_id,
		verification.id AS pack_verification_id, COALESCE(verification.package_code, wave_order.package_code, '') AS package_code,
		COALESCE((SELECT SUM(line.required_qty) FROM fulfillment_wave_lines AS line
			WHERE line.tenant_id = wave_order.tenant_id AND line.wave_order_id = wave_order.id), 0) AS item_quantity,
		CASE WHEN verification.id IS NULL THEN 0 ELSE 1 END AS package_quantity,
		wave_order.fulfilled_at, verification.created_at AS verified_at`)
}

func (s *Service) ListWarehouseFeeFacts(ctx context.Context, db *gorm.DB, tenantID int64, scope WarehouseFeeScope, in WarehouseFeeFactQuery) (*WarehouseFeeFactList, error) {
	if db == nil {
		if s == nil {
			return nil, ErrFulfillmentWaveInvalidInput
		}
		db = s.DB
	}
	if db == nil || tenantID < 0 {
		return nil, ErrFulfillmentWaveInvalidInput
	}
	if in.Page < 1 {
		in.Page = 1
	}
	if in.PageSize < 1 {
		in.PageSize = 20
	}
	if in.PageSize > 100 {
		return nil, ErrFulfillmentWaveInvalidInput
	}
	q := warehouseFeeFactQuery(db.WithContext(ctx), tenantID, scope)
	if value := strings.TrimSpace(in.OrderNo); value != "" {
		q = q.Where("LOWER(orders.order_no) LIKE ?", "%"+strings.ToLower(value)+"%")
	}
	if in.WarehouseID != nil && *in.WarehouseID != uuid.Nil {
		q = q.Where("wave.warehouse_id = ?", *in.WarehouseID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	rows := make([]WarehouseFeeFact, 0)
	if total > 0 {
		if err := selectWarehouseFeeFacts(q).Order("wave_order.fulfilled_at DESC, wave_order.id DESC").Offset((in.Page - 1) * in.PageSize).Limit(in.PageSize).Scan(&rows).Error; err != nil {
			return nil, err
		}
	}
	return &WarehouseFeeFactList{List: rows, Page: in.Page, PageSize: in.PageSize, Total: total, TotalPages: int((total + int64(in.PageSize) - 1) / int64(in.PageSize))}, nil
}

func (s *Service) GetWarehouseFeeFact(ctx context.Context, db *gorm.DB, tenantID int64, scope WarehouseFeeScope, orderID uuid.UUID, lock bool) (*WarehouseFeeFact, error) {
	if db == nil {
		if s == nil {
			return nil, ErrFulfillmentWaveInvalidInput
		}
		db = s.DB
	}
	if db == nil || tenantID < 0 || orderID == uuid.Nil {
		return nil, ErrFulfillmentWaveInvalidInput
	}
	var fact WarehouseFeeFact
	load := func() error {
		return selectWarehouseFeeFacts(warehouseFeeFactQuery(db.WithContext(ctx), tenantID, scope)).Where("orders.id = ?", orderID).Take(&fact).Error
	}
	if err := load(); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFulfillmentWaveNotFound
		}
		return nil, err
	}
	if lock {
		for _, target := range []struct {
			model any
			id    uuid.UUID
		}{{&Order{}, fact.OrderID}, {&FulfillmentWave{}, fact.WaveID}, {&FulfillmentWaveOrder{}, fact.WaveOrderID}} {
			if err := db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, target.id).First(target.model).Error; err != nil {
				return nil, err
			}
		}
		if err := load(); err != nil {
			return nil, err
		}
	}
	return &fact, nil
}
