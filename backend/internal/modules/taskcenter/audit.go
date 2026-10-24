package taskcenter

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"gorm.io/gorm"
)

var errTaskAuditUnavailable = errors.New("task center audit unavailable")

func (s *Service) requireAudit() error {
	if s == nil || s.OpLog == nil || s.OpLog.DB == nil {
		return errTaskAuditUnavailable
	}
	return nil
}

func (s *Service) writeRequiredAudit(c *gin.Context, opts operationlog.WriteOpts) error {
	if err := s.requireAudit(); err != nil {
		return err
	}
	if c == nil || c.Request == nil {
		return errTaskAuditUnavailable
	}
	if err := s.OpLog.Write(c, opts); err != nil {
		return errors.Join(errTaskAuditUnavailable, err)
	}
	return nil
}

func (s *Service) writeRequiredAuditTx(c *gin.Context, tx *gorm.DB, opts operationlog.WriteOpts) error {
	if err := s.requireAudit(); err != nil {
		return err
	}
	if c == nil || c.Request == nil || tx == nil {
		return errTaskAuditUnavailable
	}
	if err := s.OpLog.WriteTx(c, tx, opts); err != nil {
		return errors.Join(errTaskAuditUnavailable, err)
	}
	return nil
}

func (s *Service) writeRequiredAuditBackground(ctx context.Context, opts operationlog.WriteOpts) error {
	if err := s.requireAudit(); err != nil {
		return err
	}
	if err := s.OpLog.WriteBackground(ctx, opts); err != nil {
		return errors.Join(errTaskAuditUnavailable, err)
	}
	return nil
}

func (s *Service) writeRequiredAuditBackgroundTx(ctx context.Context, tx *gorm.DB, opts operationlog.WriteOpts) error {
	if err := s.requireAudit(); err != nil {
		return err
	}
	if tx == nil {
		return errTaskAuditUnavailable
	}
	if err := s.OpLog.WriteTxContext(ctx, tx, opts); err != nil {
		return errors.Join(errTaskAuditUnavailable, err)
	}
	return nil
}
