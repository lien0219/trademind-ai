package douyinpreflight

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Register mounts Douyin production preflight routes under authenticated /api/v1.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	var db *gorm.DB
	if h.Svc != nil {
		db = h.Svc.DB
	}
	withAuth := func(write bool, handler gin.HandlerFunc) gin.HandlerFunc {
		return func(c *gin.Context) {
			if !authorizePreflight(c, db, write) {
				c.Abort()
				return
			}
			handler(c)
		}
	}
	postHandlers := []gin.HandlerFunc{withAuth(true, h.Run)}
	if h.OpLog != nil {
		postHandlers = append([]gin.HandlerFunc{
			h.OpLog.AuditFailures("douyin.production.preflight.denied", "platform_douyin_shop", ""),
		}, postHandlers...)
	}
	g.POST("/platform/douyin/production-preflight", postHandlers...)
	g.GET("/platform/douyin/production-preflight/latest", withAuth(false, h.GetLatest))
}
