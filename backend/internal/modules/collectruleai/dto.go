package collectruleai

import (
	"encoding/json"

	"github.com/trademind-ai/trademind/backend/internal/modules/collectrule"
)

type GenerateBody struct {
	URL               string   `json:"url"`
	Domain            string   `json:"domain"`
	ProfileID         *string  `json:"profileId,omitempty"`
	UseBrowserProfile bool     `json:"useBrowserProfile"`
	TargetFields      []string `json:"targetFields"`
	RuleName          string   `json:"ruleName,omitempty"`
}

type GenerateAndSaveBody struct {
	GenerateBody
	Name     string `json:"name"`
	Priority *int   `json:"priority"`
	Status   string `json:"status,omitempty"`
}

type GenerateResultDTO struct {
	Rule                   json.RawMessage                `json:"rule"`
	Domain                 string                         `json:"domain"`
	SuggestedName          string                         `json:"suggestedName"`
	Confidence             float64                        `json:"confidence"`
	Explanation            string                         `json:"explanation"`
	Warnings               []string                       `json:"warnings"`
	MissingGeneratedFields []string                       `json:"missingGeneratedFields,omitempty"`
	QualityGate            QualityGateDTO                 `json:"qualityGate"`
	TestResult             *collectrule.RuleTestResultDTO `json:"testResult,omitempty"`
	PlannedHint            string                         `json:"plannedHint,omitempty"`
}

type aiRuleOutput struct {
	Rule                   json.RawMessage `json:"rule"`
	Confidence             float64         `json:"confidence"`
	Explanation            string          `json:"explanation"`
	Warnings               []string        `json:"warnings"`
	MissingGeneratedFields []string        `json:"missingGeneratedFields"`
}
