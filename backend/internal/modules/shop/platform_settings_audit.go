package shop

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"gorm.io/gorm"
)

var errPlatformSettingsAudit = errors.New("platform settings audit unavailable")

// Global credentials and publish configuration must never commit without
// their audit record. Only field names and the group identity enter the log.
func (s *Service) putPlatformSettingsWithAudit(c *gin.Context, items []settings.PutItem, audit operationlog.WriteOpts) error {
	if s == nil || s.Settings == nil || s.Settings.DB == nil || s.OpLog == nil || c == nil || c.Request == nil {
		return errPlatformSettingsAudit
	}
	return s.Settings.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := s.Settings.PutBulkTx(tx, items); err != nil {
			return err
		}
		if err := s.OpLog.WriteTx(c, tx, audit); err != nil {
			return fmt.Errorf("%w: %w", errPlatformSettingsAudit, err)
		}
		return nil
	})
}
