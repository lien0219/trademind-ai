package productpublish

import (
	"testing"

	"github.com/google/uuid"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

func TestMatchLocalSKUSkipsExistingBinding(t *testing.T) {
	local := localSKUForBinding{
		PublicationSKUID: uuid.New(),
		ExternalSKUID:    "already-bound",
		SpecName:         "红色 / L",
		PriceYuan:        99,
	}
	match := matchLocalSKU(local, []platformdouyin.PlatformProductSKU{{
		PlatformSKUID: "999",
		SpecName:      "红色 / L",
		PriceYuan:     99,
	}}, map[string]struct{}{})
	if match.Status != BindStatusSkipped {
		t.Fatalf("expected skipped, got %s", match.Status)
	}
}

func TestMatchLocalSKUExactAttrs(t *testing.T) {
	local := localSKUForBinding{
		PublicationSKUID: uuid.New(),
		Attrs:            map[string]string{"颜色": "红色", "尺码": "L"},
		PriceYuan:        88,
	}
	platform := []platformdouyin.PlatformProductSKU{{
		PlatformSKUID: "111",
		Attrs:         map[string]string{"颜色": "红色", "尺码": "L"},
		PriceYuan:     120,
	}}
	match := matchLocalSKU(local, platform, map[string]struct{}{})
	if match.Status != BindStatusBound || match.Confidence != 95 {
		t.Fatalf("expected bound by attrs, got %+v", match)
	}
	if match.Platform == nil || match.Platform.PlatformSKUID != "111" {
		t.Fatalf("unexpected platform match: %+v", match.Platform)
	}
}

func TestMatchLocalSKUNameAndPrice(t *testing.T) {
	local := localSKUForBinding{
		PublicationSKUID: uuid.New(),
		SpecName:         "红色 / l",
		PriceYuan:        99.0,
	}
	platform := []platformdouyin.PlatformProductSKU{{
		PlatformSKUID: "222",
		SpecName:      "红色 / L",
		PriceYuan:     99.0,
	}}
	match := matchLocalSKU(local, platform, map[string]struct{}{})
	if match.Status != BindStatusBound || match.Confidence != 85 {
		t.Fatalf("expected bound by name+price, got %+v", match)
	}
}

func TestMatchLocalSKUAmbiguousMultipleCandidates(t *testing.T) {
	local := localSKUForBinding{
		PublicationSKUID: uuid.New(),
		SpecName:         "红色 / L",
		PriceYuan:        99,
	}
	platform := []platformdouyin.PlatformProductSKU{
		{PlatformSKUID: "a", SpecName: "红色 / L", PriceYuan: 99},
		{PlatformSKUID: "b", SpecName: "红色 / L", PriceYuan: 99},
	}
	match := matchLocalSKU(local, platform, map[string]struct{}{})
	if match.Status != BindStatusAmbiguous {
		t.Fatalf("expected ambiguous, got %s", match.Status)
	}
}

func TestMatchLocalSKUUnmatched(t *testing.T) {
	local := localSKUForBinding{
		PublicationSKUID: uuid.New(),
		SpecName:         "蓝色 / XL",
		PriceYuan:        50,
	}
	platform := []platformdouyin.PlatformProductSKU{{
		PlatformSKUID: "x",
		SpecName:      "红色 / L",
		PriceYuan:     99,
	}}
	match := matchLocalSKU(local, platform, map[string]struct{}{})
	if match.Status != BindStatusUnmatched {
		t.Fatalf("expected unmatched, got %s", match.Status)
	}
}
