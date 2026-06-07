package product

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Service) GetPlatformPublishConfig(c *gin.Context, productID uuid.UUID, platform string) (*PlatformPublishConfigDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	plat := strings.TrimSpace(strings.ToLower(platform))
	var row ProductPlatformPublishConfig
	if err := s.DB.WithContext(c.Request.Context()).
		Where("product_id = ? AND platform = ?", productID, plat).
		First(&row).Error; err != nil {
		return nil, err
	}
	return platformConfigDTO(row), nil
}

func (s *Service) PutPlatformPublishConfig(c *gin.Context, productID uuid.UUID, platform string, body PlatformPublishConfigBody, adminID *uuid.UUID) (*PlatformPublishConfigDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	plat := strings.TrimSpace(strings.ToLower(platform))
	if plat == "" {
		return nil, fmt.Errorf("platform required")
	}
	if plat != "douyin_shop" {
		return nil, fmt.Errorf("platform config is currently supported for douyin_shop only")
	}
	var p Product
	if err := s.DB.WithContext(c.Request.Context()).Select("id").First(&p, "id = ?", productID).Error; err != nil {
		return nil, err
	}
	var shopID *uuid.UUID
	if raw := strings.TrimSpace(body.ShopID); raw != "" {
		u, err := uuid.Parse(raw)
		if err != nil || u == uuid.Nil {
			return nil, fmt.Errorf("invalid shopId")
		}
		shopID = &u
	}
	cid := strings.TrimSpace(body.CategoryID)
	if cid != "" {
		var cat shop.PlatformCategory
		if err := s.DB.WithContext(c.Request.Context()).Where("platform = ? AND category_id = ?", "douyin_shop", cid).First(&cat).Error; err != nil {
			return nil, err
		}
		if !cat.IsLeaf {
			return nil, &shop.DouyinCategoryError{Code: shop.DouyinCategoryNotLeaf, Message: "请选择抖店叶子类目。"}
		}
	}
	attrs := datatypes.JSON([]byte("{}"))
	if len(body.PlatformAttributes) > 0 && string(body.PlatformAttributes) != "null" {
		var tmp any
		if err := json.Unmarshal(body.PlatformAttributes, &tmp); err != nil {
			return nil, fmt.Errorf("platformAttributes must be valid JSON")
		}
		attrs = datatypes.JSON(body.PlatformAttributes)
	}
	row := ProductPlatformPublishConfig{
		ProductID:          productID,
		Platform:           plat,
		ShopID:             shopID,
		CategoryID:         cid,
		CategoryPath:       strings.TrimSpace(body.CategoryPath),
		PlatformAttributes: attrs,
	}
	if err := s.DB.WithContext(c.Request.Context()).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "product_id"}, {Name: "platform"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"shop_id", "category_id", "category_path", "platform_attributes", "updated_at",
		}),
	}).Create(&row).Error; err != nil {
		return nil, err
	}
	if err := s.DB.WithContext(c.Request.Context()).Where("product_id = ? AND platform = ?", productID, plat).First(&row).Error; err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		if cid != "" {
			_ = s.OpLog.Write(c, operationlog.WriteOpts{
				AdminUserID: adminID,
				Action:      "douyin.category.select",
				Resource:    "product",
				ResourceID:  productID.String(),
				Status:      "success",
				Message:     "categoryId=" + cid,
			})
		}
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "douyin.category.attr.update",
			Resource:    "product",
			ResourceID:  productID.String(),
			Status:      "success",
			Message:     "platformAttributes updated",
		})
	}
	return platformConfigDTO(row), nil
}

func platformConfigDTO(row ProductPlatformPublishConfig) *PlatformPublishConfigDTO {
	return &PlatformPublishConfigDTO{
		ProductID:          row.ProductID,
		Platform:           row.Platform,
		ShopID:             row.ShopID,
		CategoryID:         row.CategoryID,
		CategoryPath:       row.CategoryPath,
		PlatformAttributes: json.RawMessage(row.PlatformAttributes),
		Mapping:            DouyinDraftMappingFromConfig(row),
		LastMappedAt:       row.LastMappedAt,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}
}

func isRecordNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}
