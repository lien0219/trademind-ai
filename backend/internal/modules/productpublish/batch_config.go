package productpublish

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

// PublishConfigOverrides holds per-scope overrides for batch publish.
type PublishConfigOverrides struct {
	Products       map[string]map[string]any `json:"products"`
	Platforms      map[string]map[string]any `json:"platforms"`
	Shops          map[string]map[string]any `json:"shops"`
	ProductTargets map[string]map[string]any `json:"productTargets"`
}

// EffectivePublishConfig is the resolved config for one product × target cell.
type EffectivePublishConfig struct {
	Config        map[string]any    `json:"effectiveConfig"`
	ConfigSources map[string]string `json:"configSources"`
}

func productTargetOverrideKey(productID, platform, shopID string) string {
	plat := strings.TrimSpace(strings.ToLower(platform))
	sid := strings.TrimSpace(shopID)
	if sid == "" {
		return strings.TrimSpace(productID) + ":" + plat
	}
	return strings.TrimSpace(productID) + ":" + plat + ":" + sid
}

// mergeEffectiveConfig resolves config with priority:
// commonConfig → product → platform → shop → productTarget.
func mergeEffectiveConfig(
	common map[string]any,
	overrides PublishConfigOverrides,
	productID, platform, shopID string,
) EffectivePublishConfig {
	out := map[string]any{}
	sources := map[string]string{}
	plat := strings.TrimSpace(strings.ToLower(platform))
	sid := strings.TrimSpace(shopID)
	pid := strings.TrimSpace(productID)

	applyLayer := func(layer map[string]any, source string) {
		if len(layer) == 0 {
			return
		}
		for k, v := range layer {
			out[k] = v
			sources[k] = source
		}
	}

	if len(common) > 0 {
		applyLayer(common, "commonConfig")
	}
	if overrides.Products != nil {
		if layer, ok := overrides.Products[pid]; ok {
			applyLayer(layer, "productOverride")
		}
	}
	if overrides.Platforms != nil {
		if layer, ok := overrides.Platforms[plat]; ok {
			applyLayer(layer, "platformOverride")
		}
	}
	if overrides.Shops != nil && sid != "" {
		if layer, ok := overrides.Shops[sid]; ok {
			applyLayer(layer, "shopOverride")
		}
	}
	if overrides.ProductTargets != nil {
		key := productTargetOverrideKey(pid, plat, sid)
		if layer, ok := overrides.ProductTargets[key]; ok {
			applyLayer(layer, "productTargetOverride")
		}
	}

	return EffectivePublishConfig{Config: out, ConfigSources: sources}
}

func configHash(parts ...any) string {
	b, err := json.Marshal(parts)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:16])
}

func batchIdempotencyKey(adminID string, productIDs []string, targets []PublishTargetRef, common map[string]any, overrides PublishConfigOverrides) string {
	ids := append([]string(nil), productIDs...)
	sort.Strings(ids)
	tgts := make([]string, 0, len(targets))
	for _, t := range targets {
		sid := ""
		if t.ShopID != nil {
			sid = strings.TrimSpace(*t.ShopID)
		}
		tgts = append(tgts, strings.TrimSpace(strings.ToLower(t.Platform))+":"+sid)
	}
	sort.Strings(tgts)
	return "publish-batch:" + adminID + ":" + configHash(ids, tgts, common, overrides)
}

func taskIdempotencyKey(productID, platform, shopID string, eff EffectivePublishConfig) string {
	sid := strings.TrimSpace(shopID)
	return "publish-task:" + strings.TrimSpace(productID) + ":" + strings.TrimSpace(strings.ToLower(platform)) + ":" + sid + ":" + configHash(eff.Config)
}
