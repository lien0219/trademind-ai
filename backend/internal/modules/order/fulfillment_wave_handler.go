package order

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

const maxFulfillmentWaveJSONBody = int64(256 << 10)

func (h *Handler) fulfillmentWavePrincipal(c *gin.Context, write bool) (int64, *adminperm.Principal, bool) {
	if h == nil || h.Svc == nil || h.Svc.DB == nil || h.Inv == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "fulfillment waves unavailable")
		return 0, nil, false
	}
	if write {
		if h.denyWrite(c) {
			return 0, nil, false
		}
	} else if h.denyRead(c) {
		return 0, nil, false
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant context missing")
		return 0, nil, false
	}
	principal, err := adminperm.LoadPrincipal(c, h.Svc.DB)
	if err != nil || principal == nil {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "order permission denied")
		return 0, nil, false
	}
	return tenantID, principal, true
}

func fulfillmentWaveID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param(name)))
	if err != nil || id == uuid.Nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func bindFulfillmentWaveJSON(c *gin.Context, out any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFulfillmentWaveJSONBody)
	if err := c.ShouldBindJSON(out); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return false
	}
	return true
}

func handleFulfillmentWaveError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrFulfillmentWaveNotFound), errors.Is(err, gorm.ErrRecordNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "not found")
	case errors.Is(err, ErrFulfillmentWaveInvalidInput):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrFulfillmentWaveStorePermission):
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "store operation permission denied")
	case errors.Is(err, ErrFulfillmentWaveState), errors.Is(err, ErrFulfillmentWaveRevision),
		errors.Is(err, ErrFulfillmentWaveIdempotency), errors.Is(err, ErrFulfillmentWaveOrderUnavailable),
		errors.Is(err, ErrFulfillmentWaveOrderAssigned), errors.Is(err, ErrFulfillmentWaveReservation),
		errors.Is(err, ErrFulfillmentWavePickIncomplete), errors.Is(err, ErrFulfillmentWavePackingIncomplete),
		errors.Is(err, ErrFulfillmentWaveScanMismatch), errors.Is(err, ErrFulfillmentWavePackVerificationRequired),
		errors.Is(err, ErrFulfillmentWavePackageMismatch), errors.Is(err, ErrFulfillmentWaveOrderScanMismatch),
		errors.Is(err, ErrFulfillmentWavePackScanMismatch),
		errors.Is(err, ErrFulfillmentWaveCompleting), errors.Is(err, ErrFulfillmentWaveRequired),
		errors.Is(err, idempotency.ErrKeyConflict), errors.Is(err, inventory.ErrOrderInventoryState),
		errors.Is(err, inventory.ErrInsufficientSKUStock):
		response.Fail(c, http.StatusConflict, response.CodeBadRequest, err.Error())
	default:
		response.HandleError(c, err)
	}
}

// ListFulfillmentWaves GET /fulfillment-waves.
func (h *Handler) ListFulfillmentWaves(c *gin.Context) {
	tenantID, principal, ok := h.fulfillmentWavePrincipal(c, false)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page", "1")))
	pageSize, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("pageSize", "20")))
	query := FulfillmentWaveListQuery{Page: page, PageSize: pageSize, Keyword: c.Query("keyword"), Status: c.Query("status")}
	if raw := strings.TrimSpace(c.Query("warehouseId")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid warehouseId")
			return
		}
		query.WarehouseID = &id
	}
	out, err := h.Svc.ListFulfillmentWaves(c.Request.Context(), tenantID, principal, query)
	if err != nil {
		handleFulfillmentWaveError(c, err)
		return
	}
	response.OK(c, out)
}

// GetFulfillmentWave GET /fulfillment-waves/:id.
func (h *Handler) GetFulfillmentWave(c *gin.Context) {
	tenantID, principal, ok := h.fulfillmentWavePrincipal(c, false)
	if !ok {
		return
	}
	id, ok := fulfillmentWaveID(c, "id")
	if !ok {
		return
	}
	out, err := h.Svc.GetFulfillmentWave(c.Request.Context(), tenantID, principal, id)
	if err != nil {
		handleFulfillmentWaveError(c, err)
		return
	}
	response.OK(c, out)
}

// PostFulfillmentWave POST /fulfillment-waves.
func (h *Handler) PostFulfillmentWave(c *gin.Context) {
	tenantID, principal, ok := h.fulfillmentWavePrincipal(c, true)
	if !ok {
		return
	}
	var body CreateFulfillmentWaveInput
	if !bindFulfillmentWaveJSON(c, &body) {
		return
	}
	out, err := h.Svc.CreateFulfillmentWave(c.Request.Context(), tenantID, principal, adminUUID(c), body)
	if err != nil {
		handleFulfillmentWaveError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) fulfillmentWaveRevisionAction(c *gin.Context, action func(int64, *adminperm.Principal, *uuid.UUID, uuid.UUID, FulfillmentWaveRevisionInput) (*FulfillmentWave, error)) {
	tenantID, principal, ok := h.fulfillmentWavePrincipal(c, true)
	if !ok {
		return
	}
	id, ok := fulfillmentWaveID(c, "id")
	if !ok {
		return
	}
	if _, err := h.Svc.GetFulfillmentWave(c.Request.Context(), tenantID, principal, id); err != nil {
		handleFulfillmentWaveError(c, err)
		return
	}
	var body FulfillmentWaveRevisionInput
	if !bindFulfillmentWaveJSON(c, &body) {
		return
	}
	out, err := action(tenantID, principal, adminUUID(c), id, body)
	if err != nil {
		handleFulfillmentWaveError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) PostStartFulfillmentWave(c *gin.Context) {
	h.fulfillmentWaveRevisionAction(c, func(tenantID int64, principal *adminperm.Principal, actor *uuid.UUID, id uuid.UUID, body FulfillmentWaveRevisionInput) (*FulfillmentWave, error) {
		return h.Svc.StartFulfillmentWave(c.Request.Context(), tenantID, principal, actor, id, body)
	})
}

func (h *Handler) PostCancelFulfillmentWave(c *gin.Context) {
	h.fulfillmentWaveRevisionAction(c, func(tenantID int64, principal *adminperm.Principal, actor *uuid.UUID, id uuid.UUID, body FulfillmentWaveRevisionInput) (*FulfillmentWave, error) {
		return h.Svc.CancelFulfillmentWave(c.Request.Context(), tenantID, principal, actor, id, body)
	})
}

func (h *Handler) PostRecordFulfillmentWavePick(c *gin.Context) {
	tenantID, principal, ok := h.fulfillmentWavePrincipal(c, true)
	if !ok {
		return
	}
	id, ok := fulfillmentWaveID(c, "id")
	if !ok {
		return
	}
	if _, err := h.Svc.GetFulfillmentWave(c.Request.Context(), tenantID, principal, id); err != nil {
		handleFulfillmentWaveError(c, err)
		return
	}
	var body RecordFulfillmentWavePickInput
	if !bindFulfillmentWaveJSON(c, &body) {
		return
	}
	out, err := h.Svc.RecordFulfillmentWavePick(c.Request.Context(), tenantID, principal, adminUUID(c), id, body)
	if err != nil {
		handleFulfillmentWaveError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) PostPackFulfillmentWaveOrder(c *gin.Context) {
	tenantID, principal, ok := h.fulfillmentWavePrincipal(c, true)
	if !ok {
		return
	}
	waveID, ok := fulfillmentWaveID(c, "id")
	if !ok {
		return
	}
	orderID, ok := fulfillmentWaveID(c, "orderId")
	if !ok {
		return
	}
	if _, err := h.Svc.GetFulfillmentWave(c.Request.Context(), tenantID, principal, waveID); err != nil {
		handleFulfillmentWaveError(c, err)
		return
	}
	var body PackFulfillmentWaveOrderInput
	if !bindFulfillmentWaveJSON(c, &body) {
		return
	}
	out, err := h.Svc.PackFulfillmentWaveOrder(c.Request.Context(), tenantID, principal, adminUUID(c), waveID, orderID, body)
	if err != nil {
		handleFulfillmentWaveError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) PostVerifyFulfillmentWavePack(c *gin.Context) {
	tenantID, principal, ok := h.fulfillmentWavePrincipal(c, true)
	if !ok {
		return
	}
	waveID, ok := fulfillmentWaveID(c, "id")
	if !ok {
		return
	}
	orderID, ok := fulfillmentWaveID(c, "orderId")
	if !ok {
		return
	}
	var body VerifyFulfillmentWavePackInput
	if !bindFulfillmentWaveJSON(c, &body) {
		return
	}
	out, err := h.Svc.VerifyFulfillmentWavePack(c.Request.Context(), tenantID, principal, adminUUID(c), waveID, orderID, body)
	if err != nil {
		handleFulfillmentWaveError(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) PostCompleteFulfillmentWave(c *gin.Context) {
	tenantID, principal, ok := h.fulfillmentWavePrincipal(c, true)
	if !ok {
		return
	}
	id, ok := fulfillmentWaveID(c, "id")
	if !ok {
		return
	}
	var body FulfillmentWaveRevisionInput
	if !bindFulfillmentWaveJSON(c, &body) {
		return
	}
	out, err := h.Svc.CompleteFulfillmentWave(c, h.Inv, tenantID, principal, adminUUID(c), id, body)
	if err != nil {
		handleFulfillmentWaveError(c, err)
		return
	}
	response.OK(c, out)
}
