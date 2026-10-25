package tasktenant

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
)

// ErrTenantNotActive is returned when a durable tenant is missing or cannot
// execute tenant business work in its current lifecycle state.
var ErrTenantNotActive = security.ErrTaskTenantInactive

// IsPermanentWorkerScopeError reports errors that should make a dequeued task
// visible as failed instead of being retried until tenant data changes.
func IsPermanentWorkerScopeError(err error) bool {
	return errors.Is(err, ErrTenantNotActive) ||
		errors.Is(err, security.ErrTaskTenantMissing) ||
		errors.Is(err, security.ErrTaskShopScopeMismatch) ||
		errors.Is(err, security.ErrTaskTenantMismatch) ||
		errors.Is(err, security.ErrTaskResourceMismatch)
}

// TaskScope carries tenant and optional shop for worker tasks.
type TaskScope struct {
	TenantID              int64
	ShopID                uuid.UUID
	ResourceType          string
	ResourceID            string
	AllowLegacyTenantZero bool
}

// RequireTaskTenant validates task has a positive tenant id.
func RequireTaskTenant(tenantID int64) error {
	if tenantID <= 0 {
		return security.ErrTaskTenantMissing
	}
	return nil
}

// RequireTaskTenantForMode permits tenant zero only for explicitly authorized
// legacy local-development workers. All other worker paths remain fail-closed.
func RequireTaskTenantForMode(tenantID int64, allowLegacyTenantZero bool) error {
	if tenantID < 0 || (tenantID == 0 && !allowLegacyTenantZero) {
		return security.ErrTaskTenantMissing
	}
	return nil
}

// BuildWorkerContext creates tenant context from task scope.
func BuildWorkerContext(scope TaskScope, actorID uuid.UUID, operation string, parents ...context.Context) context.Context {
	parent := context.Background()
	if len(parents) > 0 && parents[0] != nil {
		parent = parents[0]
	}
	return buildWorkerContext(parent, scope, actorID, operation)
}

// BuildWorkerContextFrom preserves cancellation, deadlines, and values from
// the caller while adding the trusted worker tenant and system identity.
func BuildWorkerContextFrom(parent context.Context, scope TaskScope, actorID uuid.UUID, operation string) context.Context {
	if parent == nil {
		parent = context.Background()
	}
	return buildWorkerContext(parent, scope, actorID, operation)
}

func buildWorkerContext(parent context.Context, scope TaskScope, actorID uuid.UUID, operation string) context.Context {
	ctx := security.WithTenantContext(parent, security.WorkerTenantContext(scope.TenantID, actorID))
	ctx = security.WithSystemContext(ctx, &security.SystemContext{Operation: operation, ActorID: actorID})
	tc := security.FromContext(ctx)
	if tc != nil {
		if scope.TenantID == 0 && scope.AllowLegacyTenantZero {
			tc.AuthSource = security.AuthSourceLegacyDevZero
		} else {
			tc.AuthSource = security.AuthSourceWorker
		}
		if scope.ShopID != uuid.Nil {
			tc.ShopScope = []uuid.UUID{scope.ShopID}
		}
		ctx = security.WithTenantContext(ctx, tc)
	}
	return ctx
}

// EnsureResourceTenantMatch rejects when resource tenant differs from task tenant.
func EnsureResourceTenantMatch(taskTenantID int64, resourceTenantID int64) error {
	if err := RequireTaskTenant(taskTenantID); err != nil {
		return err
	}
	if resourceTenantID <= 0 {
		return security.ErrTaskResourceMismatch
	}
	if taskTenantID != resourceTenantID {
		return security.ErrTaskTenantMismatch
	}
	return nil
}

// EnsureShopInScope validates shop is allowed for worker task.
func EnsureShopInScope(taskShopID, resourceShopID uuid.UUID) error {
	if taskShopID == uuid.Nil {
		return nil
	}
	if resourceShopID == uuid.Nil || taskShopID != resourceShopID {
		return security.ErrTaskShopScopeMismatch
	}
	return nil
}

// WrapError returns user-safe task tenant error message.
func WrapError(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case err == security.ErrTaskTenantMissing:
		return "任务缺少租户上下文，已停止处理"
	case errors.Is(err, security.ErrTaskTenantInactive):
		return "租户当前不可执行后台任务"
	case err == security.ErrTaskTenantMismatch, err == security.ErrTaskResourceMismatch:
		return "资源不属于当前租户"
	case err == security.ErrTaskShopScopeMismatch:
		return "当前账号无权访问此店铺"
	default:
		return fmt.Sprintf("%v", err)
	}
}
