package douyinruntime

import (
	"testing"

	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

func TestAggregateOverallDisabled(t *testing.T) {
	out := &HealthDTO{
		Config:  HealthSection{Status: HealthDisabled},
		Auth:    HealthSection{Status: HealthHealthy},
		Storage: HealthSection{Status: HealthHealthy},
		Tasks:   HealthSection{Status: HealthHealthy},
		API:     HealthSection{Status: HealthHealthy},
	}
	st, label := aggregateOverall(out, platformdouyin.RuntimeState{Status: platformdouyin.RuntimeEmergencyDisabled}, platformdouyin.RuntimeConfig{})
	if st != HealthDisabled || label != healthLabel(HealthDisabled) {
		t.Fatalf("got %s %s", st, label)
	}
}

func TestAggregateOverallHealthy(t *testing.T) {
	out := &HealthDTO{
		Config:  HealthSection{Status: HealthHealthy},
		Auth:    HealthSection{Status: HealthHealthy},
		Storage: HealthSection{Status: HealthHealthy},
		Tasks:   HealthSection{Status: HealthHealthy},
		API:     HealthSection{Status: HealthHealthy},
	}
	st, _ := aggregateOverall(out, platformdouyin.RuntimeState{Status: platformdouyin.RuntimeNormal}, platformdouyin.RuntimeConfig{RealAPIEnabled: true})
	if st != HealthHealthy {
		t.Fatalf("expected healthy, got %s", st)
	}
}

func TestHealthLabels(t *testing.T) {
	if healthLabel(HealthDegraded) != "部分能力需要检查" {
		t.Fatal("unexpected degraded label")
	}
}
