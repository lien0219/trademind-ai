package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

func openImportDraftTenantTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:import_draft_tenant_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Product{}, &ProductImage{}, &ProductSKU{}))
	return db
}

func TestImportDraftWithContextPersistsTenant(t *testing.T) {
	db := openImportDraftTenantTestDB(t)
	svc := &Service{DB: db}
	ctx := security.WithTenantContext(context.Background(), security.WorkerTenantContext(42, uuid.Nil))

	created, err := svc.ImportDraftWithContext(ctx, nil, ImportDraftParams{
		Source:    "1688",
		SourceURL: "https://detail.1688.com/offer/1.html",
		Title:     "Collected product",
		Currency:  "CNY",
	})
	require.NoError(t, err)
	require.Equal(t, int64(42), created.TenantID)

	var stored Product
	require.NoError(t, db.First(&stored, "id = ?", created.ID).Error)
	require.Equal(t, int64(42), stored.TenantID)
}

func TestImportDraftKeepsSourceStockAsMetadataAndStartsERPStockAtZero(t *testing.T) {
	db := openImportDraftTenantTestDB(t)
	svc := &Service{DB: db}
	ctx := security.WithTenantContext(context.Background(), security.WorkerTenantContext(42, uuid.Nil))
	sourceStock := 17
	rawSKU := json.RawMessage(`{"skuCode":"SOURCE-17","stock":17}`)

	created, err := svc.ImportDraftWithContext(ctx, nil, ImportDraftParams{
		Source: "1688", Title: "Collected stock metadata", Currency: "CNY",
		SKUs: []ImportSKUParams{{SKUCode: "SOURCE-17", SKUName: "Source stock", Stock: &sourceStock, RawSKU: rawSKU}},
	})
	require.NoError(t, err)

	var stored ProductSKU
	require.NoError(t, db.First(&stored, "product_id = ?", created.ID).Error)
	require.NotNil(t, stored.Stock)
	require.Zero(t, *stored.Stock)
	require.JSONEq(t, string(rawSKU), string(stored.RawData))
}

func TestImportDraftWithContextPersistsLegacyDevelopmentTenantZero(t *testing.T) {
	db := openImportDraftTenantTestDB(t)
	svc := &Service{DB: db}
	tenant := security.WorkerTenantContext(0, uuid.Nil)
	tenant.AuthSource = security.AuthSourceLegacyDevZero
	ctx := security.WithTenantContext(context.Background(), tenant)

	created, err := svc.ImportDraftWithContext(ctx, nil, ImportDraftParams{
		Source:    "1688",
		SourceURL: "https://detail.1688.com/offer/1.html",
		Title:     "Legacy development product",
		Currency:  "CNY",
	})
	require.NoError(t, err)
	require.Zero(t, created.TenantID)

	var stored Product
	require.NoError(t, db.First(&stored, "id = ?", created.ID).Error)
	require.Zero(t, stored.TenantID)
}

func TestImportDraftWithContextRejectsUntrustedTenantZero(t *testing.T) {
	db := openImportDraftTenantTestDB(t)
	svc := &Service{DB: db}
	ctx := security.WithTenantContext(context.Background(), security.WorkerTenantContext(0, uuid.Nil))

	created, err := svc.ImportDraftWithContext(ctx, nil, ImportDraftParams{
		Source: "1688",
		Title:  "Must not persist",
	})
	require.Nil(t, created)
	require.True(t, errors.Is(err, security.ErrTenantContextMissing))
}

func TestImportDraftWithContextRejectsMissingTenant(t *testing.T) {
	db := openImportDraftTenantTestDB(t)
	svc := &Service{DB: db}

	created, err := svc.ImportDraftWithContext(context.Background(), nil, ImportDraftParams{
		Source: "1688",
		Title:  "Must not persist",
	})
	require.Nil(t, created)
	require.True(t, errors.Is(err, security.ErrTenantContextMissing))

	var count int64
	require.NoError(t, db.Model(&Product{}).Count(&count).Error)
	require.Zero(t, count)
}
