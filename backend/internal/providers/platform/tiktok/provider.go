package tiktok

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

type tikTokProvider struct{}

// NewProvider constructs the TikTok Shop platform integration.
func NewProvider() platformp.OrderSyncProvider { return tikTokProvider{} }

func (tikTokProvider) Platform() string { return "tiktok" }

func (tikTokProvider) Name() string { return "TikTok Shop" }

func (tikTokProvider) Status() string { return platformp.StatusBeta }

func (tikTokProvider) Capabilities() []platformp.Capability {
	return []platformp.Capability{platformp.CapOrderSync, platformp.CapShopInfo, platformp.CapCustomerMessage}
}

func (tikTokProvider) AuthSchema() platformp.AuthSchema {
	return platformp.AuthSchema{
		AuthType: "oauth2",
		Fields: []platformp.AuthField{
			{Name: "appKey", Label: "覆盖 App Key（可选）", Type: "text", Required: false, Sensitive: false, Hint: "默认读取「设置 → 平台开放配置」中的 platform_tiktok.app_key；仅多应用调试时填写"},
			{Name: "appSecret", Label: "覆盖 App Secret（可选）", Type: "password", Required: false, Sensitive: true, Hint: "默认读取平台配置中的 app_secret（加密存储）；留空不覆盖"},
			{Name: "redirectUri", Label: "覆盖 Redirect URI（可选）", Type: "text", Required: false, Sensitive: false, Hint: "默认读取平台配置的 redirect_uri；须与 Partner Center 登记一致"},
		},
	}
}

func (tikTokProvider) AppConfigSchema() platformp.PlatformAppConfigSchema {
	return platformp.TikTokShopAppConfigSchema()
}

func (tikTokProvider) TestConnection(ctx context.Context, req platformp.TestConnectionRequest) (*platformp.TestConnectionResult, error) {
	if strings.TrimSpace(req.AccessToken) == "" && strings.TrimSpace(req.RefreshToken) == "" {
		return &platformp.TestConnectionResult{OK: false, Message: "unauthorized: save tokens or complete OAuth"}, nil
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	access, a2, err := ensureFreshAccess(cctx, uuid.Nil, req)
	if err != nil {
		return &platformp.TestConnectionResult{OK: false, Message: err.Error()}, nil
	}

	cipher, name, extID, region, currency, err := PrimaryAuthorizedShop(cctx, a2, access)
	if err != nil {
		return &platformp.TestConnectionResult{OK: false, Message: err.Error()}, nil
	}
	return &platformp.TestConnectionResult{
		OK:               true,
		Message:          "tiktok connection ok",
		ShopName:         name,
		ExternalShopID:   extID,
		Region:           region,
		Currency:         currency,
		SellerMerchantID: cipher,
	}, nil
}

func (tikTokProvider) SyncOrders(ctx context.Context, req platformp.SyncOrdersRequest) (*platformp.SyncOrdersResult, error) {
	if req.ShopID == uuid.Nil {
		return nil, fmt.Errorf("shop id required")
	}

	access, a2, err := ensureFreshAccess(ctx, req.ShopID, req.Auth)
	if err != nil {
		return nil, err
	}

	orders, next, more, sum, err := OrdersSearch(ctx, a2, access, strings.TrimSpace(req.Cursor), req.Limit, req.StartTime, req.EndTime)
	if err != nil {
		_ = setAuthStatusMaybe(ctx, req.ShopID, "error")
		return nil, err
	}

	raw := map[string]any{
		"provider":       "tiktok",
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

func (tikTokProvider) PullMessages(ctx context.Context, req platformp.PullMessagesRequest) (*platformp.PullMessagesResult, error) {
	_ = ctx
	_ = req
	return nil, platformp.ErrCustomerMessageNotImplemented
}

func (tikTokProvider) SendMessage(ctx context.Context, req platformp.SendMessageRequest) (*platformp.SendMessageResult, error) {
	_ = ctx
	_ = req
	return nil, platformp.ErrCustomerMessageNotImplemented
}
