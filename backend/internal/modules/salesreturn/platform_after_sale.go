package salesreturn

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	ordermod "github.com/trademind-ai/trademind/backend/internal/modules/order"
	shopmod "github.com/trademind-ai/trademind/backend/internal/modules/shop"
	douyinshop "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrPlatformAfterSaleInvalid = errors.New("platform after-sale event is invalid")
	ErrPlatformEventConflict    = errors.New("platform after-sale event idempotency conflict")
)

type PlatformAfterSaleInput struct {
	TenantID            int64
	Platform            string
	InternalShopID      *uuid.UUID
	PlatformShopID      string
	EventID             string
	EventType           string
	ExternalAfterSaleID string
	ExternalOrderID     string
	PlatformType        string
	PlatformStatus      string
	RefundAmountMinor   int64
	Currency            string
	PlatformUpdatedAt   *time.Time
	RawPayload          []byte
}

func validPlatformReconciliationStatus(value string) bool {
	switch strings.TrimSpace(value) {
	case PlatformReconciliationMatched, PlatformReconciliationPending, PlatformReconciliationMismatch, PlatformReconciliationBlocked:
		return true
	default:
		return false
	}
}

// HandleDouyinAfterSaleEvent receives a normalized provider event. It only
// records the provider fact and never calls a platform or payment write API.
type DouyinAfterSaleWebhookHandler struct {
	Svc *Service
}

func (h *DouyinAfterSaleWebhookHandler) HandleDouyinAfterSaleEvent(ctx context.Context, ev *douyinshop.NormalizedWebhookEvent) error {
	if h == nil || h.Svc == nil {
		return fmt.Errorf("sales return: after-sale handler unavailable")
	}
	in, err := ParseDouyinAfterSaleEvent(ev)
	if err != nil {
		return err
	}
	return h.Svc.UpsertPlatformAfterSale(ctx, in)
}

func ParseDouyinAfterSaleEvent(ev *douyinshop.NormalizedWebhookEvent) (PlatformAfterSaleInput, error) {
	if ev == nil {
		return PlatformAfterSaleInput{}, ErrPlatformAfterSaleInvalid
	}
	data := ev.Data
	eventType := strings.TrimSpace(ev.EventType)
	eventID := strings.TrimSpace(ev.MsgID)
	if eventID == "" {
		eventID = stringValue(data, "event_id", "eventId", "msg_id", "msgId")
	}
	if eventID == "" {
		return PlatformAfterSaleInput{}, fmt.Errorf("%w: event id missing", ErrPlatformAfterSaleInvalid)
	}
	afterSaleID := stringValue(data, "after_sale_id", "afterSaleId", "aftersale_id", "aftersaleId", "refund_id", "refundId")
	orderID := stringValue(data, "order_id", "orderId", "shop_order_id", "shopOrderId", "order_no", "orderNo")
	status := stringValue(data, "status", "after_sale_status", "afterSaleStatus", "refund_status", "refundStatus", "state")
	platformType := stringValue(data, "after_sale_type", "afterSaleType", "refund_type", "refundType", "type")
	if platformType == "" {
		if strings.HasPrefix(eventType, "refund_") {
			platformType = TypeRefundOnly
		} else {
			platformType = TypeReturnRefund
		}
	}
	amount, err := parseRefundAmount(data)
	if err != nil {
		return PlatformAfterSaleInput{}, fmt.Errorf("%w: refund amount: %v", ErrPlatformAfterSaleInvalid, err)
	}
	currency := strings.ToUpper(stringValue(data, "currency", "currency_code", "currencyCode"))
	if eventType == "" || afterSaleID == "" || orderID == "" || status == "" || currency == "" {
		return PlatformAfterSaleInput{}, fmt.Errorf("%w: required provider fields missing", ErrPlatformAfterSaleInvalid)
	}
	updatedAt, err := parseProviderTime(data, "updated_at", "updatedAt", "update_time", "updateTime", "platform_updated_at", "platformUpdatedAt")
	if err != nil {
		return PlatformAfterSaleInput{}, fmt.Errorf("%w: platform update time: %v", ErrPlatformAfterSaleInvalid, err)
	}
	var internalShopID *uuid.UUID
	internalShopValue := strings.TrimSpace(ev.InternalShopID)
	if internalShopValue == "" {
		internalShopValue = stringValue(data, "internal_shop_id", "internalShopId")
	}
	if internalShopValue != "" {
		u, parseErr := uuid.Parse(internalShopValue)
		if parseErr != nil {
			return PlatformAfterSaleInput{}, fmt.Errorf("%w: internal shop id", ErrPlatformAfterSaleInvalid)
		}
		internalShopID = &u
	}
	return PlatformAfterSaleInput{
		TenantID:            ev.TenantID,
		Platform:            "douyin_shop",
		InternalShopID:      internalShopID,
		PlatformShopID:      strings.TrimSpace(ev.PlatformShopID),
		EventID:             eventID,
		EventType:           eventType,
		ExternalAfterSaleID: afterSaleID,
		ExternalOrderID:     orderID,
		PlatformType:        strings.ToLower(strings.TrimSpace(platformType)),
		PlatformStatus:      strings.ToLower(strings.TrimSpace(status)),
		RefundAmountMinor:   amount,
		Currency:            currency,
		PlatformUpdatedAt:   updatedAt,
		RawPayload:          append([]byte(nil), ev.Raw...),
	}, nil
}

func (s *Service) UpsertPlatformAfterSale(ctx context.Context, in PlatformAfterSaleInput) error {
	if err := s.ready(); err != nil {
		return err
	}
	if err := validatePlatformAfterSaleInput(in); err != nil {
		return err
	}
	hash := platformAfterSaleHash(in.RawPayload)
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existingEvent PlatformAfterSaleEvent
		eventQuery := tx.Where("tenant_id = ? AND platform = ? AND platform_shop_id = ? AND event_id = ?", in.TenantID, in.Platform, in.PlatformShopID, in.EventID).First(&existingEvent)
		if eventQuery.Error == nil {
			if existingEvent.PayloadHash != hash {
				return ErrPlatformEventConflict
			}
			return nil
		}
		if !errors.Is(eventQuery.Error, gorm.ErrRecordNotFound) {
			return fmt.Errorf("load platform after-sale event: %w", eventQuery.Error)
		}

		var row PlatformAfterSale
		rowQuery := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"tenant_id = ? AND platform = ? AND platform_shop_id = ? AND external_after_sale_id = ?",
			in.TenantID, in.Platform, in.PlatformShopID, in.ExternalAfterSaleID,
		).First(&row)
		if rowQuery.Error != nil && !errors.Is(rowQuery.Error, gorm.ErrRecordNotFound) {
			return fmt.Errorf("load platform after-sale: %w", rowQuery.Error)
		}

		resolvedShopID, orderID, returnID, reconciliationStatus, reconciliationReason, err := resolvePlatformAfterSale(ctx, tx, in)
		if err != nil {
			return err
		}
		if resolvedShopID != nil {
			in.InternalShopID = resolvedShopID
		}
		stale := rowQuery.Error == nil && isStalePlatformAfterSale(row.PlatformUpdatedAt, in.PlatformUpdatedAt)
		if rowQuery.Error != nil {
			row = PlatformAfterSale{
				TenantID: in.TenantID, Platform: in.Platform, InternalShopID: in.InternalShopID,
				PlatformShopID: in.PlatformShopID, ExternalAfterSaleID: in.ExternalAfterSaleID,
				ExternalOrderID: in.ExternalOrderID, PlatformType: in.PlatformType, PlatformStatus: in.PlatformStatus,
				RefundAmountMinor: in.RefundAmountMinor, Currency: in.Currency, PlatformUpdatedAt: in.PlatformUpdatedAt,
				OrderID: orderID, SalesReturnID: returnID, ReconciliationStatus: reconciliationStatus,
				ReconciliationReason: reconciliationReason, LastEventID: in.EventID, LastPayloadHash: hash,
				RawPayload: datatypes.JSON(append([]byte(nil), in.RawPayload...)),
			}
			if err := tx.Create(&row).Error; err != nil {
				return fmt.Errorf("create platform after-sale: %w", err)
			}
		} else if !stale {
			updates := map[string]any{
				"internal_shop_id": in.InternalShopID, "external_order_id": in.ExternalOrderID,
				"platform_type": in.PlatformType, "platform_status": in.PlatformStatus,
				"refund_amount_minor": in.RefundAmountMinor, "currency": in.Currency,
				"platform_updated_at": in.PlatformUpdatedAt, "order_id": orderID, "sales_return_id": returnID,
				"reconciliation_status": reconciliationStatus, "reconciliation_reason": reconciliationReason,
				"last_event_id": in.EventID, "last_payload_hash": hash, "raw_payload": datatypes.JSON(append([]byte(nil), in.RawPayload...)),
				"updated_at": time.Now().UTC(),
			}
			if err := tx.Model(&row).Updates(updates).Error; err != nil {
				return fmt.Errorf("update platform after-sale: %w", err)
			}
		}

		event := &PlatformAfterSaleEvent{
			TenantID: in.TenantID, Platform: in.Platform, PlatformShopID: in.PlatformShopID,
			EventID: in.EventID, EventType: in.EventType, PlatformAfterSaleID: row.ID,
			PayloadHash: hash, Applied: !stale, RawPayload: datatypes.JSON(append([]byte(nil), in.RawPayload...)),
		}
		if stale {
			event.IgnoredReason = "stale_platform_update"
		}
		if err := tx.Create(event).Error; err != nil {
			if isUniqueViolation(err) {
				var existing PlatformAfterSaleEvent
				if loadErr := tx.Where("tenant_id = ? AND platform = ? AND platform_shop_id = ? AND event_id = ?", in.TenantID, in.Platform, in.PlatformShopID, in.EventID).First(&existing).Error; loadErr == nil && existing.PayloadHash == hash {
					return nil
				}
				return ErrPlatformEventConflict
			}
			return fmt.Errorf("create platform after-sale event: %w", err)
		}
		return nil
	})
}

func (s *Service) ListPlatformAfterSales(ctx context.Context, q PlatformAfterSaleListQuery) (*PlatformAfterSaleListResult, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 || q.PageSize > 100 {
		q.PageSize = 20
	}
	tx := s.DB.WithContext(ctx).Model(&PlatformAfterSale{}).Where("platform_after_sales.tenant_id = ?", q.TenantID)
	tx = tx.Joins("LEFT JOIN orders ON orders.id = platform_after_sales.order_id AND orders.tenant_id = platform_after_sales.tenant_id AND orders.deleted_at IS NULL").Joins("LEFT JOIN sales_returns ON sales_returns.id = platform_after_sales.sales_return_id AND sales_returns.tenant_id = platform_after_sales.tenant_id AND sales_returns.deleted_at IS NULL")
	if value := strings.TrimSpace(q.Platform); value != "" {
		tx = tx.Where("platform_after_sales.platform = ?", value)
	}
	if value := strings.TrimSpace(q.PlatformShopID); value != "" {
		tx = tx.Where("platform_after_sales.platform_shop_id = ?", value)
	}
	if q.InternalShopID != nil && *q.InternalShopID != uuid.Nil {
		tx = tx.Where("platform_after_sales.internal_shop_id = ?", *q.InternalShopID)
	}
	if q.RestrictStoreScope {
		if len(q.AllowedShopIDs) == 0 {
			tx = tx.Where("1 = 0")
		} else {
			tx = tx.Where("platform_after_sales.internal_shop_id IN ?", q.AllowedShopIDs)
		}
	}
	if value := strings.TrimSpace(q.ExternalOrderID); value != "" {
		tx = tx.Where("platform_after_sales.external_order_id ILIKE ? OR orders.order_no ILIKE ?", "%"+value+"%", "%"+value+"%")
	}
	if value := strings.TrimSpace(q.PlatformStatus); value != "" {
		tx = tx.Where("platform_after_sales.platform_status = ?", value)
	}
	if value := strings.TrimSpace(q.ReconciliationStatus); value != "" {
		if !validPlatformReconciliationStatus(value) {
			return nil, ErrInvalidInput
		}
		tx = tx.Where("platform_after_sales.reconciliation_status = ?", value)
	}
	if q.Start != nil {
		tx = tx.Where("platform_after_sales.created_at >= ?", *q.Start)
	}
	if q.End != nil {
		tx = tx.Where("platform_after_sales.created_at <= ?", *q.End)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count platform after-sales: %w", err)
	}
	rows := make([]PlatformAfterSale, 0, q.PageSize)
	if err := tx.Select("platform_after_sales.*, orders.order_no, sales_returns.return_no").Order("platform_after_sales.updated_at DESC, platform_after_sales.id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list platform after-sales: %w", err)
	}
	return &PlatformAfterSaleListResult{List: rows, Page: q.Page, PageSize: q.PageSize, Total: total, TotalPages: int((total + int64(q.PageSize) - 1) / int64(q.PageSize))}, nil
}

func (s *Service) GetPlatformAfterSale(ctx context.Context, tenantID int64, id uuid.UUID) (*PlatformAfterSale, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if tenantID < 1 || id == uuid.Nil {
		return nil, ErrAbsent
	}
	var row PlatformAfterSale
	err := s.DB.WithContext(ctx).Model(&PlatformAfterSale{}).Where("platform_after_sales.id = ? AND platform_after_sales.tenant_id = ?", id, tenantID).Joins("LEFT JOIN orders ON orders.id = platform_after_sales.order_id AND orders.tenant_id = platform_after_sales.tenant_id AND orders.deleted_at IS NULL").Joins("LEFT JOIN sales_returns ON sales_returns.id = platform_after_sales.sales_return_id AND sales_returns.tenant_id = platform_after_sales.tenant_id AND sales_returns.deleted_at IS NULL").Select("platform_after_sales.*, orders.order_no, sales_returns.return_no").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAbsent
	}
	if err != nil {
		return nil, fmt.Errorf("get platform after-sale: %w", err)
	}
	var events []PlatformAfterSaleEvent
	if err := s.DB.WithContext(ctx).Where("platform_after_sale_id = ? AND tenant_id = ?", row.ID, tenantID).Order("created_at DESC, id DESC").Limit(20).Find(&events).Error; err != nil {
		return nil, fmt.Errorf("list platform after-sale events: %w", err)
	}
	row.Events = events
	return &row, nil
}

func resolvePlatformAfterSale(ctx context.Context, tx *gorm.DB, in PlatformAfterSaleInput) (*uuid.UUID, *uuid.UUID, *uuid.UUID, string, string, error) {
	shopID, shopStatus, shopReason, err := resolvePlatformShop(ctx, tx, in)
	if err != nil {
		return nil, nil, nil, "", "", err
	}
	if shopID == nil {
		return nil, nil, nil, shopStatus, shopReason, nil
	}
	var orders []ordermod.Order
	orderQuery := tx.WithContext(ctx).Where("tenant_id = ? AND platform = ? AND external_order_id = ? AND shop_id = ?", in.TenantID, in.Platform, in.ExternalOrderID, *shopID)
	if err := orderQuery.Order("created_at ASC, id ASC").Limit(2).Find(&orders).Error; err != nil {
		return shopID, nil, nil, "", "", fmt.Errorf("resolve platform order: %w", err)
	}
	if len(orders) == 0 {
		return shopID, nil, nil, PlatformReconciliationBlocked, "local_order_missing", nil
	}
	if len(orders) > 1 {
		return shopID, nil, nil, PlatformReconciliationBlocked, "multiple_local_orders", nil
	}
	orderID := orders[0].ID
	var returns []SalesReturn
	if err := tx.WithContext(ctx).Where("tenant_id = ? AND order_id = ? AND status <> ?", in.TenantID, orderID, StatusCancelled).Order("created_at ASC, id ASC").Limit(2).Find(&returns).Error; err != nil {
		return shopID, &orderID, nil, "", "", fmt.Errorf("resolve local sales return: %w", err)
	}
	if len(returns) == 0 {
		return shopID, &orderID, nil, PlatformReconciliationMismatch, "local_sales_return_missing", nil
	}
	if len(returns) > 1 {
		return shopID, &orderID, nil, PlatformReconciliationMismatch, "multiple_local_sales_returns", nil
	}
	local := returns[0]
	if local.RefundAmountMinor != in.RefundAmountMinor || strings.ToUpper(local.Currency) != strings.ToUpper(in.Currency) || !platformTypesMatch(local.Type, in.PlatformType) {
		return shopID, &orderID, &local.ID, PlatformReconciliationMismatch, "local_amount_type_or_currency_mismatch", nil
	}
	return shopID, &orderID, &local.ID, PlatformReconciliationMatched, "matched_order_and_sales_return", nil
}

// resolvePlatformShop turns the provider shop identifier into a tenant-scoped
// internal shop reference before any order matching is attempted. A missing or
// conflicting mapping is retained as a blocked provider fact rather than
// risking a match against another shop's order.
func resolvePlatformShop(ctx context.Context, tx *gorm.DB, in PlatformAfterSaleInput) (*uuid.UUID, string, string, error) {
	query := tx.WithContext(ctx).Model(&shopmod.Shop{}).Where(
		"tenant_id = ? AND platform = ? AND external_shop_id = ?",
		in.TenantID, in.Platform, in.PlatformShopID,
	)
	if in.InternalShopID != nil && *in.InternalShopID != uuid.Nil {
		query = query.Where("id = ?", *in.InternalShopID)
	}
	var shops []shopmod.Shop
	if err := query.Order("created_at ASC, id ASC").Limit(2).Find(&shops).Error; err != nil {
		return nil, "", "", fmt.Errorf("resolve platform shop: %w", err)
	}
	if len(shops) == 0 {
		if in.InternalShopID != nil && *in.InternalShopID != uuid.Nil {
			return nil, PlatformReconciliationBlocked, "local_shop_mismatch", nil
		}
		return nil, PlatformReconciliationBlocked, "local_shop_missing", nil
	}
	if len(shops) > 1 {
		return nil, PlatformReconciliationBlocked, "multiple_local_shops", nil
	}
	id := shops[0].ID
	return &id, "", "", nil
}

func platformTypesMatch(local, platform string) bool {
	local, platform = strings.ToLower(strings.TrimSpace(local)), strings.ToLower(strings.TrimSpace(platform))
	if local == platform {
		return true
	}
	if strings.Contains(platform, "refund") && local == TypeRefundOnly {
		return true
	}
	if strings.Contains(platform, "return") && local == TypeReturnRefund {
		return true
	}
	return false
}

func validatePlatformAfterSaleInput(in PlatformAfterSaleInput) error {
	if in.TenantID < 1 || strings.TrimSpace(in.Platform) == "" || strings.TrimSpace(in.PlatformShopID) == "" || strings.TrimSpace(in.EventID) == "" || strings.TrimSpace(in.EventType) == "" || strings.TrimSpace(in.ExternalAfterSaleID) == "" || strings.TrimSpace(in.ExternalOrderID) == "" || strings.TrimSpace(in.PlatformType) == "" || strings.TrimSpace(in.PlatformStatus) == "" || in.RefundAmountMinor < 0 || in.RefundAmountMinor > maxRefundMinor || strings.TrimSpace(in.Currency) == "" || !json.Valid(in.RawPayload) {
		return ErrPlatformAfterSaleInvalid
	}
	return nil
}

func isStalePlatformAfterSale(current, incoming *time.Time) bool {
	return current != nil && incoming != nil && incoming.Before(*current)
}

func platformAfterSaleHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func isUniqueViolation(err error) bool {
	return err != nil && (strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "duplicate"))
}

func stringValue(data map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := data[key]; ok {
			switch v := value.(type) {
			case string:
				if result := strings.TrimSpace(v); result != "" {
					return result
				}
			case json.Number:
				return v.String()
			case float64:
				return strconv.FormatFloat(v, 'f', -1, 64)
			}
		}
	}
	for _, nestedKey := range []string{"data", "after_sale", "afterSale", "refund"} {
		if nested, ok := data[nestedKey].(map[string]any); ok {
			if value := stringValue(nested, keys...); value != "" {
				return value
			}
		}
	}
	return ""
}

func parseRefundAmount(data map[string]any) (int64, error) {
	for _, key := range []string{"refund_amount_minor", "refundAmountMinor", "amount_minor", "amountMinor"} {
		if value, ok := data[key]; ok {
			return parseMinorInteger(value)
		}
	}
	for _, key := range []string{"refund_amount", "refundAmount", "amount", "refund_fee", "refundFee"} {
		if value, ok := data[key]; ok {
			return parseDecimalMinor(value)
		}
	}
	for _, nestedKey := range []string{"data", "after_sale", "afterSale", "refund"} {
		if nested, ok := data[nestedKey].(map[string]any); ok {
			if amount, err := parseRefundAmount(nested); err == nil {
				return amount, nil
			}
		}
	}
	return 0, fmt.Errorf("amount missing")
}

func parseMinorInteger(value any) (int64, error) {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" {
		return 0, fmt.Errorf("amount missing")
	}
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid minor amount")
	}
	return n, nil
}

func parseDecimalMinor(value any) (int64, error) {
	text := strings.TrimSpace(fmt.Sprint(value))
	if f, ok := value.(float64); ok {
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, fmt.Errorf("invalid decimal amount")
		}
		text = strconv.FormatFloat(f, 'f', -1, 64)
	}
	if text == "" || strings.HasPrefix(text, "-") {
		return 0, fmt.Errorf("invalid decimal amount")
	}
	parts := strings.Split(text, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, fmt.Errorf("invalid decimal amount")
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole < 0 || whole > maxRefundMinor/100 {
		return 0, fmt.Errorf("invalid decimal amount")
	}
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}
	if len(frac) > 2 || strings.Trim(frac, "0123456789") != "" {
		return 0, fmt.Errorf("decimal amount has more than two fraction digits")
	}
	frac = (frac + "00")[:2]
	minor := whole*100 + int64(parseDigits(frac))
	if whole > (maxRefundMinor-int64(parseDigits(frac)))/100 {
		return 0, fmt.Errorf("amount too large")
	}
	return minor, nil
}

func parseDigits(value string) int {
	if value == "" {
		return 0
	}
	n, _ := strconv.Atoi((value + "00")[:2])
	return n
}

func parseProviderTime(data map[string]any, keys ...string) (*time.Time, error) {
	value := stringValue(data, keys...)
	if value == "" {
		return nil, nil
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		t := time.Unix(seconds, 0).UTC()
		return &t, nil
	}
	if decoded, err := url.QueryUnescape(value); err == nil {
		value = decoded
	}
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, value); err == nil {
			t = t.UTC()
			return &t, nil
		}
	}
	return nil, fmt.Errorf("invalid timestamp")
}
