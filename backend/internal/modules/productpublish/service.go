package productpublish

import (
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"gorm.io/gorm"
)

// Service wires DB + outbound provider execution for product_publish_tasks.
type Service struct {
	DB       *gorm.DB
	Redis    *rdb.Client
	Shops    *shop.Service
	Settings *settings.Service
	OpLog    *operationlog.Service

	QueueEnabled bool
	QueueName    string
	TaskTimeout  time.Duration
}

func (s *Service) normalizedQueueName() string {
	q := strings.TrimSpace(s.QueueName)
	if q == "" {
		return "product:publish:tasks"
	}
	return q
}
