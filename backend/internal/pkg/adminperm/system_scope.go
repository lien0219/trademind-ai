package adminperm

import (
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
)

// IsSystemAdministrator checks the persisted identity, independently of tenant
// membership roles and platform-operator grants.
func IsSystemAdministrator(user *admin.AdminUser) bool {
	return user != nil && user.ID != uuid.Nil && user.TenantID == 0 &&
		strings.EqualFold(strings.TrimSpace(user.Role), RoleAdmin) &&
		strings.EqualFold(strings.TrimSpace(user.Status), admin.StatusActive)
}

func systemPermissions() []string {
	return []string{PermSettingsManage, PermSecurityKeyRotate, PermSecuritySessionManage, PermConfigRead, PermAuditRead, PermStoreView}
}

// IsSystemControlPlaneRoute accepts registered Gin route patterns, never URL
// prefixes. Tenant-zero identities cannot use this exception for business APIs.
func IsSystemControlPlaneRoute(method, route string) bool {
	switch method + " " + route {
	case "GET /api/v1/settings", "PUT /api/v1/settings",
		"GET /api/v1/settings/config-status",
		"POST /api/v1/storage/test-public-access", "POST /api/v1/settings/storage/public-check",
		"GET /api/v1/settings/storage/public-check/latest",
		"GET /api/v1/settings/integration-schemas", "GET /api/v1/settings/integrations/overview",
		"POST /api/v1/settings/test-ai", "POST /api/v1/settings/test-image", "POST /api/v1/settings/test-ocr",
		"POST /api/v1/settings/test-storage", "POST /api/v1/settings/test-platform-tiktok", "POST /api/v1/settings/test-email",
		"GET /api/v1/security/overview", "GET /api/v1/security/audit/integrity/status", "POST /api/v1/security/audit/integrity/verify",
		"POST /api/v1/security/keys/rotation/prepare", "GET /api/v1/security/keys/rotation/status", "POST /api/v1/security/keys/rotation/start",
		"GET /api/v1/security/keys/rotation/:id", "GET /api/v1/security/keys/rotation/:id/progress",
		"POST /api/v1/security/keys/rotation/:id/pause", "POST /api/v1/security/keys/rotation/:id/resume", "POST /api/v1/security/keys/rotation/:id/verify",
		"GET /api/v1/security/keys/references",
		"GET /api/v1/ai/prompts", "POST /api/v1/ai/prompts", "GET /api/v1/ai/prompts/:id", "PUT /api/v1/ai/prompts/:id", "DELETE /api/v1/ai/prompts/:id",
		"POST /api/v1/ai/prompts/:id/enable", "POST /api/v1/ai/prompts/:id/disable",
		"GET /api/v1/collect/rules", "POST /api/v1/collect/rules", "GET /api/v1/collect/rules/:id", "PUT /api/v1/collect/rules/:id", "DELETE /api/v1/collect/rules/:id",
		"POST /api/v1/collect/rules/:id/enable", "POST /api/v1/collect/rules/:id/disable", "POST /api/v1/collect/rules/:id/test",
		"POST /api/v1/collect/rules/ai-generate", "POST /api/v1/collect/rules/ai-generate-and-save",
		"GET /api/v1/platform/settings/:platform", "PUT /api/v1/platform/settings/:platform", "POST /api/v1/platform/settings/:platform/test-connection",
		"GET /api/v1/platform/publish-settings/:platform", "PUT /api/v1/platform/publish-settings/:platform",
		"GET /api/v1/platform/douyin/runtime-status", "GET /api/v1/platform/douyin/health", "GET /api/v1/platform/douyin/metrics-summary", "GET /api/v1/platform/douyin/release-gate",
		"POST /api/v1/platform/douyin/runtime-status/pause", "POST /api/v1/platform/douyin/runtime-status/resume", "POST /api/v1/platform/douyin/runtime-status/emergency-disable",
		"POST /api/v1/platform/douyin/run-health-check", "POST /api/v1/platform/douyin/production-preflight", "GET /api/v1/platform/douyin/production-preflight/latest":
		return true
	default:
		return false
	}
}
