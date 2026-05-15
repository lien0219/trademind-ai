package settings

import (
	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Handler serves settings HTTP API.
type Handler struct {
	Svc *Service
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
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "settings unavailable")
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
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "settings unavailable")
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
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	rows, err := h.Svc.List(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"items": rows})
}
