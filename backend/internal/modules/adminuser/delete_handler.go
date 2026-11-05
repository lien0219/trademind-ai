package adminuser

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

func (h *Handler) Delete(c *gin.Context) {
	if !h.requireManage(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil || id == uuid.Nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "无效的用户 ID")
		return
	}
	if err := h.Svc.Delete(c, id, adminUUID(c)); err != nil {
		handleMutationError(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

func (h *Handler) DeleteRole(c *gin.Context) {
	if !h.requireManage(c) {
		return
	}
	code := strings.TrimSpace(c.Param("code"))
	if err := h.Svc.DeleteRole(c, code, adminUUID(c)); err != nil {
		switch {
		case errors.Is(err, ErrRoleHasMembers):
			response.Fail(c, http.StatusConflict, response.CodeBadRequest, err.Error())
		case errors.Is(err, gorm.ErrRecordNotFound):
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "角色不存在")
		default:
			handleMutationError(c, err)
		}
		return
	}
	response.OK(c, gin.H{"deleted": true})
}
