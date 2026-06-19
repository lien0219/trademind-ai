package productpublish

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	defaultBatchMaxProducts = 100
	defaultBatchMaxTargets  = 20
	defaultBatchMaxTasks    = 300
)

// BatchLimitExceededMsg is returned when product × target matrix exceeds configured caps.
const BatchLimitExceededMsg = "本次选择的商品和刊登目标较多，请分批创建刊登草稿。"

func (s *Service) batchMaxProducts() int {
	if s != nil && s.BatchMaxProducts > 0 {
		return s.BatchMaxProducts
	}
	return defaultBatchMaxProducts
}

func (s *Service) batchMaxTargets() int {
	if s != nil && s.BatchMaxTargets > 0 {
		return s.BatchMaxTargets
	}
	return defaultBatchMaxTargets
}

func (s *Service) batchMaxTasks() int {
	if s != nil && s.BatchMaxTasks > 0 {
		return s.BatchMaxTasks
	}
	return defaultBatchMaxTasks
}

func batchLimitExceeded() error {
	return fmt.Errorf("%s", BatchLimitExceededMsg)
}

func (s *Service) validateBatchTaskCount(productCount, targetCount int) error {
	if productCount <= 0 || targetCount <= 0 {
		return nil
	}
	if productCount*targetCount > s.batchMaxTasks() {
		return batchLimitExceeded()
	}
	return nil
}

var ErrBatchAccessDenied = errors.New("batch access denied")

func (s *Service) assertBatchAccess(batch *ProductPublishBatch, adminID *uuid.UUID) error {
	if batch == nil {
		return gorm.ErrRecordNotFound
	}
	if batch.CreatedBy == nil || adminID == nil {
		return nil
	}
	if batch.CreatedBy.String() != adminID.String() {
		return ErrBatchAccessDenied
	}
	return nil
}

func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "constraint failed")
}
