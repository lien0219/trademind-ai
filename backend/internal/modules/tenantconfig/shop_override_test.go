package tenantconfig

import (
  "context"
  "testing"
  "github.com/glebarez/sqlite"
  "github.com/google/uuid"
  "gorm.io/datatypes"
  "gorm.io/gorm"
)

func TestEffectiveShopInventoryTenantIsolationAndFallback(t *testing.T) {
 db,err:=gorm.Open(sqlite.Open("file:shop_inventory_config?mode=memory&cache=shared"),&gorm.Config{})
 if err!=nil{t.Fatal(err)}
 if err:=db.AutoMigrate(&Setting{},&ShopInventoryOverride{});err!=nil{t.Fatal(err)}
 ctx:=context.Background()
 store:=uuid.New()
 one:=Setting{TenantID:77,Module:ModuleInventory,Version:1,Values:datatypes.JSON([]byte(`{"lowStockThreshold":17}`)),UpdatedBy:uuid.New()}
 two:=Setting{TenantID:78,Module:ModuleInventory,Version:1,Values:datatypes.JSON([]byte(`{"lowStockThreshold":9}`)),UpdatedBy:uuid.New()}
 if err:=db.Create(&one).Error;err!=nil{t.Fatal(err)}
 if err:=db.Create(&two).Error;err!=nil{t.Fatal(err)}
 threshold,origin,_,err:=EffectiveInventory(ctx,db,77,store)
 if err!=nil||threshold!=17||origin!="enterprise"{t.Fatalf("fallback: %d %s %v",threshold,origin,err)}
 override:=ShopInventoryOverride{TenantID:77,ShopID:store,WarningStock:26,Version:1,UpdatedBy:uuid.New()}
 if err:=db.Create(&override).Error;err!=nil{t.Fatal(err)}
 threshold,origin,version,err:=EffectiveInventory(ctx,db,77,store)
 if err!=nil||threshold!=26||origin!="shop"||version!=1{t.Fatalf("shop override: %d %s %d %v",threshold,origin,version,err)}
 threshold,origin,_,err=EffectiveInventory(ctx,db,78,store)
 if err!=nil||threshold!=9||origin!="enterprise"{t.Fatalf("cross tenant: %d %s %v",threshold,origin,err)}
}
