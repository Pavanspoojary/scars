package scar

import (
	"os"
	"strings"
	"testing"
)

func TestRunScenarioAnalysis(t *testing.T) {
	report := RunScenarioAnalysis()

	if report.ScenariosEvaluated != 10 {
		t.Fatalf("expected 10 evaluated scenarios, got %d", report.ScenariosEvaluated)
	}

	// Verify token volume hierarchy: GitScars < Scoped < Monolithic
	if report.GitScarsArm.AverageTokensPerTask >= report.ScopedRulesArm.AverageTokensPerTask {
		t.Errorf("expected GitScars tokens (%.0f) < Scoped tokens (%.0f)",
			report.GitScarsArm.AverageTokensPerTask, report.ScopedRulesArm.AverageTokensPerTask)
	}
	if report.ScopedRulesArm.AverageTokensPerTask >= report.MonolithicArm.AverageTokensPerTask {
		t.Errorf("expected Scoped tokens (%.0f) < Monolithic tokens (%.0f)",
			report.ScopedRulesArm.AverageTokensPerTask, report.MonolithicArm.AverageTokensPerTask)
	}

	// Verify modeled token savings
	if report.TokenSavingsVsScop <= 20.0 {
		t.Errorf("expected >20%% token savings vs scoped rules, got %.1f%%", report.TokenSavingsVsScop)
	}
	if report.TokenSavingsVsMono <= 70.0 {
		t.Errorf("expected >70%% token savings vs monolithic rules, got %.1f%%", report.TokenSavingsVsMono)
	}

	// Verify Markdown output generation
	md := report.GenerateScenarioAnalysisMarkdown()
	if !strings.Contains(md, "Design Estimates & Illustrative Scenario Analysis") {
		t.Errorf("expected honest scenario analysis header in generated Markdown")
	}
	if !strings.Contains(md, "Methodology & Scope Disclaimer") {
		t.Errorf("expected methodology disclosure in generated Markdown")
	}
	if !strings.Contains(md, "REGR-001") || !strings.Contains(md, "REGR-010") {
		t.Errorf("expected scenario IDs in generated Markdown")
	}
	if !strings.Contains(md, "The Refactored Anchor") {
		t.Errorf("expected Case 1 in generated Markdown")
	}
}

func TestWriteScenarioAnalysisResults(t *testing.T) {
	report := RunScenarioAnalysis()
	md := report.GenerateScenarioAnalysisMarkdown()
	if err := os.WriteFile("../../SCENARIO_ANALYSIS.md", []byte(md), 0644); err != nil {
		t.Fatalf("failed to write SCENARIO_ANALYSIS.md: %v", err)
	}
}
