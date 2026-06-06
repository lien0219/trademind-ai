package productcheck

import "github.com/google/uuid"

// CheckProductReadinessRequest carries inputs for a single readiness evaluation.
type CheckProductReadinessRequest struct {
	ProductID      uuid.UUID
	Platform       string
	ShopID         *uuid.UUID
	Mode           string
	PublishOptions map[string]any
}

// BatchReadinessRequest is the body for POST /products/readiness/batch.
type BatchReadinessRequest struct {
	ProductIDs []uuid.UUID
	Platform   string
	ShopID     uuid.UUID
}

// CheckProductReadinessResult aggregates readiness outcome for one product.
type CheckProductReadinessResult struct {
	ProductID    uuid.UUID   `json:"productId"`
	Platform     string      `json:"platform,omitempty"`
	ShopID       *uuid.UUID  `json:"shopId,omitempty"`
	Mode         string      `json:"mode,omitempty"`
	Status       string      `json:"status"`
	Result       string      `json:"result"`
	Score        int         `json:"score"`
	CanPublish   bool        `json:"canPublish"`
	ErrorCount   int         `json:"errorCount"`
	WarningCount int         `json:"warningCount"`
	Checks       []CheckItem `json:"checks"`
}

// CheckItem is one non-pass finding (warnings and errors only).
type CheckItem struct {
	Group               string `json:"group"`
	Code                string `json:"code"`
	Level               string `json:"level"`
	Message             string `json:"message"`
	Suggestion          string `json:"suggestion,omitempty"`
	RelatedResourceType string `json:"relatedResourceType,omitempty"`
	RelatedResourceID   string `json:"relatedResourceId,omitempty"`
}
