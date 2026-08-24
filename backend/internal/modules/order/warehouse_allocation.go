package order

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

const orderWarehouseAllocationScope = "order-warehouse-allocation"

var (
	ErrWarehouseAllocationIdempotencyRequired = errors.New("warehouse allocation idempotencyKey is required")
	ErrWarehouseAllocationInputInvalid        = errors.New("warehouseId and a 64-character expectedRevision are required")
	ErrWarehouseAllocationInProgress          = errors.New("WAREHOUSE_ALLOCATION_IN_PROGRESS")
)

type WarehouseAllocationListRow struct {
	ListOrderRow
	AllocationStatus         string                               `json:"allocationStatus"`
	WarehouseID              *uuid.UUID                           `json:"warehouseId,omitempty"`
	WarehouseCode            string                               `json:"warehouseCode,omitempty"`
	WarehouseName            string                               `json:"warehouseName,omitempty"`
	RecommendedWarehouseID   *uuid.UUID                           `json:"recommendedWarehouseId,omitempty"`
	RecommendedWarehouseCode string                               `json:"recommendedWarehouseCode,omitempty"`
	RecommendedWarehouseName string                               `json:"recommendedWarehouseName,omitempty"`
	CandidateCount           int                                  `json:"candidateCount"`
	EligibleCandidateCount   int                                  `json:"eligibleCandidateCount"`
	Blocks                   []inventory.WarehouseAllocationBlock `json:"blocks"`
}

type WarehouseAllocationListResult struct {
	Items      []WarehouseAllocationListRow
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
}

type ConfirmWarehouseAllocationInput struct {
	WarehouseID      uuid.UUID `json:"warehouseId"`
	ExpectedRevision string    `json:"expectedRevision"`
	IdempotencyKey   string    `json:"idempotencyKey"`
}

type ConfirmWarehouseAllocationResult struct {
	Allocation       inventory.WarehouseAllocationEvaluation `json:"allocation"`
	InventoryReserve *inventory.DeductionSummary             `json:"inventoryReserve,omitempty"`
}

func allocationOwner(actor *uuid.UUID) string {
	owner := "order-warehouse-allocation"
	if actor != nil && *actor != uuid.Nil {
		owner += ":" + actor.String()
	}
	return owner + ":" + uuid.NewString()
}

func allocationRequestHash(orderID uuid.UUID, in ConfirmWarehouseAllocationInput) string {
	payload, _ := json.Marshal(struct {
		OrderID          uuid.UUID `json:"orderId"`
		WarehouseID      uuid.UUID `json:"warehouseId"`
		ExpectedRevision string    `json:"expectedRevision"`
	}{OrderID: orderID, WarehouseID: in.WarehouseID, ExpectedRevision: strings.TrimSpace(in.ExpectedRevision)})
	return idempotency.HashRequest(payload)
}

func recommendedAllocationCandidate(evaluation inventory.WarehouseAllocationEvaluation) *inventory.WarehouseAllocationCandidate {
	if evaluation.RecommendedWarehouseID == nil {
		return nil
	}
	for i := range evaluation.Candidates {
		if evaluation.Candidates[i].WarehouseID == *evaluation.RecommendedWarehouseID {
			return &evaluation.Candidates[i]
		}
	}
	return nil
}

func (s *Service) ListWarehouseAllocations(c *gin.Context, inv *inventory.Service, q ListQuery) (*WarehouseAllocationListResult, error) {
	if s == nil || inv == nil {
		return nil, fmt.Errorf("warehouse allocation unavailable")
	}
	q.PaymentStatus = PaymentPaid
	q.FulfillmentStatus = FulfillmentUnfulfilled
	base, err := s.List(c, q)
	if err != nil {
		return nil, err
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(base.Items))
	for i := range base.Items {
		ids[i] = base.Items[i].ID
	}
	evaluations, err := inv.EvaluateOrderWarehouseAllocations(c.Request.Context(), tenantID, ids)
	if err != nil {
		return nil, err
	}
	items := make([]WarehouseAllocationListRow, 0, len(base.Items))
	for _, row := range base.Items {
		evaluation, ok := evaluations[row.ID]
		if !ok {
			continue
		}
		out := WarehouseAllocationListRow{
			ListOrderRow: row, AllocationStatus: evaluation.Status, WarehouseID: evaluation.WarehouseID,
			WarehouseCode: evaluation.WarehouseCode, WarehouseName: evaluation.WarehouseName,
			RecommendedWarehouseID: evaluation.RecommendedWarehouseID, CandidateCount: evaluation.CandidateCount,
			EligibleCandidateCount: evaluation.EligibleCandidateCount, Blocks: evaluation.Blocks,
		}
		if candidate := recommendedAllocationCandidate(evaluation); candidate != nil {
			out.RecommendedWarehouseCode = candidate.WarehouseCode
			out.RecommendedWarehouseName = candidate.WarehouseName
		}
		items = append(items, out)
	}
	return &WarehouseAllocationListResult{Items: items, Total: base.Total, Page: base.Page, PageSize: base.PageSize, TotalPages: base.TotalPages}, nil
}

func (s *Service) GetWarehouseAllocation(c *gin.Context, inv *inventory.Service, orderID uuid.UUID) (*inventory.WarehouseAllocationEvaluation, error) {
	if s == nil || inv == nil {
		return nil, fmt.Errorf("warehouse allocation unavailable")
	}
	orderRow, err := s.findOrderBare(c, orderID)
	if err != nil {
		return nil, err
	}
	evaluations, err := inv.EvaluateOrderWarehouseAllocations(c.Request.Context(), orderRow.TenantID, []uuid.UUID{orderID})
	if err != nil {
		return nil, err
	}
	evaluation, ok := evaluations[orderID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return &evaluation, nil
}

func (s *Service) replayWarehouseAllocation(c *gin.Context, inv *inventory.Service, orderID uuid.UUID, record *idempotency.Record) (*ConfirmWarehouseAllocationResult, error) {
	if record == nil || record.ResourceID != orderID.String() {
		return nil, fmt.Errorf("warehouse allocation idempotency resource is invalid")
	}
	evaluation, err := s.GetWarehouseAllocation(c, inv, orderID)
	if err != nil {
		return nil, err
	}
	return &ConfirmWarehouseAllocationResult{Allocation: *evaluation}, nil
}

// ConfirmWarehouseAllocation atomically validates an operator-selected
// candidate, binds the order to one warehouse and reserves every order line.
func (s *Service) ConfirmWarehouseAllocation(c *gin.Context, inv *inventory.Service, orderID uuid.UUID, in ConfirmWarehouseAllocationInput, actor *uuid.UUID) (*ConfirmWarehouseAllocationResult, error) {
	if s == nil || s.DB == nil || inv == nil || s.Idempotency == nil || s.Idempotency.DB == nil {
		return nil, fmt.Errorf("warehouse allocation unavailable")
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	if key == "" {
		return nil, ErrWarehouseAllocationIdempotencyRequired
	}
	if len(key) > 128 || in.WarehouseID == uuid.Nil || len(strings.TrimSpace(in.ExpectedRevision)) != 64 {
		return nil, ErrWarehouseAllocationInputInvalid
	}
	orderRow, err := s.findOrderBare(c, orderID)
	if err != nil {
		return nil, err
	}
	idemKey := idempotency.OrderWarehouseAllocation(orderID.String(), key)
	requestHash := allocationRequestHash(orderID, in)
	if existing, getErr := s.Idempotency.Get(c.Request.Context(), orderWarehouseAllocationScope, idemKey); getErr == nil {
		if existing.RequestHash != requestHash {
			return nil, idempotency.ErrKeyConflict
		}
		switch existing.Status {
		case idempotency.StatusSucceeded:
			return s.replayWarehouseAllocation(c, inv, orderID, existing)
		case idempotency.StatusProcessing:
			return nil, ErrWarehouseAllocationInProgress
		}
	} else if !errors.Is(getErr, gorm.ErrRecordNotFound) {
		return nil, getErr
	}
	preflight, err := s.GetWarehouseAllocation(c, inv, orderID)
	if err != nil {
		return nil, err
	}
	if preflight.Status == inventory.WarehouseAllocationAllocated {
		return nil, inventory.ErrOrderAllocationAlreadyConfirmed
	}
	if preflight.Status != inventory.WarehouseAllocationAllocatable {
		return nil, inventory.ErrOrderAllocationBlocked
	}
	selected := false
	for _, candidate := range preflight.Candidates {
		if candidate.WarehouseID == in.WarehouseID && candidate.Eligible && candidate.Revision == strings.TrimSpace(in.ExpectedRevision) {
			selected = true
			break
		}
	}
	if !selected {
		return nil, inventory.ErrOrderAllocationRevisionConflict
	}

	owner := allocationOwner(actor)
	acquired, acquireErr := s.Idempotency.Acquire(c.Request.Context(), orderWarehouseAllocationScope, idemKey, requestHash, owner, idempotency.DefaultLease)
	decision, record, classifyErr := idempotency.Classify(acquired, acquireErr)
	switch decision {
	case idempotency.DecisionAlreadySucceeded:
		return s.replayWarehouseAllocation(c, inv, orderID, record)
	case idempotency.DecisionInProgress:
		return nil, ErrWarehouseAllocationInProgress
	case idempotency.DecisionKeyConflict, idempotency.DecisionPermanentFailure:
		if classifyErr != nil {
			return nil, classifyErr
		}
		return nil, idempotency.ErrKeyConflict
	case idempotency.DecisionAcquired, idempotency.DecisionRetryAllowed:
		if acquired == nil || acquired.Record == nil {
			return nil, fmt.Errorf("warehouse allocation idempotency record missing")
		}
	default:
		if classifyErr != nil {
			return nil, classifyErr
		}
		return nil, fmt.Errorf("warehouse allocation idempotency unavailable")
	}

	warehouseID := in.WarehouseID
	tenantID := orderRow.TenantID
	reserve, err := inv.DeductInventoryForOrder(c.Request.Context(), orderID, inventory.OrderInventoryOptions{
		Reason: "manual_warehouse_allocation", CreatedBy: actor, WarehouseID: &warehouseID, TenantID: &tenantID,
		ExpectedAllocationRevision: strings.TrimSpace(in.ExpectedRevision), RequireAllLinesApplied: true,
		AfterApply: func(tx *gorm.DB, action string) error {
			if action != inventory.EffectTypeReserve {
				return inventory.ErrOrderAllocationBlocked
			}
			summary, _ := json.Marshal(map[string]string{"orderId": orderID.String(), "warehouseId": warehouseID.String()})
			return s.Idempotency.WithDB(tx).Complete(c.Request.Context(), acquired.Record.ID, owner, idempotency.CompleteResult{
				ResponseCode: "ORDER_WAREHOUSE_ALLOCATION_SUCCESS", ResponseSummary: string(summary),
				ResourceType: "order", ResourceID: orderID.String(),
			})
		},
	})
	if err != nil {
		_ = s.Idempotency.Fail(c.Request.Context(), acquired.Record.ID, owner, err.Error(), true)
		return nil, err
	}
	evaluation, err := s.GetWarehouseAllocation(c, inv, orderID)
	if err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: actor, Action: "order.warehouse_allocation.confirm", Resource: "order", ResourceID: orderID.String(), Status: "success",
			Message: fmt.Sprintf("orderId=%s warehouseId=%s", orderID.String(), warehouseID.String()),
		})
	}
	return &ConfirmWarehouseAllocationResult{Allocation: *evaluation, InventoryReserve: reserve}, nil
}

func (h *Handler) ListWarehouseAllocations(c *gin.Context) {
	if h == nil || h.Svc == nil || h.Inv == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "warehouse allocation unavailable")
		return
	}
	if h.denyRead(c) {
		return
	}
	assignment := strings.ToLower(strings.TrimSpace(c.Query("assignment")))
	if assignment != "" && assignment != "all" && assignment != "allocated" && assignment != "unallocated" {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid assignment")
		return
	}
	if assignment == "all" {
		assignment = ""
	}
	result, err := h.Svc.ListWarehouseAllocations(c, h.Inv, ListQuery{
		Page: atoiQ(c, "page", 1), PageSize: atoiQ(c, "pageSize", 20), Keyword: c.Query("keyword"), WarehouseAssignment: assignment,
	})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"list": result.Items, "pagination": gin.H{
		"page": result.Page, "pageSize": result.PageSize, "total": result.Total, "totalPages": result.TotalPages,
	}})
}

func (h *Handler) GetWarehouseAllocation(c *gin.Context) {
	if h == nil || h.Svc == nil || h.Inv == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "warehouse allocation unavailable")
		return
	}
	if h.denyRead(c) {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid id")
		return
	}
	result, err := h.Svc.GetWarehouseAllocation(c, h.Inv, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "not found")
		return
	}
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) PostWarehouseAllocation(c *gin.Context) {
	if h == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "warehouse allocation unavailable")
		return
	}
	if h.denyWrite(c) {
		return
	}
	if h.Svc == nil || h.Inv == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "warehouse allocation unavailable")
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid id")
		return
	}
	var body ConfirmWarehouseAllocationInput
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	result, err := h.Svc.ConfirmWarehouseAllocation(c, h.Inv, id, body, adminUUID(c))
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "not found")
		case errors.Is(err, ErrWarehouseAllocationIdempotencyRequired), errors.Is(err, ErrWarehouseAllocationInputInvalid):
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		case errors.Is(err, ErrWarehouseAllocationInProgress), errors.Is(err, idempotency.ErrKeyConflict),
			errors.Is(err, inventory.ErrOrderAllocationRevisionConflict), errors.Is(err, inventory.ErrOrderAllocationBlocked),
			errors.Is(err, inventory.ErrOrderAllocationAlreadyConfirmed), errors.Is(err, inventory.ErrInsufficientSKUStock),
			errors.Is(err, inventory.ErrOrderInventoryState), errors.Is(err, inventory.ErrOrderWarehouseConflict):
			response.Fail(c, http.StatusConflict, response.CodeBadRequest, err.Error())
		default:
			response.HandleError(c, err)
		}
		return
	}
	response.OK(c, result)
}
