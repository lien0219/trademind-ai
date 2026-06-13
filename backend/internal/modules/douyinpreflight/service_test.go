package douyinpreflight

import "testing"

func TestAggregateStatus(t *testing.T) {
	t.Parallel()
	checks := []CheckItem{
		{Status: statusPassed},
		{Status: statusWarning},
		{Status: statusFailed},
	}
	st, p, w, f := aggregateStatus(checks)
	if st != statusFailed || p != 1 || w != 1 || f != 1 {
		t.Fatalf("unexpected aggregate: %s %d %d %d", st, p, w, f)
	}
	checks2 := []CheckItem{{Status: statusPassed}, {Status: statusWarning}}
	st2, _, _, _ := aggregateStatus(checks2)
	if st2 != statusWarning {
		t.Fatalf("expected warning overall, got %s", st2)
	}
}

func TestCheckHelpers(t *testing.T) {
	t.Parallel()
	p := checkPassed("k", "t", "m", nil)
	if p.Status != statusPassed || p.Key != "k" {
		t.Fatalf("unexpected passed item")
	}
	w := checkWarning("k", "t", "m", "s", nil)
	if w.Suggestion != "s" {
		t.Fatal("expected suggestion")
	}
	f := checkFailed("k", "t", "m", "s", nil)
	if f.Status != statusFailed {
		t.Fatal("expected failed")
	}
}
