package platform

import "context"

// Provider rollout / implementation status.
const (
	StatusAvailable = "available"
	StatusPlanned   = "planned"
	StatusBeta      = "beta"
	StatusDisabled  = "disabled"
)

// Capability enumerates optional platform features (extensible string tokens).
type Capability string

const (
	CapOrderSync       Capability = "order_sync"
	CapProductPublish  Capability = "product_publish"
	CapCustomerMessage Capability = "customer_message"
	CapInventorySync   Capability = "inventory_sync"
	CapLogisticsSync   Capability = "logistics_sync"
	CapRefundAfterSale Capability = "refund_after_sale"
	CapShopInfo        Capability = "shop_info"
	CapManualManage    Capability = "manual_manage"
)

// AuthField describes one dynamic auth/config input for admin UI.
type AuthField struct {
	Name      string `json:"name"`
	Label     string `json:"label"`
	Type      string `json:"type"` // text, password, textarea, number
	Required  bool   `json:"required"`
	Sensitive bool   `json:"sensitive"`
	Hint      string `json:"hint,omitempty"`
}

// AuthSchema groups auth type and fields for front-end forms.
type AuthSchema struct {
	AuthType string      `json:"authType"`
	Fields   []AuthField `json:"fields"`
}

// TestConnectionRequest passes decrypted credentials to a provider (never log secrets).
type TestConnectionRequest struct {
	AuthType      string
	AppKey        string
	AppSecret     string
	AccessToken   string
	RefreshToken  string
	SellerID      string
	MerchantID    string
	MarketplaceID string
	Extra         map[string]string // from auth_config key/value when needed
}

// TestConnectionResult is a minimal connectivity outcome.
type TestConnectionResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

// Provider describes one sales-channel integration (implemented or planned).
type Provider interface {
	Platform() string
	Name() string
	Status() string
	Capabilities() []Capability
	AuthSchema() AuthSchema
	TestConnection(ctx context.Context, req TestConnectionRequest) (*TestConnectionResult, error)
}
