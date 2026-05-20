package collectruleai

import (
	"errors"
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

func (h *Handler) Generate(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "collect rule ai unavailable")
		return
	}
	var body GenerateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.GenerateCollectRuleWithAI(c, body, adminUUID(c))
	if err != nil {
		if pe, ok := IsPlatformBlock(err); ok {
			response.Fail(c, 400, response.CodeCustomCollectProviderConflict, pe.Message)
			return
		}
		if errors.Is(err, ErrAIRuleInvalid) {
			response.Fail(c, 422, response.CodeAIRuleInvalid, err.Error())
			return
		}
		reason := err.Error()
		if strings.Contains(strings.ToLower(reason), "collector rejected") {
			response.Fail(c, 422, response.CodeBadRequest, reason)
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, reason)
		return
	}
	response.OK(c, out)
}

func (h *Handler) GenerateAndSave(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "collect rule ai unavailable")
		return
	}
	var body GenerateAndSaveBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.GenerateAndSave(c, body, adminUUID(c))
	if err != nil {
		if pe, ok := IsPlatformBlock(err); ok {
			response.Fail(c, 400, response.CodeCustomCollectProviderConflict, pe.Message)
			return
		}
		if errors.Is(err, ErrAIRuleInvalid) {
			response.Fail(c, 422, response.CodeAIRuleInvalid, err.Error())
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}
