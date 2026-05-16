package platform

import (
	"encoding/json"
	"strings"
)

// PlatformAppConfigSchema describes deploy-level Open Platform credentials stored in settings (group_item_key snake_case).
type PlatformAppConfigSchema struct {
	GroupKey    string           `json:"groupKey"` // e.g. platform_tiktok; empty => no dedicated app settings UI
	Title       string           `json:"title"`
	Description string           `json:"description,omitempty"`
	Fields      []AppConfigField `json:"fields"` // omit or empty slice when GroupKey==""
}

// AppConfigField is one admin-editable settings row inside groupKey.
type AppConfigField struct {
	Name          string               `json:"name"`
	Label         string               `json:"label"`
	Type          string               `json:"type"` // text, password, number, switch, select, textarea
	Required      bool                 `json:"required"`
	Sensitive     bool                 `json:"sensitive"` // persists with is_encrypted=true
	Placeholder   string               `json:"placeholder,omitempty"`
	Help          string               `json:"help,omitempty"` // UX hint / description
	DefaultValue  any                  `json:"defaultValue,omitempty"`
	Options       []AppConfigOption    `json:"options,omitempty"`
}

// AppConfigOption is select option wiring.
type AppConfigOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// AppConfig holds JSON shape for PUT /platform/settings/:platform payloads.
type AppConfigValues struct {
	Values map[string]string `json:"values"`
}

// SnapshotForAPI copies known field names from persisted settings map (masked at HTTP layer).
func (s PlatformAppConfigSchema) SnapshotForAPI(payload map[string]string) map[string]string {
	if payload == nil {
		payload = map[string]string{}
	}
	out := make(map[string]string)
	for _, f := range s.Fields {
		out[f.Name] = strings.TrimSpace(payload[f.Name])
	}
	return out
}

// FieldByName indexes schema.Fields.
func (s PlatformAppConfigSchema) FieldByName() map[string]AppConfigField {
	m := make(map[string]AppConfigField)
	for _, f := range s.Fields {
		m[f.Name] = f
	}
	return m
}
func (s PlatformAppConfigSchema) MarshalStableJSON() string {
	b, err := json.Marshal(s)
	if err != nil {
		return "{}"
	}
	return string(b)
}
