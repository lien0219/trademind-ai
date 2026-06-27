package productcheck

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

type Handler struct {
	Svc   *Service
	OpLog *operationlog.Service
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

// Register mounts readiness routes (JWT group already applied by caller).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.GET("/products/:id/readiness", h.GetReadiness)
	g.POST("/products/readiness/batch", h.BatchReadiness)
}

func (h *Handler) GetReadiness(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "product readiness unavailable")
		return
	}
	pid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	mode := strings.TrimSpace(c.Query("mode"))
	if mode == "" {
		mode = "draft"
	}
	plat := strings.TrimSpace(c.Query("platform"))
	var shopPtr *uuid.UUID
	if raw := strings.TrimSpace(c.Query("shopId")); raw != "" {
		if sid, err := uuid.Parse(raw); err == nil {
			shopPtr = &sid
		} else {
			response.Fail(c, 400, response.CodeBadRequest, "invalid shopId")
			return
		}
	}
	res, err := h.Svc.CheckProductReadiness(c.Request.Context(), CheckProductReadinessRequest{
		ProductID: pid,
		Platform:  plat,
		ShopID:    shopPtr,
		Mode:      mode,
	})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminUUID(c),
			Action:      "product.readiness.check",
			Resource:    "product",
			ResourceID:  pid.String(),
			Status:      "success",
			Message: fmt.Sprintf("platform=%s shopId=%s err=%d warn=%d",
				strings.TrimSpace(plat),
				shopIDforLog(shopPtr),
				res.ErrorCount,
				res.WarningCount,
			),
		})
	}
	response.OK(c, LocalizeReadinessResult(res))
}

func (h *Handler) BatchReadiness(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "product readiness unavailable")
		return
	}
	var body struct {
		ProductIDs []string `json:"productIds"`
		Platform   string   `json:"platform"`
		ShopID     string   `json:"shopId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	if len(body.ProductIDs) == 0 {
		response.Fail(c, 400, response.CodeBadRequest, "productIds required")
		return
	}
	if len(body.ProductIDs) > 100 {
		response.Fail(c, 400, response.CodeBadRequest, "at most 100 productIds per request")
		return
	}
	plat := strings.TrimSpace(body.Platform)
	if plat == "" {
		response.Fail(c, 400, response.CodeBadRequest, "platform required")
		return
	}
	sid, err := uuid.Parse(strings.TrimSpace(body.ShopID))
	if err != nil || sid == uuid.Nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid shopId")
		return
	}
	ids := make([]uuid.UUID, 0, len(body.ProductIDs))
	for _, s := range body.ProductIDs {
		u, err := uuid.Parse(strings.TrimSpace(s))
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "invalid product id in list")
			return
		}
		ids = append(ids, u)
	}
	list := make([]*CheckProductReadinessResult, 0, len(ids))
	var sumE, sumW int
	for _, pid := range ids {
		res, err := h.Svc.CheckProductReadiness(c.Request.Context(), CheckProductReadinessRequest{
			ProductID: pid,
			Platform:  plat,
			ShopID:    &sid,
			Mode:      "batch",
		})
		if err != nil {
			response.HandleError(c, err)
			return
		}
		sumE += res.ErrorCount
		sumW += res.WarningCount
		list = append(list, LocalizeReadinessResult(res))
	}
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminUUID(c),
			Action:      "product.readiness.batch_check",
			Resource:    "product",
			ResourceID:  "batch",
			Status:      "success",
			Message: fmt.Sprintf("size=%d platform=%s shopId=%s totalErr=%d totalWarn=%d",
				len(ids), plat, sid.String(), sumE, sumW,
			),
		})
	}
	response.OK(c, gin.H{"list": list})
}

func shopIDforLog(p *uuid.UUID) string {
	if p == nil {
		return ""
	}
	return p.String()
}
