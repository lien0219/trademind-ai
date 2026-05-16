package platform

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type manualProv struct{}

func newManualProvider() Provider { return manualProv{} }

func (manualProv) Platform() string { return "manual" }

func (manualProv) Name() string { return "手工店铺" }

func (manualProv) Status() string { return StatusAvailable }

func (manualProv) Capabilities() []Capability {
	return []Capability{CapManualManage}
}

func (manualProv) AuthSchema() AuthSchema {
	return AuthSchema{AuthType: "manual", Fields: nil}
}

func (manualProv) TestConnection(ctx context.Context, req TestConnectionRequest) (*TestConnectionResult, error) {
	_ = ctx
	_ = req
	return &TestConnectionResult{OK: true, Message: "manual shop does not require remote authorization"}, nil
}

func (manualProv) SyncOrders(ctx context.Context, req SyncOrdersRequest) (*SyncOrdersResult, error) {
	_ = ctx
	_ = req
	return nil, ErrManualOrderSyncUnsupported
}

type mockProv struct{}

func newMockProvider() Provider { return mockProv{} }

func (mockProv) Platform() string { return "mock" }

func (mockProv) Name() string { return "Mock 店铺（开发测试）" }

func (mockProv) Status() string { return StatusAvailable }

func (mockProv) Capabilities() []Capability {
	return []Capability{CapOrderSync, CapCustomerMessage, CapProductPublish}
}

func (mockProv) AuthSchema() AuthSchema {
	return AuthSchema{
		AuthType: "token",
		Fields: []AuthField{
			{Name: "accessToken", Label: "Access Token（测试）", Type: "password", Required: false, Sensitive: true, Hint: "任意非空即可通过测试连接"},
			{Name: "refreshToken", Label: "Refresh Token（测试）", Type: "password", Required: false, Sensitive: true},
		},
	}
}

func (mockProv) TestConnection(ctx context.Context, req TestConnectionRequest) (*TestConnectionResult, error) {
	_ = ctx
	if req.AccessToken == "" && req.RefreshToken == "" {
		return &TestConnectionResult{OK: true, Message: "mock: credentials optional; connection check OK"}, nil
	}
	return &TestConnectionResult{OK: true, Message: "mock: connection check OK"}, nil
}

func (mockProv) SyncOrders(ctx context.Context, req SyncOrdersRequest) (*SyncOrdersResult, error) {
	_ = ctx
	lim := req.Limit
	if lim <= 0 {
		lim = 50
	}

	base := mockCatalogOrders(req.Platform)
	out := base
	if lim < len(out) {
		out = out[:lim]
	}

	return &SyncOrdersResult{
		Orders:     out,
		NextCursor: "",
		HasMore:    false,
		RawSummary: map[string]any{
			"provider":    "mock",
			"returned":    len(out),
			"shopId":      req.ShopID.String(),
			"limit":       lim,
			"mode":        req.Mode,
			"generatedAt": time.Now().UTC().Format(time.RFC3339),
		},
	}, nil
}

// Deterministic mock orders for repeat upsert testing (same external IDs every sync).
func mockCatalogOrders(platformKey string) []PlatformOrder {
	cur := strings.TrimSpace(platformKey)
	if cur == "" {
		cur = "mock"
	}
	now := time.Now().UTC().Truncate(time.Second)

	o1Ordered := now.Add(-72 * time.Hour)
	o1Paid := now.Add(-71 * time.Hour)
	o2Ordered := now.Add(-48 * time.Hour)
	o3Ordered := now.Add(-5 * time.Hour)

	return []PlatformOrder{
		{
			ExternalOrderID:   "mock-ext-order-001",
			OrderNo:           fmt.Sprintf("MOCK-%s-ORDER-001", cur),
			CustomerName:      "Mock Buyer Alice",
			Status:            "paid",
			PaymentStatus:     "paid",
			FulfillmentStatus: "fulfilled",
			Currency:          "USD",
			TotalAmount:       129.90,
			OrderedAt:         &o1Ordered,
			PaidAt:            &o1Paid,
			ShippedAt:         ptrTime(now.Add(-70 * time.Hour)),
			DeliveredAt:       ptrTime(now.Add(-12 * time.Hour)),
			Items: []PlatformOrderItem{
				{
					ExternalItemID: "mock-item-001-a",
					ProductTitle:   "Mock SKU Pack A",
					SKUName:        "Color: Blue",
					SKUCode:        "MOCK-SKU-A",
					Quantity:       2,
					UnitPrice:      49.95,
					TotalPrice:     99.90,
					ImageURL:       "https://example.com/mock/a.png",
					Attrs:          map[string]any{"color": "blue"},
				},
				{
					ExternalItemID: "mock-item-001-b",
					ProductTitle:   "Mock Addon",
					SKUCode:        "MOCK-ADDON",
					Quantity:       1,
					UnitPrice:      30,
					TotalPrice:     30,
				},
			},
			Shipments: []PlatformShipment{
				{
					Carrier:     "MockExpress",
					TrackingNo:  "MOCKTRACK001",
					TrackingURL: "https://example.com/track/MOCKTRACK001",
					Status:      "delivered",
					ShippedAt:   ptrTime(now.Add(-70 * time.Hour)),
					DeliveredAt: ptrTime(now.Add(-12 * time.Hour)),
				},
			},
			RawData: map[string]any{"mock": true, "tier": "catalog"},
		},
		{
			ExternalOrderID:   "mock-ext-order-002",
			OrderNo:           fmt.Sprintf("MOCK-%s-ORDER-002", cur),
			CustomerName:      "Mock Buyer Bob",
			Status:            "processing",
			PaymentStatus:     "paid",
			FulfillmentStatus: "partial",
			Currency:          "USD",
			TotalAmount:       59,
			OrderedAt:         &o2Ordered,
			PaidAt:            ptrTime(o2Ordered.Add(time.Hour)),
			Items: []PlatformOrderItem{
				{
					ExternalItemID: "mock-item-002-a",
					ProductTitle:   "Mock Gadget",
					SKUCode:        "MOCK-GADGET",
					Quantity:       1,
					UnitPrice:      59,
					TotalPrice:     59,
				},
			},
			Shipments: []PlatformShipment{
				{
					Carrier:    "MockExpress",
					TrackingNo: "MOCKTRACK002",
					Status:     "in_transit",
					ShippedAt:  ptrTime(now.Add(-36 * time.Hour)),
				},
			},
			RawData: map[string]any{"mock": true},
		},
		{
			ExternalOrderID:   "mock-ext-order-003",
			OrderNo:           fmt.Sprintf("MOCK-%s-ORDER-003", cur),
			CustomerName:      "Mock Buyer Chen",
			Status:            "pending",
			PaymentStatus:     "unpaid",
			FulfillmentStatus: "unfulfilled",
			Currency:          "EUR",
			TotalAmount:       24.5,
			OrderedAt:         &o3Ordered,
			Items: []PlatformOrderItem{
				{
					ExternalItemID: "mock-item-003-a",
					ProductTitle:   "Mock Lightweight Item",
					SKUCode:        "MOCK-LITE",
					Quantity:       1,
					UnitPrice:      24.5,
					TotalPrice:     24.5,
				},
			},
			Shipments: []PlatformShipment{},
			RawData:   map[string]any{"mock": true},
		},
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

// plannedProv is a placeholder provider with no live API.
type plannedProv struct {
	platformKey  string
	displayName  string
	status       string
	authType     string
	caps         []Capability
	schemaFields []AuthField
}

func newPlannedProvider(platformID, displayName, status, authType string, caps []Capability, fields []AuthField) *plannedProv {
	return &plannedProv{
		platformKey:  platformID,
		displayName:  displayName,
		status:       status,
		authType:     authType,
		caps:         caps,
		schemaFields: fields,
	}
}

func (p *plannedProv) Platform() string { return p.platformKey }

func (p *plannedProv) Name() string { return p.displayName }

func (p *plannedProv) Status() string { return p.status }

func (p *plannedProv) Capabilities() []Capability {
	out := make([]Capability, len(p.caps))
	copy(out, p.caps)
	return out
}

func (p *plannedProv) AuthSchema() AuthSchema {
	fields := p.schemaFields
	if fields == nil {
		fields = []AuthField{}
	}
	cp := make([]AuthField, len(fields))
	copy(cp, fields)
	return AuthSchema{AuthType: p.authType, Fields: cp}
}

func (p *plannedProv) TestConnection(ctx context.Context, req TestConnectionRequest) (*TestConnectionResult, error) {
	_ = ctx
	_ = req
	return nil, ErrNotImplemented
}

func (p *plannedProv) SyncOrders(ctx context.Context, req SyncOrdersRequest) (*SyncOrdersResult, error) {
	_ = p
	_ = ctx
	_ = req
	return nil, ErrOrderSyncNotImplemented
}
