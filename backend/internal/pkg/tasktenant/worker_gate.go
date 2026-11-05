package tasktenant

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

// BeginWorker builds a tenant-scoped worker context from explicit tenant and optional shop.
// When tenantID <= 0 and shopID is set, tenant is resolved from the shop row.
func BeginWorker(ctx context.Context, db *gorm.DB, tenantID int64, shopID uuid.UUID, operation string) (context.Context, TaskScope, error) {
	return beginWorker(ctx, db, tenantID, shopID, operation, false)
}

// BeginWorkerWithLegacyZero permits tenant zero only when the caller has
// already derived allowLegacyTenantZero from trusted environment config.
func BeginWorkerWithLegacyZero(ctx context.Context, db *gorm.DB, tenantID int64, shopID uuid.UUID, operation string, allowLegacyTenantZero bool) (context.Context, TaskScope, error) {
	return beginWorker(ctx, db, tenantID, shopID, operation, allowLegacyTenantZero)
}

func beginWorker(ctx context.Context, db *gorm.DB, tenantID int64, shopID uuid.UUID, operation string, allowLegacyTenantZero bool) (context.Context, TaskScope, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	tid := tenantID
	if tid <= 0 && shopID != uuid.Nil && db != nil {
		resolved, err := ResolveShopTenant(ctx, db, shopID)
		if err != nil {
			return ctx, TaskScope{}, err
		}
		tid = resolved
	}
	if err := RequireTaskTenantForMode(tid, allowLegacyTenantZero); err != nil {
		return ctx, TaskScope{}, err
	}
	if tid > 0 {
		if err := verifyTenantActive(ctx, db, tid); err != nil {
			return ctx, TaskScope{}, err
		}
		if shopID != uuid.Nil {
			if err := verifyShopTenant(ctx, db, tid, shopID); err != nil {
				return ctx, TaskScope{}, err
			}
		}
	}
	scope := TaskScope{TenantID: tid, ShopID: shopID, AllowLegacyTenantZero: allowLegacyTenantZero}
	wctx := BuildWorkerContextFrom(ctx, scope, uuid.Nil, operation)
	return wctx, scope, nil
}

// verifyTenantActive checks the durable lifecycle registry before a worker can
// run tenant business. A missing registry table is tolerated only for legacy
// SQLite unit tests; production PostgreSQL never falls through this path.
func verifyTenantActive(ctx context.Context, db *gorm.DB, tenantID int64) error {
	if tenantID <= 0 {
		return nil
	}
	if db == nil {
		return fmt.Errorf("tasktenant: tenant registry unavailable")
	}
	var row struct {
		Status string `gorm:"column:status"`
	}
	err := db.WithContext(ctx).Table("tenants").Select("status").Where("id = ?", tenantID).Take(&row).Error
	if err != nil {
		if isLegacySQLiteMissingTable(db, err) {
			return nil
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTenantNotActive
		}
		return fmt.Errorf("tasktenant: verify tenant lifecycle: %w", err)
	}
	if strings.ToLower(strings.TrimSpace(row.Status)) != "active" {
		return ErrTenantNotActive
	}
	return nil
}

func verifyShopTenant(ctx context.Context, db *gorm.DB, tenantID int64, shopID uuid.UUID) error {
	if tenantID <= 0 || shopID == uuid.Nil || db == nil {
		return security.ErrTaskShopScopeMismatch
	}
	resolved, err := ResolveShopTenant(ctx, db, shopID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, security.ErrTaskTenantMissing) {
			return security.ErrTaskShopScopeMismatch
		}
		return fmt.Errorf("tasktenant: verify shop tenant: %w", err)
	}
	if resolved != tenantID {
		return security.ErrTaskShopScopeMismatch
	}
	return nil
}

func isLegacySQLiteMissingTable(db *gorm.DB, err error) bool {
	if db == nil || err == nil || !strings.EqualFold(db.Dialector.Name(), "sqlite") {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such table") && strings.Contains(msg, "tenants")
}

// BeginWorkerFromShop resolves tenant from shop and returns worker context.
func BeginWorkerFromShop(ctx context.Context, db *gorm.DB, shopID uuid.UUID, operation string) (context.Context, TaskScope, error) {
	return BeginWorker(ctx, db, 0, shopID, operation)
}

// RejectUnstartedTask makes a dequeued, denied task visible to the existing
// failed-task retry flow. The caller supplies a model query scoped to the
// persisted task ID and tenant. Duplicate messages cannot overwrite running or
// completed work. Extra fields follow the caller's existing task state model.
func RejectUnstartedTask(ctx context.Context, scoped *gorm.DB, cause error, extra map[string]any) error {
	if scoped == nil || cause == nil {
		return fmt.Errorf("tasktenant: invalid task rejection")
	}
	updates := make(map[string]any, len(extra)+3)
	for key, value := range extra {
		updates[key] = value
	}
	updates["status"] = "failed"
	updates["error_message"] = WrapError(cause)
	updates["finished_at"] = time.Now().UTC()
	return scoped.WithContext(ctx).Where("status IN ?", []string{"pending", "retrying"}).Updates(updates).Error
}
