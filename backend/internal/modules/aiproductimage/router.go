package aiproductimage

import "github.com/gin-gonic/gin"

// Register mounts JWT-protected AI product image batch routes under /products/ai-images.
func Register(r gin.IRouter, h *Handler) {
	if h == nil {
		return
	}
	g := r.Group("/products/ai-images")
	g.POST("/batches/check", h.CheckBatch)
	g.POST("/batches", h.CreateBatch)
	g.GET("/batches", h.ListBatches)
	g.GET("/batches/:id", h.GetBatch)
	g.POST("/batches/:id/retry-failed", h.RetryFailed)
	g.POST("/batches/:id/cancel-pending", h.CancelPending)
	g.POST("/batches/:id/apply-selected", h.ApplySelected)
	g.POST("/batches/:id/undo-applied", h.UndoApplied)
	g.POST("/items/:id/regenerate", h.RegenerateItem)
	g.POST("/items/:id/apply", h.ApplyItem)
	g.POST("/items/:id/reject", h.RejectItem)
}
