package settings

import "time"

// Setting matches docs/settings table (PostgreSQL BIGSERIAL; GORM autoIncrement).
type Setting struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID    int64     `gorm:"default:0;not null;uniqueIndex:ux_settings_tenant_group_item" json:"tenantId"`
	GroupKey    string    `gorm:"size:100;not null;uniqueIndex:ux_settings_tenant_group_item" json:"groupKey"`
	ItemKey     string    `gorm:"size:100;not null;uniqueIndex:ux_settings_tenant_group_item" json:"itemKey"`
	ItemValue   string    `gorm:"type:text" json:"itemValue"`
	ValueType   string    `gorm:"size:50;default:string" json:"valueType"`
	IsEncrypted bool      `gorm:"default:false" json:"isEncrypted"`
	Remark      string    `gorm:"size:255" json:"remark"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (Setting) TableName() string {
	return "settings"
}
