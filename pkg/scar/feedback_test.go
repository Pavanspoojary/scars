package scar

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFeedbackQualityAndRecallPenalty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitscars-feedback-test-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewStore(tmpDir)
	_ = store.Init()

	s, _, _ := store.CreateDraft("Test Scar", "Lesson", []string{"pkg/test.go"})
	s.Lifecycle.State = LifecycleActive
	s.Review.Status = ReviewApproved
	s.Verifier = failToPassVerifier("go", "test", "-race", "./...")
	_ = os.MkdirAll(filepath.Join(tmpDir, "pkg"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "pkg/test.go"), []byte("package test\n"), 0644)
	_ = store.RecordBaseline(s, "hash-test")
	_, _ = store.SaveScar(s)

	// Baseline recall without feedback (score = 5 for exact file match)
	matches1, err := store.Recall(QueryOptions{Files: []string{"pkg/test.go"}})
	if err != nil {
		t.Fatalf("recall failed: %v", err)
	}
	if len(matches1) != 1 || matches1[0].Score != 5 {
		t.Fatalf("expected 1 match with score 5, got %+v", matches1)
	}

	// Record 2 incorrect feedbacks
	if err := store.RecordFeedback(s.ID, "incorrect", "cli", "Wrong diagnosis"); err != nil {
		t.Fatalf("record feedback failed: %v", err)
	}
	if err := store.RecordFeedback(s.ID, "incorrect", "agent", "Misleading prompt"); err != nil {
		t.Fatalf("record feedback failed: %v", err)
	}

	// Verify quality stats
	quality, err := store.GetScarQuality(s.ID)
	if err != nil {
		t.Fatalf("get quality failed: %v", err)
	}
	if quality.Total != 2 || quality.Incorrect != 2 {
		t.Errorf("expected 2 incorrect feedbacks, got %+v", quality)
	}
	if quality.Score >= 0 {
		t.Errorf("expected negative score for incorrect feedbacks, got %.2f", quality.Score)
	}

	// Re-run recall: should apply -2 penalty (score becomes 3 instead of 5)
	matches2, err := store.Recall(QueryOptions{Files: []string{"pkg/test.go"}})
	if err != nil {
		t.Fatalf("recall with penalty failed: %v", err)
	}
	if len(matches2) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches2))
	}
	if matches2[0].Score != 3 {
		t.Errorf("expected score penalized to 3, got %d", matches2[0].Score)
	}
	foundPenalty := false
	for _, m := range matches2[0].Matches {
		if m == "feedback_penalty(-2)" {
			foundPenalty = true
			break
		}
	}
	if !foundPenalty {
		t.Errorf("expected feedback_penalty(-2) match signal in %+v", matches2[0].Matches)
	}
}
