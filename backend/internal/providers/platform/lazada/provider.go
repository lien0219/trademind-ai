package lazada

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

type lazadaProvider struct{}

// NewProvider constructs the Lazada platform integration (beta).
func NewProvider() platformp.OrderSyncProvider { return lazadaProvider{} }

func (lazadaProvider) Platform() string { return "lazada" }

func (lazadaProvider) Name() string { return "Lazada" }

func (lazadaProvider) Status() string { return platformp.StatusBeta }

func (lazadaProvider) Capabilities() []platformp.Capability {
	return []platformp.Capability{platformp.CapOrderSync, platformp.CapShopInfo, platformp.CapCustomerMessage}
}

func (lazadaProvider) AuthSchema() platformp.AuthSchema {
	return platformp.AuthSchema{
		AuthType: "oauth2",
		Fields: []platformp.AuthField{
			{Name: "appKey", Label: "覆盖 App Key（可选）", Type: "text", Required: false, Sensitive: false, Hint: "默认读取「设置 → 平台开放配置」platform_lazada.app_key"},
			{Name: "appSecret", Label: "覆盖 App Secret（可选）", Type: "password", Required: false, Sensitive: true, Hint: "默认读取平台配置 app_secret；留空不覆盖"},
			{Name: "redirectUri", Label: "覆盖 Redirect URI（可选）", Type: "text", Required: false, Sensitive: false, Hint: "默认读取平台配置的 redirect_uri"},
		},
	}
}

func (lazadaProvider) AppConfigSchema() platformp.PlatformAppConfigSchema {
	return platformp.LazadaAppConfigSchema()
}

func (lazadaProvider) TestConnection(ctx context.Context, req platformp.TestConnectionRequest) (*platformp.TestConnectionResult, error) {
	if _, err := ResolveRuntime(req); err != nil {
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
	name, sellerID, region, shortCode, err := GetSellerInfo(cctx, cfg, access)
	if err != nil {
		return &platformp.TestConnectionResult{OK: false, Message: err.Error()}, nil
	}
	ext := firstNonEmpty(shortCode, sellerID)
	return &platformp.TestConnectionResult{
		OK:             true,
		Message:        "lazada connection ok",
		ShopName:       name,
		ExternalShopID: ext,
		Region:         region,
	}, nil
}

func (p lazadaProvider) SyncOrders(ctx context.Context, req platformp.SyncOrdersRequest) (*platformp.SyncOrdersResult, error) {
	_ = p
	if req.ShopID == uuid.Nil {
		return nil, fmt.Errorf("shop id required")
	}
	access, a2, err := ensureFreshAccess(ctx, req.ShopID, req.Auth)
	if err != nil {
		return nil, err
	}

	orders, next, more, sum, err := SyncOrders(ctx, a2, access, strings.TrimSpace(req.Cursor), req.Limit, req.StartTime, req.EndTime)
	if err != nil {
		_ = setAuthStatusMaybe(ctx, req.ShopID, "error")
		return nil, err
	}

	raw := map[string]any{
		"provider":       "lazada",
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

func (lazadaProvider) PullMessages(ctx context.Context, req platformp.PullMessagesRequest) (*platformp.PullMessagesResult, error) {
	_ = ctx
	_ = req
	return nil, platformp.ErrCustomerMessageNotImplemented
}

func (lazadaProvider) SendMessage(ctx context.Context, req platformp.SendMessageRequest) (*platformp.SendMessageResult, error) {
	_ = ctx
	_ = req
	return nil, platformp.ErrCustomerMessageNotImplemented
}
