package order

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// GetShipments GET /orders/:id/shipments
func (h *Handler) GetShipments(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "orders unavailable")
		return
	}
	if h.denyRead(c) {
		return
	}
	oid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid id")
		return
	}
	rows, err := h.Svc.ListShipments(c, oid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"list": rows})
}

// GetShipmentEvents GET /orders/:id/shipments/:shipmentId/events
func (h *Handler) GetShipmentEvents(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "orders unavailable")
		return
	}
	if h.denyRead(c) {
		return
	}
	oid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid id")
		return
	}
	sid, err := uuid.Parse(strings.TrimSpace(c.Param("shipmentId")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid shipmentId")
		return
	}
	shipment, events, provider, err := h.Svc.ListShipmentEvents(c, oid, sid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"shipment": shipment, "events": events, "provider": provider})
}

// PostShipmentEvent POST /orders/:id/shipments/:shipmentId/events
func (h *Handler) PostShipmentEvent(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "orders unavailable")
		return
	}
	if h.denyWrite(c) {
		return
	}
	oid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid id")
		return
	}
	sid, err := uuid.Parse(strings.TrimSpace(c.Param("shipmentId")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid shipmentId")
		return
	}
	var body ShipmentEventInput
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.AppendShipmentEvent(c, oid, sid, body, adminUUID(c))
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "not found")
		case errors.Is(err, ErrShipmentEventConflict), errors.Is(err, ErrShipmentEventTransition), errors.Is(err, ErrShipmentEventImmutable):
			response.Fail(c, http.StatusConflict, response.CodeBadRequest, err.Error())
		default:
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		}
		return
	}
	response.OK(c, out)
}
