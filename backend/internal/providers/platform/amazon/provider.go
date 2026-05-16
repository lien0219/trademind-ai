package amazon

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

type amazonProvider struct{}

// NewProvider constructs the Amazon SP-API integration (beta).
func NewProvider() platformp.OrderSyncProvider { return amazonProvider{} }

func (amazonProvider) Platform() string { return "amazon" }

func (amazonProvider) Name() string { return "Amazon SP-API" }

func (amazonProvider) Status() string { return platformp.StatusBeta }

func (amazonProvider) Capabilities() []platformp.Capability {
	return []platformp.Capability{
		platformp.CapOrderSync,
		platformp.CapShopInfo,
	}
}

func (amazonProvider) AuthSchema() platformp.AuthSchema {
	return platformp.AuthSchema{AuthType: "oauth2", Fields: []platformp.AuthField{}}
}

func (amazonProvider) AppConfigSchema() platformp.PlatformAppConfigSchema {
	return platformp.AmazonSPAPIAppConfigSchema()
}

func (amazonProvider) TestConnection(ctx context.Context, req platformp.TestConnectionRequest) (*platformp.TestConnectionResult, error) {
	cfg, err := ResolveRuntime(req)
	if err != nil {
		return &platformp.TestConnectionResult{OK: false, Message: err.Error()}, nil
	}
	if strings.TrimSpace(req.AccessToken) == "" && strings.TrimSpace(req.RefreshToken) == "" {
		return &platformp.TestConnectionResult{OK: false, Message: "unauthorized: save tokens or complete OAuth"}, nil
	}
	if EffectiveMarketplaceID(req, cfg) == "" {
		return &platformp.TestConnectionResult{OK: false, Message: "missing marketplace_id (shop token or platform_amazon.marketplace_id)"}, nil
	}
	timeout := cfg.HTTPTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	access, _, err := ensureFreshAccess(cctx, uuid.Nil, req)
	if err != nil {
		return &platformp.TestConnectionResult{OK: false, Message: err.Error()}, nil
	}
	res, err := MarketplaceParticipationsProbe(cctx, cfg, access)
	if err != nil {
		return &platformp.TestConnectionResult{OK: false, Message: err.Error()}, nil
	}
	if res != nil && res.OK {
		if sid := strings.TrimSpace(req.SellerID); sid != "" {
			res.ExternalShopID = sid
			res.SellerMerchantID = sid
		}
		if mp := EffectiveMarketplaceID(req, cfg); mp != "" {
			res.Message = fmt.Sprintf("amazon SP-API ok (marketplace %s)", mp)
		}
	}
	return res, nil
}

func (p amazonProvider) SyncOrders(ctx context.Context, req platformp.SyncOrdersRequest) (*platformp.SyncOrdersResult, error) {
	_ = p
	if req.ShopID == uuid.Nil {
		return nil, fmt.Errorf("shop id required")
	}
	cfg, err := ResolveRuntime(req.Auth)
	if err != nil {
		return nil, err
	}
	mp := EffectiveMarketplaceID(req.Auth, cfg)
	if mp == "" {
		return nil, fmt.Errorf("missing marketplace_id")
	}
	access, _, err := ensureFreshAccess(ctx, req.ShopID, req.Auth)
	if err != nil {
		return nil, err
	}
	rawOrders, next, err := FetchOrdersPage(ctx, cfg, access, mp, req.Cursor, req.Limit, req.StartTime, req.EndTime)
	if err != nil {
		_ = setAuthStatusMaybe(ctx, req.ShopID, "error")
		return nil, err
	}
	out := make([]platformp.PlatformOrder, 0, len(rawOrders))
	for _, ro := range rawOrders {
		oid := strFromAny(ro["AmazonOrderId"])
		if oid == "" {
			continue
		}
		items, ierr := FetchOrderItems(ctx, cfg, access, oid)
		if ierr != nil {
			_ = setAuthStatusMaybe(ctx, req.ShopID, "error")
			return nil, ierr
		}
		out = append(out, MapAmazonOrder(ro, items))
	}
	summary := map[string]any{
		"provider":       "amazon",
		"returnedOrders": len(out),
		"hasMore":        next != "",
		"marketplaceId":  mp,
	}
	return &platformp.SyncOrdersResult{
		Orders:     out,
		NextCursor: next,
		HasMore:    next != "",
		RawSummary: summary,
	}, nil
}
