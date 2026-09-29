package logistics

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"gorm.io/gorm"
)

var (
	ErrInvalid         = errors.New("invalid logistics input")
	ErrNotFound        = errors.New("logistics record not found")
	ErrConflict        = errors.New("logistics revision or code conflict")
	ErrDestination     = errors.New("shipping destination is incomplete")
	ErrNoQuote         = errors.New("no matching local shipping rate")
	codePattern        = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{0,63}$`)
	countryCodePattern = regexp.MustCompile(`^[A-Z]{2}$`)
)

type Service struct{ DB *gorm.DB }

type CreateChannelInput struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Carrier string `json:"carrier"`
}
type UpdateChannelInput struct {
	ExpectedRevision int    `json:"expectedRevision"`
	Name             string `json:"name"`
	Carrier          string `json:"carrier"`
	Status           string `json:"status"`
}
type RateInput struct {
	ChannelID           uuid.UUID  `json:"channelId"`
	WarehouseID         *uuid.UUID `json:"warehouseId"`
	Code                string     `json:"code"`
	Name                string     `json:"name"`
	CountryCode         string     `json:"countryCode"`
	Region              string     `json:"region"`
	PostalCodePrefix    string     `json:"postalCodePrefix"`
	MinWeightGrams      int        `json:"minWeightGrams"`
	MaxWeightGrams      int        `json:"maxWeightGrams"`
	BaseFeeMinor        int64      `json:"baseFeeMinor"`
	PerKilogramFeeMinor int64      `json:"perKilogramFeeMinor"`
	Currency            string     `json:"currency"`
	Priority            int        `json:"priority"`
}
type CreateRateInput struct{ RateInput }
type UpdateRateInput struct {
	ExpectedRevision int    `json:"expectedRevision"`
	Status           string `json:"status"`
	RateInput
}
type QuoteInput struct {
	WarehouseID uuid.UUID
	CountryCode string
	Region      string
	PostalCode  string
	WeightGrams int
}
type QuoteCandidate struct {
	RateTemplateID       uuid.UUID `json:"rateTemplateId"`
	RateTemplateCode     string    `json:"rateTemplateCode"`
	RateTemplateName     string    `json:"rateTemplateName"`
	RateTemplateRevision int       `json:"rateTemplateRevision"`
	ChannelID            uuid.UUID `json:"channelId"`
	ChannelCode          string    `json:"channelCode"`
	ChannelName          string    `json:"channelName"`
	Carrier              string    `json:"carrier"`
	WeightGrams          int       `json:"weightGrams"`
	MinWeightGrams       int       `json:"minWeightGrams"`
	MaxWeightGrams       int       `json:"maxWeightGrams"`
	AmountMinor          int64     `json:"amountMinor"`
	Currency             string    `json:"currency"`
	Explanation          string    `json:"explanation"`
}

type rateJoinedRow struct {
	ShippingRateTemplate
	JoinedChannelCode   string `gorm:"column:joined_channel_code"`
	JoinedChannelName   string `gorm:"column:joined_channel_name"`
	JoinedCarrier       string `gorm:"column:joined_carrier"`
	JoinedWarehouseCode string `gorm:"column:joined_warehouse_code"`
	JoinedWarehouseName string `gorm:"column:joined_warehouse_name"`
}

func normalizeCode(v string) string   { return strings.ToUpper(strings.TrimSpace(v)) }
func normalizeStatus(v string) string { return strings.ToLower(strings.TrimSpace(v)) }

func (s *Service) ListChannels(ctx context.Context, tenantID int64) ([]ShippingChannel, error) {
	var rows []ShippingChannel
	if s == nil || s.DB == nil || tenantID < 0 {
		return nil, ErrInvalid
	}
	if err := s.DB.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("code ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Service) CreateChannel(ctx context.Context, tenantID int64, actor *uuid.UUID, in CreateChannelInput) (*ShippingChannel, error) {
	code, name, carrier := normalizeCode(in.Code), strings.TrimSpace(in.Name), strings.TrimSpace(in.Carrier)
	if s == nil || s.DB == nil || tenantID < 0 || !codePattern.MatchString(code) || name == "" || carrier == "" || len([]rune(name)) > 160 || len([]rune(carrier)) > 128 {
		return nil, ErrInvalid
	}
	row := &ShippingChannel{TenantID: tenantID, Code: code, Name: name, Carrier: carrier, Status: StatusActive, Revision: 1, CreatedBy: actor}
	if err := s.DB.WithContext(ctx).Create(row).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, ErrConflict
		}
		return nil, err
	}
	return row, nil
}

func (s *Service) UpdateChannel(ctx context.Context, tenantID int64, id uuid.UUID, in UpdateChannelInput) (*ShippingChannel, error) {
	name, carrier, status := strings.TrimSpace(in.Name), strings.TrimSpace(in.Carrier), normalizeStatus(in.Status)
	if s == nil || s.DB == nil || tenantID < 0 || id == uuid.Nil || in.ExpectedRevision < 1 || name == "" || carrier == "" || (status != StatusActive && status != StatusInactive) || len([]rune(name)) > 160 || len([]rune(carrier)) > 128 {
		return nil, ErrInvalid
	}
	result := s.DB.WithContext(ctx).Model(&ShippingChannel{}).Where("tenant_id = ? AND id = ? AND revision = ?", tenantID, id, in.ExpectedRevision).Updates(map[string]any{"name": name, "carrier": carrier, "status": status, "revision": in.ExpectedRevision + 1})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, ErrConflict
	}
	var row ShippingChannel
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func validateRate(in RateInput) (RateInput, error) {
	in.Code = normalizeCode(in.Code)
	in.Name = strings.TrimSpace(in.Name)
	in.CountryCode = normalizeCode(in.CountryCode)
	in.Region = strings.TrimSpace(in.Region)
	in.PostalCodePrefix = strings.ToUpper(strings.TrimSpace(in.PostalCodePrefix))
	in.Currency = normalizeCode(in.Currency)
	if in.ChannelID == uuid.Nil || !codePattern.MatchString(in.Code) || in.Name == "" || len([]rune(in.Name)) > 160 || !countryCodePattern.MatchString(in.CountryCode) || len([]rune(in.Region)) > 120 || len([]rune(in.PostalCodePrefix)) > 32 || in.MinWeightGrams < 0 || in.MaxWeightGrams <= in.MinWeightGrams || in.MaxWeightGrams > 5_000_000 || in.BaseFeeMinor < 0 || in.PerKilogramFeeMinor < 0 || len(in.Currency) != 3 || in.Priority < 0 || in.Priority > 999 {
		return in, ErrInvalid
	}
	if in.WarehouseID != nil && *in.WarehouseID == uuid.Nil {
		return in, ErrInvalid
	}
	return in, nil
}

func (s *Service) validateReferences(ctx context.Context, tenantID int64, in RateInput) error {
	var count int64
	if err := s.DB.WithContext(ctx).Model(&ShippingChannel{}).Where("tenant_id = ? AND id = ?", tenantID, in.ChannelID).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return ErrNotFound
	}
	if in.WarehouseID != nil {
		if err := s.DB.WithContext(ctx).Model(&warehouse.Warehouse{}).Where("tenant_id = ? AND id = ?", tenantID, *in.WarehouseID).Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return ErrNotFound
		}
	}
	return nil
}

func (s *Service) CreateRate(ctx context.Context, tenantID int64, actor *uuid.UUID, raw CreateRateInput) (*ShippingRateTemplate, error) {
	if s == nil || s.DB == nil || tenantID < 0 {
		return nil, ErrInvalid
	}
	in, err := validateRate(raw.RateInput)
	if err != nil {
		return nil, err
	}
	if err := s.validateReferences(ctx, tenantID, in); err != nil {
		return nil, err
	}
	row := &ShippingRateTemplate{TenantID: tenantID, ChannelID: in.ChannelID, WarehouseID: in.WarehouseID, Code: in.Code, Name: in.Name, CountryCode: in.CountryCode, Region: in.Region, PostalCodePrefix: in.PostalCodePrefix, MinWeightGrams: in.MinWeightGrams, MaxWeightGrams: in.MaxWeightGrams, BaseFeeMinor: in.BaseFeeMinor, PerKilogramFeeMinor: in.PerKilogramFeeMinor, Currency: in.Currency, Priority: in.Priority, Status: StatusActive, Revision: 1, CreatedBy: actor}
	if err := s.DB.WithContext(ctx).Create(row).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, ErrConflict
		}
		return nil, err
	}
	return row, nil
}

func (s *Service) UpdateRate(ctx context.Context, tenantID int64, id uuid.UUID, raw UpdateRateInput) (*ShippingRateTemplate, error) {
	if s == nil || s.DB == nil || tenantID < 0 || id == uuid.Nil || raw.ExpectedRevision < 1 {
		return nil, ErrInvalid
	}
	in, err := validateRate(raw.RateInput)
	if err != nil {
		return nil, err
	}
	status := normalizeStatus(raw.Status)
	if status != StatusActive && status != StatusInactive {
		return nil, ErrInvalid
	}
	if err := s.validateReferences(ctx, tenantID, in); err != nil {
		return nil, err
	}
	var existing ShippingRateTemplate
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if existing.Code != in.Code {
		return nil, ErrInvalid
	}
	updates := map[string]any{"channel_id": in.ChannelID, "warehouse_id": in.WarehouseID, "code": in.Code, "name": in.Name, "country_code": in.CountryCode, "region": in.Region, "postal_code_prefix": in.PostalCodePrefix, "min_weight_grams": in.MinWeightGrams, "max_weight_grams": in.MaxWeightGrams, "base_fee_minor": in.BaseFeeMinor, "per_kilogram_fee_minor": in.PerKilogramFeeMinor, "currency": in.Currency, "priority": in.Priority, "status": status, "revision": raw.ExpectedRevision + 1}
	result := s.DB.WithContext(ctx).Model(&ShippingRateTemplate{}).Where("tenant_id = ? AND id = ? AND revision = ?", tenantID, id, raw.ExpectedRevision).Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, ErrConflict
	}
	var row ShippingRateTemplate
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Service) ListRates(ctx context.Context, tenantID int64) ([]ShippingRateTemplate, error) {
	if s == nil || s.DB == nil || tenantID < 0 {
		return nil, ErrInvalid
	}
	var joined []rateJoinedRow
	err := s.DB.WithContext(ctx).Table("shipping_rate_templates AS r").Select("r.*, c.code AS joined_channel_code, c.name AS joined_channel_name, c.carrier AS joined_carrier, w.code AS joined_warehouse_code, w.name AS joined_warehouse_name").Joins("JOIN shipping_channels c ON c.id = r.channel_id AND c.tenant_id = r.tenant_id AND c.deleted_at IS NULL").Joins("LEFT JOIN warehouses w ON w.id = r.warehouse_id AND w.tenant_id = r.tenant_id AND w.deleted_at IS NULL").Where("r.tenant_id = ? AND r.deleted_at IS NULL", tenantID).Order("r.priority DESC, r.code ASC, r.id ASC").Scan(&joined).Error
	rows := make([]ShippingRateTemplate, 0, len(joined))
	for _, item := range joined {
		row := item.ShippingRateTemplate
		row.ChannelCode, row.ChannelName, row.Carrier = item.JoinedChannelCode, item.JoinedChannelName, item.JoinedCarrier
		row.WarehouseCode, row.WarehouseName = item.JoinedWarehouseCode, item.JoinedWarehouseName
		rows = append(rows, row)
	}
	return rows, err
}

func (s *Service) Quote(ctx context.Context, tenantID int64, in QuoteInput) ([]QuoteCandidate, error) {
	if s == nil || s.DB == nil || tenantID < 0 || in.WarehouseID == uuid.Nil || in.WeightGrams <= 0 || in.WeightGrams > 5_000_000 {
		return nil, ErrInvalid
	}
	country, region, postal := normalizeCode(in.CountryCode), strings.TrimSpace(in.Region), strings.ToUpper(strings.TrimSpace(in.PostalCode))
	if !countryCodePattern.MatchString(country) {
		return nil, ErrDestination
	}
	var rows []rateJoinedRow
	err := s.DB.WithContext(ctx).Table("shipping_rate_templates AS r").Select("r.*, c.code AS joined_channel_code, c.name AS joined_channel_name, c.carrier AS joined_carrier").Joins("JOIN shipping_channels c ON c.id = r.channel_id AND c.tenant_id = r.tenant_id AND c.status = ? AND c.deleted_at IS NULL", StatusActive).Where("r.tenant_id = ? AND r.status = ? AND r.country_code = ? AND r.min_weight_grams <= ? AND r.max_weight_grams >= ? AND (r.warehouse_id IS NULL OR r.warehouse_id = ?)", tenantID, StatusActive, country, in.WeightGrams, in.WeightGrams, in.WarehouseID).Order("r.priority DESC, r.code ASC, r.id ASC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]QuoteCandidate, 0, len(rows))
	for _, joined := range rows {
		row := joined.ShippingRateTemplate
		if row.Region != "" && !strings.EqualFold(row.Region, region) {
			continue
		}
		if row.PostalCodePrefix != "" && !strings.HasPrefix(postal, strings.ToUpper(row.PostalCodePrefix)) {
			continue
		}
		extra := in.WeightGrams - row.MinWeightGrams
		if extra < 0 {
			extra = 0
		}
		startedKG := int64((extra + 999) / 1000)
		amount := row.BaseFeeMinor + startedKG*row.PerKilogramFeeMinor
		out = append(out, QuoteCandidate{RateTemplateID: row.ID, RateTemplateCode: row.Code, RateTemplateName: row.Name, RateTemplateRevision: row.Revision, ChannelID: row.ChannelID, ChannelCode: joined.JoinedChannelCode, ChannelName: joined.JoinedChannelName, Carrier: joined.JoinedCarrier, WeightGrams: in.WeightGrams, MinWeightGrams: row.MinWeightGrams, MaxWeightGrams: row.MaxWeightGrams, AmountMinor: amount, Currency: row.Currency, Explanation: fmt.Sprintf("基础费 %d；超过 %dg 后按每起始千克 %d 计费", row.BaseFeeMinor, row.MinWeightGrams, row.PerKilogramFeeMinor)})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].AmountMinor == out[j].AmountMinor {
			return out[i].RateTemplateCode < out[j].RateTemplateCode
		}
		return out[i].AmountMinor < out[j].AmountMinor
	})
	if len(out) == 0 {
		return nil, ErrNoQuote
	}
	return out, nil
}

func (s *Service) QuoteByRate(ctx context.Context, tenantID int64, rateID uuid.UUID, in QuoteInput) (*QuoteCandidate, error) {
	quotes, err := s.Quote(ctx, tenantID, in)
	if err != nil {
		return nil, err
	}
	for i := range quotes {
		if quotes[i].RateTemplateID == rateID {
			return &quotes[i], nil
		}
	}
	return nil, ErrNoQuote
}
