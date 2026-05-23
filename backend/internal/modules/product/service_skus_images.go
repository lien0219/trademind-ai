package product

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
)

func validateProductStatus(s string) error {
	switch strings.TrimSpace(s) {
	case StatusDraft, StatusAIProcessing, StatusReady, StatusPublished, StatusArchived:
		return nil
	default:
		return fmt.Errorf("invalid product status")
	}
}

func normalizeImageType(raw string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case ImageTypeMain:
		return ImageTypeMain, nil
	case ImageTypeDetail, ImageTypeDescription:
		return ImageTypeDetail, nil
	case ImageTypeMarketing:
		return ImageTypeMarketing, nil
	case ImageTypeAIGenerated:
		return ImageTypeAIGenerated, nil
	case ImageTypeSKU:
		return ImageTypeSKU, nil
	default:
		return "", fmt.Errorf("invalid image type (use main, detail, marketing, ai_generated, or sku)")
	}
}

func attrsToDatatypes(attrs json.RawMessage) (datatypes.JSON, error) {
	if len(attrs) == 0 || string(attrs) == "null" {
		return nil, nil
	}
	if !json.Valid(attrs) {
		return nil, fmt.Errorf("invalid attrs JSON")
	}
	var hold any
	if err := json.Unmarshal(attrs, &hold); err != nil {
		return nil, fmt.Errorf("invalid attrs JSON")
	}
	return datatypes.JSON(attrs), nil
}

func ptrAttrsJSON(src *json.RawMessage) (*datatypes.JSON, error) {
	if src == nil {
		return nil, nil
	}
	if len(*src) == 0 || string(*src) == "null" {
		var empty datatypes.JSON
		return &empty, nil
	}
	if !json.Valid(*src) {
		return nil, fmt.Errorf("invalid attrs JSON")
	}
	var hold any
	if err := json.Unmarshal(*src, &hold); err != nil {
		return nil, fmt.Errorf("invalid attrs JSON")
	}
	j := datatypes.JSON(*src)
	return &j, nil
}

func (s *Service) CreateSKU(c *gin.Context, productID uuid.UUID, body SKUBody, adminID *uuid.UUID) (*ProductSKU, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	name := strings.TrimSpace(body.SKUName)
	code := strings.TrimSpace(body.SKUCode)
	if name == "" {
		return nil, fmt.Errorf("skuName is required")
	}

	var probe Product
	if err := s.DB.WithContext(c.Request.Context()).Select("id").First(&probe, "id = ?", productID).Error; err != nil {
		return nil, err
	}

	attrs, err := attrsToDatatypes(body.Attrs)
	if err != nil {
		return nil, err
	}
	row := &ProductSKU{
		ProductID:       productID,
		SKUCode:         code,
		SKUName:         name,
		Attrs:           attrs,
		Price:           body.Price,
		CostPrice:       body.CostPrice,
		CompareAtPrice:  body.CompareAtPrice,
		MinPublishPrice: body.MinPublishPrice,
		Stock:           body.Stock,
		ImageURL:        strings.TrimSpace(body.ImageURL),
		WarningStock:    5,
		SafetyStock:     0,
	}
	if s.Settings != nil {
		if m, err := s.Settings.PlainByGroup(c.Request.Context(), 0, "inventory"); err == nil {
			w := settings.DefaultWarningStockFromMap(m)
			sa := settings.DefaultSafetyStockFromMap(m)
			w, sa = settings.CoalesceDefaultStockLines(w, sa)
			row.WarningStock = w
			row.SafetyStock = sa
		}
	}
	if err := s.DB.WithContext(c.Request.Context()).Create(row).Error; err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "product.sku.create",
			Resource:    "product",
			ResourceID:  productID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("skuId=%s", row.ID.String()),
		})
	}
	return row, nil
}

// UpdateSKU patches a SKU; raw_data is never updated from API.
func (s *Service) UpdateSKU(c *gin.Context, productID, skuID uuid.UUID, body SKUUpdateBody, adminID *uuid.UUID) (*ProductSKU, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	var row ProductSKU
	if err := s.DB.WithContext(c.Request.Context()).First(&row, "id = ? AND product_id = ?", skuID, productID).Error; err != nil {
		return nil, err
	}

	if body.SKUCode != nil {
		row.SKUCode = strings.TrimSpace(*body.SKUCode)
	}
	if body.SKUName != nil {
		n := strings.TrimSpace(*body.SKUName)
		if n == "" {
			return nil, fmt.Errorf("skuName cannot be empty")
		}
		row.SKUName = n
	}
	if body.Attrs != nil {
		attrsPtr, err := ptrAttrsJSON(body.Attrs)
		if err != nil {
			return nil, err
		}
		if attrsPtr != nil {
			row.Attrs = *attrsPtr
		}
	}
	if body.Price != nil {
		row.Price = body.Price
	}
	if body.CostPrice != nil {
		row.CostPrice = body.CostPrice
	}
	if body.CompareAtPrice != nil {
		row.CompareAtPrice = body.CompareAtPrice
	}
	if body.MinPublishPrice != nil {
		row.MinPublishPrice = body.MinPublishPrice
	}
	if body.Stock != nil {
		row.Stock = body.Stock
	}
	if body.ImageURL != nil {
		row.ImageURL = strings.TrimSpace(*body.ImageURL)
	}

	if err := s.DB.WithContext(c.Request.Context()).Save(&row).Error; err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "product.sku.update",
			Resource:    "product",
			ResourceID:  productID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("skuId=%s", skuID.String()),
		})
	}
	return &row, nil
}

// DeleteSKU removes a SKU row (hard delete; see ProductSKU model).
func (s *Service) DeleteSKU(c *gin.Context, productID, skuID uuid.UUID, adminID *uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("product: no db")
	}
	res := s.DB.WithContext(c.Request.Context()).Delete(&ProductSKU{}, "id = ? AND product_id = ?", skuID, productID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "product.sku.delete",
			Resource:    "product",
			ResourceID:  productID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("skuId=%s", skuID.String()),
		})
	}
	return nil
}

// CreateProductImage links an image to a draft (via uploaded file metadata and/or URLs).
func (s *Service) CreateProductImage(c *gin.Context, productID uuid.UUID, body ImageCreateBody, adminID *uuid.UUID) (*ProductImage, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	imgType, err := normalizeImageType(body.ImageType)
	if err != nil {
		return nil, err
	}

	var probe Product
	if err := s.DB.WithContext(c.Request.Context()).Select("id").First(&probe, "id = ?", productID).Error; err != nil {
		return nil, err
	}

	pub := strings.TrimSpace(body.PublicURL)
	origin := strings.TrimSpace(body.OriginURL)
	objKey := strings.TrimSpace(body.ObjectKey)
	storageKey := strings.TrimSpace(body.StorageKey)
	if storageKey == "" {
		storageKey = objKey
	}

	if body.FileID != nil && *body.FileID != uuid.Nil {
		var fr files.FileRecord
		if err := s.DB.WithContext(c.Request.Context()).First(&fr, "id = ?", *body.FileID).Error; err != nil {
			return nil, err
		}
		if objKey == "" {
			objKey = fr.ObjectKey
		}
		if storageKey == "" {
			storageKey = fr.ObjectKey
		}
		if pub == "" {
			pub = strings.TrimSpace(fr.PublicURL)
		}
		if origin == "" {
			origin = pub
		}
	}

	if pub == "" && origin == "" {
		return nil, fmt.Errorf("publicUrl or originUrl is required (or provide fileId)")
	}
	if pub == "" {
		pub = origin
	}
	if origin == "" {
		origin = pub
	}

	sortOrder := 0
	if body.SortOrder != nil {
		sortOrder = *body.SortOrder
	} else {
		var max sql.NullInt64
		if err := s.DB.WithContext(c.Request.Context()).Model(&ProductImage{}).
			Where("product_id = ?", productID).
			Select("MAX(sort_order)").
			Scan(&max).Error; err != nil {
			return nil, err
		}
		mx := int64(-1)
		if max.Valid {
			mx = max.Int64
		}
		sortOrder = int(mx) + 1
	}

	row := &ProductImage{
		ProductID:       productID,
		ImageType:       imgType,
		Source:          strings.TrimSpace(body.Source),
		SourceTaskID:    body.SourceTaskID,
		OriginalImageID: body.OriginalImageID,
		OriginURL:       origin,
		ObjectKey:       objKey,
		StorageKey:      storageKey,
		PublicURL:       pub,
		Score:           body.Score,
		SortOrder:       sortOrder,
	}
	if body.IsBestMain != nil {
		row.IsBestMain = *body.IsBestMain
	}
	if row.Source == "" {
		row.Source = ImageSourceUpload
	}

	if err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if row.IsBestMain {
			if err := tx.Model(&ProductImage{}).Where("product_id = ?", productID).Update("is_best_main", false).Error; err != nil {
				return err
			}
		}
		return tx.Create(row).Error
	}); err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "product.image.create",
			Resource:    "product",
			ResourceID:  productID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("imageId=%s type=%s", row.ID.String(), imgType),
		})
	}
	return row, nil
}

// UpdateProductImage patches image metadata.
func (s *Service) UpdateProductImage(c *gin.Context, productID, imageID uuid.UUID, body ImageUpdateBody, adminID *uuid.UUID) (*ProductImage, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	var row ProductImage
	if err := s.DB.WithContext(c.Request.Context()).First(&row, "id = ? AND product_id = ?", imageID, productID).Error; err != nil {
		return nil, err
	}

	if body.ImageType != nil {
		t, err := normalizeImageType(*body.ImageType)
		if err != nil {
			return nil, err
		}
		row.ImageType = t
	}
	if body.ObjectKey != nil {
		row.ObjectKey = strings.TrimSpace(*body.ObjectKey)
	}
	if body.StorageKey != nil {
		row.StorageKey = strings.TrimSpace(*body.StorageKey)
	} else if body.ObjectKey != nil {
		row.StorageKey = row.ObjectKey
	}
	if body.OriginURL != nil {
		row.OriginURL = strings.TrimSpace(*body.OriginURL)
	}
	if body.PublicURL != nil {
		row.PublicURL = strings.TrimSpace(*body.PublicURL)
	}
	if body.SortOrder != nil {
		row.SortOrder = *body.SortOrder
	}
	if body.Score != nil {
		row.Score = body.Score
	}
	if body.IsBestMain != nil {
		row.IsBestMain = *body.IsBestMain
	}

	if err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if body.IsBestMain != nil && *body.IsBestMain {
			if err := tx.Model(&ProductImage{}).Where("product_id = ?", productID).Update("is_best_main", false).Error; err != nil {
				return err
			}
		}
		return tx.Save(&row).Error
	}); err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "product.image.update",
			Resource:    "product",
			ResourceID:  productID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("imageId=%s", imageID.String()),
		})
	}
	return &row, nil
}

// DeleteProductImage deletes the association row only (does not delete files storage).
func (s *Service) DeleteProductImage(c *gin.Context, productID, imageID uuid.UUID, adminID *uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("product: no db")
	}
	res := s.DB.WithContext(c.Request.Context()).Delete(&ProductImage{}, "id = ? AND product_id = ?", imageID, productID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "product.image.delete",
			Resource:    "product",
			ResourceID:  productID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("imageId=%s", imageID.String()),
		})
	}
	return nil
}

// ReorderProductImages sets sort_order from the given full id list.
func (s *Service) ReorderProductImages(c *gin.Context, productID uuid.UUID, body ImageReorderBody, adminID *uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("product: no db")
	}

	var probe Product
	if err := s.DB.WithContext(c.Request.Context()).Select("id").First(&probe, "id = ?", productID).Error; err != nil {
		return err
	}

	var existing []uuid.UUID
	if err := s.DB.WithContext(c.Request.Context()).Model(&ProductImage{}).
		Where("product_id = ?", productID).
		Order("sort_order ASC, created_at ASC").
		Pluck("id", &existing).Error; err != nil {
		return err
	}

	if len(existing) == 0 {
		if len(body.ImageIDs) == 0 {
			return nil
		}
		return fmt.Errorf("imageIds must include all product images exactly once")
	}

	if len(body.ImageIDs) == 0 {
		return fmt.Errorf("imageIds is required")
	}

	if len(existing) != len(body.ImageIDs) {
		return fmt.Errorf("imageIds must include all product images exactly once")
	}
	seen := make(map[uuid.UUID]struct{}, len(body.ImageIDs))
	for _, id := range body.ImageIDs {
		if _, dup := seen[id]; dup {
			return fmt.Errorf("duplicate image id")
		}
		seen[id] = struct{}{}
	}
	want := make(map[uuid.UUID]struct{}, len(existing))
	for _, id := range existing {
		want[id] = struct{}{}
	}
	for _, id := range body.ImageIDs {
		if _, ok := want[id]; !ok {
			return fmt.Errorf("missing or unknown image id in reorder list")
		}
	}

	if err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		for i, id := range body.ImageIDs {
			if err := tx.Model(&ProductImage{}).
				Where("id = ? AND product_id = ?", id, productID).
				Update("sort_order", i).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "product.image.reorder",
			Resource:    "product",
			ResourceID:  productID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("count=%d", len(body.ImageIDs)),
		})
	}
	return nil
}
