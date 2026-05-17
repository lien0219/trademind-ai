package shopee

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

type shopeeProvider struct{}

// NewProvider constructs the Shopee platform integration (beta).
func NewProvider() platformp.OrderSyncProvider { return shopeeProvider{} }

func (shopeeProvider) Platform() string { return "shopee" }

func (shopeeProvider) Name() string { return "Shopee" }

func (shopeeProvider) Status() string { return platformp.StatusBeta }

func (shopeeProvider) Capabilities() []platformp.Capability {
	return []platformp.Capability{platformp.CapOrderSync, platformp.CapShopInfo, platformp.CapCustomerMessage}
}

func (shopeeProvider) AuthSchema() platformp.AuthSchema {
	return platformp.AuthSchema{
		AuthType: "oauth2",
		Fields: []platformp.AuthField{
			{Name: "appKey", Label: "覆盖 Partner ID（可选）", Type: "text", Required: false, Sensitive: false, Hint: "默认读取「设置 → 平台开放配置」platform_shopee.partner_id"},
			{Name: "appSecret", Label: "覆盖 Partner Key（可选）", Type: "password", Required: false, Sensitive: true, Hint: "默认读取平台配置 partner_key；留空不覆盖"},
			{Name: "redirectUri", Label: "覆盖 Redirect URI（可选）", Type: "text", Required: false, Sensitive: false, Hint: "默认读取平台配置的 redirect_uri"},
		},
	}
}

func (shopeeProvider) AppConfigSchema() platformp.PlatformAppConfigSchema {
	return platformp.ShopeeAppConfigSchema()
}

func (shopeeProvider) TestConnection(ctx context.Context, req platformp.TestConnectionRequest) (*platformp.TestConnectionResult, error) {
	if _, err := ResolveRuntime(req); err != nil {
		return &platformp.TestConnectionResult{OK: false, Message: err.Error()}, nil
	}
	sid, err := parseShopID(req)
	if err != nil {
		return &platformp.TestConnectionResult{OK: false, Message: err.Error()}, nil
	}
	if strings.TrimSpace(req.AccessToken) == "" && strings.TrimSpace(req.RefreshToken) == "" {
		return &platformp.TestConnectionResult{OK: false, Message: "unauthorized: save tokens or complete OAuth"}, nil
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	access, _, err := ensureFreshAccess(cctx, uuid.Nil, req)
	if err != nil {
		return &platformp.TestConnectionResult{OK: false, Message: err.Error()}, nil
	}
	cfg, err := ResolveRuntime(req)
	if err != nil {
		return &platformp.TestConnectionResult{OK: false, Message: err.Error()}, nil
	}
	name, region, currency, ext, err := GetShopInfo(cctx, cfg, sid, access)
	if err != nil {
		return &platformp.TestConnectionResult{OK: false, Message: err.Error()}, nil
	}
	return &platformp.TestConnectionResult{
		OK:             true,
		Message:        "shopee connection ok",
		ShopName:       name,
		ExternalShopID: ext,
		Region:         region,
		Currency:       currency,
	}, nil
}

func (p shopeeProvider) SyncOrders(ctx context.Context, req platformp.SyncOrdersRequest) (*platformp.SyncOrdersResult, error) {
	_ = p
	if req.ShopID == uuid.Nil {
		return nil, fmt.Errorf("shop id required")
	}

	access, a2, err := ensureFreshAccess(ctx, req.ShopID, req.Auth)
	if err != nil {
		return nil, err
	}
	sid, err := parseShopID(a2)
	if err != nil {
		_ = setAuthStatusMaybe(ctx, req.ShopID, "error")
		return nil, err
	}

	orders, next, more, sum, err := SyncOrders(ctx, a2, sid, access, strings.TrimSpace(req.Cursor), req.Limit, req.StartTime, req.EndTime)
	if err != nil {
		_ = setAuthStatusMaybe(ctx, req.ShopID, "error")
		return nil, err
	}

	raw := map[string]any{
		"provider":       "shopee",
		"returnedOrders": len(orders),
		"hasMore":        more,
		"receivedAt":     time.Now().UTC().Format(time.RFC3339),
	}
	for k, v := range sum {
		raw[k] = v
	}
	return &platformp.SyncOrdersResult{
		Orders:     orders,
		NextCursor: next,
		HasMore:    more,
		RawSummary: raw,
	}, nil
}

func (shopeeProvider) PullMessages(ctx context.Context, req platformp.PullMessagesRequest) (*platformp.PullMessagesResult, error) {
	return PullCustomerMessages(ctx, req)
}

func (shopeeProvider) SendMessage(ctx context.Context, req platformp.SendMessageRequest) (*platformp.SendMessageResult, error) {
	return SendCustomerMessage(ctx, req)
}
