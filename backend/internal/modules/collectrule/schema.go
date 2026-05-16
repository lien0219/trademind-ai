package collectrule

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const maxRuleJSONBytes = 64 * 1024
const maxSelectorStringLen = 512
const maxSelectorsPerField = 40

var (
	ErrRuleTooLarge       = errors.New("rule json exceeds size limit")
	ErrRuleInvalidJSON    = errors.New("rule must be a JSON object")
	ErrRuleSchema         = errors.New("rule schema validation failed")
	ErrDomainEmpty        = errors.New("domain is required")
	ErrMatchPatternRegexp = errors.New("matchPattern must be a valid regexp")
)

func clampPriority(p int) int {
	if p < 0 {
		return 0
	}
	if p > 1_000_000 {
		return 1_000_000
	}
	return p
}

// ValidateMatchPattern ensures optional regexp compiles when non-empty.
func ValidateMatchPattern(pat string) error {
	p := strings.TrimSpace(pat)
	if p == "" {
		return nil
	}
	if len(p) > 4096 {
		return ErrMatchPatternRegexp
	}
	if _, err := regexp.Compile(p); err != nil {
		return ErrMatchPatternRegexp
	}
	return nil
}

func ValidateDomain(domain string) error {
	d := strings.TrimSpace(strings.ToLower(domain))
	if d == "" {
		return ErrDomainEmpty
	}
	if len(d) > 512 {
		return fmt.Errorf("%w: domain too long", ErrRuleSchema)
	}
	return nil
}

// ValidateRuleJSON checks declarative rule payload (size + basic shape).
func ValidateRuleJSON(raw []byte) error {
	if len(raw) == 0 {
		return fmt.Errorf("%w: empty rule", ErrRuleSchema)
	}
	if len(raw) > maxRuleJSONBytes {
		return ErrRuleTooLarge
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return ErrRuleInvalidJSON
	}
	for topKey, val := range root {
		switch topKey {
		case "title", "currency", "mainImages", "descriptionImages":
			if err := validateFieldSpec(val); err != nil {
				return fmt.Errorf("%w: %s", ErrRuleSchema, err.Error())
			}
		case "attributes":
			if err := validateAttributes(val); err != nil {
				return fmt.Errorf("%w: attributes: %v", ErrRuleSchema, err)
			}
		case "skus":
			if err := validateSkus(val); err != nil {
				return fmt.Errorf("%w: skus: %v", ErrRuleSchema, err)
			}
		case "fallbacks":
			if err := validateFallbacks(val); err != nil {
				return fmt.Errorf("%w: fallbacks: %v", ErrRuleSchema, err)
			}
		default:
			return fmt.Errorf("%w: unknown key %q", ErrRuleSchema, topKey)
		}
	}
	return nil
}

func validateFallbacks(raw json.RawMessage) error {
	var m map[string]bool
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	for k := range m {
		switch k {
		case "jsonLd", "openGraph", "meta":
		default:
			return fmt.Errorf("unknown fallback %q", k)
		}
	}
	return nil
}

func validateSkus(raw json.RawMessage) error {
	var m struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	switch strings.TrimSpace(strings.ToLower(m.Mode)) {
	case "", "disabled", "simple":
		return nil
	default:
		return fmt.Errorf("unsupported skus.mode")
	}
}

func validateAttributes(raw json.RawMessage) error {
	var m struct {
		Mode          string `json:"mode"`
		RowSelector   string `json:"rowSelector"`
		KeySelector   string `json:"keySelector"`
		ValueSelector string `json:"valueSelector"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	mode := strings.TrimSpace(strings.ToLower(m.Mode))
	switch mode {
	case "", "pairs":
		if err := checkSelectorLen(m.RowSelector); err != nil {
			return err
		}
		if err := checkSelectorLen(m.KeySelector); err != nil {
			return err
		}
		if err := checkSelectorLen(m.ValueSelector); err != nil {
			return err
		}
	case "disabled":
		return nil
	default:
		return fmt.Errorf("unsupported attributes.mode")
	}
	return nil
}

func validateFieldSpec(raw json.RawMessage) error {
	var m struct {
		Selectors []string `json:"selectors"`
		Attr      string   `json:"attr"`
		Multiple  bool     `json:"multiple"`
		Limit     int      `json:"limit"`
		Fallback  string   `json:"fallback"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	if len(m.Selectors) > maxSelectorsPerField {
		return fmt.Errorf("too many selectors")
	}
	for _, s := range m.Selectors {
		if err := checkSelectorLen(s); err != nil {
			return err
		}
	}
	if len(m.Fallback) > 1024 {
		return fmt.Errorf("fallback too long")
	}
	attr := strings.TrimSpace(strings.ToLower(m.Attr))
	switch attr {
	case "text", "html", "src", "href", "content", "data-src", "data-original":
	default:
		if attr != "" {
			return fmt.Errorf("unsupported attr %q", m.Attr)
		}
	}
	if m.Limit < 0 || m.Limit > 500 {
		return fmt.Errorf("limit out of range")
	}
	return nil
}

func checkSelectorLen(s string) error {
	if len(s) > maxSelectorStringLen {
		return fmt.Errorf("selector longer than %d chars", maxSelectorStringLen)
	}
	return nil
}
