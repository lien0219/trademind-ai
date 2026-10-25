package security

import "errors"

// Tenant and system context error codes (API / logs).
const (
	ErrCodeTenantContextMissing   = "TENANT_CONTEXT_MISSING"
	ErrCodeTenantContextInvalid   = "TENANT_CONTEXT_INVALID"
	ErrCodeTenantAccessDenied     = "TENANT_ACCESS_DENIED"
	ErrCodeSystemContextRequired  = "SYSTEM_CONTEXT_REQUIRED"
	ErrCodeSystemContextForbidden = "SYSTEM_CONTEXT_FORBIDDEN"
	ErrCodeTaskTenantMissing      = "TASK_TENANT_MISSING"
	ErrCodeTaskTenantInactive     = "TASK_TENANT_INACTIVE"
	ErrCodeTaskTenantMismatch     = "TASK_TENANT_MISMATCH"
	ErrCodeTaskShopScopeMismatch  = "TASK_SHOP_SCOPE_MISMATCH"
	ErrCodeTaskResourceMismatch   = "TASK_RESOURCE_TENANT_MISMATCH"
	ErrCodeLegacyAuthForbidden    = "INSECURE_LEGACY_AUTH_MODE_FORBIDDEN"
	ErrCodeProdTenantFallback     = "PRODUCTION_TENANT_FALLBACK_FORBIDDEN"
)

var (
	ErrTenantContextMissing   = errors.New(ErrCodeTenantContextMissing)
	ErrTenantContextInvalid   = errors.New(ErrCodeTenantContextInvalid)
	ErrSystemContextRequired  = errors.New(ErrCodeSystemContextRequired)
	ErrSystemContextForbidden = errors.New(ErrCodeSystemContextForbidden)
	ErrTaskTenantMissing      = errors.New(ErrCodeTaskTenantMissing)
	ErrTaskTenantInactive     = errors.New(ErrCodeTaskTenantInactive)
	ErrTaskTenantMismatch     = errors.New(ErrCodeTaskTenantMismatch)
	ErrTaskShopScopeMismatch  = errors.New(ErrCodeTaskShopScopeMismatch)
	ErrTaskResourceMismatch   = errors.New(ErrCodeTaskResourceMismatch)
)
