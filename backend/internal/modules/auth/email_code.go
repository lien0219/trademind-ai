package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// SendEmailCode cannot enable a public console-account provisioning path.
func (h *Handler) SendEmailCode(c *gin.Context) {
	response.Fail(c, http.StatusForbidden, response.CodeForbidden, "请联系管理员创建账号，当前不开放自助注册")
}
