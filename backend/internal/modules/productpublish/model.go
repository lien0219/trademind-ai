package productpublish

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// ProductPublishTask is one async listings job.
type ProductPublishTask struct {
	model.HardDeleteBase
	ProductID    uuid.UUID      `gorm:"type:char(36);index;not null" json:"productId"`
	ShopID       uuid.UUID      `gorm:"type:char(36);index;not null" json:"shopId"`
	Platform     string         `gorm:"size:64;index;not null" json:"platform"`
	TaskType     string         `gorm:"size:64;index;not null" json:"taskType"`
	Status       string         `gorm:"size:32;index;not null" json:"status"`
	Mode         string         `gorm:"size:32;index;not null" json:"mode"`
	StartedAt    *time.Time     `json:"startedAt,omitempty"`
	FinishedAt   *time.Time     `json:"finishedAt,omitempty"`
	ErrorMessage string         `gorm:"type:text" json:"errorMessage,omitempty"`
	Input        datatypes.JSON `gorm:"type:jsonb" json:"input,omitempty"`
	Output       datatypes.JSON `gorm:"type:jsonb" json:"output,omitempty"`
	CreatedBy    *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	LockedBy     *string        `gorm:"size:220;index" json:"lockedBy,omitempty"`
	LockedUntil  *time.Time     `gorm:"index" json:"lockedUntil,omitempty"`
	LockVersion  int            `gorm:"default:0;not null" json:"lockVersion"`
}

func (ProductPublishTask) TableName() string { return "product_publish_tasks" }

// ProductPublication tracks remote listing linkage for a draft + shop pair.
type ProductPublication struct {
	model.Base
	ProductID         uuid.UUID      `gorm:"type:char(36);index;not null" json:"productId"`
	ShopID            uuid.UUID      `gorm:"type:char(36);index;not null" json:"shopId"`
	Platform          string         `gorm:"size:64;index;not null" json:"platform"`
	PublishTaskID     *uuid.UUID     `gorm:"type:char(36);index" json:"publishTaskId,omitempty"`
	ExternalProductID string         `gorm:"size:512;index" json:"externalProductId,omitempty"`
	ExternalSPUID     string         `gorm:"size:512" json:"externalSpuId,omitempty"`
	Status            string         `gorm:"size:32;index;not null" json:"status"`
	PublishStatus     string         `gorm:"size:32;index;not null" json:"publishStatus"`
	Title             string         `gorm:"size:512" json:"title,omitempty"`
	Currency          string         `gorm:"size:16" json:"currency,omitempty"`
	ExternalURL       string         `gorm:"type:text" json:"externalUrl,omitempty"`
	PublishedAt       *time.Time     `json:"publishedAt,omitempty"`
	LastSyncedAt      *time.Time     `json:"lastSyncedAt,omitempty"`
	RawData           datatypes.JSON `gorm:"type:jsonb" json:"rawData,omitempty"`
	CreatedBy         *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (ProductPublication) TableName() string { return "product_publications" }

// ProductPublicationSKU maps local SKU to external listing SKU.
type ProductPublicationSKU struct {
	model.HardDeleteBase
	PublicationID uuid.UUID      `gorm:"type:char(36);index;not null" json:"publicationId"`
	ProductSKUID  *uuid.UUID     `gorm:"type:char(36);index" json:"productSkuId,omitempty"`
	ExternalSKUID string         `gorm:"size:256" json:"externalSkuId,omitempty"`
	SKUCode       string         `gorm:"size:128" json:"skuCode,omitempty"`
	Price         *float64       `json:"price,omitempty"`
	Stock         *int           `json:"stock,omitempty"`
	RawData       datatypes.JSON `gorm:"type:jsonb" json:"rawData,omitempty"`
}

func (ProductPublicationSKU) TableName() string { return "product_publication_skus" }
