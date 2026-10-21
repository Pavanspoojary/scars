package scar

import (
	"encoding/json"
	"testing"
)

func TestSARIFRuleOrderingIsDeterministic(t *testing.T) {
	res := &EnforceResult{
		Blocked: true,
		BlockingScars: []ScoredScar{
			{Scar: &Scar{ID: "zeta-scar", Title: "Zeta", Lesson: "Lesson Z"}, Score: 9},
			{Scar: &Scar{ID: "alpha-scar", Title: "Alpha", Lesson: "Lesson A"}, Score: 8},
			{Scar: &Scar{ID: "mid-scar", Title: "Mid", Lesson: "Lesson M"}, Score: 7},
		},
	}

	first, err := FormatEnforceSARIF(res)
	if err != nil {
		t.Fatalf("unexpected SARIF error: %v", err)
	}
	for i := 0; i < 25; i++ {
		next, err := FormatEnforceSARIF(res)
		if err != nil {
			t.Fatalf("unexpected SARIF error: %v", err)
		}
		if string(next) != string(first) {
			t.Fatalf("SARIF output is not deterministic across runs")
		}
	}

	var report SARIFReport
	if err := json.Unmarshal(first, &report); err != nil {
		t.Fatalf("failed decoding SARIF output: %v", err)
	}
	if len(report.Runs) == 0 || len(report.Runs[0].Tool.Driver.Rules) != 3 {
		t.Fatalf("expected 3 rules, got %+v", report.Runs)
	}
	rules := report.Runs[0].Tool.Driver.Rules
	if rules[0].ID != "git-scars/alpha-scar" {
		t.Errorf("expected rules sorted by identifier, got %q first", rules[0].ID)
	}
	if rules[2].ID != "git-scars/zeta-scar" {
		t.Errorf("expected rules sorted by identifier, got %q last", rules[2].ID)
	}
}
