package logistics

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/httpapi"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

const maxJSONBody = int64(32 << 10)

type Handler struct {
	Svc   *Service
	OpLog *operationlog.Service
}

func (h *Handler) authorize(c *gin.Context, permission string) (int64, *adminperm.Principal, bool) {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, 500, response.CodeInternalError, "logistics unavailable")
		return 0, nil, false
	}
	p, err := adminperm.LoadPrincipal(c, h.Svc.DB)
	if err != nil || p == nil || !p.Can(permission) || (permission == adminperm.PermLogisticsManage && p.IsReadonly()) {
		response.Fail(c, 403, response.CodeForbidden, "logistics permission denied")
		return 0, nil, false
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, 403, response.CodeForbidden, "tenant context missing")
		return 0, nil, false
	}
	return tid, p, true
}

func actorID(p *adminperm.Principal) *uuid.UUID {
	if p == nil || p.UserID == uuid.Nil {
		return nil
	}
	id := p.UserID
	return &id
}
func parseID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil || id == uuid.Nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid logistics id")
		return uuid.Nil, false
	}
	return id, true
}
func bind(c *gin.Context, dst any) bool {
	if err := httpapi.BindStrictJSON(c, dst, maxJSONBody); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return false
	}
	return true
}
func handle(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalid):
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrNotFound):
		response.Fail(c, 404, response.CodeNotFound, err.Error())
	case errors.Is(err, ErrConflict):
		response.Fail(c, 409, response.CodeBadRequest, err.Error())
	default:
		response.HandleError(c, err)
	}
}

func (h *Handler) ListChannels(c *gin.Context) {
	tid, _, ok := h.authorize(c, adminperm.PermLogisticsView)
	if !ok {
		return
	}
	rows, err := h.Svc.ListChannels(c, tid)
	if err != nil {
		handle(c, err)
		return
	}
	response.OK(c, gin.H{"list": rows})
}
func (h *Handler) CreateChannel(c *gin.Context) {
	tid, p, ok := h.authorize(c, adminperm.PermLogisticsManage)
	if !ok {
		return
	}
	var in CreateChannelInput
	if !bind(c, &in) {
		return
	}
	row, err := h.Svc.CreateChannel(c, tid, actorID(p), in)
	if err != nil {
		handle(c, err)
		return
	}
	h.log(c, tid, "logistics.channel.create", "shipping_channel", row.ID)
	response.OK(c, row)
}
func (h *Handler) UpdateChannel(c *gin.Context) {
	tid, _, ok := h.authorize(c, adminperm.PermLogisticsManage)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var in UpdateChannelInput
	if !bind(c, &in) {
		return
	}
	row, err := h.Svc.UpdateChannel(c, tid, id, in)
	if err != nil {
		handle(c, err)
		return
	}
	h.log(c, tid, "logistics.channel.update", "shipping_channel", row.ID)
	response.OK(c, row)
}
func (h *Handler) ListRates(c *gin.Context) {
	tid, _, ok := h.authorize(c, adminperm.PermLogisticsView)
	if !ok {
		return
	}
	rows, err := h.Svc.ListRates(c, tid)
	if err != nil {
		handle(c, err)
		return
	}
	response.OK(c, gin.H{"list": rows})
}
func (h *Handler) CreateRate(c *gin.Context) {
	tid, p, ok := h.authorize(c, adminperm.PermLogisticsManage)
	if !ok {
		return
	}
	var in CreateRateInput
	if !bind(c, &in) {
		return
	}
	row, err := h.Svc.CreateRate(c, tid, actorID(p), in)
	if err != nil {
		handle(c, err)
		return
	}
	h.log(c, tid, "logistics.rate.create", "shipping_rate_template", row.ID)
	response.OK(c, row)
}
func (h *Handler) UpdateRate(c *gin.Context) {
	tid, _, ok := h.authorize(c, adminperm.PermLogisticsManage)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var in UpdateRateInput
	if !bind(c, &in) {
		return
	}
	row, err := h.Svc.UpdateRate(c, tid, id, in)
	if err != nil {
		handle(c, err)
		return
	}
	h.log(c, tid, "logistics.rate.update", "shipping_rate_template", row.ID)
	response.OK(c, row)
}

func (h *Handler) log(c *gin.Context, tid int64, action, resource string, id uuid.UUID) {
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{TenantID: tid, Action: action, Resource: resource, ResourceID: id.String(), Permission: adminperm.PermLogisticsManage, Status: "success"})
	}
}
