package auth

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Handler serves auth HTTP API.
type Handler struct {
	LoginSvc *LoginService
	Admins   *admin.Store
}

type loginBody struct {
	Username string `json:"username" binding:"required,min=1,max=64"`
	Password string `json:"password" binding:"required,min=1,max=128"`
}

// Login POST /api/v1/auth/login
func (h *Handler) Login(c *gin.Context) {
	if h == nil || h.LoginSvc == nil {
		response.Fail(c, 500, response.CodeInternalError, "auth unavailable")
		return
	}
	var body loginBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	res, err := h.LoginSvc.Login(c.Request.Context(), body.Username, body.Password)
	if err != nil {
		response.Fail(c, 401, response.CodeUnauthorized, err.Error())
		return
	}
	response.OK(c, gin.H{
		"token":     res.Token,
		"expiresAt": res.ExpiresAt,
		"user":      res.User,
	})
}

// Profile GET /api/v1/auth/profile
func (h *Handler) Profile(c *gin.Context) {
	if h == nil || h.Admins == nil {
		response.Fail(c, 500, response.CodeInternalError, "auth unavailable")
		return
	}
	idStr, ok := c.Get(ctxkey.AdminID)
	if !ok {
		response.Fail(c, 401, response.CodeUnauthorized, "unauthorized")
		return
	}
	s, _ := idStr.(string)
	uid, err := uuid.Parse(s)
	if err != nil {
		response.Fail(c, 401, response.CodeUnauthorized, "unauthorized")
		return
	}
	u, err := h.Admins.ByID(c.Request.Context(), uid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 401, response.CodeUnauthorized, "unauthorized")
			return
		}
		response.HandleError(c, err)
		return
	}
	dn := u.DisplayName
	if dn == "" {
		dn = u.Username
	}
	response.OK(c, gin.H{
		"id":          u.ID.String(),
		"username":    u.Username,
		"displayName": dn,
		"createdAt":   u.CreatedAt,
		"updatedAt":   u.UpdatedAt,
	})
}

// Logout POST /api/v1/auth/logout — stateless JWT; client discards token.
func (h *Handler) Logout(c *gin.Context) {
	response.OK(c, gin.H{"ok": true})
}
