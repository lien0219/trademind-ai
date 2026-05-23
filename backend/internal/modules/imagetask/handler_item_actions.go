package imagetask

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

type saveItemBody struct {
	ProductID string `json:"productId" binding:"required"`
	ApplyMode string `json:"applyMode"`
	SetBest   bool   `json:"setBest"`
}

type scoreImageBody struct {
	ProductID      string `json:"productId"`
	SourceImageID  string `json:"sourceImageId"`
	SourceImageURL string `json:"sourceImageUrl"`
	ImageType      string `json:"imageType"`
}

// SaveItemToProduct POST /api/v1/ai/image/task-items/:id/save-to-product
func (h *Handler) SaveItemToProduct(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "image tasks unavailable")
		return
	}
	itemID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid item id")
		return
	}
	var body saveItemBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	pid, err := uuid.Parse(strings.TrimSpace(body.ProductID))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid productId")
		return
	}
	mode := strings.TrimSpace(body.ApplyMode)
	if mode == "" {
		mode = "ai_generated"
	}
	row, err := h.Svc.ApplyItemByID(c.Request.Context(), itemID, pid, mode, body.SetBest, adminUUID(c))
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
			Action:     "image.task_item.save",
			Resource:   "ai_image_task_item",
			ResourceID: itemID.String(),
			Status:     "success",
			Message:    "productId=" + pid.String(),
		})
	}
	response.OK(c, row)
}

// SetItemAsMain POST /api/v1/ai/image/task-items/:id/set-as-main
func (h *Handler) SetItemAsMain(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "image tasks unavailable")
		return
	}
	itemID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid item id")
		return
	}
	var body struct {
		ProductID string `json:"productId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	pid, err := uuid.Parse(strings.TrimSpace(body.ProductID))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid productId")
		return
	}
	row, err := h.Svc.SetItemAsMain(c.Request.Context(), itemID, pid, adminUUID(c))
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
			Action:     "image.task_item.set_main",
			Resource:   "ai_image_task_item",
			ResourceID: itemID.String(),
			Status:     "success",
			Message:    "productId=" + pid.String(),
		})
	}
	response.OK(c, row)
}

// ScoreImage POST /api/v1/ai/image/score
func (h *Handler) ScoreImage(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "image tasks unavailable")
		return
	}
	var body scoreImageBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	req := ScoreImageRequest{
		SourceImageURL: strings.TrimSpace(body.SourceImageURL),
		ImageType:      strings.TrimSpace(body.ImageType),
	}
	if raw := strings.TrimSpace(body.ProductID); raw != "" {
		pid, err := uuid.Parse(raw)
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "invalid productId")
			return
		}
		req.ProductID = &pid
	}
	if raw := strings.TrimSpace(body.SourceImageID); raw != "" {
		sid, err := uuid.Parse(raw)
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "invalid sourceImageId")
			return
		}
		req.SourceImageID = &sid
	}
	score, err := h.Svc.ScoreImageHTTP(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, score)
}

type selectBestMainBody struct {
	Mode string `json:"mode"`
}

// SelectBestMainForProduct POST /api/v1/products/:id/images/select-best-main
func (h *Handler) SelectBestMainForProduct(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "image tasks unavailable")
		return
	}
	pid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body selectBestMainBody
	_ = c.ShouldBindJSON(&body)
	mode := strings.TrimSpace(body.Mode)
	if mode == "" {
		mode = "recommend"
	}
	var n int64
	if err := h.Svc.DB.WithContext(c.Request.Context()).Table("products").Where("id = ?", pid).Count(&n).Error; err != nil || n == 0 {
		response.Fail(c, 404, response.CodeNotFound, "product not found")
		return
	}
	row, err := h.Svc.CreateSelectBestMainTask(c.Request.Context(), pid, mode, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	if err := h.Svc.FinalizeNewImageTask(c.Request.Context(), c, row); err != nil {
		response.Fail(c, 503, response.CodeServiceUnavailable, err.Error())
		return
	}
	fresh, err := h.Svc.GetByID(c.Request.Context(), row.ID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	if h.Svc.OpLog != nil {
		_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "product.image.select_best_main",
			Resource:   "product",
			ResourceID: pid.String(),
			Status:     "success",
			Message:    "mode=" + mode + " taskId=" + fresh.ID.String(),
		})
	}
	response.OK(c, fresh)
}
