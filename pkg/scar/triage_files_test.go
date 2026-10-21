package scar

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTriageErrorWithFilesUsesSuppliedFiles(t *testing.T) {
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

	s, _, err := store.CreateDraft("Worker Channel Deadlock", "Buffer worker task channels", []string{"pkg/worker.go"})
	if err != nil {
		t.Fatalf("failed creating draft: %v", err)
	}
	s.Lifecycle.State = LifecycleActive
	s.Review.Status = ReviewApproved
	s.Verifier = failToPassVerifier("go", "test", "./...")
	if err := store.RecordBaseline(s, s.Verifier.CommandHash); err != nil {
		t.Fatalf("failed recording baseline: %v", err)
	}
	if _, err := store.SaveScar(s); err != nil {
		t.Fatalf("failed saving scar: %v", err)
	}

	// A trace that names no file cannot be matched on the file anchor alone.
	trace := "worker pool stalled while dispatching tasks"

	res, err := store.TriageError(trace)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Matches) != 0 {
		t.Fatalf("expected no match without a file signal, got %d", len(res.Matches))
	}

	// Supplying the affected file allows the anchor to match.
	res, err = store.TriageErrorWithFiles(trace, []string{"pkg/worker.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Matches) == 0 {
		t.Fatal("expected a match when the affected file is supplied")
	}
	if res.Matches[0].Scar.ID != s.ID {
		t.Errorf("expected matched scar %s, got %s", s.ID, res.Matches[0].Scar.ID)
	}
}
