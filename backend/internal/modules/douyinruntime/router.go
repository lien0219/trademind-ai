package douyinruntime

import "github.com/gin-gonic/gin"

// Register mounts Douyin runtime control routes.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil || h.Svc == nil {
		return
	}
	read := func(c *gin.Context) bool { return authorizeRuntime(c, h.Svc.DB, false) }
	write := func(c *gin.Context) bool { return authorizeRuntime(c, h.Svc.DB, true) }
	withRead := func(handler gin.HandlerFunc) gin.HandlerFunc {
		return func(c *gin.Context) {
			if !read(c) {
				c.Abort()
				return
			}
			handler(c)
		}
	}
	withWrite := func(handler gin.HandlerFunc) gin.HandlerFunc {
		return func(c *gin.Context) {
			if !write(c) {
				c.Abort()
				return
			}
			handler(c)
		}
	}
	g.GET("/platform/douyin/runtime-status", withRead(h.Get))
	g.POST("/platform/douyin/runtime-status/pause", h.Svc.OpLog.AuditFailures("douyin.platform.pause.denied", "douyin_runtime", ""), withWrite(h.Pause))
	g.POST("/platform/douyin/runtime-status/resume", h.Svc.OpLog.AuditFailures("douyin.platform.resume.denied", "douyin_runtime", ""), withWrite(h.Resume))
	g.POST("/platform/douyin/runtime-status/emergency-disable", h.Svc.OpLog.AuditFailures("douyin.platform.emergency_disable.denied", "douyin_runtime", ""), withWrite(h.EmergencyDisable))
	g.GET("/platform/douyin/health", withRead(h.GetHealth))
	g.GET("/platform/douyin/metrics-summary", withRead(h.GetMetricsSummary))
	g.GET("/platform/douyin/release-gate", withRead(h.GetReleaseGate))
	g.POST("/platform/douyin/run-health-check", h.Svc.OpLog.AuditFailures("douyin.platform.health_check.denied", "douyin_runtime", ""), withWrite(h.RunHealthCheck))
}
