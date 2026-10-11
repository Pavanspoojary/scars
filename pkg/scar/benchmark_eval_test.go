package scar

import (
	"os"
	"strings"
	"testing"
)

func TestRunBenchmarkEvaluation(t *testing.T) {
	report := RunBenchmarkEvaluation()

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

	// Verify significant token savings
	if report.TokenSavingsVsScop <= 20.0 {
		t.Errorf("expected >20%% token savings vs scoped rules, got %.1f%%", report.TokenSavingsVsScop)
	}
	if report.TokenSavingsVsMono <= 70.0 {
		t.Errorf("expected >70%% token savings vs monolithic rules, got %.1f%%", report.TokenSavingsVsMono)
	}

	// Verify drift sensitivity and rejection rates
	if report.GitScarsArm.DriftDetectionRate != 1.0 {
		t.Errorf("expected 100%% drift detection rate for GitScars, got %.2f", report.GitScarsArm.DriftDetectionRate)
	}
	if report.ScopedRulesArm.DriftDetectionRate != 0.0 {
		t.Errorf("expected 0%% drift detection rate for Scoped rules, got %.2f", report.ScopedRulesArm.DriftDetectionRate)
	}
	if report.GitScarsArm.ToxicClaimRejectionRate != 1.0 {
		t.Errorf("expected 100%% toxic rejection rate for GitScars, got %.2f", report.GitScarsArm.ToxicClaimRejectionRate)
	}

	// Verify Markdown output generation
	md := report.GenerateBenchmarkMarkdown()
	if !strings.Contains(md, "Empirical Evaluation Results") {
		t.Errorf("expected header in generated Markdown")
	}
	if !strings.Contains(md, "REGR-001") || !strings.Contains(md, "REGR-010") {
		t.Errorf("expected scenario IDs in generated Markdown")
	}
	if !strings.Contains(md, "The Refactored Anchor") {
		t.Errorf("expected Case 1 in generated Markdown")
	}
}

func TestWriteBenchmarkResults(t *testing.T) {
	report := RunBenchmarkEvaluation()
	md := report.GenerateBenchmarkMarkdown()
	if err := os.WriteFile("../../EVAL_RESULTS.md", []byte(md), 0644); err != nil {
		t.Fatalf("failed to write EVAL_RESULTS.md: %v", err)
	}
}
