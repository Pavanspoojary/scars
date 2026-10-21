package scar

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecordBaselineReplacesPreviousAnchorSet(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	if err := store.Init(); err != nil {
		t.Fatalf("failed initializing store: %v", err)
	}

	oldFile := filepath.Join(dir, "pkg", "old.go")
	newFile := filepath.Join(dir, "pkg", "new.go")
	for _, f := range []string{oldFile, newFile} {
		if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
			t.Fatalf("failed creating dir: %v", err)
		}
		if err := os.WriteFile(f, []byte("package pkg\n"), 0644); err != nil {
			t.Fatalf("failed writing %s: %v", f, err)
		}
	}

	scar := &Scar{
		SchemaVersion: CurrentSchemaVersion,
		ID:            "reanchor",
		Title:         "Re-anchored",
		Lesson:        "Lesson",
		Anchors:       Anchors{Files: []string{"pkg/old.go"}},
		Verifier:      Verifier{Result: VerificationNotAttempted},
		Review:        Review{Status: ReviewUnreviewed},
		Lifecycle:     Lifecycle{State: LifecycleDraft},
	}

	if err := store.RecordBaseline(scar, "hash-1"); err != nil {
		t.Fatalf("failed recording baseline: %v", err)
	}
	if _, ok := scar.Baseline.AnchorHashes["pkg/old.go"]; !ok {
		t.Fatal("expected the original anchor to be recorded")
	}

	// Re-anchor to a different file.
	scar.Anchors.Files = []string{"pkg/new.go"}
	if err := store.RecordBaseline(scar, "hash-2"); err != nil {
		t.Fatalf("failed re-recording baseline: %v", err)
	}
	if _, ok := scar.Baseline.AnchorHashes["pkg/old.go"]; ok {
		t.Error("expected the previous anchor to be dropped from the baseline")
	}
	if _, ok := scar.Baseline.AnchorHashes["pkg/new.go"]; !ok {
		t.Error("expected the new anchor to be recorded")
	}
}
