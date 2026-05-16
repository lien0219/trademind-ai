package database

import (
	"fmt"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiprompt"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/collectrule"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"gorm.io/gorm"
)

// AutoMigrate applies schema for core foundation tables.
func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("auto migrate: db is nil")
	}
	return db.AutoMigrate(
		&admin.AdminUser{},
		&settings.Setting{},
		&operationlog.OperationLog{},
		&files.FileRecord{},
		&imagetask.ImageTask{},
		&product.Product{},
		&product.ProductImage{},
		&product.ProductSKU{},
		&order.Order{},
		&order.OrderItem{},
		&order.OrderShipment{},
		&collect.CollectBatch{},
		&collect.CollectTask{},
		&collect.CollectTaskEvent{},
		&collectrule.CollectRule{},
		&aiprompt.AIPrompt{},
		&aitask.AITask{},
		&customerchat.CustomerConversation{},
		&customerchat.CustomerMessage{},
		&customerchat.CustomerReplySuggestion{},
	)
}
