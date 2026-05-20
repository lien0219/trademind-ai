package collectruleai

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/collectrule"
)

var ErrAIRuleInvalid = errors.New("AI_RULE_INVALID")

var forbiddenRulePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bscript\b`),
	regexp.MustCompile(`(?i)\beval\s*\(`),
	regexp.MustCompile(`(?i)\bfunction\s*\(`),
	regexp.MustCompile(`(?i)javascript\s*:`),
}

var allowedFilterKeys = map[string]struct{}{
	"minWidth": {}, "minHeight": {}, "excludeKeywords": {}, "dedupeByImageKey": {},
}

func validateAIRuleSecurity(raw []byte) error {
	s := string(raw)
	for _, re := range forbiddenRulePatterns {
		if re.MatchString(s) {
			return fmt.Errorf("%w: forbidden token in rule JSON", ErrAIRuleInvalid)
		}
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return fmt.Errorf("%w: not valid JSON object", ErrAIRuleInvalid)
	}
	for key, val := range root {
		switch key {
		case "title", "price", "currency", "mainImages", "descriptionImages":
			if err := validateAIFieldSpec(val); err != nil {
				return err
			}
		case "attributes":
			if err := validateAIAttributes(val); err != nil {
				return err
			}
		case "skus", "fallbacks":
			// validated by collectrule.ValidateRuleJSON
		default:
			return fmt.Errorf("%w: unknown key %q", ErrAIRuleInvalid, key)
		}
	}
	return nil
}

func validateAIFieldSpec(raw json.RawMessage) error {
	var m struct {
		Selectors []string        `json:"selectors"`
		Attr      string          `json:"attr"`
		Filters   json.RawMessage `json:"filters"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("%w: invalid field spec", ErrAIRuleInvalid)
	}
	if len(m.Selectors) == 0 {
		return fmt.Errorf("%w: selectors required", ErrAIRuleInvalid)
	}
	for _, sel := range m.Selectors {
		if strings.TrimSpace(sel) == "" {
			return fmt.Errorf("%w: empty selector", ErrAIRuleInvalid)
		}
		for _, re := range forbiddenRulePatterns {
			if re.MatchString(sel) {
				return fmt.Errorf("%w: forbidden selector content", ErrAIRuleInvalid)
			}
		}
	}
	if len(m.Filters) > 0 {
		var filters map[string]json.RawMessage
		if err := json.Unmarshal(m.Filters, &filters); err != nil {
			return fmt.Errorf("%w: invalid filters", ErrAIRuleInvalid)
		}
		for k := range filters {
			if _, ok := allowedFilterKeys[k]; !ok {
				return fmt.Errorf("%w: filter key %q not allowed", ErrAIRuleInvalid, k)
			}
		}
	}
	return nil
}

func validateAIAttributes(raw json.RawMessage) error {
	var m struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("%w: invalid attributes", ErrAIRuleInvalid)
	}
	mode := strings.TrimSpace(strings.ToLower(m.Mode))
	if mode != "" && mode != "pairs" && mode != "row" && mode != "text_all" && mode != "disabled" {
		return fmt.Errorf("%w: unsupported attributes.mode", ErrAIRuleInvalid)
	}
	return nil
}

func normalizeAndValidateRule(raw json.RawMessage) (json.RawMessage, error) {
	norm, err := collectrule.NormalizeRuleJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAIRuleInvalid, err)
	}
	if err := validateAIRuleSecurity(norm); err != nil {
		return nil, err
	}
	if err := collectrule.ValidateRuleJSON(norm); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAIRuleInvalid, err)
	}
	return norm, nil
}
