package inventory

import (
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func ensureShopAuthorizedForInventory(row *shop.Shop, auth platformp.TestConnectionRequest) error {
	if row == nil {
		return fmt.Errorf("shop not found")
	}
	if strings.TrimSpace(row.Status) != shop.StatusActive {
		return fmt.Errorf("shop is not active")
	}
	if strings.TrimSpace(row.AuthStatus) != shop.AuthAuthorized {
		return fmt.Errorf("shop is not authorized")
	}
	pl := strings.TrimSpace(strings.ToLower(row.Platform))
	if pl != "mock" {
		if strings.TrimSpace(auth.AccessToken) == "" && strings.TrimSpace(auth.RefreshToken) == "" {
			return fmt.Errorf("shop is not authorized")
		}
	}
	return nil
}

// ValidateShopInventoryPush checks shop row + decrypted auth + rollout before enqueueing outbound sync tasks.
func ValidateShopInventoryPush(row *shop.Shop, auth platformp.TestConnectionRequest, prov platformp.Provider) error {
	if row == nil {
		return fmt.Errorf("shop not found")
	}
	pl := strings.TrimSpace(strings.ToLower(row.Platform))
	if pl == "manual" {
		return platformp.ErrManualInventorySyncUnsupported
	}
	if prov == nil {
		return fmt.Errorf("unknown platform")
	}
	if err := ensureShopAuthorizedForInventory(row, auth); err != nil {
		return err
	}
	if !platformp.HasCapability(prov, platformp.CapInventorySync) {
		return fmt.Errorf("platform does not advertise inventory_sync capability")
	}
	if !platformp.IsInventorySyncRunnable(prov) {
		return platformp.ErrInventorySyncNotImplemented
	}
	if _, ok := platformp.AsInventorySync(prov); !ok {
		return platformp.ErrInventorySyncNotImplemented
	}
	return nil
}
