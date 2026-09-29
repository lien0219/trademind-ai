package inventory

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

func stocktakeCreateBody(fixture *warehouseLedgerFixture, key string) string {
	return fmt.Sprintf(`{"idempotencyKey":%q,"warehouseId":%q,"items":[{"productSkuId":%q}]}`, key, fixture.main.ID, fixture.sku.ID)
}

func stocktakeActionBody(revision int, key string) string {
	return fmt.Sprintf(`{"expectedRevision":%d,"idempotencyKey":%q}`, revision, key)
}

func countedStocktake(t *testing.T, fixture *warehouseLedgerFixture, key string, counted int) *InventoryStocktake {
	t.Helper()
	row := createStocktake(t, fixture, key)
	row, err := fixture.service.UpdateInventoryStocktakeItem(t.Context(), 1, row.ID, row.Items[0].ID, nil, InventoryStocktakeItemBody{
		ExpectedRevision: row.Revision,
		IdempotencyKey:   key + "-count",
		CountedOnHand:    intPtr(counted),
	})
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func TestInventoryStocktakeHTTPEnforcesReadonlyAndTenantScope(t *testing.T) {
	t.Run("readonly create", func(t *testing.T) {
		fixture := newWarehouseLedgerFixture(t, true)
		router := warehouseLedgerHTTPRouter(t, fixture, 1, adminperm.RoleReadonly)
		recorder, envelope := performWarehouseLedgerRequest(t, router, http.MethodPost, "/api/v1/inventory/stocktakes", stocktakeCreateBody(fixture, "readonly-stocktake-001"))
		if recorder.Code != http.StatusForbidden || envelope.Code != response.CodeForbidden {
			t.Fatalf("unexpected readonly response: status=%d envelope=%#v", recorder.Code, envelope)
		}
	})

	t.Run("cross tenant detail", func(t *testing.T) {
		fixture := newWarehouseLedgerFixture(t, true)
		row := createStocktake(t, fixture, "cross-tenant-stocktake-001")
		router := warehouseLedgerHTTPRouter(t, fixture, 2, admin.RoleAdmin)
		recorder, envelope := performWarehouseLedgerRequest(t, router, http.MethodGet, "/api/v1/inventory/stocktakes/"+row.ID.String(), "")
		if recorder.Code != http.StatusNotFound || envelope.Code != response.CodeNotFound || envelope.Data != nil {
			t.Fatalf("unexpected cross-tenant response: status=%d envelope=%#v", recorder.Code, envelope)
		}
	})

	t.Run("missing counted quantity", func(t *testing.T) {
		fixture := newWarehouseLedgerFixture(t, true)
		row := createStocktake(t, fixture, "missing-count-stocktake-001")
		router := warehouseLedgerHTTPRouter(t, fixture, 1, admin.RoleAdmin)
		path := fmt.Sprintf("/api/v1/inventory/stocktakes/%s/items/%s", row.ID, row.Items[0].ID)
		body := `{"expectedRevision":1,"idempotencyKey":"missing-count-update-001"}`
		recorder, envelope := performWarehouseLedgerRequest(t, router, http.MethodPatch, path, body)
		if recorder.Code != http.StatusBadRequest || envelope.Code != response.CodeBadRequest {
			t.Fatalf("unexpected missing count response: status=%d envelope=%#v", recorder.Code, envelope)
		}
	})
}

func TestInventoryStocktakeHTTPSeparatesApprovalPermission(t *testing.T) {
	t.Run("operator cannot approve", func(t *testing.T) {
		fixture := newWarehouseLedgerFixture(t, true)
		row := countedStocktake(t, fixture, "operator-stocktake-001", 9)
		row, err := fixture.service.SubmitInventoryStocktake(t.Context(), 1, row.ID, nil, InventoryStocktakeActionBody{ExpectedRevision: row.Revision, IdempotencyKey: "operator-submit-001"})
		if err != nil {
			t.Fatal(err)
		}
		router := warehouseLedgerHTTPRouter(t, fixture, 1, adminperm.RoleOperator)
		path := "/api/v1/inventory/stocktakes/" + row.ID.String() + "/approve"
		recorder, envelope := performWarehouseLedgerRequest(t, router, http.MethodPost, path, stocktakeActionBody(row.Revision, "operator-approve-001"))
		if recorder.Code != http.StatusForbidden || envelope.Code != response.CodeForbidden {
			t.Fatalf("unexpected operator approval response: status=%d envelope=%#v", recorder.Code, envelope)
		}
	})

	t.Run("reviewer can approve", func(t *testing.T) {
		fixture := newWarehouseLedgerFixture(t, true)
		row := countedStocktake(t, fixture, "reviewer-stocktake-001", 9)
		row, err := fixture.service.SubmitInventoryStocktake(t.Context(), 1, row.ID, nil, InventoryStocktakeActionBody{ExpectedRevision: row.Revision, IdempotencyKey: "reviewer-submit-001"})
		if err != nil {
			t.Fatal(err)
		}
		router := warehouseLedgerHTTPRouter(t, fixture, 1, adminperm.RoleReviewer)
		path := "/api/v1/inventory/stocktakes/" + row.ID.String() + "/approve"
		recorder, envelope := performWarehouseLedgerRequest(t, router, http.MethodPost, path, stocktakeActionBody(row.Revision, "reviewer-approve-001"))
		if recorder.Code != http.StatusOK || envelope.Code != response.CodeOK {
			t.Fatalf("unexpected reviewer approval response: status=%d envelope=%#v", recorder.Code, envelope)
		}
	})
}

func TestInventoryStocktakeHTTPReturnsConflictForStaleSnapshot(t *testing.T) {
	fixture := newWarehouseLedgerFixture(t, true)
	row := countedStocktake(t, fixture, "stale-http-stocktake-001", 11)
	if _, err := fixture.service.AdjustWarehouseStock(t.Context(), 1, fixture.product.ID, fixture.sku.ID, AdjustStockBody{
		WarehouseID: fixture.main.ID, Stock: 12, IdempotencyKey: "stale-http-adjust-001",
	}, nil); err != nil {
		t.Fatal(err)
	}
	row, err := fixture.service.SubmitInventoryStocktake(t.Context(), 1, row.ID, nil, InventoryStocktakeActionBody{ExpectedRevision: row.Revision, IdempotencyKey: "stale-http-submit-001"})
	if err != nil {
		t.Fatal(err)
	}
	row, err = fixture.service.ApproveInventoryStocktake(t.Context(), 1, row.ID, nil, InventoryStocktakeActionBody{ExpectedRevision: row.Revision, IdempotencyKey: "stale-http-approve-001"})
	if err != nil {
		t.Fatal(err)
	}
	router := warehouseLedgerHTTPRouter(t, fixture, 1, admin.RoleAdmin)
	path := "/api/v1/inventory/stocktakes/" + row.ID.String() + "/post"
	recorder, envelope := performWarehouseLedgerRequest(t, router, http.MethodPost, path, stocktakeActionBody(row.Revision, "stale-http-post-001"))
	if recorder.Code != http.StatusConflict || envelope.Code == response.CodeOK || envelope.Data != nil {
		t.Fatalf("unexpected stale snapshot response: status=%d envelope=%#v", recorder.Code, envelope)
	}
	stored, err := fixture.service.GetInventoryStocktake(t.Context(), 1, row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != StocktakeApproved || stored.PostedAt != nil || stored.Revision != row.Revision {
		t.Fatalf("stale HTTP post should roll back: %#v", stored)
	}
}
