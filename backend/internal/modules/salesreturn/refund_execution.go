package salesreturn

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	ordermod "github.com/trademind-ai/trademind/backend/internal/modules/order"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrRefundExecutionAbsent              = errors.New("refund execution not found")
	ErrRefundExecutionInvalidInput        = errors.New("invalid refund execution input")
	ErrRefundExecutionExists              = errors.New("refund execution already exists")
	ErrRefundExecutionInvalidTransition   = errors.New("invalid refund execution transition")
	ErrRefundExecutionRevisionConflict    = errors.New("refund execution revision conflict")
	ErrRefundExecutionIdempotencyConflict = errors.New("refund execution idempotency conflict")
	ErrRefundExecutionDutyConflict        = errors.New("sales return approver cannot record refund result")
	ErrRefundPlatformFactConflict         = errors.New("platform refund fact does not match sales return")
	ErrRefundPlatformFactPending          = errors.New("platform refund fact is not final")
)

const (
	refundActionRecordResult    = "record_result"
	refundActionConfirmPlatform = "confirm_platform"
	refundActionCancel          = "cancel"
)

func (s *Service) CreateRefundExecution(ctx context.Context, tenantID int64, salesReturnID uuid.UUID, actor *uuid.UUID, in CreateRefundExecutionInput) (*RefundExecution, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	if tenantID < 1 || salesReturnID == uuid.Nil || actor == nil || *actor == uuid.Nil || len(key) < 8 || len(key) > 128 || (in.PlatformAfterSaleID != nil && *in.PlatformAfterSaleID == uuid.Nil) {
		return nil, ErrRefundExecutionInvalidInput
	}
	hash := refundHash("create", salesReturnID.String(), optionalUUID(in.PlatformAfterSaleID))
	if existing, err := loadRefundExecutionByKey(s.DB.WithContext(ctx), tenantID, key); err != nil {
		return nil, err
	} else if existing != nil {
		if existing.PayloadHash != hash {
			return nil, ErrRefundExecutionIdempotencyConflict
		}
		return s.GetRefundExecution(ctx, tenantID, existing.ID)
	}

	var executionID uuid.UUID
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if existing, err := loadRefundExecutionByKey(tx, tenantID, key); err != nil {
			return err
		} else if existing != nil {
			if existing.PayloadHash != hash {
				return ErrRefundExecutionIdempotencyConflict
			}
			executionID = existing.ID
			return nil
		}

		var existing RefundExecution
		if err := tx.Where("tenant_id = ? AND sales_return_id = ?", tenantID, salesReturnID).First(&existing).Error; err == nil {
			return ErrRefundExecutionExists
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var salesReturn SalesReturn
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, salesReturnID).First(&salesReturn).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAbsent
		} else if err != nil {
			return err
		}
		if salesReturn.Status != StatusCompleted || salesReturn.RefundAmountMinor < 1 || strings.TrimSpace(salesReturn.Currency) == "" {
			return ErrRefundExecutionInvalidTransition
		}

		var orderRow ordermod.Order
		if err := tx.Select("id", "shop_id").Where("tenant_id = ? AND id = ?", tenantID, salesReturn.OrderID).First(&orderRow).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAbsent
		} else if err != nil {
			return err
		}
		if in.PlatformAfterSaleID != nil {
			if _, err := requireMatchingPlatformFact(tx, tenantID, salesReturn, *in.PlatformAfterSaleID, false); err != nil {
				return err
			}
		}

		row := &RefundExecution{
			TenantID: tenantID, ExecutionNo: newRefundExecutionNumber(), IdempotencyKey: key, PayloadHash: hash,
			SalesReturnID: salesReturn.ID, OrderID: salesReturn.OrderID, InternalShopID: orderRow.ShopID,
			PlatformAfterSaleID: in.PlatformAfterSaleID, Status: RefundExecutionStatusPending,
			Currency: strings.ToUpper(strings.TrimSpace(salesReturn.Currency)), RefundAmountMinor: salesReturn.RefundAmountMinor,
			Revision: 1, CreatedBy: actor,
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(row)
		if created.Error != nil {
			return fmt.Errorf("create refund execution: %w", created.Error)
		}
		if created.RowsAffected != 1 {
			if existing, err := loadRefundExecutionByKey(tx, tenantID, key); err != nil {
				return err
			} else if existing == nil || existing.PayloadHash != hash {
				return ErrRefundExecutionIdempotencyConflict
			} else {
				executionID = existing.ID
				return nil
			}
		}
		executionID = row.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetRefundExecution(ctx, tenantID, executionID)
}

func (s *Service) GetRefundExecution(ctx context.Context, tenantID int64, id uuid.UUID) (*RefundExecution, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if tenantID < 1 || id == uuid.Nil {
		return nil, ErrRefundExecutionAbsent
	}
	var row RefundExecution
	err := refundExecutionQuery(s.DB.WithContext(ctx), tenantID).Where("refund_executions.id = ?", id).Scan(&row).Error
	if err != nil {
		return nil, fmt.Errorf("get refund execution: %w", err)
	}
	if row.ID == uuid.Nil {
		return nil, ErrRefundExecutionAbsent
	}
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND refund_execution_id = ?", tenantID, row.ID).Order("created_at ASC, id ASC").Find(&row.Events).Error; err != nil {
		return nil, fmt.Errorf("list refund execution events: %w", err)
	}
	rows := []RefundExecution{row}
	if err := enrichRefundPlatformReviews(ctx, s.DB, tenantID, rows); err != nil {
		return nil, err
	}
	return &rows[0], nil
}

func (s *Service) ListRefundExecutions(ctx context.Context, q RefundExecutionListQuery) (*RefundExecutionListResult, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if q.TenantID < 1 {
		return nil, ErrRefundExecutionInvalidInput
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 || q.PageSize > 100 {
		q.PageSize = 20
	}
	tx := refundExecutionQuery(s.DB.WithContext(ctx), q.TenantID)
	if status := strings.ToLower(strings.TrimSpace(q.Status)); status != "" {
		if !validRefundExecutionStatus(status) {
			return nil, ErrRefundExecutionInvalidInput
		}
		tx = tx.Where("refund_executions.status = ?", status)
	}
	if q.SalesReturnID != nil && *q.SalesReturnID != uuid.Nil {
		tx = tx.Where("refund_executions.sales_return_id = ?", *q.SalesReturnID)
	}
	if q.OrderID != nil && *q.OrderID != uuid.Nil {
		tx = tx.Where("refund_executions.order_id = ?", *q.OrderID)
	}
	if q.RestrictStoreScope {
		if len(q.AllowedShopIDs) == 0 {
			tx = tx.Where("1 = 0")
		} else {
			tx = tx.Where("refund_executions.internal_shop_id IN ?", q.AllowedShopIDs)
		}
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count refund executions: %w", err)
	}
	rows := make([]RefundExecution, 0, q.PageSize)
	if err := tx.Order("refund_executions.created_at DESC, refund_executions.id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list refund executions: %w", err)
	}
	if err := enrichRefundPlatformReviews(ctx, s.DB, q.TenantID, rows); err != nil {
		return nil, err
	}
	return &RefundExecutionListResult{List: rows, Page: q.Page, PageSize: q.PageSize, Total: total, TotalPages: int((total + int64(q.PageSize) - 1) / int64(q.PageSize))}, nil
}

func (s *Service) RecordRefundResult(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, in RecordRefundResultInput) (*RefundExecution, error) {
	result := strings.ToLower(strings.TrimSpace(in.Result))
	externalID := strings.TrimSpace(in.ExternalRefundID)
	reason := strings.TrimSpace(in.Reason)
	key := strings.TrimSpace(in.IdempotencyKey)
	executedAt := in.ExecutedAt.UTC()
	if tenantID < 1 || id == uuid.Nil || actor == nil || *actor == uuid.Nil || in.ExpectedRevision < 1 || len(key) < 8 || len(key) > 128 || !validRefundResult(result) || in.ExecutedAt.IsZero() || in.ExecutedAt.After(time.Now().UTC().Add(5*time.Minute)) || len([]rune(externalID)) > 255 || len([]rune(reason)) > 255 || (result == RefundExecutionStatusSucceeded && externalID == "") || ((result == RefundExecutionStatusFailed || result == RefundExecutionStatusUnknown) && reason == "") {
		return nil, ErrRefundExecutionInvalidInput
	}
	hash := refundHash(refundActionRecordResult, fmt.Sprint(in.ExpectedRevision), result, externalID, executedAt.Format(time.RFC3339Nano), reason)
	if err := s.finalizeRefundExecution(ctx, tenantID, id, actor, refundActionRecordResult, in.ExpectedRevision, key, hash, func(_ *gorm.DB, row *RefundExecution, _ *SalesReturn) (*refundFinalState, error) {
		return &refundFinalState{Status: result, Source: RefundExecutionSourceManual, ExternalRefundID: externalID, Reason: reason, ExecutedAt: &executedAt, PlatformAfterSaleID: row.PlatformAfterSaleID}, nil
	}); err != nil {
		return nil, err
	}
	return s.GetRefundExecution(ctx, tenantID, id)
}

func (s *Service) ConfirmRefundFromPlatform(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, in ConfirmRefundFromPlatformInput) (*RefundExecution, error) {
	key := strings.TrimSpace(in.IdempotencyKey)
	reason := strings.TrimSpace(in.Reason)
	if tenantID < 1 || id == uuid.Nil || actor == nil || *actor == uuid.Nil || in.ExpectedRevision < 1 || len(key) < 8 || len(key) > 128 || in.PlatformAfterSaleID == uuid.Nil || len([]rune(reason)) > 255 {
		return nil, ErrRefundExecutionInvalidInput
	}
	hash := refundHash(refundActionConfirmPlatform, fmt.Sprint(in.ExpectedRevision), in.PlatformAfterSaleID.String(), reason)
	if err := s.finalizeRefundExecution(ctx, tenantID, id, actor, refundActionConfirmPlatform, in.ExpectedRevision, key, hash, func(tx *gorm.DB, row *RefundExecution, salesReturn *SalesReturn) (*refundFinalState, error) {
		fact, err := requireMatchingPlatformFact(tx, tenantID, *salesReturn, in.PlatformAfterSaleID, true)
		if err != nil {
			return nil, err
		}
		status := platformRefundResult(fact.PlatformStatus)
		if status == "" {
			return nil, ErrRefundPlatformFactPending
		}
		executedAt := fact.UpdatedAt.UTC()
		if fact.PlatformUpdatedAt != nil {
			executedAt = fact.PlatformUpdatedAt.UTC()
		}
		factID := fact.ID
		return &refundFinalState{Status: status, Source: RefundExecutionSourcePlatformFact, ExternalRefundID: strings.TrimSpace(fact.ExternalAfterSaleID), Reason: reason, ExecutedAt: &executedAt, PlatformAfterSaleID: &factID}, nil
	}); err != nil {
		return nil, err
	}
	return s.GetRefundExecution(ctx, tenantID, id)
}

func (s *Service) CancelRefundExecution(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, in ActionInput) (*RefundExecution, error) {
	key := strings.TrimSpace(in.IdempotencyKey)
	reason := strings.TrimSpace(in.Reason)
	if tenantID < 1 || id == uuid.Nil || actor == nil || *actor == uuid.Nil || in.ExpectedRevision < 1 || len(key) < 8 || len(key) > 128 || reason == "" || len([]rune(reason)) > 128 {
		return nil, ErrRefundExecutionInvalidInput
	}
	hash := refundHash(refundActionCancel, fmt.Sprint(in.ExpectedRevision), reason)
	if err := s.finalizeRefundExecution(ctx, tenantID, id, actor, refundActionCancel, in.ExpectedRevision, key, hash, func(_ *gorm.DB, row *RefundExecution, _ *SalesReturn) (*refundFinalState, error) {
		now := time.Now().UTC()
		return &refundFinalState{Status: RefundExecutionStatusCancelled, Reason: reason, ExecutedAt: &now, PlatformAfterSaleID: row.PlatformAfterSaleID}, nil
	}); err != nil {
		return nil, err
	}
	return s.GetRefundExecution(ctx, tenantID, id)
}

func (s *Service) SalesReturnShopID(ctx context.Context, tenantID int64, id uuid.UUID) (*uuid.UUID, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	var orderRow ordermod.Order
	err := s.DB.WithContext(ctx).Model(&ordermod.Order{}).Select("orders.shop_id").Joins("JOIN sales_returns ON sales_returns.order_id = orders.id AND sales_returns.tenant_id = orders.tenant_id AND sales_returns.deleted_at IS NULL").Where("sales_returns.tenant_id = ? AND sales_returns.id = ?", tenantID, id).Scan(&orderRow).Error
	if err != nil {
		return nil, fmt.Errorf("get sales return shop: %w", err)
	}
	if orderRow.ShopID == nil || *orderRow.ShopID == uuid.Nil {
		return nil, ErrAbsent
	}
	return orderRow.ShopID, nil
}

type refundFinalState struct {
	Status              string
	Source              string
	ExternalRefundID    string
	Reason              string
	ExecutedAt          *time.Time
	PlatformAfterSaleID *uuid.UUID
}

func (s *Service) finalizeRefundExecution(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, action string, expectedRevision int, key, hash string, resolve func(*gorm.DB, *RefundExecution, *SalesReturn) (*refundFinalState, error)) error {
	if err := s.ready(); err != nil {
		return err
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row RefundExecution
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRefundExecutionAbsent
		} else if err != nil {
			return err
		}
		var completed RefundExecutionEvent
		if err := tx.Where("tenant_id = ? AND refund_execution_id = ? AND action = ?", tenantID, id, action).First(&completed).Error; err == nil {
			if completed.IdempotencyKey != key || completed.RequestHash != hash {
				return ErrRefundExecutionIdempotencyConflict
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var used RefundExecutionEvent
		if err := tx.Where("tenant_id = ? AND refund_execution_id = ? AND idempotency_key = ?", tenantID, id, key).First(&used).Error; err == nil {
			return ErrRefundExecutionIdempotencyConflict
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if row.Status != RefundExecutionStatusPending && !(action == refundActionConfirmPlatform && row.Status == RefundExecutionStatusUnknown) {
			return ErrRefundExecutionInvalidTransition
		}
		if row.Revision != expectedRevision {
			return ErrRefundExecutionRevisionConflict
		}

		var salesReturn SalesReturn
		if err := tx.Where("tenant_id = ? AND id = ?", tenantID, row.SalesReturnID).First(&salesReturn).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAbsent
		} else if err != nil {
			return err
		}
		if action != refundActionCancel && (salesReturn.ApprovedBy == nil || actor == nil || *salesReturn.ApprovedBy == *actor) {
			return ErrRefundExecutionDutyConflict
		}
		state, err := resolve(tx, &row, &salesReturn)
		if err != nil {
			return err
		}
		fromStatus := row.Status
		now := time.Now().UTC()
		var externalRefundID *string
		if state.ExternalRefundID != "" {
			externalRefundID = &state.ExternalRefundID
			var duplicate RefundExecution
			if err := tx.Where("tenant_id = ? AND external_refund_id = ? AND id <> ?", tenantID, state.ExternalRefundID, row.ID).First(&duplicate).Error; err == nil {
				return ErrRefundExecutionIdempotencyConflict
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		updates := map[string]any{
			"status": state.Status, "revision": row.Revision + 1, "source": state.Source,
			"external_refund_id": externalRefundID, "result_reason": state.Reason,
			"platform_after_sale_id": state.PlatformAfterSaleID, "updated_at": now,
		}
		if action == refundActionCancel {
			updates["cancelled_by"], updates["cancelled_at"] = actor, now
		} else {
			updates["executed_by"], updates["executed_at"] = actor, state.ExecutedAt
		}
		updated := tx.Model(&RefundExecution{}).Where("tenant_id = ? AND id = ? AND revision = ? AND status = ?", tenantID, row.ID, row.Revision, fromStatus).Updates(updates)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrRefundExecutionRevisionConflict
		}
		event := &RefundExecutionEvent{
			TenantID: tenantID, RefundExecutionID: row.ID, Action: action, IdempotencyKey: key, RequestHash: hash,
			ActorID: actor, FromStatus: fromStatus, ToStatus: state.Status, Source: state.Source,
			ExternalRefundID: state.ExternalRefundID, Reason: state.Reason, PlatformAfterSaleID: state.PlatformAfterSaleID,
		}
		if action != refundActionCancel {
			event.ExecutedAt = state.ExecutedAt
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(event)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected != 1 {
			return ErrRefundExecutionIdempotencyConflict
		}
		return nil
	})
}

func validRefundExecutionStatus(status string) bool {
	switch status {
	case RefundExecutionStatusPending, RefundExecutionStatusSucceeded, RefundExecutionStatusFailed, RefundExecutionStatusUnknown, RefundExecutionStatusCancelled:
		return true
	default:
		return false
	}
}

func validRefundResult(result string) bool {
	return result == RefundExecutionStatusSucceeded || result == RefundExecutionStatusFailed || result == RefundExecutionStatusUnknown
}

func loadRefundExecutionByKey(tx *gorm.DB, tenantID int64, key string) (*RefundExecution, error) {
	var row RefundExecution
	err := tx.Where("tenant_id = ? AND idempotency_key = ?", tenantID, key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load refund execution by idempotency key: %w", err)
	}
	return &row, nil
}

func refundHash(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = fmt.Fprintf(hash, "%d:%s|", len(part), part)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func optionalUUID(value *uuid.UUID) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func newRefundExecutionNumber() string {
	return fmt.Sprintf("RF-%s-%s", time.Now().UTC().Format("20060102"), strings.ToUpper(uuid.NewString()[:8]))
}
