package scar

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepoStats(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitscars-stats-test-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewStore(tmpDir)
	_ = store.Init()

	// Initially empty stats
	stats, err := store.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if stats.TotalScars != 0 {
		t.Errorf("expected 0 scars, got %d", stats.TotalScars)
	}

	// Create test scar
	testFile := filepath.Join(tmpDir, "pkg/service.go")
	_ = os.MkdirAll(filepath.Dir(testFile), 0755)
	_ = os.WriteFile(testFile, []byte("package service\n"), 0644)

	scar, _, _ := store.CreateDraft("Service Timeout", "Use timeout on context", []string{"pkg/service.go"})
	scar.Anchors.Symbols = []string{"HandleService"}
	scar.Anchors.ErrorSignatures = []string{"context deadline exceeded"}
	scar.Lifecycle.State = LifecycleActive
	scar.Review.Status = ReviewApproved
	scar.Verifier.Result = VerificationFailToPass
	_ = store.RecordBaseline(scar, "cmd-hash")
	_, _ = store.SaveScar(scar)

	// Record feedback
	_ = store.RecordFeedback(scar.ID, "helpful", "recall", "Saved production bug")
	_ = store.RecordFeedback(scar.ID, "helpful", "recall", "Great catch")

	stats, err = store.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats.TotalScars != 1 {
		t.Errorf("expected 1 scar, got %d", stats.TotalScars)
	}
	if stats.LifecycleCounts[LifecycleActive] != 1 {
		t.Errorf("expected 1 active scar, got %d", stats.LifecycleCounts[LifecycleActive])
	}
	if stats.TotalFiles != 1 {
		t.Errorf("expected 1 file, got %d", stats.TotalFiles)
	}
	if stats.TotalSymbols != 1 {
		t.Errorf("expected 1 symbol, got %d", stats.TotalSymbols)
	}
	if stats.TotalErrors != 1 {
		t.Errorf("expected 1 error signature, got %d", stats.TotalErrors)
	}
	if stats.FeedbackTotal != 2 {
		t.Errorf("expected 2 feedback entries, got %d", stats.FeedbackTotal)
	}
	if stats.HelpfulRatio != 100.0 {
		t.Errorf("expected 100%% helpful ratio, got %f", stats.HelpfulRatio)
	}
}
