package settings

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
	"gorm.io/gorm"
)

// Handler serves settings HTTP API.
type Handler struct {
	Svc       *Service
	OpLog     *operationlog.Service
	AIGateway *aigate.Gateway
	DB        *gorm.DB
}

func (h *Handler) requireSettingsManage(c *gin.Context) bool {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "settings unavailable")
		return false
	}
	return adminperm.RequireWrite(c, h.DB, adminperm.PermSettingsManage)
}

type putBody struct {
	Items []putItemJSON `json:"items" binding:"required,dive"`
}

type putItemJSON struct {
	TenantID    *int64 `json:"tenantId"`
	GroupKey    string `json:"groupKey" binding:"required,max=100"`
	ItemKey     string `json:"itemKey" binding:"required,max=100"`
	ItemValue   string `json:"itemValue"`
	ValueType   string `json:"valueType" binding:"omitempty,max=50"`
	IsEncrypted bool   `json:"isEncrypted"`
	Remark      string `json:"remark" binding:"omitempty,max=255"`
}

// List GET /api/v1/settings
func (h *Handler) List(c *gin.Context) {
	if !h.requireSettingsManage(c) {
		return
	}
	rows, err := h.Svc.List(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"items": rows})
}

// Put PUT /api/v1/settings
func (h *Handler) Put(c *gin.Context) {
	if !h.requireSettingsManage(c) {
		return
	}
	var body putBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	items := make([]PutItem, 0, len(body.Items))
	for _, it := range body.Items {
		tid := int64(0)
		if it.TenantID != nil {
			tid = *it.TenantID
		}
		items = append(items, PutItem{
			TenantID:    tid,
			GroupKey:    it.GroupKey,
			ItemKey:     it.ItemKey,
			ItemValue:   it.ItemValue,
			ValueType:   it.ValueType,
			IsEncrypted: it.IsEncrypted,
			Remark:      it.Remark,
		})
	}
	if err := h.Svc.PutBulk(c.Request.Context(), items); err != nil {
		if h.OpLog != nil {
			_ = h.OpLog.Write(c, operationlog.WriteOpts{
				Action:   "settings_update",
				Resource: "settings",
				Status:   "failed",
				Message:  err.Error(),
			})
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	alertNotifyTouch := false
	pricingTouch := false
	for _, it := range items {
		gk := strings.TrimSpace(it.GroupKey)
		if gk == "alert_notify" {
			alertNotifyTouch = true
		}
		if gk == "pricing" {
			pricingTouch = true
		}
	}
	if alertNotifyTouch && h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			Action:   "settings.alert_notify.update",
			Resource: "settings",
			Status:   "success",
			Message:  "alert_notify bulk upsert",
		})
	}
	if pricingTouch && h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			Action:   "settings.pricing.update",
			Resource: "settings",
			Status:   "success",
			Message:  "pricing bulk upsert",
		})
	}
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			Action:   "settings_update",
			Resource: "settings",
			Status:   "success",
			Message:  "bulk upsert",
		})
	}
	rows, err := h.Svc.List(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"items": rows})
}

// TestPlatformTikTok POST /api/v1/settings/test-platform-tiktok validates platform_tiktok settings structure (no live TikTok call).
func (h *Handler) TestPlatformTikTok(c *gin.Context) {
	if !h.requireSettingsManage(c) {
		return
	}
	if err := h.Svc.ValidateTikTokPlatformConfig(c.Request.Context()); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

type testAIBody struct {
	Provider   string `json:"provider"`
	BaseURL    string `json:"base_url"`
	Model      string `json:"model"`
	APIKey     string `json:"api_key"`
	TimeoutSec string `json:"timeout_sec"`
}

// TestAI POST /api/v1/settings/test-ai
// Optional JSON body lets the admin test unsaved form values; empty body uses saved settings.ai only.
func (h *Handler) TestAI(c *gin.Context) {
	if !h.requireSettingsManage(c) {
		return
	}
	if h.AIGateway == nil {
		response.Fail(c, 500, response.CodeInternalError, "ai gateway unavailable")
		return
	}
	plain, err := h.Svc.PlainByGroup(c.Request.Context(), 0, "ai")
	if err != nil {
		response.Fail(c, 500, response.CodeInternalError, err.Error())
		return
	}
	var body testAIBody
	_ = c.ShouldBindJSON(&body)
	plain = MergeAIPlain(plain, &TestAIOverrides{
		Provider:   body.Provider,
		BaseURL:    body.BaseURL,
		Model:      body.Model,
		APIKey:     body.APIKey,
		TimeoutSec: body.TimeoutSec,
	})
	res, err := h.AIGateway.TestConnectionWithPlain(c.Request.Context(), plain)
	if err != nil {
		msg := err.Error()
		if res != nil && strings.TrimSpace(res.Message) != "" {
			msg = res.Message
		}
		if h.OpLog != nil {
			_ = h.OpLog.Write(c, operationlog.WriteOpts{
				Action:   "test_ai",
				Resource: "settings",
				Status:   "failed",
				Message:  msg,
			})
		}
		response.Fail(c, 400, response.CodeBadRequest, msg)
		return
	}
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			Action:   "test_ai",
			Resource: "settings",
			Status:   "success",
		})
	}
	out := gin.H{"ok": true, "message": "连接成功"}
	if res != nil {
		out["provider"] = res.Provider
		out["model"] = res.Model
		out["latencyMs"] = res.LatencyMs
		if res.Message != "" {
			out["message"] = res.Message
		}
	}
	response.OK(c, out)
}

// TestStorage POST /api/v1/settings/test-storage
func (h *Handler) TestStorage(c *gin.Context) {
	if !h.requireSettingsManage(c) {
		return
	}
	if err := h.Svc.TestStorageConnection(c.Request.Context()); err != nil {
		if h.OpLog != nil {
			_ = h.OpLog.Write(c, operationlog.WriteOpts{
				Action:   "test_storage",
				Resource: "settings",
				Status:   "failed",
				Message:  err.Error(),
			})
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			Action:   "test_storage",
			Resource: "settings",
			Status:   "success",
		})
	}
	response.OK(c, gin.H{"ok": true})
}

type testEmailBody struct {
	To string `json:"to" binding:"required,email"`
}

// TestEmail POST /api/v1/settings/test-email
func (h *Handler) TestEmail(c *gin.Context) {
	if !h.requireSettingsManage(c) {
		return
	}
	var body testEmailBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid email address")
		return
	}
	if err := h.Svc.TestEmailConnection(c.Request.Context(), body.To); err != nil {
		if h.OpLog != nil {
			_ = h.OpLog.Write(c, operationlog.WriteOpts{
				Action:   "test_email",
				Resource: "settings",
				Status:   "failed",
				Message:  err.Error(),
			})
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			Action:   "test_email",
			Resource: "settings",
			Status:   "success",
		})
	}
	response.OK(c, gin.H{"ok": true})
}

// IntegrationSchemas GET /api/v1/settings/integration-schemas — static registry for admin UX.
func (h *Handler) IntegrationSchemas(c *gin.Context) {
	if !h.requireSettingsManage(c) {
		return
	}
	response.OK(c, gin.H{"schemas": IntegrationConfigDefinitions()})
}

// IntegrationOverview GET /api/v1/settings/integrations/overview
func (h *Handler) IntegrationOverview(c *gin.Context) {
	if !h.requireSettingsManage(c) {
		return
	}
	out, err := h.Svc.BuildIntegrationOverview(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, out)
}
