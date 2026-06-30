package demoseed

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Handler serves dev/demo-only seed endpoints.
type Handler struct {
	Svc *Service
}

func adminUUID(c *gin.Context) *uuid.UUID {
	if v, ok := c.Get(ctxkey.AdminID); ok {
		if s, ok := v.(string); ok {
			if u, err := uuid.Parse(strings.TrimSpace(s)); err == nil {
				return &u
			}
		}
	}
	return nil
}

func (h *Handler) requireDevAdmin(c *gin.Context) bool {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, 500, response.CodeInternalError, "Demo 种子服务不可用")
		return false
	}
	if strings.EqualFold(strings.TrimSpace(h.Svc.AppEnv), "production") {
		response.Fail(c, 403, response.CodeForbidden, "生产环境禁止 Demo 种子接口")
		return false
	}
	p, err := adminperm.LoadPrincipal(c, h.Svc.DB)
	if err != nil {
		response.HandleError(c, err)
		return false
	}
	if !p.IsAdmin() {
		adminperm.DenyUserManage(c)
		return false
	}
	return true
}

// SeedFullProjectEdgeCases POST /api/v1/dev/demo-seed/full-project-edge-cases
func (h *Handler) SeedFullProjectEdgeCases(c *gin.Context) {
	if !h.requireDevAdmin(c) {
		return
	}
	out, err := h.Svc.SeedFullProjectEdgeCases(c.Request.Context(), adminUUID(c))
	if err != nil {
		if errors.Is(err, ErrProductionForbidden) {
			response.Fail(c, 403, response.CodeForbidden, "生产环境禁止 Demo 种子接口")
			return
		}
		response.HandleError(c, err)
		return
	}
	if h.Svc.OpLog != nil {
		_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminUUID(c),
			Action:      "dev.demo_seed.full_project_edge_cases",
			Resource:    "demo_seed",
			Status:      "success",
			Message:     "F8 dev-only edge-case demo samples seeded",
		})
	}
	response.OK(c, out)
}
