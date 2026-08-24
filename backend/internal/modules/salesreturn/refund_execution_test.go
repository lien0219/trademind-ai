package salesreturn

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func completeRefundOnly(t *testing.T, fx *fixture, key string, quantity int) (*SalesReturn, uuid.UUID) {
	t.Helper()
	row := fx.create(t, key+"-create", TypeRefundOnly, "", quantity)
	approver := uuid.New()
	row = advanceToApproved(t, fx, row, key, approver)
	var err error
	row, err = fx.service.Complete(t.Context(), 7, row.ID, uuidPointer(uuid.New()), ActionInput{
		ExpectedRevision: row.Revision, IdempotencyKey: key + "-complete", Reason: "local after-sale complete",
	})
	if err != nil {
		t.Fatal(err)
	}
	return row, approver
}

func TestRefundExecutionRequiresCompletedReturnAndIsCreateIdempotent(t *testing.T) {
	fx := newFixture(t, 2)
	draft := fx.create(t, "refund-execution-draft", TypeRefundOnly, "", 1)
	creator := uuid.New()
	if _, err := fx.service.CreateRefundExecution(t.Context(), 7, draft.ID, &creator, CreateRefundExecutionInput{IdempotencyKey: "refund-execution-before-complete"}); !errors.Is(err, ErrRefundExecutionInvalidTransition) {
		t.Fatalf("expected completed sales return requirement, got %v", err)
	}

	approver := uuid.New()
	row := advanceToApproved(t, fx, draft, "refund-execution-ready", approver)
	row, err := fx.service.Complete(t.Context(), 7, row.ID, uuidPointer(uuid.New()), ActionInput{ExpectedRevision: row.Revision, IdempotencyKey: "refund-execution-ready-complete"})
	if err != nil {
		t.Fatal(err)
	}
	execution, err := fx.service.CreateRefundExecution(t.Context(), 7, row.ID, &creator, CreateRefundExecutionInput{IdempotencyKey: "refund-execution-create-001"})
	if err != nil {
		t.Fatal(err)
	}
	if execution.Status != RefundExecutionStatusPending || execution.Revision != 1 || execution.RefundAmountMinor != row.RefundAmountMinor || execution.InternalShopID == nil || *execution.InternalShopID != fx.shop.ID || execution.PlatformReview == nil || execution.PlatformReview.Reason != "platform_fact_missing" {
		t.Fatalf("unexpected execution: %#v", execution)
	}
	replay, err := fx.service.CreateRefundExecution(t.Context(), 7, row.ID, &creator, CreateRefundExecutionInput{IdempotencyKey: "refund-execution-create-001"})
	if err != nil || replay.ID != execution.ID {
		t.Fatalf("create replay failed: row=%#v err=%v", replay, err)
	}
	if _, err := fx.service.CreateRefundExecution(t.Context(), 7, row.ID, &creator, CreateRefundExecutionInput{IdempotencyKey: "refund-execution-create-002"}); !errors.Is(err, ErrRefundExecutionExists) {
		t.Fatalf("expected one execution per sales return, got %v", err)
	}
}

func TestRefundExecutionManualResultEnforcesDutyRevisionAndExternalUniqueness(t *testing.T) {
	fx := newFixture(t, 2)
	row, approver := completeRefundOnly(t, fx, "refund-result-one", 1)
	execution, err := fx.service.CreateRefundExecution(t.Context(), 7, row.ID, uuidPointer(uuid.New()), CreateRefundExecutionInput{IdempotencyKey: "refund-result-create-one"})
	if err != nil {
		t.Fatal(err)
	}
	input := RecordRefundResultInput{
		ExpectedRevision: execution.Revision, IdempotencyKey: "refund-result-success-one", Result: RefundExecutionStatusSucceeded,
		ExternalRefundID: "external-refund-001", ExecutedAt: time.Now().UTC().Add(-time.Minute), Reason: "refunded in provider console",
	}
	if _, err := fx.service.RecordRefundResult(t.Context(), 7, execution.ID, &approver, input); !errors.Is(err, ErrRefundExecutionDutyConflict) {
		t.Fatalf("approver must not record refund result: %v", err)
	}
	executor := uuid.New()
	result, err := fx.service.RecordRefundResult(t.Context(), 7, execution.ID, &executor, input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != RefundExecutionStatusSucceeded || result.Source != RefundExecutionSourceManual || result.ExternalRefundID == nil || *result.ExternalRefundID != input.ExternalRefundID || result.Revision != 2 || len(result.Events) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	replay, err := fx.service.RecordRefundResult(t.Context(), 7, execution.ID, &executor, input)
	if err != nil || replay.Revision != 2 || len(replay.Events) != 1 {
		t.Fatalf("result replay failed: row=%#v err=%v", replay, err)
	}
	changed := input
	changed.Reason = "changed replay"
	if _, err := fx.service.RecordRefundResult(t.Context(), 7, execution.ID, &executor, changed); !errors.Is(err, ErrRefundExecutionIdempotencyConflict) {
		t.Fatalf("expected changed replay conflict, got %v", err)
	}

	secondReturn, _ := completeRefundOnly(t, fx, "refund-result-two", 1)
	second, err := fx.service.CreateRefundExecution(t.Context(), 7, secondReturn.ID, uuidPointer(uuid.New()), CreateRefundExecutionInput{IdempotencyKey: "refund-result-create-two"})
	if err != nil {
		t.Fatal(err)
	}
	duplicate := input
	duplicate.ExpectedRevision = second.Revision
	duplicate.IdempotencyKey = "refund-result-success-two"
	if _, err := fx.service.RecordRefundResult(t.Context(), 7, second.ID, uuidPointer(uuid.New()), duplicate); !errors.Is(err, ErrRefundExecutionIdempotencyConflict) {
		t.Fatalf("expected duplicate external refund rejection, got %v", err)
	}
}

func TestRefundExecutionCanConfirmMatchedFinalPlatformFact(t *testing.T) {
	fx := newFixture(t, 1)
	externalOrderID := "refund-confirm-order"
	if err := fx.db.Model(fx.order).Updates(map[string]any{"platform": "douyin_shop", "external_order_id": externalOrderID}).Error; err != nil {
		t.Fatal(err)
	}
	row, approver := completeRefundOnly(t, fx, "refund-confirm", 1)
	platformAt := time.Now().UTC().Add(-2 * time.Minute)
	input := PlatformAfterSaleInput{
		TenantID: 7, Platform: "douyin_shop", InternalShopID: &fx.shop.ID, PlatformShopID: fx.shop.ExternalShopID,
		EventID: "refund-confirm-event", EventType: "refund_success", ExternalAfterSaleID: "refund-confirm-external",
		ExternalOrderID: externalOrderID, PlatformType: TypeRefundOnly, PlatformStatus: "success",
		RefundAmountMinor: row.RefundAmountMinor, Currency: row.Currency, PlatformUpdatedAt: &platformAt,
		RawPayload: []byte(`{"event":"refund_success","id":"refund-confirm-external"}`),
	}
	if err := fx.service.UpsertPlatformAfterSale(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	factID := mustPlatformAfterSaleID(t, fx.db, input.ExternalAfterSaleID)
	execution, err := fx.service.CreateRefundExecution(t.Context(), 7, row.ID, uuidPointer(uuid.New()), CreateRefundExecutionInput{IdempotencyKey: "refund-confirm-create", PlatformAfterSaleID: &factID})
	if err != nil {
		t.Fatal(err)
	}
	executor := uuidPointerDifferentFrom(approver)
	execution, err = fx.service.RecordRefundResult(t.Context(), 7, execution.ID, executor, RecordRefundResultInput{
		ExpectedRevision: execution.Revision, IdempotencyKey: "refund-confirm-unknown", Result: RefundExecutionStatusUnknown,
		ExecutedAt: time.Now().UTC().Add(-time.Minute), Reason: "provider console timed out",
	})
	if err != nil || execution.Status != RefundExecutionStatusUnknown {
		t.Fatalf("record unknown result: row=%#v err=%v", execution, err)
	}
	result, err := fx.service.ConfirmRefundFromPlatform(t.Context(), 7, execution.ID, uuidPointerDifferentFrom(approver), ConfirmRefundFromPlatformInput{
		ExpectedRevision: execution.Revision, IdempotencyKey: "refund-confirm-result", PlatformAfterSaleID: factID, Reason: "confirmed from provider event",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != RefundExecutionStatusSucceeded || result.Source != RefundExecutionSourcePlatformFact || result.Revision != 3 || len(result.Events) != 2 || result.PlatformReview == nil || result.PlatformReview.Status != PlatformReconciliationMatched || result.ExternalRefundID == nil || *result.ExternalRefundID != input.ExternalAfterSaleID {
		t.Fatalf("unexpected platform-confirmed execution: %#v", result)
	}
}

func TestRefundExecutionReviewBlocksAmbiguousPlatformFacts(t *testing.T) {
	fx := newFixture(t, 1)
	row, _ := completeRefundOnly(t, fx, "refund-ambiguous", 1)
	execution, err := fx.service.CreateRefundExecution(t.Context(), 7, row.ID, uuidPointer(uuid.New()), CreateRefundExecutionInput{IdempotencyKey: "refund-ambiguous-create"})
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"one", "two"} {
		fact := &PlatformAfterSale{
			TenantID: 7, Platform: "douyin_shop", InternalShopID: &fx.shop.ID, PlatformShopID: fx.shop.ExternalShopID,
			ExternalAfterSaleID: "refund-ambiguous-" + suffix, ExternalOrderID: "refund-ambiguous-order",
			PlatformType: TypeRefundOnly, PlatformStatus: "success", RefundAmountMinor: row.RefundAmountMinor,
			Currency: row.Currency, OrderID: &row.OrderID, SalesReturnID: &row.ID,
			ReconciliationStatus: PlatformReconciliationMatched, ReconciliationReason: "matched_order_and_sales_return",
			LastEventID: "refund-ambiguous-event-" + suffix, LastPayloadHash: suffix,
		}
		if err := fx.db.Create(fact).Error; err != nil {
			t.Fatal(err)
		}
	}
	execution, err = fx.service.GetRefundExecution(t.Context(), 7, execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if execution.PlatformReview == nil || execution.PlatformReview.Status != PlatformReconciliationBlocked || execution.PlatformReview.Reason != "multiple_platform_refund_facts" || execution.PlatformReview.PlatformAfterSaleID != nil {
		t.Fatalf("ambiguous platform facts must fail closed: %#v", execution.PlatformReview)
	}
}

func TestRefundExecutionListSupportsLegacyTenantZero(t *testing.T) {
	fx := newFixture(t, 1)

	result, err := fx.service.ListRefundExecutions(t.Context(), RefundExecutionListQuery{
		TenantID: 0,
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("legacy tenant zero list failed: %v", err)
	}
	if result == nil || len(result.List) != 0 || result.Page != 1 || result.PageSize != 20 {
		t.Fatalf("unexpected legacy tenant zero list: %#v", result)
	}
}

func uuidPointerDifferentFrom(other uuid.UUID) *uuid.UUID {
	value := uuid.New()
	for value == other {
		value = uuid.New()
	}
	return &value
}
