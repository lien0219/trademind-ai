package collectbrowserprofile

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

type Handler struct {
	Svc *Service
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

func atoiQP(c *gin.Context, key string, def int) int {
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

func (h *Handler) List(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "browser profiles unavailable")
		return
	}
	q := ListQuery{
		Page:     atoiQP(c, "page", 1),
		PageSize: atoiQP(c, "pageSize", 20),
		Domain:   c.Query("domain"),
		Provider: c.Query("provider"),
		Status:   c.Query("status"),
	}
	list, total, err := h.Svc.List(c.Request.Context(), q)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	ps := q.PageSize
	if ps < 1 {
		ps = 20
	}
	totalPages := int(total) / ps
	if int(total)%ps != 0 {
		totalPages++
	}
	response.OK(c, gin.H{
		"list": list,
		"pagination": gin.H{
			"page":       q.Page,
			"pageSize":   ps,
			"total":      total,
			"totalPages": totalPages,
		},
	})
}

func (h *Handler) Create(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "browser profiles unavailable")
		return
	}
	var body CreateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.Create(c, body, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

func (h *Handler) OpenLogin(c *gin.Context) {
	h.profileAction(c, "open-login", func(id uuid.UUID, body URLBody) (any, error) {
		return h.Svc.OpenLogin(c, id, body, adminUUID(c))
	})
}

func (h *Handler) Check(c *gin.Context) {
	h.profileAction(c, "check", func(id uuid.UUID, body URLBody) (any, error) {
		return h.Svc.Check(c, id, body, adminUUID(c))
	})
}

func (h *Handler) Delete(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "browser profiles unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if err := h.Svc.Disable(c, id, adminUUID(c)); err != nil {
		if errors.Is(err, ErrProfileNotFound) {
			response.Fail(c, 404, response.CodeNotFound, err.Error())
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *Handler) profileAction(
	c *gin.Context,
	_ string,
	fn func(id uuid.UUID, body URLBody) (any, error),
) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "browser profiles unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body URLBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := fn(id, body)
	if err != nil {
		msg := err.Error()
		if errors.Is(err, ErrProfileNotFound) {
			response.Fail(c, 404, response.CodeNotFound, msg)
			return
		}
		if errors.Is(err, ErrProfileDomainMismatch) {
			response.Fail(c, 400, response.CodeBadRequest, msg)
			return
		}
		if strings.Contains(msg, "HEADED_BROWSER_REQUIRED") {
			response.Fail(c, 422, response.CodeBadRequest, msg)
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, msg)
		return
	}
	response.OK(c, out)
}
