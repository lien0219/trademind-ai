package logistics

import "github.com/gin-gonic/gin"

func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.GET("/logistics/channels", h.ListChannels)
	g.POST("/logistics/channels", h.CreateChannel)
	g.PUT("/logistics/channels/:id", h.UpdateChannel)
	g.GET("/logistics/rate-templates", h.ListRates)
	g.POST("/logistics/rate-templates", h.CreateRate)
	g.PUT("/logistics/rate-templates/:id", h.UpdateRate)
}
