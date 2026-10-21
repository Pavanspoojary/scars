package scar

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, dir, content string) {
	t.Helper()
	storeDir := filepath.Join(dir, DefaultStoreDir)
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		t.Fatalf("failed creating store dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(storeDir, ConfigFileName), []byte(content), 0644); err != nil {
		t.Fatalf("failed writing config: %v", err)
	}
}

func TestLoadConfigUsesDefaultsWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	cfg := store.LoadConfig()
	if cfg.AbstainThreshold != 3 {
		t.Errorf("expected default abstain threshold 3, got %d", cfg.AbstainThreshold)
	}
	if cfg.TopK != 3 {
		t.Errorf("expected default top_k 3, got %d", cfg.TopK)
	}
}

func TestLoadConfigReadsConfiguredValues(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "# comment\nschema_version: \"1\"\nabstain_threshold: 5\ntop_k: 7\n")

	store := NewStore(dir)
	cfg := store.LoadConfig()
	if cfg.AbstainThreshold != 5 {
		t.Errorf("expected abstain threshold 5, got %d", cfg.AbstainThreshold)
	}
	if cfg.TopK != 7 {
		t.Errorf("expected top_k 7, got %d", cfg.TopK)
	}
}

func TestLoadConfigIgnoresUnusableValues(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "abstain_threshold: not-a-number\ntop_k: 0\n")

	store := NewStore(dir)
	cfg := store.LoadConfig()
	if cfg.AbstainThreshold != 3 {
		t.Errorf("expected the default abstain threshold for an unusable value, got %d", cfg.AbstainThreshold)
	}
	if cfg.TopK != 3 {
		t.Errorf("expected the default top_k for an unusable value, got %d", cfg.TopK)
	}
}

func TestRecallHonoursConfiguredThresholds(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	if err := store.Init(); err != nil {
		t.Fatalf("failed initializing store: %v", err)
	}

	// The dependency anchor scores exactly 3, which distinguishes a threshold
	// of 3 from a threshold of 4.
	gomod := "module example.com/demo\n\ngo 1.26\n\nrequire example.com/dep v1.0.0\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644); err != nil {
		t.Fatalf("failed writing go.mod: %v", err)
	}

	anchor := filepath.Join(dir, "pkg", "worker.go")
	if err := os.MkdirAll(filepath.Dir(anchor), 0755); err != nil {
		t.Fatalf("failed creating anchor dir: %v", err)
	}
	if err := os.WriteFile(anchor, []byte("package worker\n"), 0644); err != nil {
		t.Fatalf("failed writing anchor: %v", err)
	}

	s, _, err := store.CreateDraft("Configured Threshold", "Lesson", []string{"pkg/worker.go"})
	if err != nil {
		t.Fatalf("failed creating draft: %v", err)
	}
	s.Anchors.Dependencies = []DependencyAnchor{{Name: "example.com/dep", Version: "1.0.0"}}
	s.Lifecycle.State = LifecycleActive
	s.Review.Status = ReviewApproved
	s.Verifier = failToPassVerifier("go", "test", "./...")
	if err := store.RecordBaseline(s, s.Verifier.CommandHash); err != nil {
		t.Fatalf("failed recording baseline: %v", err)
	}
	if _, err := store.SaveScar(s); err != nil {
		t.Fatalf("failed saving scar: %v", err)
	}

	// A threshold of 4 must abstain where the default of 3 would retrieve.
	candidates, err := store.Recall(QueryOptions{Threshold: 4, TopK: 3})
	if err != nil {
		t.Fatalf("recall failed: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected an explicit threshold of 4 to abstain, got %d results", len(candidates))
	}

	// An unset threshold falls back to the configured value.
	writeConfig(t, dir, "abstain_threshold: 4\ntop_k: 3\n")
	candidates, err = store.Recall(QueryOptions{})
	if err != nil {
		t.Fatalf("recall failed: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected the configured threshold of 4 to abstain, got %d results", len(candidates))
	}

	writeConfig(t, dir, "abstain_threshold: 3\ntop_k: 3\n")
	candidates, err = store.Recall(QueryOptions{})
	if err != nil {
		t.Fatalf("recall failed: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected the configured threshold of 3 to retrieve the scar, got %d results", len(candidates))
	}
}
