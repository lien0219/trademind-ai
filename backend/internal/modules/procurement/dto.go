package procurement

import "github.com/google/uuid"

type CreatePurchaseOrderInput struct {
	IdempotencyKey string                         `json:"idempotencyKey"`
	SupplierID     uuid.UUID                      `json:"supplierId"`
	WarehouseID    uuid.UUID                      `json:"warehouseId"`
	Currency       string                         `json:"currency"`
	Remark         string                         `json:"remark"`
	Items          []CreatePurchaseOrderItemInput `json:"items"`
}

type CreatePurchaseOrderItemInput struct {
	ProductSKUID  uuid.UUID  `json:"productSkuId"`
	SupplierSKUID *uuid.UUID `json:"supplierSkuId"`
	Quantity      int        `json:"quantity"`
	UnitCostMinor int64      `json:"unitCostMinor"`
}

type TransitionInput struct {
	ExpectedRevision int    `json:"expectedRevision"`
	Reason           string `json:"reason"`
}

type ReceivePurchaseOrderInput struct {
	ExpectedRevision int                             `json:"expectedRevision"`
	IdempotencyKey   string                          `json:"idempotencyKey"`
	Items            []ReceivePurchaseOrderItemInput `json:"items"`
}

type ReceivePurchaseOrderItemInput struct {
	PurchaseOrderItemID uuid.UUID `json:"purchaseOrderItemId"`
	Quantity            int       `json:"quantity"`
}

type ReceiptResult struct {
	PurchaseOrder *PurchaseOrder `json:"purchaseOrder"`
	Receipt       *GoodsReceipt  `json:"receipt"`
}

type ListResult struct {
	List       []PurchaseOrder `json:"list"`
	Page       int             `json:"page"`
	PageSize   int             `json:"pageSize"`
	Total      int64           `json:"total"`
	TotalPages int             `json:"totalPages"`
}

type CreatePurchaseReturnInput struct {
	IdempotencyKey  string                          `json:"idempotencyKey"`
	PurchaseOrderID uuid.UUID                       `json:"purchaseOrderId"`
	Reason          string                          `json:"reason"`
	Remark          string                          `json:"remark"`
	Items           []CreatePurchaseReturnItemInput `json:"items"`
}

type CreatePurchaseReturnItemInput struct {
	GoodsReceiptItemID uuid.UUID `json:"goodsReceiptItemId"`
	Quantity           int       `json:"quantity"`
}

type PurchaseReturnActionInput struct {
	ExpectedRevision int    `json:"expectedRevision"`
	IdempotencyKey   string `json:"idempotencyKey"`
	Reason           string `json:"reason"`
}

type PurchaseReturnListRow struct {
	PurchaseReturn
	PurchaseOrderNo string `json:"purchaseOrderNo"`
	SupplierName    string `json:"supplierName"`
	WarehouseName   string `json:"warehouseName"`
	ItemCount       int    `json:"itemCount"`
}

type PurchaseReturnListResult struct {
	List       []PurchaseReturnListRow `json:"list"`
	Page       int                     `json:"page"`
	PageSize   int                     `json:"pageSize"`
	Total      int64                   `json:"total"`
	TotalPages int                     `json:"totalPages"`
}

type ReturnableReceiptItem struct {
	GoodsReceiptItemID      uuid.UUID `json:"goodsReceiptItemId"`
	GoodsReceiptID          uuid.UUID `json:"goodsReceiptId"`
	ReceiptNo               string    `json:"receiptNo"`
	PurchaseOrderItemID     uuid.UUID `json:"purchaseOrderItemId"`
	ProductSKUID            uuid.UUID `json:"productSkuId"`
	ProductTitle            string    `json:"productTitle"`
	SKUCode                 string    `json:"skuCode"`
	SKUName                 string    `json:"skuName"`
	ReceivedQuantity        int       `json:"receivedQuantity"`
	AllocatedReturnQuantity int       `json:"allocatedReturnQuantity"`
	RemainingQuantity       int       `json:"remainingQuantity"`
}

type ReturnableReceiptItemResult struct {
	List []ReturnableReceiptItem `json:"list"`
}
