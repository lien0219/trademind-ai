package orderexception

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Handler serves authenticated /orders/exceptions routes.
type Handler struct {
	Svc   *Service
	Cmds  *Commands
	OpLog *operationlog.Service
}

func adminUUID(c *gin.Context) *uuid.UUID {
	if v, ok := c.Get(ctxkey.AdminID); ok {
		if s, ok := v.(string); ok {
			if u, err := uuid.Parse(strings.TrimSpace(s)); err == nil {
				return &u
			}
		}
	}
	return nil
}

func parseRFC3339(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func parseOptionalBoolQuery(c *gin.Context, key string) *bool {
	raw := strings.TrimSpace(strings.ToLower(c.Query(key)))
	if raw == "" {
		return nil
	}
	if raw == "1" || raw == "true" || raw == "yes" || raw == "on" {
		b := true
		return &b
	}
	b := false
	return &b
}

func atoiPage(raw string, def int, min int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < min {
		return def
	}
	return n
}

// List GET /orders/exceptions
func (h *Handler) List(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "exceptions unavailable")
		return
	}
	startPtr, err := parseRFC3339(c.Query("start"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid start time (RFC3339)")
		return
	}
	endPtr, err := parseRFC3339(c.Query("end"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid end time (RFC3339)")
		return
	}
	req := ListOrderExceptionsRequest{
		ExceptionType: strings.TrimSpace(c.Query("exceptionType")),
		Severity:      strings.TrimSpace(c.Query("severity")),
		Platform:      strings.TrimSpace(c.Query("platform")),
		ShopID:        strings.TrimSpace(c.Query("shopId")),
		OrderID:       strings.TrimSpace(c.Query("orderId")),
		Keyword:       strings.TrimSpace(c.Query("keyword")),
		Handled:       parseOptionalBoolQuery(c, "handled"),
		Ignored:       parseOptionalBoolQuery(c, "ignored"),
		Start:         startPtr,
		End:           endPtr,
		Page:          atoiPage(c.Query("page"), 1, 1),
		PageSize:      atoiPage(c.Query("pageSize"), 20, 1),
	}
	out, err := h.Svc.ListOrderExceptions(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// Detail GET /orders/exceptions/:sourceType/:sourceId
func (h *Handler) Detail(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "exceptions unavailable")
		return
	}
	st := strings.TrimSpace(c.Param("sourceType"))
	sid := strings.TrimSpace(c.Param("sourceId"))
	d, err := h.Svc.GetOrderExceptionDetail(c.Request.Context(), st, sid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, d)
}

// Handle POST /orders/exceptions/:sourceType/:sourceId/handle
func (h *Handler) Handle(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "exceptions unavailable")
		return
	}
	var body HandleBody
	_ = c.ShouldBindJSON(&body)
	if strings.TrimSpace(body.ExceptionType) == "" {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "exceptionType required")
		return
	}
	st := strings.TrimSpace(c.Param("sourceType"))
	sid := strings.TrimSpace(c.Param("sourceId"))
	if err := h.Svc.UpsertMark(c.Request.Context(), body.ExceptionType, st, sid, MarkHandled, body.Remark, adminUUID(c)); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminUUID(c),
			Action:      "order.exception.handle",
			Resource:    "order_exception",
			ResourceID:  sid,
			Status:      "success",
			Message:     truncateExcMsg("exceptionType=" + body.ExceptionType + " sourceType=" + st + " sourceId=" + sid),
		})
	}
	response.OK(c, gin.H{"ok": true})
}

// Ignore POST /orders/exceptions/:sourceType/:sourceId/ignore
func (h *Handler) Ignore(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "exceptions unavailable")
		return
	}
	var body HandleBody
	_ = c.ShouldBindJSON(&body)
	if strings.TrimSpace(body.ExceptionType) == "" {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "exceptionType required")
		return
	}
	st := strings.TrimSpace(c.Param("sourceType"))
	sid := strings.TrimSpace(c.Param("sourceId"))
	if err := h.Svc.UpsertMark(c.Request.Context(), body.ExceptionType, st, sid, MarkIgnored, body.Remark, adminUUID(c)); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminUUID(c),
			Action:      "order.exception.ignore",
			Resource:    "order_exception",
			ResourceID:  sid,
			Status:      "success",
			Message:     truncateExcMsg("exceptionType=" + body.ExceptionType + " sourceType=" + st + " sourceId=" + sid),
		})
	}
	response.OK(c, gin.H{"ok": true})
}

// Unmark DELETE /orders/exceptions/:sourceType/:sourceId/mark
func (h *Handler) Unmark(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "exceptions unavailable")
		return
	}
	st := strings.TrimSpace(c.Param("sourceType"))
	sid := strings.TrimSpace(c.Param("sourceId"))
	if err := h.Svc.DeleteMarks(c.Request.Context(), st, sid); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminUUID(c),
			Action:      "order.exception.unmark",
			Resource:    "order_exception",
			ResourceID:  sid,
			Status:      "success",
			Message:     truncateExcMsg("sourceType=" + st + " sourceId=" + sid),
		})
	}
	response.OK(c, gin.H{"ok": true})
}

// BindSKU POST /orders/exceptions/:sourceType/:sourceId/bind-sku
func (h *Handler) BindSKU(c *gin.Context) {
	if h == nil || h.Cmds == nil {
		response.Fail(c, 500, response.CodeInternalError, "exceptions unavailable")
		return
	}
	var body BindSKURequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	st := strings.TrimSpace(c.Param("sourceType"))
	sid := strings.TrimSpace(c.Param("sourceId"))
	out, err := h.Cmds.BindSKU(c.Request.Context(), st, sid, body, adminUUID(c))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminUUID(c),
			Action:      "order.exception.bind_sku",
			Resource:    "order_exception",
			ResourceID:  sid,
			Status:      "success",
			Message:     truncateExcMsg("sourceType=" + st + " skuId=" + strings.TrimSpace(body.ProductSKUID)),
		})
	}
	response.OK(c, out)
}

type retryDeductBody struct {
	SyncPlatforms bool `json:"syncPlatforms"`
}

// RetryDeduct POST /orders/exceptions/:sourceType/:sourceId/retry-deduct
func (h *Handler) RetryDeduct(c *gin.Context) {
	if h == nil || h.Cmds == nil {
		response.Fail(c, 500, response.CodeInternalError, "exceptions unavailable")
		return
	}
	var body retryDeductBody
	_ = c.ShouldBindJSON(&body)
	st := strings.TrimSpace(c.Param("sourceType"))
	sid := strings.TrimSpace(c.Param("sourceId"))
	sum, err := h.Cmds.RetryDeduct(c.Request.Context(), st, sid, body.SyncPlatforms, adminUUID(c))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminUUID(c),
			Action:      "order.exception.retry_deduct",
			Resource:    "order_exception",
			ResourceID:  sid,
			Status:      "success",
			Message:     truncateExcMsg("sourceType=" + st + " sync=" + strconv.FormatBool(body.SyncPlatforms)),
		})
	}
	response.OK(c, gin.H{"inventoryDeduction": sum})
}

// RetryInventorySync POST /orders/exceptions/:sourceType/:sourceId/retry-inventory-sync
func (h *Handler) RetryInventorySync(c *gin.Context) {
	if h == nil || h.Cmds == nil {
		response.Fail(c, 500, response.CodeInternalError, "exceptions unavailable")
		return
	}
	st := strings.TrimSpace(c.Param("sourceType"))
	sid := strings.TrimSpace(c.Param("sourceId"))
	if !strings.EqualFold(st, SourceInventorySyncTask) {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "only inventory_sync_task sources support inventory sync retry")
		return
	}
	tid, err := uuid.Parse(sid)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid sourceId")
		return
	}
	task, err := h.Cmds.RetryInventorySync(c.Request.Context(), tid, adminUUID(c))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminUUID(c),
			Action:      "order.exception.retry_inventory_sync",
			Resource:    "inventory_sync_task",
			ResourceID:  sid,
			Status:      "success",
			Message:     truncateExcMsg("taskId=" + sid),
		})
	}
	response.OK(c, gin.H{"task": task})
}

func truncateExcMsg(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 480 {
		return s[:480] + "…"
	}
	return s
}
