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
	ProductID         uuid.UUID      `gorm:"type:char(36);index;not null" json:"productId"`
	ShopID            uuid.UUID      `gorm:"type:char(36);index;not null" json:"shopId"`
	BatchID           *uuid.UUID     `gorm:"type:char(36);index" json:"batchId,omitempty"`
	TargetKey         string         `gorm:"size:128;index" json:"targetKey,omitempty"`
	Platform          string         `gorm:"size:64;index;not null" json:"platform"`
	TargetStoreID     uuid.UUID      `gorm:"type:char(36);index" json:"targetStoreId"`
	TaskType          string         `gorm:"size:64;index;not null" json:"taskType"`
	Status            string         `gorm:"size:32;index;not null" json:"status"`
	PublishStatus     string         `gorm:"size:32;index" json:"publishStatus"`
	Mode              string         `gorm:"size:32;index;not null" json:"mode"`
	PublishMode       string         `gorm:"size:32;index" json:"publishMode"`
	Title             string         `gorm:"size:512" json:"title,omitempty"`
	Description       string         `gorm:"type:text" json:"description,omitempty"`
	Images            datatypes.JSON `gorm:"type:jsonb" json:"images,omitempty"`
	SKUs              datatypes.JSON `gorm:"type:jsonb" json:"skus,omitempty"`
	Price             *float64       `json:"price,omitempty"`
	Currency          string         `gorm:"size:16" json:"currency,omitempty"`
	CheckResult       datatypes.JSON `gorm:"type:jsonb" json:"checkResult,omitempty"`
	MappingSnapshot   datatypes.JSON `gorm:"type:jsonb" json:"mappingSnapshot,omitempty"`
	PlatformPayload   datatypes.JSON `gorm:"type:jsonb" json:"platformPayload,omitempty"`
	PlatformResult    datatypes.JSON `gorm:"type:jsonb" json:"platformResult,omitempty"`
	PlatformProductID string         `gorm:"size:512;index" json:"platformProductId,omitempty"`
	PlatformRawError  datatypes.JSON `gorm:"type:jsonb" json:"platformRawError,omitempty"`
	Retryable         bool           `gorm:"default:false;index" json:"retryable"`
	RequestID         string         `gorm:"size:128;index" json:"requestId,omitempty"`
	StartedAt         *time.Time     `json:"startedAt,omitempty"`
	FinishedAt        *time.Time     `json:"finishedAt,omitempty"`
	ErrorCode         string         `gorm:"size:96;index" json:"errorCode,omitempty"`
	ErrorMessage      string         `gorm:"type:text" json:"errorMessage,omitempty"`
	Input             datatypes.JSON `gorm:"type:jsonb" json:"input,omitempty"`
	Output            datatypes.JSON `gorm:"type:jsonb" json:"output,omitempty"`
	CreatedBy         *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	LockedBy          *string        `gorm:"size:220;index" json:"lockedBy,omitempty"`
	LockedUntil       *time.Time     `gorm:"index" json:"lockedUntil,omitempty"`
	LockVersion       int            `gorm:"default:0;not null" json:"lockVersion"`
}

func (ProductPublishTask) TableName() string { return "product_publish_tasks" }

// ProductPublication tracks remote listing linkage for a draft + shop pair.
type ProductPublication struct {
	model.Base
	ProductID          uuid.UUID      `gorm:"type:char(36);index;not null" json:"productId"`
	ShopID             uuid.UUID      `gorm:"type:char(36);index;not null" json:"shopId"`
	Platform           string         `gorm:"size:64;index;not null" json:"platform"`
	PublishTaskID      *uuid.UUID     `gorm:"type:char(36);index" json:"publishTaskId,omitempty"`
	ExternalProductID  string         `gorm:"size:512;index" json:"externalProductId,omitempty"`
	ExternalSPUID      string         `gorm:"size:512" json:"externalSpuId,omitempty"`
	Status             string         `gorm:"size:32;index;not null" json:"status"`
	PublishStatus      string         `gorm:"size:32;index;not null" json:"publishStatus"`
	PublishMode        string         `gorm:"size:32;index" json:"publishMode,omitempty"`
	PlatformCategoryID string         `gorm:"size:128;index" json:"platformCategoryId,omitempty"`
	Title              string         `gorm:"size:512" json:"title,omitempty"`
	Currency           string         `gorm:"size:16" json:"currency,omitempty"`
	ExternalURL        string         `gorm:"type:text" json:"externalUrl,omitempty"`
	PublishedAt        *time.Time     `json:"publishedAt,omitempty"`
	LastSyncedAt       *time.Time     `json:"lastSyncedAt,omitempty"`
	SkuBindingSyncedAt *time.Time     `json:"skuBindingSyncedAt,omitempty"`
	RawData            datatypes.JSON `gorm:"type:jsonb" json:"rawData,omitempty"`
	CreatedBy          *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (ProductPublication) TableName() string { return "product_publications" }

// ProductPublicationSKU maps local SKU to external listing SKU.
type ProductPublicationSKU struct {
	model.HardDeleteBase
	PublicationID  uuid.UUID      `gorm:"type:char(36);index;not null" json:"publicationId"`
	ProductSKUID   *uuid.UUID     `gorm:"column:product_sku_id;type:char(36);index" json:"productSkuId,omitempty"`
	ExternalSKUID  string         `gorm:"column:external_sku_id;size:256" json:"externalSkuId,omitempty"`
	SKUCode        string         `gorm:"size:128" json:"skuCode,omitempty"`
	Price          *float64       `json:"price,omitempty"`
	Stock          *int           `json:"stock,omitempty"`
	BindStatus     string         `gorm:"size:32;index" json:"bindStatus,omitempty"`
	BindConfidence int            `gorm:"default:0" json:"bindConfidence,omitempty"`
	BindMessage    string         `gorm:"type:text" json:"bindMessage,omitempty"`
	LastSyncedAt   *time.Time     `json:"lastSyncedAt,omitempty"`
	RawData        datatypes.JSON `gorm:"type:jsonb" json:"rawData,omitempty"`
}

func (ProductPublicationSKU) TableName() string { return "product_publication_skus" }
