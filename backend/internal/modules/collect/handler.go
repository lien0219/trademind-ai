package collect

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Handler exposes collect task HTTP API.
type Handler struct {
	Svc *Service
}

func collectAdminUUID(c *gin.Context) *uuid.UUID {
	if v, ok := c.Get(ctxkey.AdminID); ok {
		if s, ok := v.(string); ok {
			if u, err := uuid.Parse(strings.TrimSpace(s)); err == nil {
				return &u
			}
		}
	}
	return nil
}

func atoiCollectQP(c *gin.Context, key string, def int) int {
	s := strings.TrimSpace(c.Query(key))
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	return n
}

// ListProviders GET /api/v1/collect/providers
func (h *Handler) ListProviders(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "collect unavailable")
		return
	}
	out := h.Svc.ResolveCollectProviders(c.Request.Context())
	response.OK(c, out)
}

// Create POST /api/v1/collect/tasks
func (h *Handler) Create(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "collect unavailable")
		return
	}
	var body CreateTaskBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.CreateTaskAsync(c, body, collectAdminUUID(c))
	if err != nil {
		if errors.Is(err, ErrRedisQueueUnavailable) || errors.Is(err, ErrCollectQueueDisabled) {
			response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, err.Error())
			return
		}
		var conflict *CustomCollectProviderConflict
		if errors.As(err, &conflict) && conflict != nil {
			response.JSON(c, http.StatusBadRequest, response.CodeCustomCollectProviderConflict, conflict.Message, gin.H{
				"errorCode":           "CUSTOM_COLLECT_PROVIDER_CONFLICT",
				"recommendedProvider": conflict.RecommendedProvider,
				"message":             conflict.Message,
			})
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// List GET /api/v1/collect/tasks
func (h *Handler) List(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "collect unavailable")
		return
	}
	q := ListQuery{
		Page:     atoiCollectQP(c, "page", 1),
		PageSize: atoiCollectQP(c, "pageSize", 20),
		Status:   c.Query("status"),
		Source:   c.Query("source"),
		Keyword:  c.Query("keyword"),
		BatchID:  c.Query("batchId"),
	}
	res, err := h.Svc.List(c, q)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{
		"list": res.Items,
		"pagination": gin.H{
			"page":       res.Page,
			"pageSize":   res.PageSize,
			"total":      res.Total,
			"totalPages": res.TotalPages,
		},
	})
}

// ListTaskEvents GET /api/v1/collect/tasks/:id/events
func (h *Handler) ListTaskEvents(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "collect unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	q := TaskEventsListQuery{
		Page:     atoiCollectQP(c, "page", 1),
		PageSize: atoiCollectQP(c, "pageSize", 50),
	}
	res, err := h.Svc.ListTaskEvents(c.Request.Context(), id, q)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{
		"list": res.Items,
		"pagination": gin.H{
			"page":       res.Page,
			"pageSize":   res.PageSize,
			"total":      res.Total,
			"totalPages": res.TotalPages,
		},
	})
}

// Get GET /api/v1/collect/tasks/:id
func (h *Handler) Get(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "collect unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	out, err := h.Svc.GetDTO(c, id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, out)
}

// Monitor GET /api/v1/collect/monitor
func (h *Handler) Monitor(c *gin.Context) {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, 500, response.CodeInternalError, "collect unavailable")
		return
	}
	data, err := h.Svc.GetCollectMonitor(c.Request.Context())
	if err != nil {
		response.Fail(c, 500, response.CodeInternalError, err.Error())
		return
	}
	response.OK(c, data)
}

// Retry POST /api/v1/collect/tasks/:id/retry
func (h *Handler) Retry(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "collect unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	out, err := h.Svc.RetryAsync(c, id, collectAdminUUID(c))
	if err != nil {
		if errors.Is(err, ErrRedisQueueUnavailable) || errors.Is(err, ErrCollectQueueDisabled) {
			response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, err.Error())
			return
		}
		if err == gorm.ErrRecordNotFound {
			response.Fail(c, 404, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// Get1688AuthStatus GET /api/v1/collector/providers/1688/auth-status
func (h *Handler) Get1688AuthStatus(c *gin.Context) {
	if h == nil || h.Svc == nil || h.Svc.Client == nil {
		response.Fail(c, 500, response.CodeInternalError, "collect unavailable")
		return
	}
	out, err := h.Svc.Client.Get1688AuthStatus(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusBadGateway, response.CodeInternalError, err.Error())
		return
	}
	response.OK(c, out)
}

// Open1688LoginBrowser POST /api/v1/collector/providers/1688/open-login-browser
func (h *Handler) Open1688LoginBrowser(c *gin.Context) {
	if h == nil || h.Svc == nil || h.Svc.Client == nil {
		response.Fail(c, 500, response.CodeInternalError, "collect unavailable")
		return
	}
	out, err := h.Svc.Client.Open1688LoginBrowser(c.Request.Context())
	if err != nil {
		response.Fail(c, http.StatusBadGateway, response.CodeInternalError, err.Error())
		return
	}
	response.OK(c, out)
}

// GetPinduoduoAuthStatus GET /api/v1/collector/providers/pinduoduo/auth-status
func (h *Handler) GetPinduoduoAuthStatus(c *gin.Context) {
	if h == nil || h.Svc == nil || h.Svc.Client == nil {
		response.Fail(c, 500, response.CodeInternalError, "collect unavailable")
		return
	}
	contextURL, settingsTestURL := h.Svc.ResolvePinduoduoAuthCheckInputs(
		c.Request.Context(),
		strings.TrimSpace(c.Query("url")),
	)
	out, err := h.Svc.Client.CheckPinduoduoLogin(c.Request.Context(), contextURL, settingsTestURL)
	if err != nil {
		response.Fail(c, http.StatusBadGateway, response.CodeInternalError, err.Error())
		return
	}
	response.OK(c, out)
}

// CheckPinduoduoLogin POST /api/v1/collect/providers/pinduoduo/check-login
func (h *Handler) CheckPinduoduoLogin(c *gin.Context) {
	if h == nil || h.Svc == nil || h.Svc.Client == nil {
		response.Fail(c, 500, response.CodeInternalError, "collect unavailable")
		return
	}
	var body PinduoduoCheckLoginBody
	_ = c.ShouldBindJSON(&body)
	contextURL, settingsTestURL := h.Svc.ResolvePinduoduoAuthCheckInputs(c.Request.Context(), body.URL)
	if t := strings.TrimSpace(body.TestURL); t != "" {
		settingsTestURL = t
	}
	out, err := h.Svc.Client.CheckPinduoduoLogin(c.Request.Context(), contextURL, settingsTestURL)
	if err != nil {
		response.Fail(c, http.StatusBadGateway, response.CodeInternalError, err.Error())
		return
	}
	response.OK(c, out)
}

// OpenPinduoduoLoginBrowser POST /api/v1/collector/providers/pinduoduo/open-login-browser
func (h *Handler) OpenPinduoduoLoginBrowser(c *gin.Context) {
	if h == nil || h.Svc == nil || h.Svc.Client == nil {
		response.Fail(c, 500, response.CodeInternalError, "collect unavailable")
		return
	}
	var body PinduoduoOpenLoginBody
	_ = c.ShouldBindJSON(&body)
	loginURL := strings.TrimSpace(body.URL)
	out, err := h.Svc.Client.OpenPinduoduoLoginBrowser(c.Request.Context(), loginURL)
	if err != nil {
		response.Fail(c, http.StatusBadGateway, response.CodeInternalError, err.Error())
		return
	}
	response.OK(c, out)
}
