package settings

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

const (
	RegistrationClosed     = "closed"
	RegistrationInviteOnly = "invite_only"
	RegistrationOpen       = "open"
)

func RegistrationPolicy(ctx context.Context, db *gorm.DB) string {
	if db == nil {
		return RegistrationClosed
	}
	var row Setting
	if err := db.WithContext(ctx).Where("tenant_id = 0 AND group_key = ? AND item_key = ?", "auth_registration", "mode").Take(&row).Error; err != nil {
		return RegistrationClosed
	}
	switch strings.TrimSpace(row.ItemValue) {
	case RegistrationClosed, RegistrationInviteOnly, RegistrationOpen:
		return strings.TrimSpace(row.ItemValue)
	}
	return RegistrationClosed
}

func (h *Handler) GetRegistrationPolicy(c *gin.Context) {
	if !h.requireSettingsManage(c) {
		return
	}
	response.OK(c, gin.H{"mode": RegistrationPolicy(c.Request.Context(), h.DB)})
}

func (h *Handler) PutRegistrationPolicy(c *gin.Context) {
	if !h.requireSettingsManage(c) {
		return
	}
	var body struct {
		Mode string `json:"mode"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid registration policy")
		return
	}
	if body.Mode != RegistrationClosed && body.Mode != RegistrationInviteOnly && body.Mode != RegistrationOpen {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid registration policy")
		return
	}
	if h.OpLog == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, http.StatusServiceUnavailable, response.CodeInternalError, "registration policy audit unavailable")
		return
	}
	err := h.Svc.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := h.Svc.PutBulkTx(tx, []PutItem{{TenantID: 0, GroupKey: "auth_registration", ItemKey: "mode", ItemValue: body.Mode}}); err != nil {
			return err
		}
		return h.OpLog.WriteTx(c, tx, operationlog.WriteOpts{
			Action: "auth.registration_policy.update", Resource: "auth_registration", Status: "success", Message: "mode=" + body.Mode,
		})
	})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"mode": body.Mode})
}
