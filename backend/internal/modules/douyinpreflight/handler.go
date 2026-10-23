package douyinpreflight

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
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
	if h.OpLog == nil || h.OpLog.DB == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "预检审计服务不可用")
		return
	}
	var body RunRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "预检请求格式无效")
		return
	}
	if err := h.OpLog.Write(c, operationlog.WriteOpts{
		TenantID: 0,
		Action:   "douyin.production.preflight.started",
		Resource: "platform_douyin_shop",
		Status:   "started",
		Message:  "production preflight started",
	}); err != nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "预检审计记录暂时无法保存，请稍后重试")
		return
	}
	res, err := h.Svc.Run(c, body)
	if err != nil {
		h.failRun(c, "抖店生产预检执行失败")
		return
	}
	st := res.Status
	if st == statusPassed {
		st = "success"
	}
	if err := h.Svc.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := h.Svc.saveLatest(tx, res); err != nil {
			return err
		}
		return h.OpLog.WriteTx(c, tx, operationlog.WriteOpts{
			TenantID: 0,
			Action:   "douyin.production.preflight",
			Resource: "platform_douyin_shop",
			Status:   st,
			Message:  fmt.Sprintf("%s passed=%d warning=%d failed=%d", res.Status, res.PassedCount, res.WarningCount, res.FailedCount),
		})
	}); err != nil {
		h.failRun(c, "预检结果与审计记录暂时无法保存，请稍后重试")
		return
	}
	response.OK(c, res)
}

func (h *Handler) failRun(c *gin.Context, message string) {
	if h == nil || h.OpLog == nil || h.OpLog.DB == nil || h.OpLog.Write(c, operationlog.WriteOpts{
		TenantID: 0,
		Action:   "douyin.production.preflight",
		Resource: "platform_douyin_shop",
		Status:   "failed",
		Message:  message,
	}) != nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "预检失败，审计记录暂时无法保存")
		return
	}
	response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, message)
}

// GetLatest GET /api/v1/platform/douyin/production-preflight/latest
func (h *Handler) GetLatest(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "douyin preflight unavailable")
		return
	}
	res, err := h.Svc.GetLatest(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "预检结果暂时无法读取")
		return
	}
	if res == nil {
		response.OK(c, gin.H{"result": nil, "message": "尚未运行生产预检"})
		return
	}
	response.OK(c, res)
}
