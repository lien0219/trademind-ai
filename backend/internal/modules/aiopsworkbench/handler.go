package aiopsworkbench

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// Handler serves AI operation workbench endpoints.
type Handler struct {
	Svc *Service
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

func atoiQP(raw string, def int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return def
	}
	return n
}

func parseQuery(c *gin.Context) (Query, error) {
	startPtr, err := parseRFC3339Ptr(c.Query("start"))
	if err != nil {
		return Query{}, err
	}
	endPtr, err := parseRFC3339Ptr(c.Query("end"))
	if err != nil {
		return Query{}, err
	}
	return Query{
		Type:     strings.TrimSpace(c.Query("type")),
		Priority: strings.TrimSpace(c.Query("priority")),
		Platform: strings.TrimSpace(c.Query("platform")),
		ShopID:   strings.TrimSpace(c.Query("shopId")),
		Keyword:  strings.TrimSpace(c.Query("keyword")),
		Status:   strings.TrimSpace(c.Query("status")),
		Start:    startPtr,
		End:      endPtr,
		Page:     atoiQP(c.DefaultQuery("page", "1"), 1),
		PageSize: atoiQP(c.DefaultQuery("pageSize", "50"), defaultPageSize),
	}, nil
}

// Summary GET /api/v1/ai/operation-workbench/summary
func (h *Handler) Summary(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "工作台不可用")
		return
	}
	q, err := parseQuery(c)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid time range (RFC3339)")
		return
	}
	sum, err := h.Svc.GetSummary(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, SummaryResponse{Summary: sum})
}

// ListTodos GET /api/v1/ai/operation-workbench/todos
func (h *Handler) ListTodos(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "工作台不可用")
		return
	}
	q, err := parseQuery(c)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid time range (RFC3339)")
		return
	}
	out, err := h.Svc.ListTodos(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// GetTodo GET /api/v1/ai/operation-workbench/todos/:id
func (h *Handler) GetTodo(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "工作台不可用")
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	q, err := parseQuery(c)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid time range (RFC3339)")
		return
	}
	item, err := h.Svc.GetTodo(c.Request.Context(), id, q)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "待办不存在或已处理")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, item)
}

// RefreshTodos POST /api/v1/ai/operation-workbench/todos/refresh
func (h *Handler) RefreshTodos(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "工作台不可用")
		return
	}
	q, err := parseQuery(c)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid time range (RFC3339)")
		return
	}
	out, err := h.Svc.RefreshTodos(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}
