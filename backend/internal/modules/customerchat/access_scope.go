package customerchat

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

// authorizedConversation prevents a global UUID from crossing tenant/store scope.
func (s *Service) authorizedConversation(c *gin.Context, id uuid.UUID, write bool) (*CustomerConversation, error) {
	if s == nil || s.DB == nil || c == nil || id == uuid.Nil {
		return nil, gorm.ErrRecordNotFound
	}
	if !adminperm.CanViewCustomer(c, s.DB) || (write && !adminperm.CanWriteCustomer(c, s.DB)) {
		return nil, gorm.ErrRecordNotFound
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var row CustomerConversation
	if err := s.DB.WithContext(c.Request.Context()).
		Where("id = ? AND tenant_id = ?", id, tenantID).First(&row).Error; err != nil {
		return nil, err
	}
	if err := adminperm.EnsureStoreVisible(c, s.DB, row.ShopID); err != nil {
		return nil, err
	}
	if write && row.ShopID != nil {
		principal, err := adminperm.LoadPrincipal(c, s.DB)
		if err != nil {
			return nil, err
		}
		if principal == nil || !principal.CanOperateStore(*row.ShopID) {
			return nil, gorm.ErrRecordNotFound
		}
	}
	return &row, nil
}

func (s *Service) authorizeSuggestion(c *gin.Context, id uuid.UUID, write bool) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("customerchat: no db")
	}
	var suggestion CustomerReplySuggestion
	if err := s.DB.WithContext(c.Request.Context()).
		Select("conversation_id").First(&suggestion, "id = ?", id).Error; err != nil {
		return err
	}
	_, err := s.authorizedConversation(c, suggestion.ConversationID, write)
	return err
}
