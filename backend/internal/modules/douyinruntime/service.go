package douyinruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

const groupKey = "platform_douyin_shop"

// Service manages Douyin platform runtime status.
type Service struct {
	Settings *settings.Service
	OpLog    *operationlog.Service
}

// RuntimeStatusDTO is the API response for runtime status.
type RuntimeStatusDTO struct {
	Status    string  `json:"status"`
	Reason    string  `json:"reason,omitempty"`
	ChangedAt *string `json:"changedAt,omitempty"`
	Message   string  `json:"message,omitempty"`
}

// ChangeBody is the request to change runtime status.
type ChangeBody struct {
	Reason string `json:"reason"`
}

// GetRuntimeStatus returns current Douyin runtime status from settings.
func (s *Service) GetRuntimeStatus(ctx context.Context) (*RuntimeStatusDTO, error) {
	if s == nil || s.Settings == nil {
		return nil, fmt.Errorf("douyinruntime: misconfigured")
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, groupKey)
	if err != nil {
		return nil, err
	}
	rt, _ := platformdouyin.RuntimeStateFromMergedMap(m)
	out := &RuntimeStatusDTO{
		Status:  rt.Status,
		Reason:  rt.Reason,
		Message: statusLabel(rt.Status),
	}
	if rt.ChangedAt != nil {
		ts := rt.ChangedAt.UTC().Format(time.RFC3339)
		out.ChangedAt = &ts
	}
	return out, nil
}

// Pause sets platform to paused.
func (s *Service) Pause(c *gin.Context, adminID *uuid.UUID, body ChangeBody) (*RuntimeStatusDTO, error) {
	return s.setStatus(c, adminID, platformdouyin.RuntimePaused, "douyin.platform.pause", body.Reason)
}

// Resume sets platform to normal.
func (s *Service) Resume(c *gin.Context, adminID *uuid.UUID, body ChangeBody) (*RuntimeStatusDTO, error) {
	return s.setStatus(c, adminID, platformdouyin.RuntimeNormal, "douyin.platform.resume", body.Reason)
}

// EmergencyDisable sets platform to emergency_disabled.
func (s *Service) EmergencyDisable(c *gin.Context, adminID *uuid.UUID, body ChangeBody) (*RuntimeStatusDTO, error) {
	return s.setStatus(c, adminID, platformdouyin.RuntimeEmergencyDisabled, "douyin.platform.emergency_disable", body.Reason)
}

func (s *Service) setStatus(c *gin.Context, adminID *uuid.UUID, status, action, reason string) (*RuntimeStatusDTO, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, fmt.Errorf("reason is required")
	}
	if s == nil || s.Settings == nil {
		return nil, fmt.Errorf("douyinruntime: misconfigured")
	}
	ctx := c.Request.Context()
	now := time.Now().UTC().Format(time.RFC3339)
	items := []settings.PutItem{
		{TenantID: 0, GroupKey: groupKey, ItemKey: "platform_runtime_status", ItemValue: status, ValueType: "string"},
		{TenantID: 0, GroupKey: groupKey, ItemKey: "platform_runtime_status_reason", ItemValue: reason, ValueType: "string"},
		{TenantID: 0, GroupKey: groupKey, ItemKey: "platform_runtime_status_changed_at", ItemValue: now, ValueType: "string"},
	}
	if err := s.Settings.PutBulk(ctx, items); err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      action,
			Resource:    groupKey,
			ResourceID:  "runtime_status",
			Status:      "success",
			Message:     fmt.Sprintf("status=%s reason=%s changedAt=%s", status, reason, now),
		})
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "douyin.platform.runtime_status.changed",
			Resource:    groupKey,
			ResourceID:  status,
			Status:      "success",
			Message:     fmt.Sprintf("from_change action=%s reason=%s", action, reason),
		})
	}
	return s.GetRuntimeStatus(ctx)
}

func statusLabel(status string) string {
	switch strings.TrimSpace(status) {
	case platformdouyin.RuntimePaused:
		return "已暂停"
	case platformdouyin.RuntimeEmergencyDisabled:
		return "紧急停用"
	default:
		return "正常运行"
	}
}

// Handler exposes HTTP handlers.
type Handler struct {
	Svc *Service
}

func (h *Handler) Get(c *gin.Context) {
	out, err := h.Svc.GetRuntimeStatus(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"code": 50000, "message": err.Error(), "data": nil})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": out})
}

func (h *Handler) Pause(c *gin.Context) {
	h.change(c, "pause")
}

func (h *Handler) Resume(c *gin.Context) {
	h.change(c, "resume")
}

func (h *Handler) EmergencyDisable(c *gin.Context) {
	h.change(c, "emergency_disable")
}

func (h *Handler) change(c *gin.Context, action string) {
	var body ChangeBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"code": 40001, "message": "invalid body", "data": nil})
		return
	}
	adminID := parseAdminID(c)
	var out *RuntimeStatusDTO
	var err error
	switch action {
	case "pause":
		out, err = h.Svc.Pause(c, adminID, body)
	case "resume":
		out, err = h.Svc.Resume(c, adminID, body)
	case "emergency_disable":
		out, err = h.Svc.EmergencyDisable(c, adminID, body)
	default:
		c.JSON(400, gin.H{"code": 40001, "message": "unknown action", "data": nil})
		return
	}
	if err != nil {
		c.JSON(400, gin.H{"code": 40001, "message": err.Error(), "data": nil})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": out})
}

func parseAdminID(c *gin.Context) *uuid.UUID {
	if c == nil {
		return nil
	}
	if v, ok := c.Get("adminUserId"); ok {
		if s, ok := v.(string); ok {
			if u, err := uuid.Parse(strings.TrimSpace(s)); err == nil {
				return &u
			}
		}
	}
	return nil
}

// MarshalForAudit returns sanitized JSON for logs.
func MarshalForAudit(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
