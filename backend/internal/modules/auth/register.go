package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Register retains the endpoint for existing clients, but console accounts
// must be provisioned by an authorized administrator with transactional audit.
func (h *Handler) Register(c *gin.Context) {
	response.Fail(c, http.StatusForbidden, response.CodeForbidden, "请联系管理员创建账号，当前不开放自助注册")
}
