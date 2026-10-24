package aiproductimage

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Register mounts JWT-protected AI product image batch routes under /products/ai-images.
func Register(r gin.IRouter, h *Handler) {
	if h == nil {
		return
	}
	g := r.Group("/products/ai-images")
	guard := func(perm string, write bool) gin.HandlerFunc {
		return func(c *gin.Context) {
			if h.Svc == nil {
				response.Fail(c, 500, response.CodeInternalError, "服务不可用")
				c.Abort()
				return
			}
			allowed := false
			if write {
				allowed = adminperm.RequireWrite(c, h.Svc.DB, perm)
			} else {
				allowed = adminperm.RequirePermission(c, h.Svc.DB, perm)
			}
			if !allowed {
				c.Abort()
				return
			}
			c.Next()
		}
	}
	view := guard(adminperm.PermProductView, false)
	write := guard(adminperm.PermAIImageApply, true)

	g.POST("/batches/check", view, h.CheckBatch)
	g.POST("/batches", write, h.CreateBatch)
	g.GET("/batches", view, h.ListBatches)
	g.GET("/batches/:id", view, h.GetBatch)
	g.POST("/batches/:id/retry-failed", write, h.RetryFailed)
	g.POST("/batches/:id/cancel-pending", write, h.CancelPending)
	g.POST("/batches/:id/apply-selected", write, h.ApplySelected)
	g.POST("/batches/:id/undo-applied", write, h.UndoApplied)
	g.POST("/items/:id/regenerate", write, h.RegenerateItem)
	g.POST("/items/:id/apply", write, h.ApplyItem)
	g.POST("/items/:id/reject", write, h.RejectItem)
}
