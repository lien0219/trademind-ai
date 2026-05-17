package lazada

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func buildLazadaPriceQuantityPayload(itemID string, sellerSku string, qty int, warehouseCode string) (string, error) {
	pl := strings.TrimSpace(itemID)
	ss := strings.TrimSpace(sellerSku)
	if pl == "" || ss == "" {
		return "", fmt.Errorf("product publication sku mapping incomplete")
	}
	if qty < 0 {
		return "", fmt.Errorf("invalid stock quantity")
	}
	w := strings.TrimSpace(warehouseCode)
	skuObj := map[string]any{"SellerSku": ss}
	if w != "" {
		skuObj["WarehouseQty"] = map[string]any{
			"WarehouseQuantity": []any{
				map[string]any{
					"WarehouseCode": w,
					"Quantity":      qty,
				},
			},
		}
	} else {
		skuObj["Quantity"] = qty
	}
	wrap := map[string]any{
		"Request": map[string]any{
			"Product": map[string]any{
				"ItemId": pl,
				"Skus": map[string]any{
					"Sku": []any{skuObj},
				},
			},
		},
	}
	b, err := json.Marshal(wrap)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func postLazadaPriceQuantityUpdate(ctx context.Context, cfg RuntimeConfig, access, payload string) (map[string]any, int, error) {
	pl := strings.TrimSpace(payload)
	if pl == "" {
		return nil, 0, fmt.Errorf("lazada inventory sync: empty payload")
	}
	return signedPOSTForm(ctx, cfg, cfg.APIRESTBase, PathProductPriceQuantityUpdate, access, map[string]string{
		"payload": pl,
	})
}

func maybeRetryableLazadaInventoryTransportErr(err error) error {
	if err == nil {
		return nil
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return fmt.Errorf("lazada inventory sync: retryable: %w", err)
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "timeout") || strings.Contains(s, "timed out") ||
		strings.Contains(s, "connection reset") || strings.Contains(s, "eof") {
		return fmt.Errorf("lazada inventory sync: retryable: %w", err)
	}
	return err
}

func lazadaInventoryErrDetail(root map[string]any, err error) string {
	if root != nil {
		if msg := strings.TrimSpace(pickStr(root, "message", "detail", "msg")); msg != "" {
			return msg
		}
	}
	return strings.TrimSpace(fmt.Sprint(err))
}

func isLikelyLazadaWarehouseRelatedErr(msg string) bool {
	s := strings.ToLower(strings.TrimSpace(msg))
	if s == "" {
		return false
	}
	return strings.Contains(s, "warehouse") ||
		strings.Contains(s, "warehousecode") ||
		strings.Contains(s, "warehouse_code") ||
		strings.Contains(s, "warehouseqty") ||
		strings.Contains(s, "warehouse_qty") ||
		strings.Contains(s, "warehousequantity") ||
		strings.Contains(s, "multi warehouse") ||
		strings.Contains(s, "sellable_quantity") ||
		strings.Contains(s, "sellable qty") ||
		strings.Contains(s, "allocation") ||
		strings.Contains(s, "inventory type")
}

func mapLazadaInventorySyncTransportErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, platformp.ErrPlatformInventorySyncPermissionDenied) {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return err
	}
	if strings.Contains(strings.ToLower(err.Error()), "retryable") {
		return err
	}
	es := strings.ToLower(err.Error())
	if isPermissionLikeLazadaMessage(es) {
		return platformp.ErrPlatformInventorySyncPermissionDenied
	}
	if isLikelyLazadaWarehouseRelatedErr(es) {
		return fmt.Errorf("platform inventory config incomplete: missing warehouse_id")
	}
	return maybeRetryableLazadaInventoryTransportErr(err)
}

func mapLazadaInventoryPostErr(httpStatus int, root map[string]any, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, platformp.ErrPlatformInventorySyncPermissionDenied) {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return err
	}
	if strings.Contains(strings.ToLower(err.Error()), "retryable") {
		return err
	}
	detail := strings.TrimSpace(lazadaInventoryErrDetail(root, err))
	combined := strings.ToLower(detail + " " + err.Error())

	switch httpStatus {
	case http.StatusUnauthorized, http.StatusForbidden:
		return platformp.ErrPlatformInventorySyncPermissionDenied
	case http.StatusTooManyRequests:
		return fmt.Errorf("lazada inventory sync: retryable: rate limited (http 429)")
	default:
		if httpStatus >= 500 {
			return fmt.Errorf("lazada inventory sync: retryable: upstream error (http %d)", httpStatus)
		}
	}
	if isPermissionLikeLazadaMessage(combined) {
		return platformp.ErrPlatformInventorySyncPermissionDenied
	}
	if isLikelyLazadaWarehouseRelatedErr(combined) {
		return fmt.Errorf("platform inventory config incomplete: missing warehouse_id")
	}
	if httpStatus >= 400 && httpStatus < 500 {
		return fmt.Errorf("lazada inventory sync: %s", clampLazInvErr(trimPreview(detail+" "+err.Error(), 380)))
	}
	return err
}

func clampLazInvErr(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 420 {
		return s
	}
	return s[:420] + "..."
}
