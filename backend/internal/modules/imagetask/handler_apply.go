package imagetask

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

type applyBody struct {
	ProductID string `json:"productId" binding:"required"`
	ItemID    string `json:"itemId"`
	ApplyMode string `json:"applyMode"`
	SetBest   bool   `json:"setBest"`
}

// Apply POST /api/v1/image/tasks/:id/apply
func (h *Handler) Apply(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "image tasks unavailable")
		return
	}
	taskID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body applyBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	pid, err := uuid.Parse(strings.TrimSpace(body.ProductID))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid productId")
		return
	}
	var itemID *uuid.UUID
	if raw := strings.TrimSpace(body.ItemID); raw != "" {
		u, err := uuid.Parse(raw)
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "invalid itemId")
			return
		}
		itemID = &u
	}
	row, err := h.Svc.ApplyTaskResultHTTP(c, pid, taskID, itemID, body.ApplyMode, body.SetBest, adminUUID(c))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	if h.Svc.OpLog != nil {
		_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "image.task.apply",
			Resource:   "image_task",
			ResourceID: taskID.String(),
			Status:     "success",
			Message:    "productId=" + pid.String() + " imageId=" + row.ID.String(),
		})
	}
	response.OK(c, row)
}

// ListItems GET /api/v1/image/tasks/:id/items
func (h *Handler) ListItems(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "image tasks unavailable")
		return
	}
	taskID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	items, err := h.Svc.ListTaskItems(c.Request.Context(), taskID)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"list": items})
}

// DeleteItem DELETE /api/v1/image/tasks/:id/items/:itemId
func (h *Handler) DeleteItem(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "image tasks unavailable")
		return
	}
	taskID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	itemID, err := uuid.Parse(strings.TrimSpace(c.Param("itemId")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid itemId")
		return
	}
	if err := h.Svc.DeleteTaskItem(c.Request.Context(), taskID, itemID); err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func parseScoreField(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return raw
}
