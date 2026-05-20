package collectruleai

import "github.com/gin-gonic/gin"

// Register mounts AI collect rule routes (must register before /collect/rules/:id).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.POST("/collect/rules/ai-generate", h.Generate)
	g.POST("/collect/rules/ai-generate-and-save", h.GenerateAndSave)
}
