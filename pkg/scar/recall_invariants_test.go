package scar

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecallExcludesScarsViolatingInvariants(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	if err := store.Init(); err != nil {
		t.Fatalf("failed initializing store: %v", err)
	}

	anchor := filepath.Join(dir, "pkg", "worker.go")
	if err := os.MkdirAll(filepath.Dir(anchor), 0755); err != nil {
		t.Fatalf("failed creating anchor dir: %v", err)
	}
	if err := os.WriteFile(anchor, []byte("package worker\n"), 0644); err != nil {
		t.Fatalf("failed writing anchor: %v", err)
	}

	// An active scar whose verifier evidence was fabricated: the recorded
	// command hash does not match its own argv.
	fabricated, _, err := store.CreateDraft("Fabricated Verification", "Unverified guardrail", []string{"pkg/worker.go"})
	if err != nil {
		t.Fatalf("failed creating draft: %v", err)
	}
	fabricated.Lifecycle.State = LifecycleActive
	fabricated.Review.Status = ReviewApproved
	fabricated.Verifier = Verifier{
		Argv:        []string{"go", "test", "./..."},
		Result:      VerificationFailToPass,
		CommandHash: ComputeCommandHash([]string{"go", "test", "-race", "./..."}, "", nil),
	}
	data, err := FormatFrontmatter(fabricated)
	if err != nil {
		t.Fatalf("failed formatting scar: %v", err)
	}
	// Written directly because SaveScar rejects the invalid record.
	if err := os.WriteFile(filepath.Join(store.ScarsDir, fabricated.ID+".md"), data, 0644); err != nil {
		t.Fatalf("failed writing scar: %v", err)
	}

	candidates, err := store.Recall(QueryOptions{Files: []string{"pkg/worker.go"}, Threshold: 3, TopK: 3})
	if err != nil {
		t.Fatalf("recall failed: %v", err)
	}
	for _, c := range candidates {
		if c.Scar.ID == fabricated.ID {
			t.Fatalf("expected the invariant-violating scar to be excluded from recall")
		}
	}
}
