package collectruleai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/collectrule"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

type testProfileEnricher struct {
	err       error
	called    bool
	tenantID  int64
	profileID *uuid.UUID
}

func (e *testProfileEnricher) EnrichCollectorOptions(_ context.Context, tenantID int64, _ map[string]any, profileID *uuid.UUID, _ bool, _ string) error {
	e.called = true
	e.tenantID = tenantID
	e.profileID = profileID
	return e.err
}

type testCollectorRunner struct {
	called bool
}

func (r *testCollectorRunner) RunCollect(_ context.Context, _, _ string, _ map[string]any) (json.RawMessage, error) {
	r.called = true
	return nil, nil
}

func (r *testCollectorRunner) CustomRuleTest(_ context.Context, _ string, _ map[string]any) (*collectrule.RuleTestResultDTO, error) {
	r.called = true
	return &collectrule.RuleTestResultDTO{}, nil
}

func TestRunRuleTestRejectsInvalidBrowserProfileID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/v1/collect/rules/ai-preview", nil)
	c.Set(ctxkey.TenantID, int64(71))

	profiles := &testProfileEnricher{}
	runner := &testCollectorRunner{}
	service := &Service{Profiles: profiles, Runner: runner}
	_, err := service.runRuleTest(c, "https://shop.example.test/item/1", "shop.example.test", nil, GenerateBody{
		UseBrowserProfile: true,
		ProfileID:         stringPointer("invalid-profile-id"),
	}, nil)

	if err == nil || !strings.Contains(err.Error(), "invalid UUID") {
		t.Fatalf("expected invalid profile ID error, got %v", err)
	}
	if profiles.called {
		t.Fatal("profile resolver should not be called for an invalid profile ID")
	}
	if runner.called {
		t.Fatal("collector must not run when the requested browser profile ID is invalid")
	}
}

func TestRunRuleTestStopsWhenBrowserProfileIsNotAuthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/v1/collect/rules/ai-preview", nil)
	c.Set(ctxkey.TenantID, int64(71))

	profileErr := errors.New("browser profile is not available to tenant")
	profiles := &testProfileEnricher{err: profileErr}
	runner := &testCollectorRunner{}
	service := &Service{Profiles: profiles, Runner: runner}
	profileID := uuid.NewString()
	_, err := service.runRuleTest(c, "https://shop.example.test/item/1", "shop.example.test", nil, GenerateBody{
		UseBrowserProfile: true,
		ProfileID:         &profileID,
	}, nil)

	if !errors.Is(err, profileErr) {
		t.Fatalf("expected profile authorization error, got %v", err)
	}
	if !profiles.called || profiles.tenantID != 71 {
		t.Fatalf("expected profile resolution in trusted tenant 71; called=%t tenant=%d", profiles.called, profiles.tenantID)
	}
	if runner.called {
		t.Fatal("collector must not run when browser profile resolution fails")
	}
}

func stringPointer(value string) *string {
	return &value
}
