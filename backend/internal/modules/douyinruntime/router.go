package douyinruntime

import "github.com/gin-gonic/gin"

// Register mounts Douyin runtime control routes.
func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.GET("/platform/douyin/runtime-status", h.Get)
	g.POST("/platform/douyin/runtime-status/pause", h.Pause)
	g.POST("/platform/douyin/runtime-status/resume", h.Resume)
	g.POST("/platform/douyin/runtime-status/emergency-disable", h.EmergencyDisable)
	g.GET("/platform/douyin/health", h.GetHealth)
	g.GET("/platform/douyin/metrics-summary", h.GetMetricsSummary)
	g.GET("/platform/douyin/release-gate", h.GetReleaseGate)
	g.POST("/platform/douyin/run-health-check", h.RunHealthCheck)
}
