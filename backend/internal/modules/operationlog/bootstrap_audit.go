package operationlog

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WriteBootstrapAuditTx records a startup security mutation in its existing
// database transaction. Bootstrap audit rows intentionally have no user actor;
// the generated request ID identifies each startup event without exposing
// credentials or configuration values.
func (s *Service) WriteBootstrapAuditTx(ctx context.Context, tx *gorm.DB, tenantID int64, action, resource, resourceID, message string) error {
	if s == nil || s.DB == nil || tx == nil {
		return fmt.Errorf("operationlog: bootstrap audit transaction unavailable")
	}
	if ctx == nil {
		return fmt.Errorf("operationlog: bootstrap audit context unavailable")
	}
	action = strings.TrimSpace(action)
	resource = strings.TrimSpace(resource)
	resourceID = strings.TrimSpace(resourceID)
	if action == "" || resource == "" || resourceID == "" {
		return fmt.Errorf("operationlog: bootstrap audit identity unavailable")
	}

	row := &OperationLog{
		TenantID:       tenantID,
		Action:         action,
		Resource:       resource,
		ResourceID:     resourceID,
		Method:         "INTERNAL",
		Path:           "/internal/bootstrap",
		RequestID:      uuid.NewString(),
		Status:         "success",
		Message:        message,
		CreatedAt:      time.Now().UTC().Truncate(time.Microsecond),
		ChainPartition: chainPartition(tenantID, time.Now().UTC()),
	}
	release, err := s.appendHashChain(tx.WithContext(ctx), row)
	if err != nil {
		return fmt.Errorf("operationlog: append bootstrap audit chain: %w", err)
	}
	defer release()
	if err := tx.WithContext(ctx).Create(row).Error; err != nil {
		return fmt.Errorf("operationlog: insert bootstrap audit row: %w", err)
	}
	return nil
}
