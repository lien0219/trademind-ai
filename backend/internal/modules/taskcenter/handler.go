package taskcenter

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter/failureclassifier"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Handler exposes /api/v1/task-center endpoints.
type Handler struct {
	Svc *Service
}

func atoiQ(threshold int, raw string, def int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < threshold {
		return def
	}
	return n
}

func parseRFC3339Ptr(s string) (*time.Time, error) {
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

func applyMarkFiltersFromQuery(q *gin.Context, p *ListFailureParams) bool {
	switch strings.TrimSpace(strings.ToLower(q.Query("ignored"))) {
	case "true", "1", "yes":
		p.RequireIgnored = true
	}
	switch strings.TrimSpace(strings.ToLower(q.Query("handled"))) {
	case "true", "1", "yes":
		p.RequireHandled = true
	}
	return !(p.RequireIgnored && p.RequireHandled)
}

// ListFailures GET /task-center/failures
func (h *Handler) ListFailures(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "task center unavailable")
		return
	}
	page := atoiQ(1, c.DefaultQuery("page", "1"), 1)
	pageSize := atoiQ(1, c.DefaultQuery("pageSize", "20"), 20)
	startPtr, err := parseRFC3339Ptr(c.Query("start"))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid start time (RFC3339)")
		return
	}
	endPtr, err := parseRFC3339Ptr(c.Query("end"))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid end time (RFC3339)")
		return
	}
	p := ListFailureParams{
		TaskType:         strings.TrimSpace(c.Query("taskType")),
		Status:           strings.TrimSpace(c.Query("status")),
		NormalizedStatus: strings.TrimSpace(c.Query("normalizedStatus")),
		Platform:         strings.TrimSpace(c.Query("platform")),
		ShopID:           strings.TrimSpace(c.Query("shopId")),
		Keyword:          strings.TrimSpace(c.Query("keyword")),
		FailureCategory:  strings.TrimSpace(c.Query("failureCategory")),
		Severity:         strings.TrimSpace(c.Query("severity")),
		RecoveryStatus:   strings.TrimSpace(c.Query("recoveryStatus")),
		IncludeResolved:  strings.EqualFold(c.Query("includeResolved"), "true") || c.Query("includeResolved") == "1",
		IncludeMarked:    strings.EqualFold(c.Query("includeMarked"), "true") || c.Query("includeMarked") == "1",
		Start:            startPtr,
		End:              endPtr,
		Page:             page,
		PageSize:         pageSize,
	}
	if !applyMarkFiltersFromQuery(c, &p) {
		response.Fail(c, 400, response.CodeBadRequest, "ignored and handled filters are mutually exclusive")
		return
	}
	if h.Svc != nil && h.Svc.DB != nil {
		if pr, err := adminperm.LoadPrincipal(c, h.Svc.DB); err == nil && pr != nil && !pr.IsAdmin() {
			p.AllowedShopIDs = pr.AllowedStoreIDs()
		}
	}
	out, err := h.Svc.ListFailures(c.Request.Context(), p)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// Summary GET /task-center/summary
func (h *Handler) Summary(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "task center unavailable")
		return
	}
	startPtr, err := parseRFC3339Ptr(c.Query("start"))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid start time (RFC3339)")
		return
	}
	endPtr, err := parseRFC3339Ptr(c.Query("end"))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid end time (RFC3339)")
		return
	}
	p := ListFailureParams{
		TaskType:        strings.TrimSpace(c.Query("taskType")),
		Platform:        strings.TrimSpace(c.Query("platform")),
		ShopID:          strings.TrimSpace(c.Query("shopId")),
		Keyword:         strings.TrimSpace(c.Query("keyword")),
		IncludeResolved: strings.EqualFold(c.Query("includeResolved"), "true"),
		IncludeMarked:   strings.EqualFold(c.Query("includeMarked"), "true") || c.Query("includeMarked") == "1",
		Start:           startPtr,
		End:             endPtr,
	}
	var mf ListFailureParams
	if !applyMarkFiltersFromQuery(c, &mf) {
		response.Fail(c, 400, response.CodeBadRequest, "ignored and handled filters are mutually exclusive")
		return
	}
	p.RequireIgnored = mf.RequireIgnored
	p.RequireHandled = mf.RequireHandled
	su, err := h.Svc.Summary(c.Request.Context(), p)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, su)
}

// GetFailure GET /task-center/failures/:taskType/:id
func (h *Handler) GetFailure(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "task center unavailable")
		return
	}
	tid := strings.TrimSpace(c.Param("taskType"))
	rawID := strings.TrimSpace(c.Param("id"))
	id, err := uuid.Parse(rawID)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	out, err := h.Svc.GetFailureDetail(c, tid, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// Retry POST /task-center/failures/:taskType/:id/retry
func (h *Handler) Retry(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "task center unavailable")
		return
	}
	tid := strings.TrimSpace(c.Param("taskType"))
	rawID := strings.TrimSpace(c.Param("id"))
	id, err := uuid.Parse(rawID)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if err := h.Svc.RetryFailure(c, tid, id); err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "collect queue disabled"):
			response.Fail(c, http.StatusConflict, response.CodeBadRequest, msg)
			return
		default:
			response.Fail(c, 400, response.CodeBadRequest, msg)
			return
		}
	}
	d, err := h.Svc.GetFailureDetail(c, tid, id)
	if err != nil {
		response.OK(c, gin.H{"retried": true})
		return
	}
	response.OK(c, d)
}

// BatchRetry POST /task-center/failures/batch-retry
func (h *Handler) BatchRetry(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "task center unavailable")
		return
	}
	var body BatchRetryRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	out := h.Svc.BatchRetryFailure(c, body)
	response.OK(c, out)
}

// Ignore POST /task-center/failures/:taskType/:id/ignore
func (h *Handler) Ignore(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "task center unavailable")
		return
	}
	tid := strings.TrimSpace(c.Param("taskType"))
	rawID := strings.TrimSpace(c.Param("id"))
	id, err := uuid.Parse(rawID)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body MarkRemarkBody
	_ = c.ShouldBindJSON(&body)
	if err := h.Svc.IgnoreFailure(c, tid, id, body.Remark); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// Handle POST /task-center/failures/:taskType/:id/handle
func (h *Handler) Handle(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "task center unavailable")
		return
	}
	tid := strings.TrimSpace(c.Param("taskType"))
	rawID := strings.TrimSpace(c.Param("id"))
	id, err := uuid.Parse(rawID)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body MarkRemarkBody
	_ = c.ShouldBindJSON(&body)
	if err := h.Svc.HandleFailure(c, tid, id, body.Remark); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// Unmark DELETE /task-center/failures/:taskType/:id/mark
func (h *Handler) Unmark(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "task center unavailable")
		return
	}
	tid := strings.TrimSpace(c.Param("taskType"))
	rawID := strings.TrimSpace(c.Param("id"))
	id, err := uuid.Parse(rawID)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if err := h.Svc.UnmarkFailure(c, tid, id); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// BatchIgnore POST /task-center/failures/batch-ignore
func (h *Handler) BatchIgnore(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "task center unavailable")
		return
	}
	var body BatchMarkRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	out := h.Svc.BatchIgnoreFailures(c, body)
	response.OK(c, out)
}

// BatchHandle POST /task-center/failures/batch-handle
func (h *Handler) BatchHandle(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "task center unavailable")
		return
	}
	var body BatchMarkRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	out := h.Svc.BatchHandleFailures(c, body)
	response.OK(c, out)
}

// FailureCategories GET /task-center/failure-categories
func (h *Handler) FailureCategories(c *gin.Context) {
	response.OK(c, gin.H{
		"categories": failureclassifier.AllCategories(),
		"severities": failureclassifier.AllSeverities(),
	})
}

// ListAlerts GET /task-center/alerts
func (h *Handler) ListAlerts(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "task center unavailable")
		return
	}
	startPtr, err := parseRFC3339Ptr(c.Query("start"))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid start time (RFC3339)")
		return
	}
	endPtr, err := parseRFC3339Ptr(c.Query("end"))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid end time (RFC3339)")
		return
	}
	p := ListAlertsParams{
		Status:          strings.TrimSpace(c.Query("status")),
		Severity:        strings.TrimSpace(c.Query("severity")),
		FailureCategory: strings.TrimSpace(c.Query("failureCategory")),
		TaskType:        strings.TrimSpace(c.Query("taskType")),
		Platform:        strings.TrimSpace(c.Query("platform")),
		Start:           startPtr,
		End:             endPtr,
		Page:            atoiQ(1, c.DefaultQuery("page", "1"), 1),
		PageSize:        atoiQ(1, c.DefaultQuery("pageSize", "20"), 20),
	}
	res, err := h.Svc.ListAlerts(c.Request.Context(), p)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	page := p.Page
	if page < 1 {
		page = 1
	}
	ps := p.PageSize
	if ps < 1 {
		ps = 20
	}
	tp := res.Total / int64(ps)
	if res.Total%int64(ps) != 0 {
		tp++
	}
	response.OK(c, gin.H{
		"list": res.List,
		"pagination": gin.H{
			"page":       page,
			"pageSize":   ps,
			"total":      res.Total,
			"totalPages": tp,
		},
	})
}

// ScanAlerts POST /task-center/alerts/scan
func (h *Handler) ScanAlerts(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "task center unavailable")
		return
	}
	sum, err := h.Svc.ScanAndGenerateTaskAlerts(c.Request.Context())
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	if h.Svc.OpLog != nil {
		msg := truncateRunes(fmt.Sprintf("scan scanned=%d gen=%d up=%d skip=%d", sum.ScannedCount, sum.GeneratedCount, sum.UpdatedCount, sum.IgnoredCount), 2000)
		_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "task_center.alert.scan",
			Resource:   "task_alert",
			ResourceID: "scan",
			Status:     "success",
			Message:    msg,
		})
		if sum.GeneratedCount > 0 {
			_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{
				Action:     "task_center.alert.generated",
				Resource:   "task_alert",
				ResourceID: "scan",
				Status:     "success",
				Message:    truncateRunes(fmt.Sprintf("generatedCount=%d", sum.GeneratedCount), 2000),
			})
		}
	}
	response.OK(c, sum)
}

// HandleAlert POST /task-center/alerts/:id/handle
func (h *Handler) HandleAlert(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "task center unavailable")
		return
	}
	raw := strings.TrimSpace(c.Param("id"))
	id, err := uuid.Parse(raw)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if err := h.Svc.HandleTaskAlert(c, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// IgnoreAlert POST /task-center/alerts/:id/ignore
func (h *Handler) IgnoreAlert(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "task center unavailable")
		return
	}
	raw := strings.TrimSpace(c.Param("id"))
	id, err := uuid.Parse(raw)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if err := h.Svc.IgnoreTaskAlert(c, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// UnmarkAlert DELETE /task-center/alerts/:id/mark
func (h *Handler) UnmarkAlert(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "task center unavailable")
		return
	}
	raw := strings.TrimSpace(c.Param("id"))
	id, err := uuid.Parse(raw)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if err := h.Svc.UnmarkTaskAlert(c, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// GenerateAlertFromFailure POST /task-center/failures/:taskType/:id/generate-alert
func (h *Handler) GenerateAlertFromFailure(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "task center unavailable")
		return
	}
	tid := strings.TrimSpace(c.Param("taskType"))
	rawID := strings.TrimSpace(c.Param("id"))
	id, err := uuid.Parse(rawID)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	al, err := h.Svc.GenerateAlertForFailure(c, tid, id)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	if h.Svc.OpLog != nil {
		_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "task_center.alert.generated",
			Resource:   "task_alert",
			ResourceID: al.ID.String(),
			Status:     "success",
			Message:    truncateRunes(fmt.Sprintf("taskType=%s sourceId=%s category=%s severity=%s", al.TaskType, al.SourceID, al.FailureCategory, al.Severity), 480),
		})
	}
	badges := h.Svc.notificationBadgeMap(c.Request.Context(), []uuid.UUID{al.ID})
	st := badges[al.ID]
	if st == "" {
		st = "none"
	}
	response.OK(c, toAlertDTO(*al, st))
}

// ListAlertNotifications GET /task-center/alert-notifications
func (h *Handler) ListAlertNotifications(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "task center unavailable")
		return
	}
	var alertIDPtr *uuid.UUID
	if raw := strings.TrimSpace(c.Query("alertId")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "invalid alertId")
			return
		}
		alertIDPtr = &id
	}
	startPtr, err := parseRFC3339Ptr(c.Query("start"))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid start time (RFC3339)")
		return
	}
	endPtr, err := parseRFC3339Ptr(c.Query("end"))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid end time (RFC3339)")
		return
	}
	p := ListAlertNotificationsParams{
		AlertID: alertIDPtr,
		Channel: strings.TrimSpace(c.Query("channel")),
		Status:  strings.TrimSpace(c.Query("status")),
		Start:   startPtr,
		End:     endPtr,
		Page:    atoiQ(1, c.DefaultQuery("page", "1"), 1),
		PageSz:  atoiQ(1, c.DefaultQuery("pageSize", "20"), 20),
	}
	res, err := h.Svc.ListAlertNotifications(c.Request.Context(), p)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, res)
}

type notifyAlertBody struct {
	Channels []string `json:"channels"`
}

// NotifyAlert POST /task-center/alerts/:id/notify
func (h *Handler) NotifyAlert(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "task center unavailable")
		return
	}
	raw := strings.TrimSpace(c.Param("id"))
	id, err := uuid.Parse(raw)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body notifyAlertBody
	_ = c.ShouldBindJSON(&body)
	if err := h.Svc.NotifyTaskAlertManual(c.Request.Context(), c, id, body.Channels); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	if h.Svc.OpLog != nil {
		_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "task_center.alert.notify.manual",
			Resource:   "task_alert",
			ResourceID: id.String(),
			Status:     "success",
			Message:    truncateRunes(fmt.Sprintf("channels=%v", body.Channels), 240),
		})
	}
	response.OK(c, gin.H{"ok": true})
}
