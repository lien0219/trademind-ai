package adminperm

import (
	"testing"

	"github.com/google/uuid"
)

func TestAllowedStoreIDsAdmin(t *testing.T) {
	p := &Principal{Role: RoleAdmin}
	if ids := p.AllowedStoreIDs(); ids != nil {
		t.Fatalf("admin should return nil allowed ids, got %v", ids)
	}
}

func TestCanViewStoreScoped(t *testing.T) {
	sid := uuid.New()
	p := &Principal{
		Role:        RoleOperator,
		StoreGrants: []StoreGrant{{StoreID: sid, PermissionScope: "view"}},
	}
	if !p.CanViewStore(sid) {
		t.Fatal("expected view access")
	}
	if p.CanViewStore(uuid.New()) {
		t.Fatal("expected deny for other store")
	}
}
