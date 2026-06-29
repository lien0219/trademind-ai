package customerchat

import "github.com/gin-gonic/gin"

// Register mounts authenticated routes on g (already under /api/v1).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	registerCustomerRoutes(g.Group("/customer"), h)
	registerCustomerRoutes(g.Group("/customer-service"), h)
}

func registerCustomerRoutes(c *gin.RouterGroup, h *Handler) {
	if c == nil || h == nil {
		return
	}
	c.GET("/dashboard", h.GetDashboard)
	c.GET("/conversations", h.ListConversations)
	c.POST("/conversations", h.CreateConversation)

	c.GET("/conversations/:id/messages", h.ListMessages)
	c.POST("/conversations/:id/messages", h.CreateMessage)
	c.POST("/conversations/:id/mark-replied", h.MarkReplied)
	c.POST("/conversations/:id/ai/generate-reply", h.GenerateReply)
	c.POST("/conversations/:id/ai-suggestions", h.GenerateAISuggestion)
	c.GET("/conversations/:id/ai-suggestions", h.ListSuggestions)
	c.POST("/conversations/:id/send-platform-message", h.SendPlatformMessage)

	c.GET("/conversations/:id", h.GetConversation)
	c.PUT("/conversations/:id", h.UpdateConversation)
	c.DELETE("/conversations/:id", h.DeleteConversation)

	c.PUT("/reply-suggestions/:id", h.UpdateSuggestion)
	c.POST("/reply-suggestions/:id/accept", h.AcceptSuggestion)
	c.POST("/reply-suggestions/:id/discard", h.DiscardSuggestion)
	c.POST("/ai-suggestions/:id/apply", h.ApplySuggestion)
	c.POST("/ai-suggestions/:id/reject", h.RejectSuggestion)
}
