package order

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ErrNotFound is returned when order is absent or soft-deleted.
var ErrNotFound = errors.New("order not found")

// Service orchestrates internal orders manually entered from admin (no marketplace sync).
type Service struct {
	DB       *gorm.DB
	OpLog    *operationlog.Service
	Shops    *shop.Service
	Settings *settings.Service
}

// AIContext holds serializable subsets for Prompt / ai_tasks audit (minimal PII).
type AIContext struct {
	OrderInfo    map[string]any   `json:"orderInfo,omitempty"`
	ShipmentInfo []map[string]any `json:"shipmentInfo,omitempty"`
	OrderItems   []map[string]any `json:"orderItems,omitempty"`
}

// ConversationOrderSummary is safe for admin detail (omit email/phone in AI prompt).
type ConversationOrderSummary struct {
	ID                   uuid.UUID      `json:"id"`
	OrderNo              string         `json:"orderNo"`
	Platform             string         `json:"platform"`
	Status               string         `json:"status"`
	PaymentStatus        string         `json:"paymentStatus"`
	FulfillmentStatus    string         `json:"fulfillmentStatus"`
	Currency             string         `json:"currency"`
	TotalAmount          float64        `json:"totalAmount"`
	OrderedAt            *time.Time     `json:"orderedAt,omitempty"`
	LatestShipmentStatus string         `json:"latestShipmentStatus,omitempty"`
	Shipments            []ShipmentCard `json:"shipments,omitempty"`
}

// ShipmentCard is a compact shipment row for UI + AI summaries.
type ShipmentCard struct {
	Carrier     string     `json:"carrier"`
	TrackingNo  string     `json:"trackingNo"`
	TrackingURL string     `json:"trackingUrl,omitempty"`
	Status      string     `json:"status"`
	ShippedAt   *time.Time `json:"shippedAt,omitempty"`
	DeliveredAt *time.Time `json:"deliveredAt,omitempty"`
}

// ListQuery GET /orders
type ListQuery struct {
	Page                  int
	PageSize              int
	Platform              string
	ShopID                *uuid.UUID
	OrderNo               string
	CustomerName          string
	Keyword               string
	Status                string
	PaymentStatus         string
	FulfillmentStatus     string
	SkuMatchStatus        string
	InventoryDeductStatus string
	SyncStatus            string
	HasException          bool
	Start                 *time.Time
	End                   *time.Time
}

// ListOrderRow is returned from list endpoint.
type ListOrderRow struct {
	ID                    uuid.UUID  `json:"id"`
	Platform              string     `json:"platform"`
	ShopID                *uuid.UUID `json:"shopId,omitempty"`
	ShopName              string     `json:"shopName,omitempty"`
	ShopPlatform          string     `json:"shopPlatform,omitempty"`
	ExternalOrderID       string     `json:"externalOrderId,omitempty"`
	OrderNo               string     `json:"orderNo"`
	CustomerName          string     `json:"customerName"`
	Status                string     `json:"status"`
	PaymentStatus         string     `json:"paymentStatus"`
	FulfillmentStatus     string     `json:"fulfillmentStatus"`
	Currency              string     `json:"currency"`
	TotalAmount           float64    `json:"totalAmount"`
	ItemCount             int        `json:"itemCount"`
	SkuMatchStatus        string     `json:"skuMatchStatus,omitempty"`
	SkuMatchedCount       int        `json:"skuMatchedCount"`
	SkuTotalCount         int        `json:"skuTotalCount"`
	InventoryDeductStatus string     `json:"inventoryDeductStatus,omitempty"`
	SyncStatus            string     `json:"syncStatus,omitempty"`
	OpenExceptionCount    int        `json:"openExceptionCount"`
	DetailURL             string     `json:"detailUrl,omitempty"`
	OrderedAt             *time.Time `json:"orderedAt,omitempty"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
	LatestShipmentStatus  string     `json:"latestShipmentStatus,omitempty"`
}

// ListResult pagination bundle.
type ListResult struct {
	Items      []ListOrderRow
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
}

func pagesOf(total int64, ps int) int {
	if ps < 1 {
		ps = 20
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

func (s *Service) validateShopRef(c *gin.Context, id *uuid.UUID) error {
	if id == nil || *id == uuid.Nil {
		return nil
	}
	if s.Shops == nil {
		return nil
	}
	ok, err := s.Shops.Exists(c, *id)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("shop not found")
	}
	return nil
}

// OrderItemInput for create/update body.
type OrderItemInput struct {
	ID             *uuid.UUID     `json:"id"`
	ProductID      *uuid.UUID     `json:"productId,omitempty"`
	ProductSKUID   *uuid.UUID     `json:"productSkuId,omitempty"`
	ExternalItemID *string        `json:"externalItemId,omitempty"`
	ProductTitle   string         `json:"productTitle"`
	SKUName        string         `json:"skuName"`
	SKUCode        string         `json:"skuCode"`
	Quantity       int            `json:"quantity"`
	UnitPrice      float64        `json:"unitPrice"`
	TotalPrice     float64        `json:"totalPrice"`
	ImageURL       string         `json:"imageUrl,omitempty"`
	Attrs          map[string]any `json:"attrs,omitempty"`
}

// OrderShipmentInput for create/update body.
type OrderShipmentInput struct {
	ID          *uuid.UUID `json:"id"`
	Carrier     string     `json:"carrier"`
	TrackingNo  string     `json:"trackingNo"`
	TrackingURL string     `json:"trackingUrl,omitempty"`
	Status      string     `json:"status"`
	ShippedAt   *time.Time `json:"shippedAt,omitempty"`
	DeliveredAt *time.Time `json:"deliveredAt,omitempty"`
}

// CreateBody POST /orders
type CreateBody struct {
	Platform          string               `json:"platform"`
	ShopID            *uuid.UUID           `json:"shopId,omitempty"`
	ExternalOrderID   *string              `json:"externalOrderId,omitempty"`
	OrderNo           string               `json:"orderNo"`
	CustomerName      string               `json:"customerName"`
	CustomerEmail     string               `json:"customerEmail,omitempty"`
	CustomerPhone     string               `json:"customerPhone,omitempty"`
	Status            string               `json:"status"`
	PaymentStatus     string               `json:"paymentStatus"`
	FulfillmentStatus string               `json:"fulfillmentStatus"`
	Currency          string               `json:"currency"`
	TotalAmount       float64              `json:"totalAmount"`
	PaidAt            *time.Time           `json:"paidAt,omitempty"`
	OrderedAt         *time.Time           `json:"orderedAt,omitempty"`
	ShippedAt         *time.Time           `json:"shippedAt,omitempty"`
	DeliveredAt       *time.Time           `json:"deliveredAt,omitempty"`
	Items             []OrderItemInput     `json:"items,omitempty"`
	Shipments         []OrderShipmentInput `json:"shipments,omitempty"`

	DeductInventory bool `json:"deductInventory"`
	SyncInventory   bool `json:"syncInventory"`
}

// UpdateBody PATCH-like PUT semantics (only non-nil / non-empty fragments apply).
type UpdateBody struct {
	ShopID            *uuid.UUID           `json:"shopId,omitempty"`
	SetShopIDNil      bool                 `json:"setShopIdNil,omitempty"`
	ExternalOrderID   *string              `json:"externalOrderId,omitempty"`
	Status            string               `json:"status,omitempty"`
	PaymentStatus     string               `json:"paymentStatus,omitempty"`
	FulfillmentStatus string               `json:"fulfillmentStatus,omitempty"`
	Currency          string               `json:"currency,omitempty"`
	CustomerName      string               `json:"customerName,omitempty"`
	CustomerEmail     *string              `json:"customerEmail,omitempty"`
	CustomerPhone     *string              `json:"customerPhone,omitempty"`
	TotalAmount       *float64             `json:"totalAmount,omitempty"`
	PaidAt            *time.Time           `json:"paidAt,omitempty"`
	OrderedAt         *time.Time           `json:"orderedAt,omitempty"`
	ShippedAt         *time.Time           `json:"shippedAt,omitempty"`
	DeliveredAt       *time.Time           `json:"deliveredAt,omitempty"`
	SetPaidAtNil      bool                 `json:"setPaidAtNil,omitempty"`
	SetOrderedAtNil   bool                 `json:"setOrderedAtNil,omitempty"`
	SetShippedAtNil   bool                 `json:"setShippedAtNil,omitempty"`
	SetDeliveredAtNil bool                 `json:"setDeliveredAtNil,omitempty"`
	Items             []OrderItemInput     `json:"items,omitempty"`
	Shipments         []OrderShipmentInput `json:"shipments,omitempty"`
	ReplaceItems      bool                 `json:"replaceItems,omitempty"`
	ReplaceShipments  bool                 `json:"replaceShipments,omitempty"`
}

// DetailDTO GET /orders/:id (flattened header + nested children).
type DetailDTO struct {
	OrderRow
	ShopSummary *shop.SummaryDTO `json:"shopSummary,omitempty"`
	ShippedAt   *time.Time       `json:"shippedAt,omitempty"`
	DeliveredAt *time.Time       `json:"deliveredAt,omitempty"`
	Items       []OrderItem      `json:"items"`
	Shipments   []OrderShipment  `json:"shipments"`

	InventorySummary *InventoryUIMini `json:"inventorySummary,omitempty"`
}

// InventoryUIMini exposes stock-effect flags from order_inventory_effects without importing inventory in service helpers.
type InventoryUIMini struct {
	HasDeductionSuccess bool `json:"hasDeductionSuccess"`
	HasRestoreSuccess   bool `json:"hasRestoreSuccess"`
	FullyRestored       bool `json:"fullyRestored"`
}

// OrderRow base scalar fields shared by list-ish projections.
type OrderRow struct {
	ID                uuid.UUID  `json:"id"`
	TenantID          int64      `json:"tenantId"`
	Platform          string     `json:"platform"`
	ShopID            *uuid.UUID `json:"shopId,omitempty"`
	ExternalOrderID   *string    `json:"externalOrderId,omitempty"`
	OrderNo           string     `json:"orderNo"`
	CustomerName      string     `json:"customerName"`
	CustomerEmail     string     `json:"customerEmail,omitempty"`
	CustomerPhone     string     `json:"customerPhone,omitempty"`
	Status            string     `json:"status"`
	PaymentStatus     string     `json:"paymentStatus"`
	FulfillmentStatus string     `json:"fulfillmentStatus"`
	Currency          string     `json:"currency"`
	TotalAmount       float64    `json:"totalAmount"`
	PaidAt            *time.Time `json:"paidAt,omitempty"`
	OrderedAt         *time.Time `json:"orderedAt,omitempty"`
	Remark            string     `json:"remark,omitempty"`
	CreatedBy         *uuid.UUID `json:"createdBy,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

func mapAttrs(a map[string]any) datatypes.JSON {
	if a == nil {
		return nil
	}
	b, err := json.Marshal(a)
	if err != nil {
		return nil
	}
	return b
}

func (s *Service) normalizedCreate(body CreateBody) (*Order, []OrderItem, []OrderShipment, error) {
	platform := strings.TrimSpace(body.Platform)
	if platform == "" {
		platform = "manual"
	}
	orderNo := strings.TrimSpace(body.OrderNo)
	if orderNo == "" {
		return nil, nil, nil, fmt.Errorf("orderNo is required")
	}
	name := strings.TrimSpace(body.CustomerName)
	if name == "" {
		return nil, nil, nil, fmt.Errorf("customerName is required")
	}
	st := strings.TrimSpace(body.Status)
	if st == "" {
		st = StatusPending
	}
	if !validOrderStatus(st) {
		return nil, nil, nil, fmt.Errorf("invalid status")
	}
	ps := strings.TrimSpace(body.PaymentStatus)
	if ps == "" {
		ps = PaymentUnpaid
	}
	if !validPaymentStatus(ps) {
		return nil, nil, nil, fmt.Errorf("invalid paymentStatus")
	}
	fs := strings.TrimSpace(body.FulfillmentStatus)
	if fs == "" {
		fs = FulfillmentUnfulfilled
	}
	if !validFulfillmentStatus(fs) {
		return nil, nil, nil, fmt.Errorf("invalid fulfillmentStatus")
	}
	cur := strings.TrimSpace(body.Currency)
	if cur == "" {
		cur = "USD"
	}
	o := &Order{
		Platform:          platform,
		ShopID:            body.ShopID,
		ExternalOrderID:   body.ExternalOrderID,
		OrderNo:           orderNo,
		CustomerName:      name,
		CustomerEmail:     strings.TrimSpace(body.CustomerEmail),
		CustomerPhone:     strings.TrimSpace(body.CustomerPhone),
		Status:            st,
		PaymentStatus:     ps,
		FulfillmentStatus: fs,
		Currency:          strings.ToUpper(cur),
		TotalAmount:       body.TotalAmount,
		PaidAt:            body.PaidAt,
		OrderedAt:         body.OrderedAt,
		ShippedAt:         body.ShippedAt,
		DeliveredAt:       body.DeliveredAt,
	}

	var items []OrderItem
	for _, it := range body.Items {
		title := strings.TrimSpace(it.ProductTitle)
		if title == "" && strings.TrimSpace(it.SKUCode) != "" {
			title = strings.TrimSpace(it.SKUCode)
		}
		if title == "" {
			title = "(item)"
		}
		qty := it.Quantity
		if qty < 1 {
			qty = 1
		}
		row := OrderItem{
			ProductID:      it.ProductID,
			ProductSKUID:   it.ProductSKUID,
			ExternalItemID: it.ExternalItemID,
			ProductTitle:   title,
			SKUName:        strings.TrimSpace(it.SKUName),
			SKUCode:        strings.TrimSpace(it.SKUCode),
			Quantity:       qty,
			UnitPrice:      it.UnitPrice,
			TotalPrice:     it.TotalPrice,
			ImageURL:       strings.TrimSpace(it.ImageURL),
		}
		if len(it.Attrs) > 0 {
			row.Attrs = mapAttrs(it.Attrs)
		}
		items = append(items, row)
	}

	var shipments []OrderShipment
	for _, sh := range body.Shipments {
		cr := strings.TrimSpace(sh.Carrier)
		if cr == "" {
			cr = "unknown"
		}
		tno := strings.TrimSpace(sh.TrackingNo)
		stsh := strings.TrimSpace(sh.Status)
		if stsh == "" {
			stsh = ShipmentPending
		}
		if !validShipmentStatus(stsh) {
			return nil, nil, nil, fmt.Errorf("invalid shipment status")
		}
		shipments = append(shipments, OrderShipment{
			Carrier:     cr,
			TrackingNo:  tno,
			TrackingURL: strings.TrimSpace(sh.TrackingURL),
			Status:      stsh,
			ShippedAt:   sh.ShippedAt,
			DeliveredAt: sh.DeliveredAt,
		})
	}
	return o, items, shipments, nil
}

// List paginates orders (excludes soft-deleted).
func (s *Service) List(c *gin.Context, q ListQuery) (*ListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
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
	tx := s.DB.WithContext(c.Request.Context()).Model(&Order{})
	if v := strings.TrimSpace(q.Platform); v != "" {
		tx = tx.Where("platform = ?", v)
	}
	if q.ShopID != nil && *q.ShopID != uuid.Nil {
		tx = tx.Where("shop_id = ?", *q.ShopID)
	}
	if v := strings.TrimSpace(q.OrderNo); v != "" {
		tx = tx.Where("order_no ILIKE ?", "%"+v+"%")
	}
	if v := strings.TrimSpace(q.CustomerName); v != "" {
		tx = tx.Where("customer_name ILIKE ?", "%"+v+"%")
	}
	if v := strings.TrimSpace(q.Keyword); v != "" {
		like := "%" + v + "%"
		tx = tx.Where(
			"order_no ILIKE ? OR customer_name ILIKE ? OR external_order_id ILIKE ?",
			like, like, like,
		)
	}
	if v := strings.TrimSpace(q.Status); v != "" {
		tx = tx.Where("status = ?", v)
	}
	if v := strings.TrimSpace(q.PaymentStatus); v != "" {
		tx = tx.Where("payment_status = ?", v)
	}
	if v := strings.TrimSpace(q.FulfillmentStatus); v != "" {
		tx = tx.Where("fulfillment_status = ?", v)
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
	var rows []Order
	if err := tx.Order("created_at DESC").Offset(offset).Limit(ps).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return &ListResult{
			Items:      []ListOrderRow{},
			Total:      total,
			Page:       page,
			PageSize:   ps,
			TotalPages: pagesOf(total, ps),
		}, nil
	}
	ids := make([]uuid.UUID, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	latest := latestShipmentStatuses(s.DB.WithContext(c.Request.Context()), ids)
	shopIDs := make([]uuid.UUID, 0)
	for _, r := range rows {
		if r.ShopID != nil {
			shopIDs = append(shopIDs, *r.ShopID)
		}
	}
	var sm map[uuid.UUID]shop.SummaryDTO
	if len(shopIDs) > 0 && s.Shops != nil {
		sm, _ = s.Shops.BatchSummaries(c, shopIDs)
	}
	out := make([]ListOrderRow, len(rows))
	for i, r := range rows {
		row := ListOrderRow{
			ID:                   r.ID,
			Platform:             r.Platform,
			ShopID:               r.ShopID,
			OrderNo:              r.OrderNo,
			CustomerName:         r.CustomerName,
			Status:               r.Status,
			PaymentStatus:        r.PaymentStatus,
			FulfillmentStatus:    r.FulfillmentStatus,
			Currency:             r.Currency,
			TotalAmount:          r.TotalAmount,
			OrderedAt:            r.OrderedAt,
			CreatedAt:            r.CreatedAt,
			LatestShipmentStatus: latest[r.ID],
		}
		if r.ShopID != nil && sm != nil {
			if ssum, ok := sm[*r.ShopID]; ok {
				row.ShopName = ssum.ShopName
				row.ShopPlatform = ssum.Platform
			}
		}
		if r.ExternalOrderID != nil {
			row.ExternalOrderID = *r.ExternalOrderID
		}
		out[i] = row
	}
	enrichListRows(c.Request.Context(), s.DB, rows, out)

	if q.SkuMatchStatus != "" || q.InventoryDeductStatus != "" || q.HasException || q.SyncStatus != "" {
		out = applyListPostFilters(out, q)
		total = int64(len(out))
	}

	return &ListResult{
		Items:      out,
		Total:      total,
		Page:       page,
		PageSize:   ps,
		TotalPages: pagesOf(total, ps),
	}, nil
}

func latestShipmentStatuses(tx *gorm.DB, ids []uuid.UUID) map[uuid.UUID]string {
	latest := map[uuid.UUID]string{}
	var shipRows []OrderShipment
	_ = tx.Model(&OrderShipment{}).
		Where("order_id IN ?", ids).
		Order("order_id ASC, updated_at DESC, created_at DESC").
		Find(&shipRows).Error
	for _, sh := range shipRows {
		if _, ok := latest[sh.OrderID]; ok {
			continue
		}
		latest[sh.OrderID] = sh.Status
	}
	return latest
}

func orderRowDTO(o *Order) OrderRow {
	if o == nil {
		return OrderRow{}
	}
	return OrderRow{
		ID:                o.ID,
		TenantID:          o.TenantID,
		Platform:          o.Platform,
		ShopID:            o.ShopID,
		ExternalOrderID:   o.ExternalOrderID,
		OrderNo:           o.OrderNo,
		CustomerName:      o.CustomerName,
		CustomerEmail:     o.CustomerEmail,
		CustomerPhone:     o.CustomerPhone,
		Status:            o.Status,
		PaymentStatus:     o.PaymentStatus,
		FulfillmentStatus: o.FulfillmentStatus,
		Currency:          o.Currency,
		TotalAmount:       o.TotalAmount,
		PaidAt:            o.PaidAt,
		OrderedAt:         o.OrderedAt,
		Remark:            o.Remark,
		CreatedBy:         o.CreatedBy,
		CreatedAt:         o.CreatedAt,
		UpdatedAt:         o.UpdatedAt,
	}
}

func (s *Service) shipmentCards(ctx *gin.Context, orderID uuid.UUID) ([]ShipmentCard, string) {
	var rows []OrderShipment
	_ = s.DB.WithContext(ctx.Request.Context()).Model(&OrderShipment{}).
		Where("order_id = ?", orderID).
		Order("created_at ASC, id ASC").
		Find(&rows).Error
	if len(rows) == 0 {
		return nil, ""
	}
	list := make([]ShipmentCard, 0, len(rows))
	for _, sh := range rows {
		list = append(list, ShipmentCard{
			Carrier:     sh.Carrier,
			TrackingNo:  sh.TrackingNo,
			TrackingURL: sh.TrackingURL,
			Status:      sh.Status,
			ShippedAt:   sh.ShippedAt,
			DeliveredAt: sh.DeliveredAt,
		})
	}
	latest := rows[len(rows)-1].Status
	return list, latest
}

// ConversationSummary returns nullable summary when order linked and not deleted.
func (s *Service) ConversationSummary(c *gin.Context, orderID uuid.UUID) (*ConversationOrderSummary, error) {
	if s == nil || s.DB == nil || orderID == uuid.Nil {
		return nil, nil
	}
	var o Order
	if err := s.DB.WithContext(c.Request.Context()).First(&o, "id = ?", orderID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	shList, lat := s.shipmentCards(c, o.ID)
	return &ConversationOrderSummary{
		ID:                   o.ID,
		OrderNo:              o.OrderNo,
		Platform:             o.Platform,
		Status:               o.Status,
		PaymentStatus:        o.PaymentStatus,
		FulfillmentStatus:    o.FulfillmentStatus,
		Currency:             o.Currency,
		TotalAmount:          o.TotalAmount,
		OrderedAt:            o.OrderedAt,
		LatestShipmentStatus: lat,
		Shipments:            shList,
	}, nil
}

// BuildAIContext shapes JSON-able maps for prompting (no emails/phones in fields).
func (s *Service) BuildAIContext(c *gin.Context, orderID uuid.UUID) (*AIContext, error) {
	if s == nil || s.DB == nil || orderID == uuid.Nil {
		return &AIContext{}, nil
	}
	var o Order
	if err := s.DB.WithContext(c.Request.Context()).First(&o, "id = ?", orderID).Error; err != nil {
		return nil, err
	}

	var items []OrderItem
	_ = s.DB.WithContext(c.Request.Context()).Model(&OrderItem{}).Where("order_id = ?", orderID).Find(&items).Error
	itemLines := make([]map[string]any, 0, len(items))
	for _, it := range items {
		m := map[string]any{
			"productTitle": it.ProductTitle,
			"skuName":      it.SKUName,
			"skuCode":      it.SKUCode,
			"quantity":     it.Quantity,
		}
		if len(it.Attrs) > 0 {
			var attrs map[string]any
			_ = json.Unmarshal(it.Attrs, &attrs)
			if attrs != nil {
				m["attrs"] = attrs
			}
		}
		itemLines = append(itemLines, m)
	}

	var ships []OrderShipment
	_ = s.DB.WithContext(c.Request.Context()).Model(&OrderShipment{}).Where("order_id = ?", orderID).
		Order("created_at ASC").Find(&ships).Error
	shipLines := make([]map[string]any, 0, len(ships))
	for _, sh := range ships {
		m := map[string]any{
			"carrier":     sh.Carrier,
			"trackingNo":  sh.TrackingNo,
			"trackingUrl": sh.TrackingURL,
			"status":      sh.Status,
			"shippedAt":   formatTimeRFC(sh.ShippedAt),
			"deliveredAt": formatTimeRFC(sh.DeliveredAt),
		}
		shipLines = append(shipLines, m)
	}

	orderInfo := map[string]any{
		"orderNo":           o.OrderNo,
		"status":            o.Status,
		"paymentStatus":     o.PaymentStatus,
		"fulfillmentStatus": o.FulfillmentStatus,
		"currency":          o.Currency,
		"orderedAt":         formatTimeRFC(o.OrderedAt),
		"totalAmount":       o.TotalAmount,
		"platform":          o.Platform,
	}

	return &AIContext{
		OrderInfo:    orderInfo,
		ShipmentInfo: shipLines,
		OrderItems:   itemLines,
	}, nil
}

func formatTimeRFC(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// Create order with optional nested rows.
func (s *Service) Create(c *gin.Context, body CreateBody, adminID *uuid.UUID) (*DetailDTO, error) {
	o, items, shipments, err := s.normalizedCreate(body)
	if err != nil {
		return nil, err
	}
	if err := s.validateShopRef(c, o.ShopID); err != nil {
		return nil, err
	}
	o.CreatedBy = adminID
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(o).Error; err != nil {
			return err
		}
		for i := range items {
			items[i].OrderID = o.ID
			if err := tx.Create(&items[i]).Error; err != nil {
				return err
			}
		}
		for i := range shipments {
			shipments[i].OrderID = o.ID
			if err := tx.Create(&shipments[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "order.create",
			Resource:    "order",
			ResourceID:  o.ID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("orderId=%s orderNo=%s platform=%s", o.ID.String(), o.OrderNo, o.Platform),
		})
	}
	return s.loadDetailDTO(c, o.ID)
}

func (s *Service) loadDetailDTO(c *gin.Context, orderID uuid.UUID) (*DetailDTO, error) {
	var o Order
	if err := s.DB.WithContext(c.Request.Context()).First(&o, "id = ?", orderID).Error; err != nil {
		return nil, err
	}
	var items []OrderItem
	_ = s.DB.WithContext(c.Request.Context()).Model(&OrderItem{}).Where("order_id = ?", orderID).Find(&items).Error
	var ships []OrderShipment
	_ = s.DB.WithContext(c.Request.Context()).Model(&OrderShipment{}).Where("order_id = ?", orderID).Find(&ships).Error
	out := DetailDTO{
		OrderRow:    orderRowDTO(&o),
		ShippedAt:   o.ShippedAt,
		DeliveredAt: o.DeliveredAt,
		Items:       items,
		Shipments:   ships,
	}
	if o.ShopID != nil && s.Shops != nil {
		sum, _ := s.Shops.GetSummary(c, *o.ShopID)
		if sum != nil {
			out.ShopSummary = sum
		}
	}
	return &out, nil
}

// Get returns order detail with nested rows.
func (s *Service) Get(c *gin.Context, orderID uuid.UUID) (*DetailDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	return s.loadDetailDTO(c, orderID)
}

// PeekOrderBeforeUpdate loads current row for callers that need transitions (e.g. inventory restore gates).
func (s *Service) PeekOrderBeforeUpdate(c *gin.Context, orderID uuid.UUID) (*Order, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	var o Order
	if err := s.DB.WithContext(c.Request.Context()).First(&o, "id = ? AND deleted_at IS NULL", orderID).Error; err != nil {
		return nil, err
	}
	return &o, nil
}

// ShouldAutoRestoreStock returns true when the order moved into a terminal cancel/refund-ish state vs before.
func ShouldAutoRestoreStock(before, after *Order) bool {
	if after == nil {
		return false
	}
	restoreTerminal := orderTerminalRestoreState(after)
	if !restoreTerminal {
		return false
	}
	if before == nil {
		return restoreTerminal
	}
	return !orderTerminalRestoreState(before)
}

func orderTerminalRestoreState(o *Order) bool {
	if o == nil {
		return false
	}
	st := strings.TrimSpace(o.Status)
	if st == StatusCancelled || st == StatusClosed || st == StatusRefunded {
		return true
	}
	ps := strings.TrimSpace(o.PaymentStatus)
	return ps == PaymentRefunded
}

// Update base order row and optionally replace nested collections.
func (s *Service) Update(c *gin.Context, orderID uuid.UUID, body UpdateBody, adminID *uuid.UUID) (*DetailDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	var o Order
	if err := s.DB.WithContext(c.Request.Context()).First(&o, "id = ?", orderID).Error; err != nil {
		return nil, err
	}

	if strings.TrimSpace(body.CustomerName) != "" {
		o.CustomerName = strings.TrimSpace(body.CustomerName)
	}
	if body.CustomerEmail != nil {
		o.CustomerEmail = strings.TrimSpace(*body.CustomerEmail)
	}
	if body.CustomerPhone != nil {
		o.CustomerPhone = strings.TrimSpace(*body.CustomerPhone)
	}

	if body.SetShopIDNil {
		o.ShopID = nil
	} else if body.ShopID != nil {
		if *body.ShopID == uuid.Nil {
			o.ShopID = nil
		} else {
			if err := s.validateShopRef(c, body.ShopID); err != nil {
				return nil, err
			}
			o.ShopID = body.ShopID
		}
	}
	if body.ExternalOrderID != nil {
		o.ExternalOrderID = body.ExternalOrderID
	}
	if st := strings.TrimSpace(body.Status); st != "" {
		if !validOrderStatus(st) {
			return nil, fmt.Errorf("invalid status")
		}
		o.Status = st
	}
	if ps := strings.TrimSpace(body.PaymentStatus); ps != "" {
		if !validPaymentStatus(ps) {
			return nil, fmt.Errorf("invalid paymentStatus")
		}
		o.PaymentStatus = ps
	}
	if fs := strings.TrimSpace(body.FulfillmentStatus); fs != "" {
		if !validFulfillmentStatus(fs) {
			return nil, fmt.Errorf("invalid fulfillmentStatus")
		}
		o.FulfillmentStatus = fs
	}
	if cur := strings.TrimSpace(body.Currency); cur != "" {
		o.Currency = strings.ToUpper(cur)
	}
	if body.TotalAmount != nil {
		o.TotalAmount = *body.TotalAmount
	}

	if body.PaidAt != nil {
		o.PaidAt = body.PaidAt
	}
	if body.SetPaidAtNil {
		o.PaidAt = nil
	}
	if body.OrderedAt != nil {
		o.OrderedAt = body.OrderedAt
	}
	if body.SetOrderedAtNil {
		o.OrderedAt = nil
	}
	if body.ShippedAt != nil {
		o.ShippedAt = body.ShippedAt
	}
	if body.SetShippedAtNil {
		o.ShippedAt = nil
	}
	if body.DeliveredAt != nil {
		o.DeliveredAt = body.DeliveredAt
	}
	if body.SetDeliveredAtNil {
		o.DeliveredAt = nil
	}

	err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&o).Error; err != nil {
			return err
		}
		if body.ReplaceItems {
			if _, err := normalizeAndReplaceItemsTx(tx, orderID, body.Items); err != nil {
				return err
			}
		}
		if body.ReplaceShipments {
			if _, err := normalizeAndReplaceShipmentsTx(tx, orderID, body.Shipments); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "order.update",
			Resource:    "order",
			ResourceID:  orderID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("orderId=%s orderNo=%s", orderID.String(), o.OrderNo),
		})
	}
	return s.loadDetailDTO(c, orderID)
}

func normalizeAndReplaceItemsTx(tx *gorm.DB, orderID uuid.UUID, in []OrderItemInput) ([]OrderItem, error) {
	if err := tx.Where("order_id = ?", orderID).Delete(&OrderItem{}).Error; err != nil {
		return nil, err
	}
	out := make([]OrderItem, 0, len(in))
	for _, it := range in {
		title := strings.TrimSpace(it.ProductTitle)
		if title == "" {
			title = strings.TrimSpace(it.SKUCode)
		}
		if title == "" {
			title = "(item)"
		}
		qty := it.Quantity
		if qty < 1 {
			qty = 1
		}
		row := OrderItem{
			OrderID:        orderID,
			ProductID:      it.ProductID,
			ProductSKUID:   it.ProductSKUID,
			ExternalItemID: it.ExternalItemID,
			ProductTitle:   title,
			SKUName:        strings.TrimSpace(it.SKUName),
			SKUCode:        strings.TrimSpace(it.SKUCode),
			Quantity:       qty,
			UnitPrice:      it.UnitPrice,
			TotalPrice:     it.TotalPrice,
			ImageURL:       strings.TrimSpace(it.ImageURL),
		}
		if len(it.Attrs) > 0 {
			row.Attrs = mapAttrs(it.Attrs)
		}
		if err := tx.Create(&row).Error; err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

func normalizeAndReplaceShipmentsTx(tx *gorm.DB, orderID uuid.UUID, in []OrderShipmentInput) ([]OrderShipment, error) {
	if err := tx.Where("order_id = ?", orderID).Delete(&OrderShipment{}).Error; err != nil {
		return nil, err
	}
	out := make([]OrderShipment, 0, len(in))
	for _, sh := range in {
		cr := strings.TrimSpace(sh.Carrier)
		if cr == "" {
			cr = "unknown"
		}
		st := strings.TrimSpace(sh.Status)
		if st == "" {
			st = ShipmentPending
		}
		if !validShipmentStatus(st) {
			return nil, fmt.Errorf("invalid shipment status")
		}
		row := OrderShipment{
			OrderID:     orderID,
			Carrier:     cr,
			TrackingNo:  strings.TrimSpace(sh.TrackingNo),
			TrackingURL: strings.TrimSpace(sh.TrackingURL),
			Status:      st,
			ShippedAt:   sh.ShippedAt,
			DeliveredAt: sh.DeliveredAt,
		}
		if err := tx.Create(&row).Error; err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

// Soft delete order row.
func (s *Service) Delete(c *gin.Context, orderID uuid.UUID, adminID *uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("order: no db")
	}
	var o Order
	if err := s.DB.WithContext(c.Request.Context()).First(&o, "id = ?", orderID).Error; err != nil {
		return err
	}
	if err := s.DB.WithContext(c.Request.Context()).Delete(&Order{}, "id = ?", orderID).Error; err != nil {
		return err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "order.delete",
			Resource:    "order",
			ResourceID:  orderID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("orderId=%s orderNo=%s", orderID.String(), o.OrderNo),
		})
	}
	return nil
}

func (s *Service) AppendItem(c *gin.Context, orderID uuid.UUID, body OrderItemInput, adminID *uuid.UUID) (*OrderItem, error) {
	o, err := s.findOrderBare(c, orderID)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(body.ProductTitle)
	if title == "" {
		title = strings.TrimSpace(body.SKUCode)
	}
	if title == "" {
		return nil, fmt.Errorf("productTitle or skuCode is required")
	}
	qty := body.Quantity
	if qty < 1 {
		qty = 1
	}
	row := OrderItem{
		OrderID:        orderID,
		ProductID:      body.ProductID,
		ProductSKUID:   body.ProductSKUID,
		ExternalItemID: body.ExternalItemID,
		ProductTitle:   title,
		SKUName:        strings.TrimSpace(body.SKUName),
		SKUCode:        strings.TrimSpace(body.SKUCode),
		Quantity:       qty,
		UnitPrice:      body.UnitPrice,
		TotalPrice:     body.TotalPrice,
		ImageURL:       strings.TrimSpace(body.ImageURL),
	}
	if len(body.Attrs) > 0 {
		row.Attrs = mapAttrs(body.Attrs)
	}
	if err := s.DB.WithContext(c.Request.Context()).Create(&row).Error; err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "order.item.create",
			Resource:    "order_item",
			ResourceID:  row.ID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("orderId=%s itemId=%s orderNo=%s", orderID.String(), row.ID.String(), o.OrderNo),
		})
	}
	return &row, nil
}

func (s *Service) findOrderBare(c *gin.Context, orderID uuid.UUID) (*Order, error) {
	var o Order
	if err := s.DB.WithContext(c.Request.Context()).First(&o, "id = ?", orderID).Error; err != nil {
		return nil, err
	}
	return &o, nil
}

// Update single item belonging to order.
func (s *Service) PatchItem(c *gin.Context, orderID, itemID uuid.UUID, body OrderItemInput, adminID *uuid.UUID) (*OrderItem, error) {
	o, err := s.findOrderBare(c, orderID)
	if err != nil {
		return nil, err
	}
	var row OrderItem
	if err := s.DB.WithContext(c.Request.Context()).First(&row, "id = ? AND order_id = ?", itemID, orderID).Error; err != nil {
		return nil, err
	}
	if strings.TrimSpace(body.ProductTitle) != "" {
		row.ProductTitle = strings.TrimSpace(body.ProductTitle)
	}
	row.SKUName = strings.TrimSpace(body.SKUName)
	row.SKUCode = strings.TrimSpace(body.SKUCode)
	if body.ProductID != nil {
		row.ProductID = body.ProductID
	}
	if body.ProductSKUID != nil {
		row.ProductSKUID = body.ProductSKUID
	}
	if body.ExternalItemID != nil {
		row.ExternalItemID = body.ExternalItemID
	}
	if body.Quantity >= 1 {
		row.Quantity = body.Quantity
	}
	row.UnitPrice = body.UnitPrice
	row.TotalPrice = body.TotalPrice
	row.ImageURL = strings.TrimSpace(body.ImageURL)
	if body.Attrs != nil {
		if len(body.Attrs) == 0 {
			row.Attrs = nil
		} else {
			row.Attrs = mapAttrs(body.Attrs)
		}
	}
	if err := s.DB.WithContext(c.Request.Context()).Save(&row).Error; err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "order.item.update",
			Resource:    "order_item",
			ResourceID:  itemID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("orderId=%s itemId=%s orderNo=%s", orderID.String(), itemID.String(), o.OrderNo),
		})
	}
	return &row, nil
}

// DeleteItem removes one line permanently.
func (s *Service) DeleteItem(c *gin.Context, orderID, itemID uuid.UUID, adminID *uuid.UUID) error {
	o, err := s.findOrderBare(c, orderID)
	if err != nil {
		return err
	}
	res := s.DB.WithContext(c.Request.Context()).Delete(&OrderItem{}, "id = ? AND order_id = ?", itemID, orderID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "order.item.delete",
			Resource:    "order_item",
			ResourceID:  itemID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("orderId=%s itemId=%s orderNo=%s", orderID.String(), itemID.String(), o.OrderNo),
		})
	}
	return nil
}

func (s *Service) AppendShipment(c *gin.Context, orderID uuid.UUID, body OrderShipmentInput, adminID *uuid.UUID) (*OrderShipment, error) {
	o, err := s.findOrderBare(c, orderID)
	if err != nil {
		return nil, err
	}
	cr := strings.TrimSpace(body.Carrier)
	if cr == "" {
		return nil, fmt.Errorf("carrier is required")
	}
	st := strings.TrimSpace(body.Status)
	if st == "" {
		st = ShipmentPending
	}
	if !validShipmentStatus(st) {
		return nil, fmt.Errorf("invalid shipment status")
	}
	row := OrderShipment{
		OrderID:     orderID,
		Carrier:     cr,
		TrackingNo:  strings.TrimSpace(body.TrackingNo),
		TrackingURL: strings.TrimSpace(body.TrackingURL),
		Status:      st,
		ShippedAt:   body.ShippedAt,
		DeliveredAt: body.DeliveredAt,
	}
	if err := s.DB.WithContext(c.Request.Context()).Create(&row).Error; err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "order.shipment.create",
			Resource:    "order_shipment",
			ResourceID:  row.ID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("orderId=%s shipmentId=%s orderNo=%s", orderID.String(), row.ID.String(), o.OrderNo),
		})
	}
	return &row, nil
}

func (s *Service) PatchShipment(c *gin.Context, orderID, shipmentID uuid.UUID, body OrderShipmentInput, adminID *uuid.UUID) (*OrderShipment, error) {
	o, err := s.findOrderBare(c, orderID)
	if err != nil {
		return nil, err
	}
	var row OrderShipment
	if err := s.DB.WithContext(c.Request.Context()).First(&row, "id = ? AND order_id = ?", shipmentID, orderID).Error; err != nil {
		return nil, err
	}
	if strings.TrimSpace(body.Carrier) != "" {
		row.Carrier = strings.TrimSpace(body.Carrier)
	}
	row.TrackingNo = strings.TrimSpace(body.TrackingNo)
	row.TrackingURL = strings.TrimSpace(body.TrackingURL)
	if st := strings.TrimSpace(body.Status); st != "" {
		if !validShipmentStatus(st) {
			return nil, fmt.Errorf("invalid shipment status")
		}
		row.Status = st
	}
	if body.ShippedAt != nil {
		row.ShippedAt = body.ShippedAt
	}
	if body.DeliveredAt != nil {
		row.DeliveredAt = body.DeliveredAt
	}
	if err := s.DB.WithContext(c.Request.Context()).Save(&row).Error; err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "order.shipment.update",
			Resource:    "order_shipment",
			ResourceID:  shipmentID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("orderId=%s shipmentId=%s orderNo=%s", orderID.String(), shipmentID.String(), o.OrderNo),
		})
	}
	return &row, nil
}

func (s *Service) DeleteShipment(c *gin.Context, orderID, shipmentID uuid.UUID, adminID *uuid.UUID) error {
	o, err := s.findOrderBare(c, orderID)
	if err != nil {
		return err
	}
	res := s.DB.WithContext(c.Request.Context()).Delete(&OrderShipment{}, "id = ? AND order_id = ?", shipmentID, orderID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "order.shipment.delete",
			Resource:    "order_shipment",
			ResourceID:  shipmentID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("orderId=%s shipmentId=%s orderNo=%s", orderID.String(), shipmentID.String(), o.OrderNo),
		})
	}
	return nil
}
