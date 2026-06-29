package auth

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"gorm.io/gorm"
)

// Handler serves auth HTTP API.
type Handler struct {
	LoginSvc *LoginService
	Admins   *admin.Store
	OpLog    *operationlog.Service
	Redis    *rdb.Client
	Settings *settings.Service
}

type loginBody struct {
	Account  string `json:"account" binding:"required,min=1,max=128"`
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
	account := strings.TrimSpace(body.Account)
	if account == "" {
		response.Fail(c, 400, response.CodeBadRequest, "account is required")
		return
	}
	res, err := h.LoginSvc.Login(c.Request.Context(), account, body.Password)
	if err != nil {
		if h.OpLog != nil {
			_ = h.OpLog.Write(c, operationlog.WriteOpts{
				Username: account,
				Action:   "login",
				Resource: "auth",
				Status:   "failed",
				Message:  err.Error(),
			})
		}
		response.Fail(c, 401, response.CodeUnauthorized, err.Error())
		return
	}
	uid, perr := uuid.Parse(res.User.ID)
	if perr == nil && h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: &uid,
			Username:    res.User.Username,
			Action:      "login",
			Resource:    "auth",
			Status:      "success",
		})
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
		dn = u.LoginLabel()
	}
	response.OK(c, gin.H{
		"id":          u.ID.String(),
		"username":    u.LoginLabel(),
		"email":       u.Email,
		"phone":       u.Phone,
		"displayName": dn,
		"role":        strings.TrimSpace(u.Role),
		"createdAt":   u.CreatedAt,
		"updatedAt":   u.UpdatedAt,
	})
}

// Logout POST /api/v1/auth/logout — stateless JWT; client discards token.
func (h *Handler) Logout(c *gin.Context) {
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			Action:   "logout",
			Resource: "auth",
			Status:   "success",
		})
	}
	response.OK(c, gin.H{"ok": true})
}
