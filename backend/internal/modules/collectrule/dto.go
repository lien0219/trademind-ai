package collectrule

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ListQuery struct {
	Page     int
	PageSize int
	Name     string
	Domain   string
	Status   string
}

type RuleListItemDTO struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Source       string    `json:"source"`
	Domain       string    `json:"domain"`
	MatchPattern string    `json:"matchPattern,omitempty"`
	Status       string    `json:"status"`
	Priority     int       `json:"priority"`
	Remark       string    `json:"remark,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type RuleDetailDTO struct {
	RuleListItemDTO
	Rule json.RawMessage `json:"rule"`
}

type ListResult struct {
	Items      []RuleListItemDTO `json:"list"`
	Total      int64             `json:"total"`
	Page       int               `json:"page"`
	PageSize   int               `json:"pageSize"`
	TotalPages int               `json:"totalPages"`
}

type CreateRuleBody struct {
	Name         string          `json:"name"`
	Domain       string          `json:"domain"`
	MatchPattern string          `json:"matchPattern"`
	Priority     *int            `json:"priority"`
	Status       string          `json:"status"`
	Remark       string          `json:"remark"`
	Rule         json.RawMessage `json:"rule"`
}

type UpdateRuleBody struct {
	Name         *string          `json:"name"`
	Domain       *string          `json:"domain"`
	MatchPattern *string          `json:"matchPattern"`
	Priority     *int             `json:"priority"`
	Status       *string          `json:"status"`
	Remark       *string          `json:"remark"`
	Rule         *json.RawMessage `json:"rule"`
}

type TestRuleBody struct {
	URL               string  `json:"url"`
	ProfileID         *string `json:"profileId,omitempty"`
	UseBrowserProfile bool    `json:"useBrowserProfile"`
}

// RuleTestResultDTO is returned by POST /api/v1/collect/rules/:id/test (Collector preview, no task/product).
type RuleTestResultDTO struct {
	AccessStatus    string                 `json:"accessStatus"`
	FinalURL        string                 `json:"finalUrl"`
	HTTPStatus      int                    `json:"httpStatus,omitempty"`
	ExtractedFields map[string]interface{} `json:"extractedFields"`
	MissingFields   []string               `json:"missingFields"`
	Warnings        []string               `json:"warnings"`
	ErrorCode       string                 `json:"errorCode,omitempty"`
	Suggestion      string                 `json:"suggestion"`
	Product         json.RawMessage        `json:"product,omitempty"`
}

func ruleToListDTO(r *CollectRule) RuleListItemDTO {
	if r == nil {
		return RuleListItemDTO{}
	}
	return RuleListItemDTO{
		ID:           r.ID,
		Name:         r.Name,
		Source:       r.Source,
		Domain:       r.Domain,
		MatchPattern: r.MatchPattern,
		Status:       r.Status,
		Priority:     r.Priority,
		Remark:       r.Remark,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

func ruleToDetailDTO(r *CollectRule) RuleDetailDTO {
	if r == nil {
		return RuleDetailDTO{}
	}
	var raw json.RawMessage
	if len(r.Rule) > 0 {
		raw = json.RawMessage(r.Rule)
	}
	return RuleDetailDTO{
		RuleListItemDTO: ruleToListDTO(r),
		Rule:            raw,
	}
}
