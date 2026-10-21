package scar

import (
	"os"
	"path/filepath"
	"testing"
)

func writeAnchorFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatalf("failed creating dir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatalf("failed writing %s: %v", rel, err)
	}
}

func TestLintDetectsMissingAnchorSymbol(t *testing.T) {
	dir := t.TempDir()
	writeAnchorFile(t, dir, "pkg/worker.go", "package pkg\n\nfunc Worker() {}\n")

	s := &Scar{
		SchemaVersion: CurrentSchemaVersion,
		ID:            "missing-symbol",
		Title:         "Missing symbol",
		Lesson:        "Lesson",
		Anchors:       Anchors{Files: []string{"pkg/worker.go"}, Symbols: []string{"RemovedFunction"}},
		Verifier:      Verifier{Result: VerificationNotAttempted},
		Review:        Review{Status: ReviewUnreviewed},
		Lifecycle:     Lifecycle{State: LifecycleDraft},
	}

	findings := LintScar(dir, s)
	if !hasRule(findings, "anchor_symbol_missing") {
		t.Fatalf("expected anchor_symbol_missing finding, got %+v", findings)
	}
}

func TestLintAcceptsResolvedAnchorSymbol(t *testing.T) {
	dir := t.TempDir()
	writeAnchorFile(t, dir, "pkg/worker.go", "package pkg\n\nfunc Worker() {}\n")

	s := &Scar{
		SchemaVersion: CurrentSchemaVersion,
		ID:            "resolved-symbol",
		Title:         "Resolved symbol",
		Lesson:        "Lesson",
		Anchors:       Anchors{Files: []string{"pkg/worker.go"}, Symbols: []string{"Worker"}},
		Verifier:      Verifier{Result: VerificationNotAttempted},
		Review:        Review{Status: ReviewUnreviewed},
		Lifecycle:     Lifecycle{State: LifecycleDraft},
	}

	findings := LintScar(dir, s)
	if hasRule(findings, "anchor_symbol_missing") {
		t.Fatalf("expected no anchor_symbol_missing finding, got %+v", findings)
	}
}

func TestLintDetectsBadPatternStillPresent(t *testing.T) {
	dir := t.TempDir()
	writeAnchorFile(t, dir, "pkg/worker.go", "package pkg\n\nfunc Worker() {\n\tch := make(chan int)\n}\n")

	s := &Scar{
		SchemaVersion: CurrentSchemaVersion,
		ID:            "bad-pattern",
		Title:         "Bad pattern present",
		Lesson:        "Lesson",
		Anchors:       Anchors{Files: []string{"pkg/worker.go"}},
		BadPattern:    "make(chan int)",
		Verifier:      Verifier{Result: VerificationNotAttempted},
		Review:        Review{Status: ReviewUnreviewed},
		Lifecycle:     Lifecycle{State: LifecycleDraft},
	}

	findings := LintScar(dir, s)
	if !hasRule(findings, "bad_pattern_present") {
		t.Fatalf("expected bad_pattern_present finding, got %+v", findings)
	}
}

func TestLintDetectsAbsentGoodPattern(t *testing.T) {
	dir := t.TempDir()
	writeAnchorFile(t, dir, "pkg/worker.go", "package pkg\n\nfunc Worker() {}\n")

	s := &Scar{
		SchemaVersion: CurrentSchemaVersion,
		ID:            "good-pattern",
		Title:         "Good pattern absent",
		Lesson:        "Lesson",
		Anchors:       Anchors{Files: []string{"pkg/worker.go"}},
		GoodPattern:   "make(chan int, 8)",
		Verifier:      Verifier{Result: VerificationNotAttempted},
		Review:        Review{Status: ReviewUnreviewed},
		Lifecycle:     Lifecycle{State: LifecycleDraft},
	}

	findings := LintScar(dir, s)
	if !hasRule(findings, "good_pattern_absent") {
		t.Fatalf("expected good_pattern_absent finding, got %+v", findings)
	}
}

func TestLintDetectsMissingSeverityOnActiveScar(t *testing.T) {
	dir := t.TempDir()
	writeAnchorFile(t, dir, "pkg/worker.go", "package pkg\n\nfunc Worker() {}\n")

	s := &Scar{
		SchemaVersion: CurrentSchemaVersion,
		ID:            "no-severity",
		Title:         "No severity",
		Lesson:        "Lesson",
		Anchors:       Anchors{Files: []string{"pkg/worker.go"}},
		Verifier:      failToPassVerifier("go", "test", "./..."),
		Review:        Review{Status: ReviewApproved},
		Lifecycle:     Lifecycle{State: LifecycleActive},
	}

	findings := LintScar(dir, s)
	if !hasRule(findings, "missing_severity") {
		t.Fatalf("expected missing_severity finding, got %+v", findings)
	}
}

func hasRule(findings []LintFinding, rule string) bool {
	for _, f := range findings {
		if f.Rule == rule {
			return true
		}
	}
	return false
}
