package scar

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFormatEnforceSARIF(t *testing.T) {
	s := &Scar{
		ID:       "deadlock-test",
		Title:    "Worker Deadlock",
		Lesson:   "Buffer worker channels",
		Severity: SeverityHigh,
		FixHint:  "make(chan Task, 10)",
		Anchors: Anchors{
			Files: []string{"pkg/worker.go"},
		},
		Verifier: Verifier{
			Argv: []string{"go", "test", "-race", "./..."},
		},
	}

	res := &EnforceResult{
		Blocked: true,
		BlockingScars: []ScoredScar{
			{Scar: s, Score: 8},
		},
	}

	data, err := FormatEnforceSARIF(res)
	if err != nil {
		t.Fatalf("FormatEnforceSARIF error: %v", err)
	}

	var report SARIFReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("failed unmarshaling SARIF report: %v", err)
	}

	if report.Version != "2.1.0" {
		t.Errorf("expected version 2.1.0, got %s", report.Version)
	}
	if len(report.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(report.Runs))
	}
	run := report.Runs[0]
	if len(run.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(run.Results))
	}
	if run.Results[0].Level != "error" {
		t.Errorf("expected error level for high severity, got %s", run.Results[0].Level)
	}
	if run.Results[0].Locations[0].PhysicalLocation.ArtifactLocation.URI != "pkg/worker.go" {
		t.Errorf("expected location pkg/worker.go, got %s", run.Results[0].Locations[0].PhysicalLocation.ArtifactLocation.URI)
	}
}

func TestFormatCheckSARIF(t *testing.T) {
	s := &Scar{
		ID:     "stale-test",
		Title:  "Stale Anchors",
		Lesson: "Reaffirm after drift",
		Lifecycle: Lifecycle{
			State: LifecycleActive,
		},
		Anchors: Anchors{
			Files: []string{"pkg/auth.go"},
		},
	}

	scars := []ScarSummary{
		{Scar: s, Freshness: FreshnessChanged},
	}
	invariants := []string{"test-scar: invalid review"}

	data, err := FormatCheckSARIF(scars, invariants)
	if err != nil {
		t.Fatalf("FormatCheckSARIF error: %v", err)
	}

	var report SARIFReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("failed unmarshaling SARIF report: %v", err)
	}

	if len(report.Runs[0].Results) != 2 {
		t.Fatalf("expected 2 results (1 drift + 1 invariant), got %d", len(report.Runs[0].Results))
	}
}

func TestGitHubWorkflowYAML(t *testing.T) {
	yaml := GitHubWorkflowYAML()
	if !strings.Contains(yaml, "upload-sarif") {
		t.Errorf("expected upload-sarif in workflow")
	}
	if !strings.Contains(yaml, "git-scars check --format sarif") {
		t.Errorf("expected git-scars check --format sarif in workflow")
	}
}
