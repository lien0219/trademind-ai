package order

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

const (
	ReconciliationMatched  = "matched"
	ReconciliationPending  = "pending"
	ReconciliationMismatch = "mismatch"
	ReconciliationBlocked  = "blocked"
)

func validReconciliationStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case ReconciliationMatched, ReconciliationPending, ReconciliationMismatch, ReconciliationBlocked:
		return true
	default:
		return false
	}
}

type FulfillmentReconciliationQuery struct {
	Page                 int
	PageSize             int
	OrderNo              string
	WarehouseID          *uuid.UUID
	Status               string
	FulfillmentStatus    string
	ReconciliationStatus string
}

type FulfillmentActionSummary struct {
	Expected int `json:"expected"`
	Actual   int `json:"actual"`
}

type FulfillmentTimelineEntry struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Action    string    `json:"action"`
	Status    string    `json:"status,omitempty"`
	Quantity  int       `json:"quantity,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type FulfillmentReconciliation struct {
	OrderID               uuid.UUID                  `json:"orderId"`
	OrderNo               string                     `json:"orderNo"`
	Status                string                     `json:"status"`
	PaymentStatus         string                     `json:"paymentStatus"`
	FulfillmentStatus     string                     `json:"fulfillmentStatus"`
	WarehouseID           *uuid.UUID                 `json:"warehouseId,omitempty"`
	Reserve               FulfillmentActionSummary   `json:"reserve"`
	Deduct                FulfillmentActionSummary   `json:"deduct"`
	Release               FulfillmentActionSummary   `json:"release"`
	Restore               FulfillmentActionSummary   `json:"restore"`
	ShipmentCount         int                        `json:"shipmentCount"`
	EffectCount           int                        `json:"effectCount"`
	LastInventoryActionAt *time.Time                 `json:"lastInventoryActionAt,omitempty"`
	ReconciliationStatus  string                     `json:"reconciliationStatus"`
	Issues                []string                   `json:"issues,omitempty"`
	Timeline              []FulfillmentTimelineEntry `json:"timeline,omitempty"`
}

type FulfillmentReconciliationPage struct {
	Items      []FulfillmentReconciliation `json:"list"`
	Total      int64                       `json:"total"`
	Page       int                         `json:"page"`
	PageSize   int                         `json:"pageSize"`
	TotalPages int                         `json:"totalPages"`
}

type reconciliationItemAgg struct {
	OrderID      uuid.UUID
	ID           uuid.UUID
	ProductSKUID *uuid.UUID
	Quantity     int
}

type reconciliationEffectAgg struct {
	ID           uuid.UUID
	OrderID      uuid.UUID
	OrderItemID  uuid.UUID
	ProductSKUID uuid.UUID
	EffectType   string
	Quantity     int
	Status       string
	WarehouseID  *uuid.UUID
	CreatedAt    time.Time
}

type reconciliationMovementAgg struct {
	ID           uuid.UUID
	SourceID     uuid.UUID
	MovementType string
	Quantity     int
	CreatedAt    time.Time
}

type reconciliationBuildData struct {
	Items             map[uuid.UUID][]reconciliationItemAgg
	Effects           map[uuid.UUID][]reconciliationEffectAgg
	Movements         map[uuid.UUID][]reconciliationMovementAgg
	Shipments         map[uuid.UUID][]OrderShipment
	PackVerifications map[uuid.UUID][]FulfillmentWavePackVerification
	LegacyFacts       map[uuid.UUID]bool
	Balances          map[string]bool
	Warehouses        map[uuid.UUID]bool
	SKUs              map[uuid.UUID]bool
}

func reconciliationPageTotal(total int64, pageSize int) int {
	return pagesOf(total, pageSize)
}

func (s *Service) buildReconciliationData(c *gin.Context, orders []Order) (reconciliationBuildData, error) {
	d := reconciliationBuildData{
		Items: map[uuid.UUID][]reconciliationItemAgg{}, Effects: map[uuid.UUID][]reconciliationEffectAgg{},
		Movements: map[uuid.UUID][]reconciliationMovementAgg{}, Shipments: map[uuid.UUID][]OrderShipment{},
		PackVerifications: map[uuid.UUID][]FulfillmentWavePackVerification{},
		LegacyFacts:       map[uuid.UUID]bool{},
		Balances:          map[string]bool{}, Warehouses: map[uuid.UUID]bool{}, SKUs: map[uuid.UUID]bool{},
	}
	if len(orders) == 0 {
		return d, nil
	}
	ctx := c.Request.Context()
	orderIDs := make([]uuid.UUID, 0, len(orders))
	warehouseIDs := make([]uuid.UUID, 0)
	for _, o := range orders {
		orderIDs = append(orderIDs, o.ID)
		if o.WarehouseID != nil && *o.WarehouseID != uuid.Nil {
			warehouseIDs = append(warehouseIDs, *o.WarehouseID)
		}
	}
	var items []OrderItem
	if s.DB.Migrator().HasTable(&OrderItem{}) {
		if err := s.DB.WithContext(ctx).Where("order_id IN ?", orderIDs).Order("created_at ASC, id ASC").Find(&items).Error; err != nil {
			return d, err
		}
	}
	itemIDs := make([]uuid.UUID, 0, len(items))
	skuIDs := make([]uuid.UUID, 0)
	for _, it := range items {
		d.Items[it.OrderID] = append(d.Items[it.OrderID], reconciliationItemAgg{OrderID: it.OrderID, ID: it.ID, ProductSKUID: it.ProductSKUID, Quantity: it.Quantity})
		itemIDs = append(itemIDs, it.ID)
		if it.ProductSKUID != nil && *it.ProductSKUID != uuid.Nil {
			skuIDs = append(skuIDs, *it.ProductSKUID)
		}
	}
	if len(itemIDs) > 0 && s.DB.Migrator().HasTable(&inventory.OrderInventoryEffect{}) {
		var effects []inventory.OrderInventoryEffect
		if err := s.DB.WithContext(ctx).Where("(tenant_id = ? OR tenant_id = 0) AND order_id IN ?", orders[0].TenantID, orderIDs).Order("created_at ASC, id ASC").Find(&effects).Error; err != nil {
			return d, err
		}
		for _, e := range effects {
			d.Effects[e.OrderID] = append(d.Effects[e.OrderID], reconciliationEffectAgg{ID: e.ID, OrderID: e.OrderID, OrderItemID: e.OrderItemID, ProductSKUID: e.ProductSKUID, EffectType: e.EffectType, Quantity: e.Quantity, Status: e.Status, WarehouseID: e.WarehouseID, CreatedAt: e.CreatedAt})
			if orders[0].TenantID != 0 && e.TenantID == 0 {
				d.LegacyFacts[e.OrderID] = true
			}
		}
	}
	if len(itemIDs) > 0 && s.DB.Migrator().HasTable(&inventory.InventoryMovement{}) {
		var movements []inventory.InventoryMovement
		if err := s.DB.WithContext(ctx).Where("(tenant_id = ? OR tenant_id = 0) AND source_type = ? AND source_id IN ?", orders[0].TenantID, "order_item", itemIDs).Order("created_at ASC, id ASC").Find(&movements).Error; err != nil {
			return d, err
		}
		for _, m := range movements {
			d.Movements[m.SourceID] = append(d.Movements[m.SourceID], reconciliationMovementAgg{ID: m.ID, SourceID: m.SourceID, MovementType: m.MovementType, Quantity: m.Quantity, CreatedAt: m.CreatedAt})
			if orders[0].TenantID != 0 && m.TenantID == 0 {
				for _, item := range items {
					if item.ID == m.SourceID {
						d.LegacyFacts[item.OrderID] = true
						break
					}
				}
			}
		}
	}
	var shipments []OrderShipment
	if s.DB.Migrator().HasTable(&OrderShipment{}) {
		if err := s.DB.WithContext(ctx).Where("order_id IN ?", orderIDs).Order("created_at ASC, id ASC").Find(&shipments).Error; err != nil {
			return d, err
		}
	}
	for _, sh := range shipments {
		d.Shipments[sh.OrderID] = append(d.Shipments[sh.OrderID], sh)
	}
	if s.DB.Migrator().HasTable(&FulfillmentWavePackVerification{}) {
		var verifications []FulfillmentWavePackVerification
		if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND order_id IN ?", orders[0].TenantID, orderIDs).Order("created_at ASC, id ASC").Find(&verifications).Error; err != nil {
			return d, err
		}
		for _, verification := range verifications {
			d.PackVerifications[verification.OrderID] = append(d.PackVerifications[verification.OrderID], verification)
		}
	}
	if len(warehouseIDs) > 0 && s.DB.Migrator().HasTable("warehouses") {
		type warehouseRow struct {
			ID uuid.UUID `gorm:"column:id"`
		}
		var rows []warehouseRow
		if err := s.DB.WithContext(ctx).Table("warehouses").Select("id").Where("id IN ? AND tenant_id = ? AND deleted_at IS NULL", warehouseIDs, orders[0].TenantID).Scan(&rows).Error; err != nil {
			return d, err
		}
		for _, row := range rows {
			d.Warehouses[row.ID] = true
		}
	}
	if len(skuIDs) > 0 && s.DB.Migrator().HasTable("product_skus") && s.DB.Migrator().HasTable("products") {
		type skuRow struct {
			ID uuid.UUID `gorm:"column:id"`
		}
		var rows []skuRow
		if err := s.DB.WithContext(ctx).Table("product_skus AS sk").Select("sk.id").Joins("JOIN products p ON p.id = sk.product_id AND p.deleted_at IS NULL").Where("sk.id IN ? AND p.tenant_id = ? AND sk.deleted_at IS NULL", skuIDs, orders[0].TenantID).Scan(&rows).Error; err != nil {
			return d, err
		}
		for _, row := range rows {
			d.SKUs[row.ID] = true
		}
	}
	if len(warehouseIDs) > 0 && len(skuIDs) > 0 && s.DB.Migrator().HasTable(&inventory.WarehouseStockBalance{}) {
		var balances []inventory.WarehouseStockBalance
		if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND warehouse_id IN ? AND product_sku_id IN ?", orders[0].TenantID, warehouseIDs, skuIDs).Find(&balances).Error; err != nil {
			return d, err
		}
		for _, b := range balances {
			d.Balances[fmt.Sprintf("%s:%s", b.WarehouseID, b.ProductSKUID)] = true
		}
	}
	return d, nil
}

func reconciliationExpected(o Order, items []reconciliationItemAgg, effects []reconciliationEffectAgg) (reserve, deduct, release, restore int) {
	reserveActual, deductActual := 0, 0
	for _, e := range effects {
		if e.Status != inventory.InventoryEffectSuccess {
			continue
		}
		switch e.EffectType {
		case inventory.EffectTypeReserve:
			reserveActual += e.Quantity
		case inventory.EffectTypeDeduct:
			deductActual += e.Quantity
		}
	}
	itemQuantity := 0
	for _, it := range items {
		if it.Quantity > 0 {
			itemQuantity += it.Quantity
		}
	}
	paidFlow := o.PaymentStatus == PaymentPaid || o.Status == StatusPaid || o.Status == StatusProcessing
	shippedFlow := o.Status == StatusShipped || o.Status == StatusDelivered || o.FulfillmentStatus == FulfillmentFulfilled
	if paidFlow || (shippedFlow && reserveActual > 0) {
		reserve = itemQuantity
	}
	if shippedFlow {
		deduct = itemQuantity
	}
	if o.Status == StatusCancelled || o.Status == StatusClosed {
		if reserve > 0 || reserveActual > 0 {
			if reserve == 0 {
				reserve = reserveActual
			}
			release = reserve
		}
	}
	if o.Status == StatusRefunded || o.PaymentStatus == PaymentRefunded || o.PaymentStatus == PaymentPartiallyRefunded {
		if deductActual > 0 {
			// A refund after an actual deduction keeps the original outbound
			// expectation and adds a restore expectation. Direct-deduct legacy
			// orders remain valid when no reservation fact exists.
			deduct = itemQuantity
			restore = itemQuantity
			if reserveActual > 0 {
				reserve = itemQuantity
			}
		} else if reserveActual > 0 {
			reserve = reserveActual
			release = reserveActual
		}
	}
	return
}

func buildOneReconciliation(o Order, d reconciliationBuildData, timeline bool) FulfillmentReconciliation {
	r := FulfillmentReconciliation{OrderID: o.ID, OrderNo: o.OrderNo, Status: o.Status, PaymentStatus: o.PaymentStatus, FulfillmentStatus: o.FulfillmentStatus, WarehouseID: o.WarehouseID, ReconciliationStatus: ReconciliationPending, Issues: []string{}}
	items, effects := d.Items[o.ID], d.Effects[o.ID]
	r.EffectCount = len(effects)
	r.ShipmentCount = len(d.Shipments[o.ID])
	r.Reserve.Expected, r.Deduct.Expected, r.Release.Expected, r.Restore.Expected = reconciliationExpected(o, items, effects)
	seen := map[string]int{}
	movementActual := map[string]int{}
	for _, e := range effects {
		if e.Status == inventory.InventoryEffectSuccess {
			switch e.EffectType {
			case inventory.EffectTypeReserve:
				r.Reserve.Actual += e.Quantity
			case inventory.EffectTypeDeduct:
				r.Deduct.Actual += e.Quantity
			case inventory.EffectTypeRelease:
				r.Release.Actual += e.Quantity
			case inventory.EffectTypeRestore:
				r.Restore.Actual += e.Quantity
			}
		}
		key := fmt.Sprintf("%s:%s:%s", e.OrderItemID, e.ProductSKUID, e.EffectType)
		seen[key]++
		if e.Status != inventory.InventoryEffectSuccess && (e.Status == inventory.InventoryEffectFailed || e.Status == inventory.InventoryEffectPending) {
			r.Issues = append(r.Issues, "inventory_effect_"+e.Status)
		}
		if e.CreatedAt.After(time.Time{}) && (r.LastInventoryActionAt == nil || e.CreatedAt.After(*r.LastInventoryActionAt)) {
			t := e.CreatedAt
			r.LastInventoryActionAt = &t
		}
	}
	for _, it := range items {
		for _, m := range d.Movements[it.ID] {
			switch m.MovementType {
			case inventory.MovementOrderReserve:
				movementActual[inventory.EffectTypeReserve] += m.Quantity
			case inventory.MovementOrderDeduct, inventory.MovementOrderRelease:
				movementActual[map[string]string{inventory.MovementOrderDeduct: inventory.EffectTypeDeduct, inventory.MovementOrderRelease: inventory.EffectTypeRelease}[m.MovementType]] -= m.Quantity
			case inventory.MovementOrderRestore:
				movementActual[inventory.EffectTypeRestore] += m.Quantity
			}
			if m.CreatedAt.After(time.Time{}) && (r.LastInventoryActionAt == nil || m.CreatedAt.After(*r.LastInventoryActionAt)) {
				t := m.CreatedAt
				r.LastInventoryActionAt = &t
			}
		}
	}
	if d.LegacyFacts[o.ID] {
		r.Issues = append(r.Issues, "inventory_history_not_migrated")
	}
	for _, n := range seen {
		if n > 1 {
			r.Issues = append(r.Issues, "duplicate_effect")
		}
	}
	for _, it := range items {
		if it.ProductSKUID == nil || *it.ProductSKUID == uuid.Nil || !d.SKUs[*it.ProductSKUID] {
			r.Issues = append(r.Issues, "sku_not_bound")
			continue
		}
		if o.WarehouseID == nil || *o.WarehouseID == uuid.Nil || !d.Warehouses[*o.WarehouseID] {
			r.Issues = append(r.Issues, "warehouse_missing")
			continue
		}
		if !d.Balances[fmt.Sprintf("%s:%s", *o.WarehouseID, *it.ProductSKUID)] {
			r.Issues = append(r.Issues, "inventory_balance_missing")
		}
	}
	if r.Reserve.Expected != r.Reserve.Actual {
		r.Issues = append(r.Issues, "reserve_quantity_mismatch")
	}
	if r.Deduct.Expected != r.Deduct.Actual {
		r.Issues = append(r.Issues, "deduct_quantity_mismatch")
	}
	if r.Release.Expected != r.Release.Actual {
		r.Issues = append(r.Issues, "release_quantity_mismatch")
	}
	if r.Restore.Expected != r.Restore.Actual {
		r.Issues = append(r.Issues, "restore_quantity_mismatch")
	}
	for effectType, actual := range map[string]int{
		inventory.EffectTypeReserve: r.Reserve.Actual,
		inventory.EffectTypeDeduct:  r.Deduct.Actual,
		inventory.EffectTypeRelease: r.Release.Actual,
		inventory.EffectTypeRestore: r.Restore.Actual,
	} {
		if actual != movementActual[effectType] {
			r.Issues = append(r.Issues, "inventory_movement_quantity_mismatch")
			break
		}
	}
	if (o.Status == StatusShipped || o.Status == StatusDelivered || o.FulfillmentStatus == FulfillmentFulfilled) && r.ShipmentCount == 0 {
		r.Issues = append(r.Issues, "shipment_missing")
	}
	if (o.Status == StatusPending || o.Status == StatusPaid || o.Status == StatusProcessing) && r.ShipmentCount > 0 {
		r.Issues = append(r.Issues, "shipment_state_conflict")
	}
	if timeline {
		for _, e := range effects {
			r.Timeline = append(r.Timeline, FulfillmentTimelineEntry{ID: e.ID.String(), Type: "effect", Action: e.EffectType, Status: e.Status, Quantity: e.Quantity, CreatedAt: e.CreatedAt})
		}
		for _, it := range items {
			for _, m := range d.Movements[it.ID] {
				r.Timeline = append(r.Timeline, FulfillmentTimelineEntry{ID: m.ID.String(), Type: "movement", Action: m.MovementType, Quantity: m.Quantity, CreatedAt: m.CreatedAt})
				if r.LastInventoryActionAt == nil || m.CreatedAt.After(*r.LastInventoryActionAt) {
					t := m.CreatedAt
					r.LastInventoryActionAt = &t
				}
			}
		}
		for _, sh := range d.Shipments[o.ID] {
			t := sh.CreatedAt
			r.Timeline = append(r.Timeline, FulfillmentTimelineEntry{ID: sh.ID.String(), Type: "shipment", Action: sh.Status, CreatedAt: t})
		}
		for _, verification := range d.PackVerifications[o.ID] {
			r.Timeline = append(r.Timeline, FulfillmentTimelineEntry{
				ID: verification.ID.String(), Type: "packing_verification", Action: "verified", Status: "validated",
				Quantity: verification.VerifiedQty, CreatedAt: verification.CreatedAt,
			})
		}
		sort.SliceStable(r.Timeline, func(i, j int) bool {
			if r.Timeline[i].CreatedAt.Equal(r.Timeline[j].CreatedAt) {
				return r.Timeline[i].Type < r.Timeline[j].Type
			}
			return r.Timeline[i].CreatedAt.Before(r.Timeline[j].CreatedAt)
		})
	}
	uniq := map[string]bool{}
	clean := r.Issues[:0]
	for _, issue := range r.Issues {
		if !uniq[issue] {
			uniq[issue] = true
			clean = append(clean, issue)
		}
	}
	r.Issues = clean
	blocked := false
	for _, issue := range r.Issues {
		if issue == "sku_not_bound" || issue == "warehouse_missing" || issue == "inventory_balance_missing" || issue == "inventory_history_not_migrated" {
			blocked = true
		}
	}
	if blocked {
		r.ReconciliationStatus = ReconciliationBlocked
	} else if len(r.Issues) > 0 {
		r.ReconciliationStatus = ReconciliationMismatch
	} else if r.Reserve.Expected == 0 && r.Deduct.Expected == 0 && r.Release.Expected == 0 && r.Restore.Expected == 0 {
		r.ReconciliationStatus = ReconciliationPending
	} else {
		r.ReconciliationStatus = ReconciliationMatched
	}
	return r
}

func (s *Service) FulfillmentReconciliation(c *gin.Context, q FulfillmentReconciliationQuery) (*FulfillmentReconciliationPage, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 || q.PageSize > 200 {
		q.PageSize = 20
	}
	tx := s.DB.WithContext(c.Request.Context()).Model(&Order{})
	if q.OrderNo != "" {
		tx = tx.Where("order_no ILIKE ?", "%"+strings.TrimSpace(q.OrderNo)+"%")
	}
	if q.WarehouseID != nil && *q.WarehouseID != uuid.Nil {
		tx = tx.Where("warehouse_id = ?", *q.WarehouseID)
	}
	if q.Status != "" {
		tx = tx.Where("status = ?", strings.TrimSpace(q.Status))
	}
	if q.FulfillmentStatus != "" {
		tx = tx.Where("fulfillment_status = ?", strings.TrimSpace(q.FulfillmentStatus))
	}
	var err error
	if tx, _, err = adminperm.ApplyTenantScope(c, tx); err != nil {
		return nil, err
	}
	if tx, err = adminperm.ApplyStoreScope(c, s.DB, tx, "shop_id"); err != nil {
		return nil, err
	}
	if status := strings.TrimSpace(q.ReconciliationStatus); status != "" {
		matchedIDs, matchErr := s.reconciliationMatchedIDs(c, tx, status)
		if matchErr != nil {
			return nil, matchErr
		}
		if len(matchedIDs) == 0 {
			tx = tx.Where("1 = 0")
		} else {
			tx = tx.Where("id IN ?", matchedIDs)
		}
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	var orders []Order
	if err := tx.Order("created_at DESC, id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&orders).Error; err != nil {
		return nil, err
	}
	d, err := s.buildReconciliationData(c, orders)
	if err != nil {
		return nil, err
	}
	items := make([]FulfillmentReconciliation, 0, len(orders))
	for _, o := range orders {
		row := buildOneReconciliation(o, d, false)
		if q.ReconciliationStatus != "" && row.ReconciliationStatus != q.ReconciliationStatus {
			continue
		}
		items = append(items, row)
	}
	return &FulfillmentReconciliationPage{Items: items, Total: total, Page: q.Page, PageSize: q.PageSize, TotalPages: reconciliationPageTotal(total, q.PageSize)}, nil
}

// reconciliationMatchedIDs evaluates the derived status before pagination.
// Inventory facts are stored in separate ledgers, so this bounded batch scan
// keeps the result stable without loading the entire order table at once.
func (s *Service) reconciliationMatchedIDs(c *gin.Context, tx *gorm.DB, status string) ([]uuid.UUID, error) {
	if !validReconciliationStatus(status) {
		return nil, fmt.Errorf("invalid reconciliationStatus")
	}
	const batchSize = 200
	matched := make([]uuid.UUID, 0)
	for offset := 0; ; offset += batchSize {
		var orders []Order
		if err := tx.Session(&gorm.Session{}).Order("created_at DESC, id DESC").Offset(offset).Limit(batchSize).Find(&orders).Error; err != nil {
			return nil, err
		}
		if len(orders) == 0 {
			break
		}
		d, err := s.buildReconciliationData(c, orders)
		if err != nil {
			return nil, err
		}
		for _, o := range orders {
			if buildOneReconciliation(o, d, false).ReconciliationStatus == status {
				matched = append(matched, o.ID)
			}
		}
		if len(orders) < batchSize {
			break
		}
	}
	return matched, nil
}

func (s *Service) FulfillmentReconciliationDetail(c *gin.Context, id uuid.UUID) (*FulfillmentReconciliation, error) {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var o Order
	if err := s.DB.WithContext(c.Request.Context()).Where("id = ? AND tenant_id = ?", id, tid).First(&o).Error; err != nil {
		return nil, err
	}
	if err := adminperm.EnsureStoreVisible(c, s.DB, o.ShopID); err != nil {
		return nil, err
	}
	d, err := s.buildReconciliationData(c, []Order{o})
	if err != nil {
		return nil, err
	}
	return func() *FulfillmentReconciliation { r := buildOneReconciliation(o, d, true); return &r }(), nil
}

func (s *Service) ReconciliationStatuses(c *gin.Context, orders []Order) (map[uuid.UUID]string, error) {
	d, err := s.buildReconciliationData(c, orders)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]string{}
	for _, o := range orders {
		out[o.ID] = buildOneReconciliation(o, d, false).ReconciliationStatus
	}
	return out, nil
}

func (h *Handler) ListFulfillmentReconciliation(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "orders unavailable")
		return
	}
	if h.denyRead(c) {
		return
	}
	q := FulfillmentReconciliationQuery{Page: atoiQ(c, "page", 1), PageSize: atoiQ(c, "pageSize", 20), OrderNo: strings.TrimSpace(c.Query("orderNo")), Status: strings.TrimSpace(c.Query("status")), FulfillmentStatus: strings.TrimSpace(c.Query("fulfillmentStatus")), ReconciliationStatus: strings.TrimSpace(c.Query("reconciliationStatus"))}
	if q.ReconciliationStatus != "" && !validReconciliationStatus(q.ReconciliationStatus) {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid reconciliationStatus")
		return
	}
	if raw := strings.TrimSpace(c.Query("warehouseId")); raw != "" {
		id, parseErr := uuid.Parse(raw)
		if parseErr != nil {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid warehouseId")
			return
		}
		q.WarehouseID = &id
	}
	res, err := h.Svc.FulfillmentReconciliation(c, q)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"list": res.Items, "pagination": gin.H{"page": res.Page, "pageSize": res.PageSize, "total": res.Total, "totalPages": res.TotalPages}})
}

func (h *Handler) GetFulfillmentReconciliation(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "orders unavailable")
		return
	}
	if h.denyRead(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid id")
		return
	}
	row, err := h.Svc.FulfillmentReconciliationDetail(c, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "not found")
		} else {
			response.HandleError(c, err)
		}
		return
	}
	response.OK(c, row)
}
