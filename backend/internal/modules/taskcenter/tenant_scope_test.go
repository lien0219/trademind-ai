package taskcenter

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPassesUnifiedFiltersDeniesShopLinkedTasksWithNoStoreGrants(t *testing.T) {
	task := UnifiedTaskDTO{ShopID: uuid.NewString()}
	require.False(t, passesUnifiedFilters(task, ListFailureParams{RestrictShopScope: true}))
}

func TestPassesUnifiedFiltersAllowsOnlyGrantedStore(t *testing.T) {
	allowed := uuid.New()
	denied := uuid.New()
	params := ListFailureParams{RestrictShopScope: true, AllowedShopIDs: []uuid.UUID{allowed}}
	require.True(t, passesUnifiedFilters(UnifiedTaskDTO{ShopID: allowed.String()}, params))
	require.False(t, passesUnifiedFilters(UnifiedTaskDTO{ShopID: denied.String()}, params))
}

func TestPassesUnifiedFiltersKeepsTenantScopedTaskWithoutShop(t *testing.T) {
	require.True(t, passesUnifiedFilters(UnifiedTaskDTO{}, ListFailureParams{RestrictShopScope: true}))
}
