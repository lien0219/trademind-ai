package database

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/collectbrowserprofile"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestBackfillCollectBrowserProfileTenantsUsesVerifiedCreator(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &collectbrowserprofile.CollectBrowserProfile{}))

	owner := admin.AdminUser{TenantID: 27, Username: admin.NewInternalUsername(), Role: admin.RoleAdmin, Status: admin.StatusActive}
	require.NoError(t, db.Create(&owner).Error)
	owned := collectbrowserprofile.CollectBrowserProfile{
		TenantID: 0, Name: "legacy-owned", Domain: "owned.example.test", ProfileKey: "legacy_owned",
		Status: collectbrowserprofile.StatusActive, CreatedBy: &owner.ID,
	}
	unowned := collectbrowserprofile.CollectBrowserProfile{
		TenantID: 0, Name: "legacy-unowned", Domain: "unowned.example.test", ProfileKey: "legacy_unowned",
		Status: collectbrowserprofile.StatusActive,
	}
	require.NoError(t, db.Create(&owned).Error)
	require.NoError(t, db.Create(&unowned).Error)

	require.NoError(t, backfillCollectBrowserProfileTenants(db))

	var migratedOwned, migratedUnowned collectbrowserprofile.CollectBrowserProfile
	require.NoError(t, db.First(&migratedOwned, "id = ?", owned.ID).Error)
	require.NoError(t, db.First(&migratedUnowned, "id = ?", unowned.ID).Error)
	require.EqualValues(t, owner.TenantID, migratedOwned.TenantID)
	require.Zero(t, migratedUnowned.TenantID, "records without a trusted creator must remain isolated")
}
