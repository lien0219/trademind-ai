package product

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	DouyinTitleMissing           = "DOUYIN_TITLE_MISSING"
	DouyinTitleTooLong           = "DOUYIN_TITLE_TOO_LONG"
	DouyinDescriptionMissing     = "DOUYIN_DESCRIPTION_MISSING"
	DouyinDescriptionNeedsReview = "DOUYIN_DESCRIPTION_NEEDS_REVIEW"
	DouyinMainImageMissing       = "DOUYIN_MAIN_IMAGE_MISSING"
	DouyinImageNeedSync          = "DOUYIN_IMAGE_NEED_SYNC"
	DouyinMainImageNotUploaded   = "DOUYIN_MAIN_IMAGE_NOT_UPLOADED"
	DouyinMainImageUploadFailed  = "DOUYIN_MAIN_IMAGE_UPLOAD_FAILED"
	DouyinDetailImagePartialFail = "DOUYIN_DETAIL_IMAGE_UPLOAD_PARTIAL_FAILED"
	DouyinImageNeedUpload        = "DOUYIN_IMAGE_NEED_UPLOAD"
	DouyinImageUploadExpired     = "DOUYIN_IMAGE_UPLOAD_EXPIRED"
	DouyinDetailImageEmpty       = "DOUYIN_DETAIL_IMAGE_EMPTY"
	DouyinDetailImageNeedSync    = "DOUYIN_DETAIL_IMAGE_NEED_SYNC"
	DouyinAttrValueInvalid       = "DOUYIN_ATTR_VALUE_INVALID"
	DouyinSKUMissing             = "DOUYIN_SKU_MISSING"
	DouyinSKUPriceInvalid        = "DOUYIN_SKU_PRICE_INVALID"
	DouyinSKUStockUnconfirmed    = "DOUYIN_SKU_STOCK_UNCONFIRMED"
	DouyinSKUAttrIncomplete      = "DOUYIN_SKU_ATTR_INCOMPLETE"
	DouyinPriceMissing           = "DOUYIN_PRICE_MISSING"
	DouyinPriceInvalid           = "DOUYIN_PRICE_INVALID"
	DouyinProfitTooLow           = "DOUYIN_PROFIT_TOO_LOW"
	DouyinStockUnconfirmed       = "DOUYIN_STOCK_UNCONFIRMED"
	DouyinStockInvalid           = "DOUYIN_STOCK_INVALID"
	DouyinCollectNeedsReview     = "DOUYIN_COLLECT_NEEDS_REVIEW"

	douyinTitleMaxRunes = 60
)

type DouyinDraftMapping struct {
	Platform          string               `json:"platform"`
	ProductID         string               `json:"productId,omitempty"`
	Source            string               `json:"source,omitempty"`
	ShopID            string               `json:"shopId"`
	CategoryID        string               `json:"categoryId"`
	CategoryPath      string               `json:"categoryPath"`
	Title             string               `json:"title"`
	Description       string               `json:"description"`
	MainImages        []DouyinDraftImage   `json:"mainImages"`
	DetailImages      []DouyinDraftImage   `json:"detailImages"`
	Attributes        []DouyinDraftAttr    `json:"attributes"`
	SKUs              []DouyinDraftSKU     `json:"skus"`
	Price             DouyinDraftPrice     `json:"price"`
	Stock             DouyinDraftStock     `json:"stock"`
	Warnings          []DouyinMappingIssue `json:"warnings"`
	Errors            []DouyinMappingIssue `json:"errors"`
	LastMappedAt      *time.Time           `json:"lastMappedAt,omitempty"`
	PlatformDraftHint map[string]any       `json:"platformDraftHint,omitempty"`
}

type DouyinDraftImage struct {
	LocalImageID     string         `json:"localImageId,omitempty"`
	SourceURL        string         `json:"sourceUrl,omitempty"`
	StorageURL       string         `json:"storageUrl,omitempty"`
	StorageKey       string         `json:"storageKey,omitempty"`
	PlatformImageID  string         `json:"platformImageId,omitempty"`
	PlatformImageURL string         `json:"platformImageUrl,omitempty"`
	ImageType        string         `json:"imageType"`
	URL              string         `json:"url"`
	OriginURL        string         `json:"originUrl,omitempty"`
	PublicURL        string         `json:"publicUrl,omitempty"`
	ObjectKey        string         `json:"objectKey,omitempty"`
	Source           string         `json:"source,omitempty"`
	Status           string         `json:"status"`
	NeedSync         bool           `json:"needSync"`
	UploadStatus     string         `json:"uploadStatus,omitempty"`
	ErrorCode        string         `json:"errorCode,omitempty"`
	ErrorMessage     string         `json:"errorMessage,omitempty"`
	UploadedAt       *time.Time     `json:"uploadedAt,omitempty"`
	Processed        bool           `json:"processed,omitempty"`
	Raw              map[string]any `json:"raw,omitempty"`
}

type DouyinDraftAttr struct {
	AttrID    string          `json:"attrId"`
	Name      string          `json:"name"`
	Required  bool            `json:"required"`
	ValueType string          `json:"valueType,omitempty"`
	Value     any             `json:"value,omitempty"`
	Options   json.RawMessage `json:"options,omitempty"`
}

type DouyinDraftSKU struct {
	LocalSkuID       string         `json:"localSkuId"`
	Name             string         `json:"name"`
	Attrs            map[string]any `json:"attrs"`
	Price            float64        `json:"price"`
	Stock            *int           `json:"stock"`
	ImageURL         string         `json:"imageUrl"`
	PlatformSkuDraft map[string]any `json:"platformSkuDraft"`
}

type DouyinDraftPrice struct {
	Currency string   `json:"currency"`
	Min      *float64 `json:"min,omitempty"`
	Max      *float64 `json:"max,omitempty"`
	CostMin  *float64 `json:"costMin,omitempty"`
	Source   string   `json:"source,omitempty"`
}

type DouyinDraftStock struct {
	Total       *int `json:"total,omitempty"`
	Min         *int `json:"min,omitempty"`
	Unconfirmed bool `json:"unconfirmed"`
}

type DouyinMappingIssue struct {
	Code                string `json:"code"`
	Level               string `json:"level"`
	Message             string `json:"message"`
	Suggestion          string `json:"suggestion,omitempty"`
	Field               string `json:"field,omitempty"`
	RelatedResourceType string `json:"relatedResourceType,omitempty"`
	RelatedResourceID   string `json:"relatedResourceId,omitempty"`
}

type DouyinMappingValidationResult struct {
	ProductID    string               `json:"productId,omitempty"`
	Platform     string               `json:"platform"`
	Status       string               `json:"status"`
	Result       string               `json:"result"`
	CanPublish   bool                 `json:"canPublish"`
	ErrorCount   int                  `json:"errorCount"`
	WarningCount int                  `json:"warningCount"`
	Checks       []DouyinMappingIssue `json:"checks"`
}

func (s *Service) BuildDouyinDraftMapping(ctx context.Context, productID uuid.UUID, shopID string) (*DouyinDraftMapping, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var p Product
	if err := s.DB.WithContext(ctx).
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order ASC, created_at ASC") }).
		Preload("SKUs", func(db *gorm.DB) *gorm.DB { return db.Order("created_at ASC") }).
		First(&p, "id = ?", productID).Error; err != nil {
		return nil, err
	}

	var cfg ProductPlatformPublishConfig
	_ = s.DB.WithContext(ctx).Where("product_id = ? AND platform = ?", productID, "douyin_shop").First(&cfg).Error
	now := time.Now().UTC()
	m := &DouyinDraftMapping{
		Platform:     "douyin_shop",
		ProductID:    productID.String(),
		Source:       strings.TrimSpace(p.Source),
		ShopID:       strings.TrimSpace(shopID),
		CategoryID:   strings.TrimSpace(cfg.CategoryID),
		CategoryPath: strings.TrimSpace(cfg.CategoryPath),
		LastMappedAt: &now,
	}
	if m.ShopID == "" && cfg.ShopID != nil {
		m.ShopID = cfg.ShopID.String()
	}
	m.Title = cleanDouyinTitle(firstNonEmpty(p.AITitle, p.Title, p.OriginalTitle))
	m.Description = cleanDouyinDescription(firstNonEmpty(p.AIDescription, p.Description))
	if m.Description == "" {
		m.Description = descriptionFromAttributes(p.RawData)
	}
	m.MainImages, m.DetailImages = buildDouyinImages(p.Images)
	m.Attributes = buildDouyinAttributes(ctx, s.DB, m.CategoryID, cfg.PlatformAttributes)
	m.SKUs = buildDouyinSKUs(p)
	m.Price = buildDouyinPrice(p)
	m.Stock = buildDouyinStock(p)
	m.PlatformDraftHint = map[string]any{
		"phase": "phase5_preview_only",
		"note":  "no douyin product create or image upload api is called",
	}
	ApplyDouyinDraftValidation(m, s.douyinPricingProtection(ctx))
	return m, nil
}

func (s *Service) SaveDouyinDraftMapping(ctx context.Context, productID uuid.UUID, mapping *DouyinDraftMapping) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("product: no db")
	}
	if mapping == nil {
		return fmt.Errorf("mapping required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	mapping.Platform = "douyin_shop"
	mapping.ProductID = productID.String()
	now := time.Now().UTC()
	mapping.LastMappedAt = &now
	ApplyDouyinDraftValidation(mapping, s.douyinPricingProtection(ctx))

	shopID, err := parseOptionalUUID(mapping.ShopID)
	if err != nil {
		return err
	}
	attrs := attrsArrayToObject(mapping.Attributes)
	attrJSON, err := json.Marshal(attrs)
	if err != nil {
		return err
	}
	imagesJSON, err := json.Marshal(map[string]any{"mainImages": mapping.MainImages, "detailImages": mapping.DetailImages})
	if err != nil {
		return err
	}
	skuJSON, err := json.Marshal(mapping.SKUs)
	if err != nil {
		return err
	}
	priceJSON, err := json.Marshal(mapping.Price)
	if err != nil {
		return err
	}
	stockJSON, err := json.Marshal(mapping.Stock)
	if err != nil {
		return err
	}
	warnJSON, err := json.Marshal(mapping.Warnings)
	if err != nil {
		return err
	}
	errJSON, err := json.Marshal(mapping.Errors)
	if err != nil {
		return err
	}

	row := ProductPlatformPublishConfig{
		ProductID:          productID,
		Platform:           "douyin_shop",
		ShopID:             shopID,
		CategoryID:         strings.TrimSpace(mapping.CategoryID),
		CategoryPath:       strings.TrimSpace(mapping.CategoryPath),
		PlatformAttributes: datatypes.JSON(attrJSON),
		MappedTitle:        strings.TrimSpace(mapping.Title),
		MappedDescription:  strings.TrimSpace(mapping.Description),
		MappedImages:       datatypes.JSON(imagesJSON),
		MappedSKUs:         datatypes.JSON(skuJSON),
		MappedPrice:        datatypes.JSON(priceJSON),
		MappedStock:        datatypes.JSON(stockJSON),
		MappingWarnings:    datatypes.JSON(warnJSON),
		MappingErrors:      datatypes.JSON(errJSON),
		LastMappedAt:       &now,
	}
	return s.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "product_id"}, {Name: "platform"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"shop_id", "category_id", "category_path", "platform_attributes",
			"mapped_title", "mapped_description", "mapped_images", "mapped_skus",
			"mapped_price", "mapped_stock", "mapping_warnings", "mapping_errors",
			"last_mapped_at", "updated_at",
		}),
	}).Create(&row).Error
}

func (s *Service) GetDouyinDraftMapping(ctx context.Context, productID uuid.UUID) (*DouyinDraftMapping, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product: no db")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var row ProductPlatformPublishConfig
	if err := s.DB.WithContext(ctx).Where("product_id = ? AND platform = ?", productID, "douyin_shop").First(&row).Error; err != nil {
		return nil, err
	}
	out := DouyinDraftMappingFromConfig(row)
	var p Product
	if err := s.DB.WithContext(ctx).Select("source").First(&p, "id = ?", productID).Error; err == nil {
		out.Source = strings.TrimSpace(p.Source)
	}
	return out, nil
}

func (s *Service) ValidateDouyinDraftMapping(ctx context.Context, mapping *DouyinDraftMapping) (*DouyinMappingValidationResult, error) {
	if mapping == nil {
		return nil, fmt.Errorf("mapping required")
	}
	ApplyDouyinDraftValidation(mapping, s.douyinPricingProtection(ctx))
	return DouyinValidationResult(mapping), nil
}

func DouyinDraftMappingFromConfig(row ProductPlatformPublishConfig) *DouyinDraftMapping {
	m := &DouyinDraftMapping{
		Platform:          "douyin_shop",
		ProductID:         row.ProductID.String(),
		CategoryID:        row.CategoryID,
		CategoryPath:      row.CategoryPath,
		Title:             row.MappedTitle,
		Description:       row.MappedDescription,
		LastMappedAt:      row.LastMappedAt,
		PlatformDraftHint: map[string]any{"phase": "phase5_preview_only"},
	}
	if row.ShopID != nil {
		m.ShopID = row.ShopID.String()
	}
	var images struct {
		MainImages   []DouyinDraftImage `json:"mainImages"`
		DetailImages []DouyinDraftImage `json:"detailImages"`
	}
	_ = json.Unmarshal(row.MappedImages, &images)
	m.MainImages = images.MainImages
	m.DetailImages = images.DetailImages
	_ = json.Unmarshal(row.MappedSKUs, &m.SKUs)
	_ = json.Unmarshal(row.MappedPrice, &m.Price)
	_ = json.Unmarshal(row.MappedStock, &m.Stock)
	_ = json.Unmarshal(row.MappingWarnings, &m.Warnings)
	_ = json.Unmarshal(row.MappingErrors, &m.Errors)

	var attrs map[string]any
	_ = json.Unmarshal(row.PlatformAttributes, &attrs)
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	m.Attributes = make([]DouyinDraftAttr, 0, len(keys))
	for _, k := range keys {
		m.Attributes = append(m.Attributes, DouyinDraftAttr{AttrID: k, Name: k, Value: attrs[k]})
	}
	return m
}

func ApplyDouyinDraftValidation(m *DouyinDraftMapping, minProfit float64) {
	if m == nil {
		return
	}
	var warnings, errors []DouyinMappingIssue
	addErr := func(code, field, msg, sug string) {
		errors = append(errors, DouyinMappingIssue{Code: code, Level: "error", Field: field, Message: msg, Suggestion: sug})
	}
	addWarn := func(code, field, msg, sug string) {
		warnings = append(warnings, DouyinMappingIssue{Code: code, Level: "warning", Field: field, Message: msg, Suggestion: sug})
	}
	if strings.TrimSpace(m.Title) == "" {
		addErr(DouyinTitleMissing, "title", "Douyin listing title is missing.", "Fill in a Douyin listing title before creating a product.")
	} else if len([]rune(strings.TrimSpace(m.Title))) > douyinTitleMaxRunes {
		addErr(DouyinTitleTooLong, "title", "Douyin listing title is longer than 60 characters.", "Shorten the title manually; TradeMind will not silently truncate it.")
	}
	if strings.TrimSpace(m.Description) == "" {
		addWarn(DouyinDescriptionMissing, "description", "Douyin listing description is empty.", "Add a short product description for manual review before publishing.")
	} else {
		addWarn(DouyinDescriptionNeedsReview, "description", "Douyin listing description should be reviewed manually.", "Check the generated copy before creating a Douyin product.")
	}
	if len(m.MainImages) == 0 {
		addErr(DouyinMainImageMissing, "mainImages", "At least one main image is required.", "Choose or upload at least one main image.")
	}
	for _, im := range m.MainImages {
		if strings.EqualFold(im.UploadStatus, "failed") {
			addErr(DouyinMainImageUploadFailed, "mainImages", "A main image failed to upload to Douyin.", "Retry the failed main image before creating a Douyin product draft.")
			break
		}
		if strings.TrimSpace(im.PlatformImageID) == "" {
			code := DouyinMainImageNotUploaded
			if im.NeedSync {
				code = DouyinImageNeedUpload
			}
			addErr(code, "mainImages", "A main image has not been uploaded to Douyin.", "Upload product images to Douyin before creating a Douyin product draft.")
			break
		}
	}
	if len(m.DetailImages) == 0 {
		addWarn(DouyinDetailImageEmpty, "detailImages", "Detail images are empty.", "Add detail images if this product needs a visual description.")
	}
	for _, im := range m.DetailImages {
		if strings.EqualFold(im.UploadStatus, "failed") {
			addWarn(DouyinDetailImagePartialFail, "detailImages", "Some detail images failed to upload to Douyin.", "Retry failed detail images or remove them before creating a product draft.")
			break
		}
		if strings.TrimSpace(im.PlatformImageID) == "" {
			code := DouyinDetailImageNeedSync
			msg := "A detail image has not been uploaded to Douyin."
			if im.NeedSync {
				code = DouyinImageNeedSync
				msg = "A detail image is still an external URL and needs platform image sync."
			}
			addWarn(code, "detailImages", msg, "Upload detail images to Douyin before creating a product draft.")
			break
		}
	}
	if strings.TrimSpace(m.CategoryID) == "" {
		addErr(shop.DouyinCategoryNotSelected, "categoryId", "Douyin category is not selected.", "Select a Douyin leaf category first.")
	}
	if src := strings.ToLower(strings.TrimSpace(m.Source)); src != "" && src != "manual" {
		addWarn(DouyinCollectNeedsReview, "source", "This product came from collected data and needs manual review.", "Review title, description, images, SKU, price and stock before creating a Douyin product.")
	}
	for _, a := range m.Attributes {
		if a.Required && !douyinValuePresent(a.Value) {
			addErr(shop.DouyinRequiredAttrMissing, "attributes."+a.AttrID, "Required Douyin attribute is missing: "+firstNonEmpty(a.Name, a.AttrID), "Fill in all required Douyin information.")
		}
		if douyinValuePresent(a.Value) && !douyinAttributeValueValid(a) {
			addErr(DouyinAttrValueInvalid, "attributes."+a.AttrID, "Douyin attribute value is invalid: "+firstNonEmpty(a.Name, a.AttrID), "Use a value from the Douyin category options or match the expected value type.")
		}
	}
	if len(m.SKUs) == 0 {
		addErr(DouyinSKUMissing, "skus", "No SKU is available for the Douyin draft.", "Add a SKU or confirm this is a single-spec product.")
	}
	priceMissing := true
	for _, sku := range m.SKUs {
		if strings.TrimSpace(sku.LocalSkuID) != "" {
			// Keep the related resource id on the most actionable SKU findings.
		}
		if sku.Price <= 0 {
			errors = append(errors, DouyinMappingIssue{
				Code: DouyinSKUPriceInvalid, Level: "error", Field: "skus.price",
				Message: "SKU price must be greater than 0.", Suggestion: "Set a valid selling price for every SKU.",
				RelatedResourceType: "product_sku", RelatedResourceID: sku.LocalSkuID,
			})
		} else {
			priceMissing = false
		}
		if sku.Stock == nil {
			warnings = append(warnings, DouyinMappingIssue{
				Code: DouyinSKUStockUnconfirmed, Level: "warning", Field: "skus.stock",
				Message: "SKU stock is not confirmed.", Suggestion: "Confirm stock manually; unknown stock will not be treated as 999.",
				RelatedResourceType: "product_sku", RelatedResourceID: sku.LocalSkuID,
			})
		} else if *sku.Stock < 0 {
			errors = append(errors, DouyinMappingIssue{
				Code: DouyinStockInvalid, Level: "error", Field: "skus.stock",
				Message: "SKU stock cannot be negative.", Suggestion: "Set stock to a non-negative number.",
				RelatedResourceType: "product_sku", RelatedResourceID: sku.LocalSkuID,
			})
		}
		if len(sku.Attrs) == 0 {
			warnings = append(warnings, DouyinMappingIssue{
				Code: DouyinSKUAttrIncomplete, Level: "warning", Field: "skus.attrs",
				Message: "SKU specification is incomplete.", Suggestion: "Complete product specification values if the product has variants.",
				RelatedResourceType: "product_sku", RelatedResourceID: sku.LocalSkuID,
			})
		}
	}
	if priceMissing || m.Price.Min == nil {
		addErr(DouyinPriceMissing, "price", "Selling price is missing.", "Set selling price before publishing.")
	} else if *m.Price.Min <= 0 {
		addErr(DouyinPriceInvalid, "price", "Selling price must be greater than 0.", "Set a valid selling price.")
	}
	if minProfit > 0 && m.Price.Min != nil && m.Price.CostMin != nil && *m.Price.CostMin > 0 && (*m.Price.Min-*m.Price.CostMin) < minProfit {
		addErr(DouyinProfitTooLow, "price", "Estimated profit is below the minimum profit rule.", "Raise selling price or adjust the pricing rule before publishing.")
	}
	if m.Stock.Unconfirmed {
		addWarn(DouyinStockUnconfirmed, "stock", "Some SKU stock values are unconfirmed.", "Confirm stock manually before creating a Douyin product.")
	}
	m.Warnings = dedupeDouyinIssues(warnings)
	m.Errors = dedupeDouyinIssues(errors)
}

func DouyinValidationResult(m *DouyinDraftMapping) *DouyinMappingValidationResult {
	if m == nil {
		return &DouyinMappingValidationResult{Platform: "douyin_shop", Status: "blocked", Result: "failed", CanPublish: false}
	}
	checks := make([]DouyinMappingIssue, 0, len(m.Errors)+len(m.Warnings))
	checks = append(checks, m.Errors...)
	checks = append(checks, m.Warnings...)
	status := "ready"
	result := "passed"
	if len(m.Errors) > 0 {
		status = "blocked"
		result = "failed"
	} else if len(m.Warnings) > 0 {
		status = "warning"
		result = "warning"
	}
	return &DouyinMappingValidationResult{
		ProductID:    m.ProductID,
		Platform:     "douyin_shop",
		Status:       status,
		Result:       result,
		CanPublish:   len(m.Errors) == 0,
		ErrorCount:   len(m.Errors),
		WarningCount: len(m.Warnings),
		Checks:       checks,
	}
}

func (s *Service) douyinPricingProtection(ctx context.Context) float64 {
	if s == nil || s.Settings == nil {
		return 0
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "pricing")
	if err != nil {
		return 0
	}
	return parseDouyinFloat(settings.PricingRuleFromMap(m, "douyin_shop")["minProfit"])
}

func parseDouyinFloat(raw string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

func buildDouyinImages(rows []ProductImage) ([]DouyinDraftImage, []DouyinDraftImage) {
	mainImages := make([]DouyinDraftImage, 0)
	detailImages := make([]DouyinDraftImage, 0)
	for _, im := range rows {
		t := strings.ToLower(strings.TrimSpace(im.ImageType))
		if t == ImageTypeDescription {
			t = ImageTypeDetail
		}
		if t != ImageTypeMain && t != ImageTypeDetail && !(im.IsBestMain && t == ImageTypeAIGenerated) {
			continue
		}
		d := douyinImageFromRow(im, t)
		if d.URL == "" {
			continue
		}
		if t == ImageTypeDetail {
			detailImages = append(detailImages, d)
		} else {
			d.ImageType = ImageTypeMain
			mainImages = append(mainImages, d)
		}
	}
	return mainImages, detailImages
}

func douyinImageFromRow(im ProductImage, imageType string) DouyinDraftImage {
	u := strings.TrimSpace(im.PublicURL)
	if u == "" {
		u = strings.TrimSpace(im.OriginURL)
	}
	if u == "" {
		u = strings.TrimSpace(im.ObjectKey)
	}
	hasStorage := strings.TrimSpace(im.ObjectKey) != "" || strings.TrimSpace(im.StorageKey) != ""
	needSync := u != "" && !hasStorage
	status := "ready"
	if needSync {
		status = "need_sync"
	}
	return DouyinDraftImage{
		LocalImageID: im.ID.String(),
		SourceURL:    strings.TrimSpace(im.OriginURL),
		StorageURL:   strings.TrimSpace(im.PublicURL),
		StorageKey:   firstNonEmpty(strings.TrimSpace(im.StorageKey), strings.TrimSpace(im.ObjectKey)),
		ImageType:    imageType,
		URL:          u,
		OriginURL:    strings.TrimSpace(im.OriginURL),
		PublicURL:    strings.TrimSpace(im.PublicURL),
		ObjectKey:    strings.TrimSpace(im.ObjectKey),
		Source:       strings.TrimSpace(im.Source),
		Status:       status,
		NeedSync:     needSync,
		UploadStatus: "pending",
	}
}

func buildDouyinAttributes(ctx context.Context, db *gorm.DB, categoryID string, valuesJSON datatypes.JSON) []DouyinDraftAttr {
	var values map[string]any
	_ = json.Unmarshal(valuesJSON, &values)
	if values == nil {
		values = map[string]any{}
	}
	var attrs []shop.PlatformCategoryAttribute
	if db != nil && strings.TrimSpace(categoryID) != "" {
		_ = db.WithContext(ctx).Where("platform = ? AND category_id = ?", "douyin_shop", strings.TrimSpace(categoryID)).
			Order("required DESC, name ASC, attr_id ASC").Find(&attrs).Error
	}
	out := make([]DouyinDraftAttr, 0, len(attrs)+len(values))
	seen := map[string]struct{}{}
	for _, attr := range attrs {
		v, ok := values[attr.AttrID]
		if !ok {
			v = values[attr.Name]
		}
		out = append(out, DouyinDraftAttr{
			AttrID:    attr.AttrID,
			Name:      attr.Name,
			Required:  attr.Required,
			ValueType: strings.TrimSpace(attr.ValueType),
			Value:     v,
			Options:   json.RawMessage(attr.Options),
		})
		seen[attr.AttrID] = struct{}{}
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		if _, ok := seen[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, DouyinDraftAttr{AttrID: k, Name: k, Value: values[k]})
	}
	return out
}

func buildDouyinSKUs(p Product) []DouyinDraftSKU {
	if len(p.SKUs) == 0 {
		return []DouyinDraftSKU{{
			Name:             "Default",
			Attrs:            map[string]any{},
			Price:            0,
			Stock:            nil,
			PlatformSkuDraft: map[string]any{"singleSpec": true},
		}}
	}
	out := make([]DouyinDraftSKU, 0, len(p.SKUs))
	for _, s := range p.SKUs {
		attrs := map[string]any{}
		_ = json.Unmarshal(s.Attrs, &attrs)
		price := 0.0
		if s.Price != nil {
			price = *s.Price
		}
		name := strings.TrimSpace(s.SKUName)
		if name == "" {
			name = strings.TrimSpace(s.SKUCode)
		}
		if name == "" {
			name = "SKU"
		}
		out = append(out, DouyinDraftSKU{
			LocalSkuID: s.ID.String(),
			Name:       name,
			Attrs:      attrs,
			Price:      price,
			Stock:      s.Stock,
			ImageURL:   strings.TrimSpace(s.ImageURL),
			PlatformSkuDraft: map[string]any{
				"localSkuCode": strings.TrimSpace(s.SKUCode),
				"previewOnly":  true,
			},
		})
	}
	return out
}

func buildDouyinPrice(p Product) DouyinDraftPrice {
	curr := strings.TrimSpace(p.Currency)
	if curr == "" {
		curr = "CNY"
	}
	var minPrice, maxPrice, minCost *float64
	for _, s := range p.SKUs {
		if s.Price != nil {
			v := *s.Price
			if minPrice == nil || v < *minPrice {
				vv := v
				minPrice = &vv
			}
			if maxPrice == nil || v > *maxPrice {
				vv := v
				maxPrice = &vv
			}
		}
		if s.CostPrice != nil {
			v := *s.CostPrice
			if minCost == nil || v < *minCost {
				vv := v
				minCost = &vv
			}
		}
	}
	return DouyinDraftPrice{Currency: curr, Min: minPrice, Max: maxPrice, CostMin: minCost, Source: "product_skus"}
}

func buildDouyinStock(p Product) DouyinDraftStock {
	var total, min *int
	unconfirmed := len(p.SKUs) == 0
	for _, s := range p.SKUs {
		if s.Stock == nil {
			unconfirmed = true
			continue
		}
		v := *s.Stock
		if total == nil {
			vv := 0
			total = &vv
		}
		*total += v
		if min == nil || v < *min {
			vv := v
			min = &vv
		}
	}
	return DouyinDraftStock{Total: total, Min: min, Unconfirmed: unconfirmed}
}

func attrsArrayToObject(attrs []DouyinDraftAttr) map[string]any {
	out := map[string]any{}
	for _, a := range attrs {
		key := strings.TrimSpace(a.AttrID)
		if key == "" {
			key = strings.TrimSpace(a.Name)
		}
		if key == "" {
			continue
		}
		out[key] = a.Value
	}
	return out
}

func cleanDouyinTitle(raw string) string {
	s := normalizeSpaces(raw)
	for _, w := range []string{"1688", "阿里巴巴", "淘宝", "天猫", "拼多多", "PDD", "pdd", "采集", "同款"} {
		s = strings.ReplaceAll(s, w, "")
	}
	return normalizeSpaces(s)
}

func cleanDouyinDescription(raw string) string {
	s := html.UnescapeString(raw)
	s = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`).ReplaceAllString(s, " ")
	s = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`).ReplaceAllString(s, " ")
	s = regexp.MustCompile(`(?s)<[^>]+>`).ReplaceAllString(s, " ")
	return normalizeSpaces(s)
}

func descriptionFromAttributes(raw []byte) string {
	attrs := collectAttributeStrings(raw)
	if len(attrs) == 0 {
		return ""
	}
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+": "+attrs[k])
	}
	return strings.Join(parts, "\n")
}

func collectAttributeStrings(raw []byte) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil
	}
	pick := func(v any) map[string]string {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		out := map[string]string{}
		for k, v := range m {
			key := strings.TrimSpace(k)
			val := normalizeSpaces(fmt.Sprint(v))
			if key != "" && val != "" && val != "<nil>" {
				out[key] = val
			}
		}
		return out
	}
	if out := pick(root["attributes"]); len(out) > 0 {
		return out
	}
	if rawObj, ok := root["raw"].(map[string]any); ok {
		if out := pick(rawObj["attributeCandidates"]); len(out) > 0 {
			return out
		}
		if out := pick(rawObj["attributes"]); len(out) > 0 {
			return out
		}
	}
	return nil
}

func normalizeSpaces(raw string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func parseOptionalUUID(raw string) (*uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	u, err := uuid.Parse(raw)
	if err != nil || u == uuid.Nil {
		return nil, fmt.Errorf("invalid shopId")
	}
	return &u, nil
}

func douyinValuePresent(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(x) != ""
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	default:
		return true
	}
}

func douyinAttributeValueValid(a DouyinDraftAttr) bool {
	v := a.Value
	vt := strings.ToLower(strings.TrimSpace(a.ValueType))
	if strings.Contains(vt, "number") || strings.Contains(vt, "int") || strings.Contains(vt, "float") {
		switch x := v.(type) {
		case float64, int, int64, json.Number:
			return true
		case string:
			_, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
			return err == nil
		default:
			return false
		}
	}
	options := optionValueSet(a.Options)
	if len(options) == 0 {
		return true
	}
	values := []string{strings.TrimSpace(fmt.Sprint(v))}
	if arr, ok := v.([]any); ok {
		values = values[:0]
		for _, item := range arr {
			values = append(values, strings.TrimSpace(fmt.Sprint(item)))
		}
	}
	for _, val := range values {
		if val == "" {
			return false
		}
		if _, ok := options[val]; !ok {
			return false
		}
	}
	return true
}

func optionValueSet(raw json.RawMessage) map[string]struct{} {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil
	}
	out := map[string]struct{}{}
	for _, item := range arr {
		for _, k := range []string{"id", "value", "name", "label"} {
			if v := strings.TrimSpace(fmt.Sprint(item[k])); v != "" && v != "<nil>" {
				out[v] = struct{}{}
			}
		}
	}
	return out
}

func dedupeDouyinIssues(in []DouyinMappingIssue) []DouyinMappingIssue {
	out := make([]DouyinMappingIssue, 0, len(in))
	seen := map[string]struct{}{}
	for _, item := range in {
		key := item.Code + "|" + item.Field + "|" + item.RelatedResourceID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}
