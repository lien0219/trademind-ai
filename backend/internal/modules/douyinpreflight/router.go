package douyinpreflight

import "github.com/gin-gonic/gin"

// Register mounts Douyin production preflight routes under authenticated /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.POST("/platform/douyin/production-preflight", h.Run)
	g.GET("/platform/douyin/production-preflight/latest", h.GetLatest)
}
