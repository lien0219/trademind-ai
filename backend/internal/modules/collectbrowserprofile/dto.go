package collectbrowserprofile

import (
	"time"

	"github.com/google/uuid"
)

type ListQuery struct {
	Page     int
	PageSize int
	Domain   string
	Provider string
	Status   string
}

type RowDTO struct {
	ID              uuid.UUID  `json:"id"`
	Name            string     `json:"name"`
	Domain          string     `json:"domain"`
	ProfileKey      string     `json:"profileKey"`
	Provider        string     `json:"provider,omitempty"`
	Status          string     `json:"status"`
	LastCheckStatus string     `json:"lastCheckStatus,omitempty"`
	LastCheckURL    string     `json:"lastCheckUrl,omitempty"`
	LastCheckAt     *time.Time `json:"lastCheckAt,omitempty"`
	LastErrorCode   string     `json:"lastErrorCode,omitempty"`
	Remark          string     `json:"remark,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type CreateBody struct {
	Name     string `json:"name"`
	Domain   string `json:"domain"`
	Provider string `json:"provider"`
	Remark   string `json:"remark"`
}

type CreateResultDTO struct {
	ProfileID  uuid.UUID `json:"profileId"`
	ProfileKey string    `json:"profileKey"`
	Row        RowDTO    `json:"profile"`
}

type URLBody struct {
	URL string `json:"url"`
}

type OpenLoginResultDTO struct {
	Message    string `json:"message"`
	ProfileKey string `json:"profileKey"`
}

type CheckResultDTO struct {
	AccessStatus string `json:"accessStatus"`
	FinalURL     string `json:"finalUrl"`
	ErrorCode    string `json:"errorCode,omitempty"`
	Message      string `json:"message"`
}

type ProfileSnapshot struct {
	ProfileID         string `json:"profileId"`
	ProfileKey        string `json:"profileKey"`
	UseBrowserProfile bool   `json:"useBrowserProfile"`
}
