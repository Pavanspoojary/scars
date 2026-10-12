package scar

import (
	"os"
	"path/filepath"
	"testing"
)

// TestScenarioMechanicsAnchorDrift verifies that AST symbol drift is detected deterministically,
// whereas non-symbol edits (whitespace, comments) do not trigger staleness.
func TestScenarioMechanicsAnchorDrift(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Create a Python source file
	pyFile := filepath.Join(tmpDir, "calc.py")
	origCode := `def calculate_proration(amount, days):
    # original calculation
    return amount * days / 30
`
	if err := os.WriteFile(pyFile, []byte(origCode), 0644); err != nil {
		t.Fatalf("failed to write py file: %v", err)
	}

	symHash, err := ExtractSymbolHash(pyFile, "calculate_proration")
	if err != nil || symHash == "" {
		t.Fatalf("expected to extract calculate_proration symbol hash: symHash=%q, err=%v", symHash, err)
	}

	origFileHash, err := hashFile(pyFile)
	if err != nil {
		t.Fatalf("failed to hash file: %v", err)
	}

	scar := &Scar{
		ID:     "scar-drift-test",
		Title:  "Test drift detection",
		Lesson: "Detect symbol change",
		Anchors: Anchors{
			Files:   []string{"calc.py"},
			Symbols: []string{"calculate_proration"},
		},
		Baseline: Baseline{
			RecordedAt: "2026-10-12T00:00:00Z",
			AnchorHashes: map[string]string{
				"calc.py": origFileHash,
			},
			SymbolHashes: map[string]string{
				"calculate_proration": symHash,
			},
		},
	}

	store := NewStore(tmpDir)

	// 2. Modify comment in file -> file SHA changes, but symbol AST hash remains unchanged
	commentEditedCode := `def calculate_proration(amount, days):
    # updated comment only - logic unchanged
    return amount * days / 30
`
	if err := os.WriteFile(pyFile, []byte(commentEditedCode), 0644); err != nil {
		t.Fatalf("failed to edit py file comment: %v", err)
	}

	freshness := store.ComputeFreshness(scar)
	if freshness != FreshnessCurrent {
		t.Errorf("expected FreshnessCurrent when only comment changed, got %q", freshness)
	}

	// 3. Modify symbol signature -> AST hash changes, triggering FreshnessChanged
	symbolEditedCode := `def calculate_proration(amount, days, timezone_aware=True):
    return amount * days / 30
`
	if err := os.WriteFile(pyFile, []byte(symbolEditedCode), 0644); err != nil {
		t.Fatalf("failed to edit py symbol: %v", err)
	}

	freshness = store.ComputeFreshness(scar)
	if freshness != FreshnessChanged {
		t.Errorf("expected FreshnessChanged when symbol signature changed, got %q", freshness)
	}
}

// TestScenarioMechanicsGlobFragility verifies that path-scoped glob rules silently stop matching
// when files move, while anchor tracking detects the missing/modified file.
func TestScenarioMechanicsGlobFragility(t *testing.T) {
	origPath := "services/billing/calc.py"
	newPath := "core/pricing/calc.py"
	globPattern := "services/**"

	// 1. Glob rule matches original path
	if !matchesGlobRule(globPattern, origPath) {
		t.Fatalf("expected glob %s to match %s", globPattern, origPath)
	}

	// 2. Glob rule silently fails on new refactored path
	if matchesGlobRule(globPattern, newPath) {
		t.Errorf("expected glob %s to FAIL matching %s after refactor", globPattern, newPath)
	}

	// 3. Anchor tracking reports missing file
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)
	scar := &Scar{
		ID: "scar-move-test",
		Anchors: Anchors{
			Files: []string{origPath},
		},
		Baseline: Baseline{
			AnchorHashes: map[string]string{
				origPath: "dummy-hash",
			},
		},
	}
	freshness := store.ComputeFreshness(scar)
	if freshness != FreshnessUnavailable {
		t.Errorf("expected FreshnessUnavailable for missing refactored file, got %q", freshness)
	}
}

// TestScenarioMechanicsVerifierGate verifies that invariants block promotion to active
// if fail-to-pass verifier evidence is missing.
func TestScenarioMechanicsVerifierGate(t *testing.T) {
	scar := &Scar{
		ID:     "scar-gate-test",
		Title:  "Test Verifier Gate",
		Lesson: "Unverified claims must not be active",
		Lifecycle: Lifecycle{
			State: LifecycleActive,
		},
		Review: Review{
			Status:   ReviewApproved,
			Reviewer: "alice",
			Date:     "2026-10-12T00:00:00Z",
		},
		Verifier: Verifier{
			Result: "", // Empty result!
			Argv:   []string{},
		},
		Anchors: Anchors{
			Files: []string{"pkg/test.go"},
		},
	}

	err := scar.ValidateInvariants()
	if err == nil {
		t.Errorf("expected invariant violation for active scar without verifier, got nil")
	}
}
