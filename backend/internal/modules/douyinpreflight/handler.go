package douyinpreflight

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Handler serves Douyin production preflight APIs.
type Handler struct {
	Svc   *Service
	OpLog *operationlog.Service
}

// Run POST /api/v1/platform/douyin/production-preflight
func (h *Handler) Run(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "douyin preflight unavailable")
		return
	}
	var body RunRequest
	_ = c.ShouldBindJSON(&body)
	res, err := h.Svc.Run(c, body)
	if err != nil {
		if h.OpLog != nil {
			_ = h.OpLog.Write(c, operationlog.WriteOpts{
				Action:   "douyin.production.preflight",
				Resource: "platform_douyin_shop",
				Status:   "failed",
				Message:  err.Error(),
			})
		}
		response.HandleError(c, err)
		return
	}
	st := res.Status
	if st == statusPassed {
		st = "success"
	}
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			Action:   "douyin.production.preflight",
			Resource: "platform_douyin_shop",
			Status:   st,
			Message:  fmt.Sprintf("%s passed=%d warning=%d failed=%d", res.Status, res.PassedCount, res.WarningCount, res.FailedCount),
		})
	}
	response.OK(c, res)
}

// GetLatest GET /api/v1/platform/douyin/production-preflight/latest
func (h *Handler) GetLatest(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "douyin preflight unavailable")
		return
	}
	res, err := h.Svc.GetLatest(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	if res == nil {
		response.OK(c, gin.H{"result": nil, "message": "尚未运行生产预检"})
		return
	}
	response.OK(c, res)
}
