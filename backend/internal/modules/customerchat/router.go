package customerchat

import "github.com/gin-gonic/gin"

// Register mounts authenticated routes on g (already under /api/v1).
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	c := g.Group("/customer")
	c.GET("/conversations", h.ListConversations)
	c.POST("/conversations", h.CreateConversation)

	c.GET("/conversations/:id/messages", h.ListMessages)
	c.POST("/conversations/:id/messages", h.CreateMessage)
	c.POST("/conversations/:id/mark-replied", h.MarkReplied)
	c.POST("/conversations/:id/ai/generate-reply", h.GenerateReply)
	c.POST("/conversations/:id/send-platform-message", h.SendPlatformMessage)

	c.GET("/conversations/:id", h.GetConversation)
	c.PUT("/conversations/:id", h.UpdateConversation)
	c.DELETE("/conversations/:id", h.DeleteConversation)

	c.PUT("/reply-suggestions/:id", h.UpdateSuggestion)
	c.POST("/reply-suggestions/:id/accept", h.AcceptSuggestion)
	c.POST("/reply-suggestions/:id/discard", h.DiscardSuggestion)
}
