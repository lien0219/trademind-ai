package profitability

import (
	"time"

	"github.com/google/uuid"
)

const (
	FormulaVersion = "order_profit_estimate_v1"

	StatusComplete = "complete"
	StatusPending  = "pending"
	StatusMismatch = "mismatch"
	StatusBlocked  = "blocked"

	ComponentAvailable = "available"
	ComponentMissing   = "missing"
	ComponentPending   = "pending"
	ComponentMismatch  = "mismatch"
	ComponentBlocked   = "blocked"

	maxPageSize   = 100
	maxExportRows = 5000
)

// Scope is the tenant-local store visibility applied to every order query.
// A nil AllowedShopIDs slice means an administrator can view all stores.
type Scope struct {
	RestrictStoreScope bool
	AllowedShopIDs     []uuid.UUID
}

type ListQuery struct {
	Page        int
	PageSize    int
	OrderNo     string
	Platform    string
	ShopID      *uuid.UUID
	WarehouseID *uuid.UUID
	Currency    string
	Status      string
	Start       *time.Time
	End         *time.Time
	Export      bool
}

type FormulaDescriptor struct {
	Version            string `json:"version"`
	RevenueSource      string `json:"revenueSource"`
	ProductCostSource  string `json:"productCostSource"`
	FreightSource      string `json:"freightSource"`
	RefundSource       string `json:"refundSource"`
	MissingFeeBehavior string `json:"missingFeeBehavior"`
}

type MoneyComponent struct {
	AmountMinor      *int64     `json:"amountMinor"`
	KnownAmountMinor int64      `json:"knownAmountMinor"`
	Currency         string     `json:"currency"`
	Status           string     `json:"status"`
	Source           string     `json:"source"`
	SourceAt         *time.Time `json:"sourceAt,omitempty"`
	ReasonCode       string     `json:"reasonCode,omitempty"`
	Reason           string     `json:"reason,omitempty"`
}

type ProfitComponents struct {
	Revenue      MoneyComponent `json:"revenue"`
	ProductCost  MoneyComponent `json:"productCost"`
	Freight      MoneyComponent `json:"freight"`
	Refund       MoneyComponent `json:"refund"`
	PlatformFee  MoneyComponent `json:"platformFee"`
	Advertising  MoneyComponent `json:"advertisingFee"`
	WarehouseFee MoneyComponent `json:"warehouseFee"`
}

type Issue struct {
	Code      string `json:"code"`
	Component string `json:"component"`
	Message   string `json:"message"`
}

type RelatedFacts struct {
	FulfillmentWaveID  *uuid.UUID  `json:"fulfillmentWaveId,omitempty"`
	SupplierIDs        []uuid.UUID `json:"supplierIds"`
	RefundExecutionIDs []uuid.UUID `json:"refundExecutionIds"`
}

type OrderProfit struct {
	OrderID                uuid.UUID        `json:"orderId"`
	OrderNo                string           `json:"orderNo"`
	Platform               string           `json:"platform"`
	ShopID                 *uuid.UUID       `json:"shopId,omitempty"`
	ShopName               string           `json:"shopName,omitempty"`
	WarehouseID            *uuid.UUID       `json:"warehouseId,omitempty"`
	WarehouseCode          string           `json:"warehouseCode,omitempty"`
	WarehouseName          string           `json:"warehouseName,omitempty"`
	Currency               string           `json:"currency"`
	Status                 string           `json:"status"`
	OrderStatus            string           `json:"orderStatus"`
	PaymentStatus          string           `json:"paymentStatus"`
	FulfillmentStatus      string           `json:"fulfillmentStatus"`
	KnownContributionMinor *int64           `json:"knownContributionMinor"`
	EstimatedProfitMinor   *int64           `json:"estimatedProfitMinor"`
	EstimatedMarginBps     *int64           `json:"estimatedMarginBps"`
	Components             ProfitComponents `json:"components"`
	Issues                 []Issue          `json:"issues"`
	Related                RelatedFacts     `json:"related"`
	OrderedAt              *time.Time       `json:"orderedAt,omitempty"`
	CalculatedAt           time.Time        `json:"calculatedAt"`
	FormulaVersion         string           `json:"formulaVersion"`
}

type ProductCostLine struct {
	OrderItemID   uuid.UUID  `json:"orderItemId"`
	ProductSKUID  *uuid.UUID `json:"productSkuId,omitempty"`
	ProductTitle  string     `json:"productTitle"`
	SKUCode       string     `json:"skuCode,omitempty"`
	Quantity      int        `json:"quantity"`
	UnitCostMinor *int64     `json:"unitCostMinor"`
	LineCostMinor *int64     `json:"lineCostMinor"`
	Currency      string     `json:"currency"`
	Status        string     `json:"status"`
	Source        string     `json:"source"`
	SourceAt      *time.Time `json:"sourceAt,omitempty"`
	SupplierID    *uuid.UUID `json:"supplierId,omitempty"`
	SupplierName  string     `json:"supplierName,omitempty"`
	ReasonCode    string     `json:"reasonCode,omitempty"`
	Reason        string     `json:"reason,omitempty"`
}

type Detail struct {
	OrderProfit
	ProductCostLines []ProductCostLine `json:"productCostLines"`
}

type ListResult struct {
	List         []OrderProfit     `json:"list"`
	Page         int               `json:"page"`
	PageSize     int               `json:"pageSize"`
	Total        int64             `json:"total"`
	TotalPages   int               `json:"totalPages"`
	CalculatedAt time.Time         `json:"calculatedAt"`
	Formula      FormulaDescriptor `json:"formula"`
}

func formulaDescriptor() FormulaDescriptor {
	return FormulaDescriptor{
		Version:            FormulaVersion,
		RevenueSource:      "orders.total_amount converted exactly to the currency minor unit",
		ProductCostSource:  "current active supplier catalog binding; estimate only, not historical COGS",
		FreightSource:      "latest confirmed local freight quote on one non-cancelled fulfillment wave",
		RefundSource:       "succeeded local refund execution facts",
		MissingFeeBehavior: "platform, advertising and warehouse fees remain missing and are never treated as zero",
	}
}
