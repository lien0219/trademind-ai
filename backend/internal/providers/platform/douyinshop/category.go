package douyinshop

import (
	"context"
	"encoding/json"
	"strings"
)

const (
	MethodGetShopCategory     = "shop.getShopCategory"
	MethodGetCatePropertyV2   = "product.getCatePropertyV2"
	DefaultRootCategoryID     = "0"
	DefaultCategoryAPIVersion = "v2"
)

type CategoryRequest struct {
	ParentID string
	Channel  string
}

type Category struct {
	CategoryID string         `json:"categoryId"`
	ParentID   string         `json:"parentId"`
	Name       string         `json:"name"`
	Level      int            `json:"level"`
	IsLeaf     bool           `json:"isLeaf"`
	Status     string         `json:"status,omitempty"`
	Raw        map[string]any `json:"raw,omitempty"`
}

type CategoryAttribute struct {
	AttrID      string         `json:"attrId"`
	Name        string         `json:"name"`
	Required    bool           `json:"required"`
	ValueType   string         `json:"valueType,omitempty"`
	Options     []AttrOption   `json:"options,omitempty"`
	UnitOptions []AttrOption   `json:"unitOptions,omitempty"`
	Raw         map[string]any `json:"raw,omitempty"`
}

type AttrOption struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
}

func (c *Client) GetCategories(ctx context.Context, req CategoryRequest) ([]Category, error) {
	parentID := strings.TrimSpace(req.ParentID)
	if parentID == "" {
		parentID = DefaultRootCategoryID
	}
	params := map[string]any{"cid": parentID}
	if ch := strings.TrimSpace(req.Channel); ch != "" {
		params["channel"] = ch
	}
	var raw any
	if err := c.Do(ctx, MethodGetShopCategory, params, &raw); err != nil {
		return nil, err
	}
	return parseCategories(raw, parentID), nil
}

func (c *Client) GetCategoryAttributes(ctx context.Context, categoryID string) ([]CategoryAttribute, error) {
	cid := strings.TrimSpace(categoryID)
	if cid == "" {
		return nil, NewError(CodeDouyinAPIError, "douyin category id is required", "", "", "")
	}
	var raw any
	if err := c.Do(ctx, MethodGetCatePropertyV2, map[string]any{"category_leaf_id": cid}, &raw); err != nil {
		return nil, err
	}
	return parseCategoryAttributes(raw), nil
}

func parseCategories(raw any, fallbackParent string) []Category {
	var out []Category
	for _, row := range pickList(raw, "category_list", "categories", "list", "data") {
		m, ok := row.(map[string]any)
		if !ok {
			continue
		}
		cid := pickString(m, "id", "cid", "category_id", "categoryId", "cat_id", "catId")
		if cid == "" {
			continue
		}
		parentID := pickString(m, "parent_id", "parentId", "father_id", "fatherId", "pid")
		if parentID == "" {
			parentID = fallbackParent
		}
		out = append(out, Category{
			CategoryID: cid,
			ParentID:   parentID,
			Name:       pickString(m, "name", "category_name", "categoryName", "cat_name", "catName"),
			Level:      int(int64FromAny(firstAny(m, "level", "category_level", "categoryLevel"))),
			IsLeaf:     boolFromAny(firstAny(m, "is_leaf", "isLeaf", "leaf", "is_leaf_category")),
			Status:     pickString(m, "status", "category_status", "categoryStatus"),
			Raw:        m,
		})
	}
	return out
}

func parseCategoryAttributes(raw any) []CategoryAttribute {
	var out []CategoryAttribute
	for _, row := range pickList(raw, "properties", "property_list", "propertyList", "category_property_list", "categoryPropertyList", "list", "data") {
		m, ok := row.(map[string]any)
		if !ok {
			continue
		}
		id := pickString(m, "property_id", "propertyId", "attr_id", "attrId", "id")
		if id == "" {
			continue
		}
		out = append(out, CategoryAttribute{
			AttrID:      id,
			Name:        pickString(m, "property_name", "propertyName", "attr_name", "attrName", "name"),
			Required:    requiredFromAny(firstAny(m, "required", "is_required", "isRequired", "required_type", "requiredType")),
			ValueType:   pickString(m, "type", "value_type", "valueType", "property_type", "propertyType", "control_type", "controlType"),
			Options:     parseAttrOptions(firstAny(m, "options", "option_list", "optionList", "values", "value_list", "valueList", "diy_type_list")),
			UnitOptions: parseAttrOptions(firstAny(m, "unit_options", "unitOptions", "unit_list", "unitList", "measure_unit_list", "measureUnitList")),
			Raw:         m,
		})
	}
	return out
}

func pickList(raw any, keys ...string) []any {
	switch v := raw.(type) {
	case []any:
		return v
	case map[string]any:
		for _, key := range keys {
			if xs, ok := v[key].([]any); ok {
				return xs
			}
			if m, ok := v[key].(map[string]any); ok {
				if xs := pickList(m, keys...); len(xs) > 0 {
					return xs
				}
			}
		}
	}
	return nil
}

func firstAny(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v
		}
	}
	return nil
}

func boolFromAny(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		s := strings.TrimSpace(strings.ToLower(x))
		return s == "1" || s == "true" || s == "yes" || s == "y"
	case float64:
		return x != 0
	case int:
		return x != 0
	case int64:
		return x != 0
	case json.Number:
		n, _ := x.Int64()
		return n != 0
	default:
		return false
	}
}

func requiredFromAny(v any) bool {
	if boolFromAny(v) {
		return true
	}
	return strings.TrimSpace(stringFromAny(v)) == "1"
}

func parseAttrOptions(raw any) []AttrOption {
	xs, ok := raw.([]any)
	if !ok || len(xs) == 0 {
		return nil
	}
	out := make([]AttrOption, 0, len(xs))
	for _, item := range xs {
		switch v := item.(type) {
		case string:
			if s := strings.TrimSpace(v); s != "" {
				out = append(out, AttrOption{Name: s})
			}
		case map[string]any:
			name := pickString(v, "name", "value", "value_name", "valueName", "option_name", "optionName")
			if name == "" {
				continue
			}
			out = append(out, AttrOption{
				ID:   pickString(v, "id", "value_id", "valueId", "option_id", "optionId"),
				Name: name,
			})
		}
	}
	return out
}
