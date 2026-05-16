package platform

import "context"

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
