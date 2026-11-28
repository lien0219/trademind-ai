package collect

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestDefaultCollectProvidersFallbackMatchesSharedContract(t *testing.T) {
	data, err := os.ReadFile("../../../../tests/contracts/collector-providers.json")
	if err != nil {
		t.Fatalf("read shared collector provider contract: %v", err)
	}

	var expected []CollectProviderDTO
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatalf("decode shared collector provider contract: %v", err)
	}

	actual := defaultCollectProvidersFallback()
	if !reflect.DeepEqual(actual, expected) {
		for i := range actual {
			if i >= len(expected) || !reflect.DeepEqual(actual[i], expected[i]) {
				t.Logf("provider[%d] actual=%#v expected=%#v", i, actual[i], expected[i])
			}
		}
		actualJSON, _ := json.MarshalIndent(actual, "", "  ")
		expectedJSON, _ := json.MarshalIndent(expected, "", "  ")
		t.Fatalf("collector fallback differs from shared contract\nactual: %s\nexpected: %s", actualJSON, expectedJSON)
	}
}
