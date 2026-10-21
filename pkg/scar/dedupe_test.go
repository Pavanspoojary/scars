package scar

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindDuplicates(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitscars-dedupe-test-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewStore(tmpDir)
	_ = store.Init()

	// Seed two active scars with nearly identical content
	s1, _, _ := store.CreateDraft("Concurrent Map Write in Exporter", "Always guard metrics map with sync.RWMutex", []string{"pkg/exporter.go"})
	s1.Lifecycle.State = LifecycleActive
	s1.Review.Status = ReviewApproved
	s1.Verifier = failToPassVerifier("go", "test", "-race", "./...")
	_ = os.MkdirAll(filepath.Join(tmpDir, "pkg"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "pkg/exporter.go"), []byte("package exporter\n"), 0644)
	_ = store.RecordBaseline(s1, "hash1")
	_, _ = store.SaveScar(s1)

	s2, _, _ := store.CreateDraft("Concurrent Map Write in Telemetry Exporter", "Always guard telemetry metrics map with sync.RWMutex", []string{"pkg/exporter.go"})
	s2.Lifecycle.State = LifecycleActive
	s2.Review.Status = ReviewApproved
	s2.Verifier = failToPassVerifier("go", "test", "-race", "./...")
	_ = store.RecordBaseline(s2, "hash2")
	_, _ = store.SaveScar(s2)

	// Seed a third distinct active scar
	s3, _, _ := store.CreateDraft("Buffer Channel Deadlock in Dispatcher", "Worker channels must be buffered to worker count", []string{"pkg/disp.go"})
	s3.Lifecycle.State = LifecycleActive
	s3.Review.Status = ReviewApproved
	s3.Verifier = failToPassVerifier("go", "test", "-race", "./...")
	_ = os.WriteFile(filepath.Join(tmpDir, "pkg/disp.go"), []byte("package disp\n"), 0644)
	_ = store.RecordBaseline(s3, "hash3")
	_, _ = store.SaveScar(s3)

	candidates, err := store.FindDuplicates(0.70)
	if err != nil {
		t.Fatalf("find duplicates failed: %v", err)
	}

	if len(candidates) != 1 {
		t.Fatalf("expected 1 duplicate pair, got %d", len(candidates))
	}

	pair := candidates[0]
	if (pair.ScarA.ID != s1.ID && pair.ScarA.ID != s2.ID) || (pair.ScarB.ID != s1.ID && pair.ScarB.ID != s2.ID) {
		t.Errorf("unexpected duplicate pair: %s and %s", pair.ScarA.ID, pair.ScarB.ID)
	}
	if pair.Similarity < 0.70 {
		t.Errorf("expected similarity >= 0.70, got %.2f", pair.Similarity)
	}
}
