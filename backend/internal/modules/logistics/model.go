package logistics

import (
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

const (
	StatusActive   = "active"
	StatusInactive = "inactive"
)

// ShippingChannel is tenant-owned local carrier master data. It does not
// contain carrier credentials and does not enable any external carrier write.
type ShippingChannel struct {
	model.Base
	TenantID  int64      `gorm:"not null;uniqueIndex:ux_shipping_channel_code,priority:1;index" json:"tenantId"`
	Code      string     `gorm:"size:64;not null;uniqueIndex:ux_shipping_channel_code,priority:2" json:"code"`
	Name      string     `gorm:"size:160;not null;index" json:"name"`
	Carrier   string     `gorm:"size:128;not null" json:"carrier"`
	Status    string     `gorm:"size:24;not null;default:active;index" json:"status"`
	Revision  int        `gorm:"not null;default:1" json:"revision"`
	CreatedBy *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (ShippingChannel) TableName() string { return "shipping_channels" }

// ShippingRateTemplate is a deterministic local rate rule. BaseFeeMinor
// covers MinWeightGrams; each started kilogram above it adds PerKilogramFeeMinor.
type ShippingRateTemplate struct {
	model.Base
	TenantID            int64      `gorm:"not null;uniqueIndex:ux_shipping_rate_code,priority:1;index" json:"tenantId"`
	ChannelID           uuid.UUID  `gorm:"type:char(36);not null;index" json:"channelId"`
	WarehouseID         *uuid.UUID `gorm:"type:char(36);index" json:"warehouseId,omitempty"`
	Code                string     `gorm:"size:64;not null;uniqueIndex:ux_shipping_rate_code,priority:2" json:"code"`
	Name                string     `gorm:"size:160;not null" json:"name"`
	CountryCode         string     `gorm:"size:2;not null;index" json:"countryCode"`
	Region              string     `gorm:"size:120;index" json:"region,omitempty"`
	PostalCodePrefix    string     `gorm:"size:32;index" json:"postalCodePrefix,omitempty"`
	MinWeightGrams      int        `gorm:"not null;default:0" json:"minWeightGrams"`
	MaxWeightGrams      int        `gorm:"not null" json:"maxWeightGrams"`
	BaseFeeMinor        int64      `gorm:"not null;default:0" json:"baseFeeMinor"`
	PerKilogramFeeMinor int64      `gorm:"not null;default:0" json:"perKilogramFeeMinor"`
	Currency            string     `gorm:"size:8;not null;default:CNY" json:"currency"`
	Priority            int        `gorm:"not null;default:0;index" json:"priority"`
	Status              string     `gorm:"size:24;not null;default:active;index" json:"status"`
	Revision            int        `gorm:"not null;default:1" json:"revision"`
	CreatedBy           *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	ChannelCode         string     `gorm:"-" json:"channelCode,omitempty"`
	ChannelName         string     `gorm:"-" json:"channelName,omitempty"`
	Carrier             string     `gorm:"-" json:"carrier,omitempty"`
	WarehouseCode       string     `gorm:"-" json:"warehouseCode,omitempty"`
	WarehouseName       string     `gorm:"-" json:"warehouseName,omitempty"`
}

func (ShippingRateTemplate) TableName() string { return "shipping_rate_templates" }
