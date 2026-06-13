package storagepublic

import "github.com/gin-gonic/gin"

// Register mounts storage public access routes under authenticated /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.POST("/storage/test-public-access", h.TestPublicAccess)
}
