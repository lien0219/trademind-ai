package tenantconfig

import (
	"context"
	"errors"
	"fmt"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// ShopInventoryOverride stores a store-scoped SKU warning default. It does
// not change existing SKU rows or platform inventory safety settings.
type ShopInventoryOverride struct {
	TenantID     int64     `gorm:"primaryKey" json:"-"`
	ShopID       uuid.UUID `gorm:"primaryKey;type:char(36)" json:"shopId"`
	WarningStock int       `gorm:"not null" json:"warningStock"`
	Version      int64     `gorm:"not null" json:"version"`
	UpdatedBy    uuid.UUID `gorm:"type:char(36)" json:"updatedBy"`
}

func (ShopInventoryOverride) TableName() string { return "tenant_shop_inventory_overrides" }

// EffectiveInventory returns the store-specific value or the tenant default;
// the parent snapshot is always read from the caller's verified tenant scope.
func EffectiveInventory(ctx context.Context, db *gorm.DB, tenantID int64, shopID uuid.UUID) (int, string, int64, error) {
	tenant, _, err := ReadInventory(ctx, db, tenantID)
	if err != nil {
		return 0, "", 0, err
	}
	if shopID == uuid.Nil {
		return tenant.LowStockThreshold, "enterprise", 0, nil
	}
	var row ShopInventoryOverride
	err = db.WithContext(ctx).Where("tenant_id=? AND shop_id=?", tenantID, shopID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tenant.LowStockThreshold, "enterprise", 0, nil
	}
	if err != nil {
		return 0, "", 0, err
	}
	return row.WarningStock, "shop", row.Version, nil
}

func (h *Handler) ShopInventory(c *gin.Context) {
	tid, _, ok := h.identity(c, false)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("shopId"))
	if err != nil || id == uuid.Nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid shop")
		return
	}
	if !adminperm.RequireStoreView(c, h.DB, id) {
		return
	}
	threshold, origin, version, err := EffectiveInventory(c.Request.Context(), h.DB, tid, id)
	if err != nil {
		response.Fail(c, 503, response.CodeInternalError, "shop inventory settings unavailable")
		return
	}
	response.OK(c, gin.H{"shopId": id, "warningStock": threshold, "origin": origin, "version": version})
}

type shopOverrideInput struct {
	WarningStock    *int   `json:"warningStock" binding:"required"`
	ExpectedVersion *int64 `json:"expectedVersion" binding:"required"`
}

func (h *Handler) PutShopInventory(c *gin.Context) {
	tid, actor, ok := h.identity(c, false)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("shopId"))
	if err != nil || id == uuid.Nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid shop")
		return
	}
	if !adminperm.RequireStoreOperate(c, h.DB, id) {
		return
	}
	if h.Audit == nil {
		response.Fail(c, 503, response.CodeInternalError, "shop settings audit unavailable")
		return
	}
	var body shopOverrideInput
	if err := c.ShouldBindJSON(&body); err != nil || body.WarningStock == nil || body.ExpectedVersion == nil ||
		*body.WarningStock < 0 || *body.WarningStock > 1000000 || *body.ExpectedVersion < 0 {
		response.Fail(c, 400, response.CodeBadRequest, "invalid inventory threshold")
		return
	}
	next := *body.ExpectedVersion + 1
	err = h.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var row ShopInventoryOverride
		e := tx.Where("tenant_id=? AND shop_id=?", tid, id).Take(&row).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			if *body.ExpectedVersion != 0 {
				return ErrVersionConflict
			}
			if err := tx.Create(&ShopInventoryOverride{TenantID: tid, ShopID: id, WarningStock: *body.WarningStock, Version: 1, UpdatedBy: actor}).Error; err != nil {
				return err
			}
			return h.Audit.WriteTxContext(c.Request.Context(), tx, operationlog.WriteOpts{
				TenantID: tid, AdminUserID: &actor, Action: "tenant.shop.inventory.create",
				Resource: "tenant_shop_inventory_override", ResourceID: id.String(), Status: "success",
				Message: "version=1",
			})
		}
		if e != nil {
			return e
		}
		if row.Version != *body.ExpectedVersion {
			return ErrVersionConflict
		}
		result := tx.Model(&ShopInventoryOverride{}).Where("tenant_id=? AND shop_id=? AND version=?", tid, id, *body.ExpectedVersion).
			Updates(map[string]any{"warning_stock": *body.WarningStock, "version": next, "updated_by": actor})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrVersionConflict
		}
		return h.Audit.WriteTxContext(c.Request.Context(), tx, operationlog.WriteOpts{
			TenantID: tid, AdminUserID: &actor, Action: "tenant.shop.inventory.update",
			Resource: "tenant_shop_inventory_override", ResourceID: id.String(), Status: "success",
			Message: fmt.Sprintf("version=%d", next),
		})
	})
	if errors.Is(err, ErrVersionConflict) {
		response.Fail(c, 409, response.CodeConflict, "shop setting changed; reload")
		return
	}
	if err != nil {
		response.Fail(c, 503, response.CodeInternalError, "shop inventory configuration failed")
		return
	}
	response.OK(c, gin.H{"shopId": id, "warningStock": *body.WarningStock, "origin": "shop", "version": next})
}
