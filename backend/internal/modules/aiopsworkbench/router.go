package aiopsworkbench

import "github.com/gin-gonic/gin"

// Register mounts JWT-protected AI operation workbench routes.
func Register(r gin.IRouter, h *Handler) {
	if h == nil {
		return
	}
	g := r.Group("/ai/operation-workbench")
	g.GET("/summary", h.Summary)
	g.GET("/todos", h.ListTodos)
	g.GET("/todos/:id", h.GetTodo)
	g.POST("/todos/refresh", h.RefreshTodos)
}
