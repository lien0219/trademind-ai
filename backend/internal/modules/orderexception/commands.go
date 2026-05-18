package orderexception

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
)

// Commands bundles writes orchestrating existing order/inventory services.
type Commands struct {
	Svc    *Service
	Orders *order.Service
	Inv    *inventory.Service
}

// BindSKU binds a local SKU via order.Service then optionally deducts / syncs like POST /order-items/:id/bind-sku.
func (c *Commands) BindSKU(ctx context.Context, sourceType, sourceID string, body BindSKURequest, admin *uuid.UUID) (map[string]any, error) {
	if c == nil || c.Svc == nil || c.Orders == nil || c.Inv == nil || c.Orders.DB == nil {
		return nil, fmt.Errorf("orderexception bind unavailable")
	}
	itemID, err := c.Svc.ResolveOrderItemForBind(ctx, sourceType, sourceID)
	if err != nil {
		return nil, err
	}
	skuID, err := uuid.Parse(strings.TrimSpace(body.ProductSKUID))
	if err != nil {
		return nil, fmt.Errorf("invalid productSkuId")
	}
	pol, err := c.Inv.InventoryPolicy(ctx)
	if err != nil {
		return nil, err
	}
	var line order.OrderItem
	if err := c.Orders.DB.WithContext(ctx).First(&line, "id = ?", itemID).Error; err != nil {
		return nil, fmt.Errorf("order item not found")
	}
	has, err := c.Inv.HasSuccessfulOrderDeduction(ctx, line.OrderID)
	if err != nil {
		return nil, err
	}
	if has && !pol.AllowManualSkuBindAfterDeduct {
		return nil, fmt.Errorf("manual sku bind after deduct is disabled in settings")
	}

	if _, err := c.Orders.BindOrderItemSKU(ctx, order.BindOrderItemSKUInput{
		OrderItemID:         itemID,
		ProductSKUID:        skuID,
		CandidateConfidence: body.CandidateConfidence,
		CandidateSource:     strings.TrimSpace(body.CandidateSource),
	}, admin); err != nil {
		return nil, err
	}

	out := map[string]any{"bind": "ok"}
	autoMark := body.AutoMarkHandled == nil || *body.AutoMarkHandled

	deduct := body.DeductInventory == nil || *body.DeductInventory
	syncPlat := body.SyncInventory != nil && *body.SyncInventory

	if deduct {
		sum, derr := c.Inv.DeductInventoryForOrder(ctx, line.OrderID, inventory.OrderInventoryOptions{
			Reason:        "exception_workbench_bind",
			SyncPlatforms: syncPlat,
			CreatedBy:     admin,
		})
		out["inventoryDeduction"] = sum
		if derr != nil {
			if errors.Is(derr, inventory.ErrInsufficientSKUStock) {
				out["inventoryDeductionError"] = derr.Error()
				return out, nil
			}
			return nil, derr
		}
		if autoMark && strings.TrimSpace(body.ExceptionType) != "" {
			_ = c.Svc.UpsertMark(ctx, strings.TrimSpace(body.ExceptionType), sourceType, sourceID, MarkHandled, "bind-sku + deduct ok", admin)
		}
		return out, nil
	}

	if syncPlat {
		var sku product.ProductSKU
		if err := c.Inv.DB.WithContext(ctx).First(&sku, "id = ? AND deleted_at IS NULL", skuID).Error; err != nil {
			return nil, err
		}
		st := 0
		if sku.Stock != nil {
			st = *sku.Stock
		}
		if _, err := c.Inv.CreateInventorySyncTasksForSKUStock(ctx, sku.ProductID, sku.ID, st, admin); err != nil {
			return nil, err
		}
		out["inventorySyncTasks"] = "enqueued"
	}

	if autoMark && strings.TrimSpace(body.ExceptionType) != "" {
		_ = c.Svc.UpsertMark(ctx, strings.TrimSpace(body.ExceptionType), sourceType, sourceID, MarkHandled, "bind-sku ok", admin)
	}
	return out, nil
}

// RetryDeduct calls DeductInventoryForOrder for an exception-bearing order line / effect source.
func (c *Commands) RetryDeduct(ctx context.Context, sourceType, sourceID string, syncPlatforms bool, admin *uuid.UUID) (*inventory.DeductionSummary, error) {
	if c == nil || c.Svc == nil || c.Inv == nil {
		return nil, fmt.Errorf("orderexception retry unavailable")
	}
	orderID, err := c.resolveOrderID(ctx, sourceType, sourceID)
	if err != nil {
		return nil, err
	}
	return c.Inv.DeductInventoryForOrder(ctx, orderID, inventory.OrderInventoryOptions{
		Reason:        "exception_workbench_retry",
		SyncPlatforms: syncPlatforms,
		CreatedBy:     admin,
	})
}

func (c *Commands) resolveOrderID(ctx context.Context, sourceType, sourceID string) (uuid.UUID, error) {
	st := strings.TrimSpace(sourceType)
	switch st {
	case SourceOrderItemSKUMatch, SourceOrderItem, SourceOrderInventoryEffect:
		oid, _, err := c.Svc.resolveOrderPointers(ctx, st, sourceID)
		if err != nil || oid == nil {
			return uuid.Nil, err
		}
		return *oid, nil
	case SourceInventorySyncTask:
		return uuid.Nil, fmt.Errorf("retry deduct does not apply to inventory_sync_task sources")
	default:
		return uuid.Nil, fmt.Errorf("unsupported sourceType")
	}
}

// RetryInventorySync enqueues retry for a failed inventory_sync_tasks row.
func (c *Commands) RetryInventorySync(ctx context.Context, taskID uuid.UUID, admin *uuid.UUID) (*inventory.TaskDTO, error) {
	if c == nil || c.Inv == nil {
		return nil, fmt.Errorf("inventory unavailable")
	}
	return c.Inv.RetryInventorySyncTask(ctx, taskID, admin)
}
