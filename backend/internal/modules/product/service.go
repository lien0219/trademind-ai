package product

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/aiprompt"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/opslabels"
	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

// Service handles product draft persistence.
type Service struct {
	DB        *gorm.DB
	OpLog     *operationlog.Service
	Settings  *settings.Service
	Prompts   *aiprompt.Service
	AITasks   *aitask.Service
	AIGateway *aigate.Gateway

	Shops               DouyinShopClientFactory
	DouyinImageUploader DouyinImageUploader
	Readiness           func(context.Context, OperationReadinessRequest) (*OperationReadinessResult, error)
}

type DouyinShopClientFactory interface {
	DouyinClientForShop(c *gin.Context, ctx context.Context, shopID uuid.UUID, adminID *uuid.UUID) (*platformdouyin.Client, *shop.Shop, error)
}

type DouyinImageUploader interface {
	UploadImage(ctx context.Context, shopID string, req platformdouyin.UploadImageRequest) (*platformdouyin.PlatformImage, error)
}

func clampPage(page, ps int) (int, int) {
	if page < 1 {
		page = 1
	}
	if ps < 1 {
		ps = 20
	}
	if ps > 100 {
		ps = 100
	}
	return page, ps
}

func pickCoverURL(origin, pub string) string {
	pub = strings.TrimSpace(pub)
	if pub != "" {
		return pub
	}
	return strings.TrimSpace(origin)
}

func preferredDraftTextExpr(primary, fallback string) string {
	return fmt.Sprintf("COALESCE(NULLIF(TRIM(%s), ''), NULLIF(TRIM(%s), ''), '')", primary, fallback)
}

func progressImageExistsClause(alias string) string {
	urlExpr := fmt.Sprintf("(TRIM(COALESCE(%[1]s.public_url,'')) <> '' OR TRIM(COALESCE(%[1]s.origin_url,'')) <> '' OR TRIM(COALESCE(%[1]s.object_key,'')) <> '' OR TRIM(COALESCE(%[1]s.storage_key,'')) <> '')", alias)
	imageType := fmt.Sprintf("LOWER(TRIM(COALESCE(%s.image_type,'')))", alias)
	return fmt.Sprintf(`EXISTS (
		SELECT 1
		FROM product_images %s
		WHERE %s.product_id = products.id
		  AND (
			(%s = '%s' AND %s)
			OR (%s <> '%s' AND %s <> '%s' AND %s)
		  )
	)`, alias, alias, imageType, ImageTypeMain, urlExpr, imageType, ImageTypeMain, imageType, ImageTypeSKU, urlExpr)
}

// List returns paginated drafts with optional filters.
func (s *Service) List(c *gin.Context, q ListQuery) (*ListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	page, ps := clampPage(q.Page, q.PageSize)
	titleExpr := preferredDraftTextExpr("title", "original_title")
	descriptionExpr := preferredDraftTextExpr("description", "ai_description")
	hasProgressImage := progressImageExistsClause("pi")

	tx := s.DB.WithContext(c.Request.Context()).Model(&Product{})
	if v := strings.TrimSpace(q.Status); v != "" {
		tx = tx.Where("status = ?", v)
	}
	if v := strings.TrimSpace(q.Source); v != "" {
		tx = tx.Where("source = ?", v)
	}
	if v := strings.TrimSpace(q.Keyword); v != "" {
		pat := "%" + strings.ToLower(v) + "%"
		tx = tx.Where("LOWER(title) LIKE ? OR LOWER(original_title) LIKE ?", pat, pat)
	}
	switch strings.TrimSpace(strings.ToLower(q.OperationStep)) {
	case string(OperationStepCollectReview):
		tx = tx.Where(`(
			TRIM(COALESCE(source,'')) = ''
			OR ` + titleExpr + ` = ''
			OR NOT ` + hasProgressImage + `
			OR NOT EXISTS (SELECT 1 FROM product_skus ps WHERE ps.product_id = products.id AND ((ps.price IS NOT NULL AND ps.price > 0) OR (ps.cost_price IS NOT NULL AND ps.cost_price > 0)))
		)`)
	case string(OperationStepTitle):
		tx = tx.Where(titleExpr+" = '' OR LENGTH("+titleExpr+") < ?", 4)
	case string(OperationStepDescription):
		tx = tx.Where(descriptionExpr+" = '' OR LENGTH("+descriptionExpr+") < ?", 20)
	case string(OperationStepImages):
		tx = tx.Where("NOT " + hasProgressImage)
	case string(OperationStepPricing):
		tx = tx.Where(`(
			TRIM(COALESCE(currency,'')) = ''
			OR NOT EXISTS (SELECT 1 FROM product_skus ps WHERE ps.product_id = products.id)
			OR EXISTS (SELECT 1 FROM product_skus ps2 WHERE ps2.product_id = products.id AND (ps2.price IS NULL OR ps2.price <= 0))
		)`)
	case string(OperationStepPublishCheck):
		tx = tx.Where("status <> ?", StatusArchived).
			Where(`(
			` + titleExpr + ` = ''
			OR TRIM(COALESCE(currency,'')) = ''
			OR NOT ` + hasProgressImage + `
			OR NOT EXISTS (SELECT 1 FROM product_skus ps0 WHERE ps0.product_id = products.id)
			OR EXISTS (SELECT 1 FROM product_skus ps1 WHERE ps1.product_id = products.id AND (ps1.price IS NULL OR ps1.price <= 0))
		)`)
	case string(OperationStepReady):
		tx = tx.Where("status IN ?", []string{StatusDraft, StatusReady}).
			Where(titleExpr+" <> ''").
			Where("LENGTH("+descriptionExpr+") >= ?", 20).
			Where("TRIM(COALESCE(currency,'')) <> ''").
			Where(hasProgressImage).
			Where(`EXISTS (SELECT 1 FROM product_skus ps WHERE ps.product_id = products.id)`).
			Where(`NOT EXISTS (SELECT 1 FROM product_skus ps2 WHERE ps2.product_id = products.id AND (ps2.price IS NULL OR ps2.price <= 0))`)
	}
	if q.MissingAiTitle {
		tx = tx.Where("status <> ?", StatusArchived).
			Where("TRIM(COALESCE(ai_title,'')) = ''").
			Where("( TRIM(COALESCE(title,'')) <> '' OR TRIM(COALESCE(original_title,'')) <> '' )")
	}
	if q.MissingAiDescription {
		tx = tx.Where("status <> ?", StatusArchived).
			Where("TRIM(COALESCE(ai_description,'')) = ''").
			Where("( TRIM(COALESCE(description,'')) = '' OR LENGTH(TRIM(COALESCE(description,''))) < ? )", 60)
	}
	if q.ReadinessBlocked {
		tx = tx.Where("status <> ?", StatusArchived).
			Where(`(
			` + titleExpr + ` = ''
			OR TRIM(COALESCE(currency,'')) = ''
			OR NOT ` + hasProgressImage + `
			OR NOT EXISTS (SELECT 1 FROM product_skus ps0 WHERE ps0.product_id = products.id)
			OR EXISTS (SELECT 1 FROM product_skus ps1 WHERE ps1.product_id = products.id AND (ps1.price IS NULL OR ps1.price <= 0))
		)`)
	}
	if q.Publishable {
		tx = tx.Where("status IN ?", []string{StatusDraft, StatusReady}).
			Where(titleExpr+" <> ''").
			Where("LENGTH("+descriptionExpr+") >= ?", 20).
			Where("TRIM(COALESCE(currency,'')) <> ''").
			Where(hasProgressImage).
			Where(`EXISTS (
			SELECT 1 FROM product_skus ps
			WHERE ps.product_id = products.id
		)`).
			Where(`NOT EXISTS (
			SELECT 1 FROM product_skus ps2
			WHERE ps2.product_id = products.id AND (ps2.price IS NULL OR ps2.price <= 0)
		)`)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page - 1) * ps
	var rows []Product
	if err := tx.Order("created_at DESC").Offset(offset).Limit(ps).Find(&rows).Error; err != nil {
		return nil, err
	}

	covers := map[uuid.UUID]string{}
	if len(rows) > 0 {
		ids := make([]uuid.UUID, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.ID)
		}
		var imgs []ProductImage
		if err := s.DB.WithContext(c.Request.Context()).
			Where("product_id IN ? AND image_type = ?", ids, ImageTypeMain).
			Order("sort_order ASC").
			Find(&imgs).Error; err != nil {
			return nil, err
		}
		for _, img := range imgs {
			if _, ok := covers[img.ProductID]; ok {
				continue
			}
			covers[img.ProductID] = pickCoverURL(img.OriginURL, img.PublicURL)
		}
	}

	items := make([]ListItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, ListItem{
			ID:        r.ID,
			TenantID:  r.TenantID,
			CreatedBy: r.CreatedBy,
			Source:    r.Source,
			SourceURL: r.SourceURL,
			Title:     r.Title,
			Status:    r.Status,
			Currency:  r.Currency,
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
			CoverURL:  covers[r.ID],
		})
	}
	var err error
	items, err = s.attachOperationProgressSummaries(c.Request.Context(), rows, items)
	if err != nil {
		return nil, err
	}

	pages := int(total) / ps
	if int(total)%ps != 0 {
		pages++
	}
	if pages == 0 && total > 0 {
		pages = 1
	}

	return &ListResult{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   ps,
		TotalPages: pages,
	}, nil
}

// Create inserts a manual draft.
func (s *Service) Create(c *gin.Context, body CreateBody, adminID *uuid.UUID) (*DetailDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	source := strings.TrimSpace(body.Source)
	if source == "" {
		source = "manual"
	}
	title := strings.TrimSpace(body.Title)
	if title == "" {
		title = strings.TrimSpace(body.OriginalTitle)
	}
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	status := strings.TrimSpace(body.Status)
	if status == "" {
		status = StatusDraft
	}
	curr := strings.TrimSpace(body.Currency)
	if curr == "" {
		curr = "CNY"
	}

	raw := datatypes.JSON(nil)
	if len(body.RawData) > 0 {
		raw = datatypes.JSON(body.RawData)
	}

	p := &Product{
		TenantID:      body.TenantID,
		CreatedBy:     adminID,
		Source:        source,
		SourceURL:     strings.TrimSpace(body.SourceURL),
		OriginalTitle: strings.TrimSpace(body.OriginalTitle),
		Title:         title,
		Description:   strings.TrimSpace(body.Description),
		Currency:      curr,
		Status:        status,
		RawData:       raw,
	}
	if p.OriginalTitle == "" {
		p.OriginalTitle = p.Title
	}

	if err := s.DB.WithContext(c.Request.Context()).Create(p).Error; err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "product.create",
			Resource:    "product",
			ResourceID:  p.ID.String(),
			Status:      "success",
			Message:     "manual draft created",
		})
	}
	return s.Get(c, p.ID)
}

// Get loads product with images and SKUs.
func (s *Service) Get(c *gin.Context, id uuid.UUID) (*DetailDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	var p Product
	if err := s.DB.WithContext(c.Request.Context()).
		Preload("Images", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC")
		}).
		Preload("SKUs", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		First(&p, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return toDetailDTO(&p), nil
}

func toDetailDTO(p *Product) *DetailDTO {
	if p == nil {
		return nil
	}
	var raw json.RawMessage
	if len(p.RawData) > 0 {
		raw = json.RawMessage(p.RawData)
	}
	mainImages, descriptionImages := productImageURLs(p.Images)
	attrs, skuGroups := rawDraftDebugFields(raw)
	costPrice, salePrice, stock := productAggregatePricesAndStock(p.SKUs)
	return &DetailDTO{
		ID:                p.ID,
		TenantID:          p.TenantID,
		CreatedBy:         p.CreatedBy,
		Source:            p.Source,
		SourceURL:         p.SourceURL,
		OriginalTitle:     p.OriginalTitle,
		Title:             p.Title,
		AITitle:           p.AITitle,
		Description:       p.Description,
		AIDescription:     p.AIDescription,
		Currency:          p.Currency,
		Status:            p.Status,
		RawData:           raw,
		Raw:               raw,
		MainImages:        mainImages,
		DescriptionImages: descriptionImages,
		Attributes:        attrs,
		SKUGroups:         skuGroups,
		CostPrice:         costPrice,
		SalePrice:         salePrice,
		Stock:             stock,
		CollectWarnings:   collectWarningsFromRaw(raw),
		PublishStatus:     draftPublishStatus(p.Status),
		CreatedAt:         p.CreatedAt,
		UpdatedAt:         p.UpdatedAt,
		Images:            p.Images,
		SKUs:              p.SKUs,
	}
}

func productImageURLs(rows []ProductImage) ([]string, []string) {
	main := make([]string, 0)
	detail := make([]string, 0)
	for _, im := range rows {
		u := strings.TrimSpace(im.PublicURL)
		if u == "" {
			u = strings.TrimSpace(im.OriginURL)
		}
		if u == "" {
			u = strings.TrimSpace(im.ObjectKey)
		}
		if u == "" {
			continue
		}
		t := strings.TrimSpace(strings.ToLower(im.ImageType))
		if t == ImageTypeDescription {
			t = ImageTypeDetail
		}
		switch t {
		case ImageTypeMain:
			main = append(main, u)
		case ImageTypeDetail:
			detail = append(detail, u)
		}
	}
	return main, detail
}

func productAggregatePricesAndStock(rows []ProductSKU) (*float64, *float64, *int) {
	var cost, sale *float64
	var stock *int
	for _, row := range rows {
		if row.CostPrice != nil {
			v := *row.CostPrice
			if cost == nil || v < *cost {
				cost = &v
			}
		}
		if row.Price != nil {
			v := *row.Price
			if sale == nil || v < *sale {
				sale = &v
			}
		}
		if row.Stock != nil {
			cur := 0
			if stock != nil {
				cur = *stock
			}
			cur += *row.Stock
			stock = &cur
		}
	}
	return cost, sale, stock
}

func rawDraftDebugFields(raw json.RawMessage) (json.RawMessage, json.RawMessage) {
	if len(raw) == 0 {
		return nil, nil
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, nil
	}
	attrs := firstRawField(root, "attributes", "attrs")
	skuGroups := firstRawField(root, "skuGroups", "sku_groups")
	if skuGroups == nil {
		if rawObj := nestedRawMap(root); rawObj != nil {
			skuGroups = firstRawField(rawObj, "skuGroups", "sku_groups", "skuBase")
		}
	}
	return attrs, skuGroups
}

func firstRawField(m map[string]json.RawMessage, keys ...string) json.RawMessage {
	for _, k := range keys {
		if v, ok := m[k]; ok && len(v) > 0 && string(v) != "null" {
			return v
		}
	}
	return nil
}

func nestedRawMap(root map[string]json.RawMessage) map[string]json.RawMessage {
	var rawObj map[string]json.RawMessage
	if v, ok := root["raw"]; ok {
		_ = json.Unmarshal(v, &rawObj)
	}
	return rawObj
}

func collectWarningsFromRaw(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil
	}
	out := make([]string, 0)
	seen := map[string]struct{}{}
	add := func(items []string) {
		for _, s := range items {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			key := strings.ToLower(s)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, s)
		}
	}
	readList := func(m map[string]json.RawMessage, key string) []string {
		var ss []string
		if v, ok := m[key]; ok {
			_ = json.Unmarshal(v, &ss)
		}
		return ss
	}
	for _, k := range []string{"collectWarnings", "warnings", "qualityWarnings"} {
		add(readList(root, k))
	}
	if rawObj := nestedRawMap(root); rawObj != nil {
		for _, k := range []string{"collectWarnings", "warnings", "qualityWarnings"} {
			add(readList(rawObj, k))
		}
	}
	return opslabels.LocalizeCollectWarnings(out)
}

func draftPublishStatus(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case StatusPublished:
		return "success"
	case StatusArchived:
		return "cancelled"
	case StatusReady:
		return "ready"
	default:
		return "draft"
	}
}

// Update patches editable fields (source / rawData are immutable here).
func (s *Service) Update(c *gin.Context, id uuid.UUID, body UpdateBody, adminID *uuid.UUID) (*DetailDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	var p Product
	if err := s.DB.WithContext(c.Request.Context()).First(&p, "id = ?", id).Error; err != nil {
		return nil, err
	}

	if body.OriginalTitle != nil {
		p.OriginalTitle = strings.TrimSpace(*body.OriginalTitle)
	}
	if body.Title != nil {
		t := strings.TrimSpace(*body.Title)
		if t == "" {
			return nil, fmt.Errorf("title cannot be empty")
		}
		p.Title = t
	}
	if body.AITitle != nil {
		p.AITitle = strings.TrimSpace(*body.AITitle)
	}
	if body.Description != nil {
		p.Description = strings.TrimSpace(*body.Description)
	}
	if body.AIDescription != nil {
		p.AIDescription = strings.TrimSpace(*body.AIDescription)
	}
	if body.Currency != nil {
		curr := strings.TrimSpace(*body.Currency)
		if curr == "" {
			curr = "CNY"
		}
		p.Currency = curr
	}
	if body.Status != nil {
		st := strings.TrimSpace(*body.Status)
		if err := validateProductStatus(st); err != nil {
			return nil, err
		}
		p.Status = st
	}

	if err := s.DB.WithContext(c.Request.Context()).Save(&p).Error; err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "product.update",
			Resource:    "product",
			ResourceID:  p.ID.String(),
			Status:      "success",
			Message:     "product draft fields updated",
		})
	}
	return s.Get(c, p.ID)
}

// Delete soft-deletes a draft (or archives conceptually via GORM DeletedAt).
func (s *Service) Delete(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("product: no db")
	}
	res := s.DB.WithContext(c.Request.Context()).Delete(&Product{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "product.delete",
			Resource:    "product",
			ResourceID:  id.String(),
			Status:      "success",
			Message:     "product soft-deleted",
		})
	}
	return nil
}

// ImportDraft creates product + images + SKUs from collector-normalized data inside one transaction.
func (s *Service) ImportDraft(c *gin.Context, adminID *uuid.UUID, p ImportDraftParams) (*Product, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	out, err := s.importDraftCore(c.Request.Context(), adminID, p)
	if err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "product.create",
			Resource:    "product",
			ResourceID:  out.ID.String(),
			Status:      "success",
			Message:     "draft imported from collect",
		})
	}
	return out, nil
}

// ImportDraftWithContext is the same as ImportDraft but for non-HTTP callers (e.g. collect worker).
func (s *Service) ImportDraftWithContext(ctx context.Context, adminID *uuid.UUID, p ImportDraftParams) (*Product, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	out, err := s.importDraftCore(ctx, adminID, p)
	if err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "product.create",
			Resource:    "product",
			ResourceID:  out.ID.String(),
			Status:      "success",
			Message:     "draft imported from collect",
		})
	}
	return out, nil
}

func (s *Service) importDraftCore(ctx context.Context, adminID *uuid.UUID, p ImportDraftParams) (*Product, error) {
	title := strings.TrimSpace(p.Title)
	if title == "" {
		title = "（未命名商品）"
	}
	curr := strings.TrimSpace(p.Currency)
	if curr == "" {
		curr = "CNY"
	}

	raw := datatypes.JSON(nil)
	if len(p.FullNormalizedJSON) > 0 {
		raw = datatypes.JSON(p.FullNormalizedJSON)
	}

	var out *Product
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		pr := &Product{
			TenantID:      0,
			CreatedBy:     adminID,
			Source:        strings.TrimSpace(p.Source),
			SourceURL:     strings.TrimSpace(p.SourceURL),
			OriginalTitle: title,
			Title:         title,
			Description:   strings.TrimSpace(p.Description),
			Currency:      curr,
			Status:        StatusDraft,
			RawData:       raw,
		}
		if pr.Source == "" {
			pr.Source = "unknown"
		}
		if err := tx.Create(pr).Error; err != nil {
			return err
		}

		for i, u := range p.MainImages {
			u = strings.TrimSpace(u)
			if u == "" {
				continue
			}
			img := &ProductImage{
				ProductID: pr.ID,
				ImageType: ImageTypeMain,
				OriginURL: u,
				PublicURL: u,
				SortOrder: i,
			}
			if err := tx.Create(img).Error; err != nil {
				return err
			}
		}
		for i, u := range p.DescriptionImages {
			u = strings.TrimSpace(u)
			if u == "" {
				continue
			}
			img := &ProductImage{
				ProductID: pr.ID,
				ImageType: ImageTypeDetail,
				OriginURL: u,
				PublicURL: u,
				SortOrder: i,
			}
			if err := tx.Create(img).Error; err != nil {
				return err
			}
		}

		for _, line := range p.SKUs {
			var attrs datatypes.JSON
			if len(line.Attrs) > 0 {
				attrs = datatypes.JSON(line.Attrs)
			}
			var rawSKU datatypes.JSON
			if len(line.RawSKU) > 0 {
				rawSKU = datatypes.JSON(line.RawSKU)
			}
			warn, safe := 5, 0
			if s.Settings != nil {
				if m, e := s.Settings.PlainByGroup(ctx, 0, "inventory"); e == nil {
					warn = settings.DefaultWarningStockFromMap(m)
					safe = settings.DefaultSafetyStockFromMap(m)
					warn, safe = settings.CoalesceDefaultStockLines(warn, safe)
				}
			}
			row := &ProductSKU{
				ProductID:    pr.ID,
				SKUCode:      strings.TrimSpace(line.SKUCode),
				SKUName:      strings.TrimSpace(line.SKUName),
				Attrs:        attrs,
				Price:        line.Price,
				CostPrice:    line.CostPrice,
				Stock:        line.Stock,
				ImageURL:     strings.TrimSpace(line.ImageURL),
				RawData:      rawSKU,
				WarningStock: warn,
				SafetyStock:  safe,
			}
			if err := tx.Create(row).Error; err != nil {
				return err
			}
		}

		out = pr
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SKUNameFromProps builds a display name from attribute map keys (deterministic order).
func SKUNameFromProps(props map[string]string) string {
	if len(props) == 0 {
		return ""
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s:%s", k, strings.TrimSpace(props[k])))
	}
	return strings.Join(parts, " · ")
}

func skuPropsJSON(props map[string]string) (json.RawMessage, error) {
	if len(props) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(props)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// BuildImportSKU converts a loose JSON sku object into ImportSKUParams.
func BuildImportSKU(raw json.RawMessage) (ImportSKUParams, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return ImportSKUParams{}, err
	}
	var line ImportSKUParams
	line.RawSKU = raw

	if v, ok := m["skuCode"]; ok {
		var s string
		_ = json.Unmarshal(v, &s)
		line.SKUCode = s
	}
	if v, ok := m["image"]; ok {
		var s string
		_ = json.Unmarshal(v, &s)
		line.ImageURL = s
	}
	if v, ok := m["price"]; ok && string(v) != "null" {
		var f float64
		if err := json.Unmarshal(v, &f); err == nil && f > 0 {
			line.Price = &f
			line.CostPrice = &f
		}
	}
	if v, ok := m["stock"]; ok && string(v) != "null" {
		var f float64
		if err := json.Unmarshal(v, &f); err == nil {
			n := int(f)
			line.Stock = &n
		}
	}
	if v, ok := m["name"]; ok {
		var s string
		_ = json.Unmarshal(v, &s)
		if strings.TrimSpace(s) != "" {
			line.SKUName = strings.TrimSpace(s)
		}
	}
	propsKey := "properties"
	if _, hasProps := m[propsKey]; !hasProps {
		if _, hasAttrs := m["attrs"]; hasAttrs {
			propsKey = "attrs"
		}
	}
	if v, ok := m[propsKey]; ok {
		var props map[string]string
		if err := json.Unmarshal(v, &props); err == nil && len(props) > 0 {
			a, err := skuPropsJSON(props)
			if err != nil {
				return ImportSKUParams{}, err
			}
			line.Attrs = a
			if line.SKUName == "" {
				line.SKUName = SKUNameFromProps(props)
			}
		}
	}
	if line.SKUName == "" && line.SKUCode != "" {
		line.SKUName = line.SKUCode
	}
	return line, nil
}
