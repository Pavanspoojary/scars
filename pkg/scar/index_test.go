package scar

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRebuildAndLoadIndex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitscars-index-test-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewStore(tmpDir)
	if err := store.Init(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Create test scar
	sc, _, err := store.CreateDraft("Test Index Scar", "Index testing lesson", []string{"pkg/index.go"})
	if err != nil {
		t.Fatalf("create draft failed: %v", err)
	}

	// Initially LoadIndex should return not exist
	if _, err := store.LoadIndex(); err == nil {
		t.Error("expected LoadIndex to fail before rebuild")
	}

	// Rebuild index
	idx, err := store.RebuildIndex()
	if err != nil {
		t.Fatalf("rebuild index failed: %v", err)
	}

	if idx.TotalScars != 1 {
		t.Errorf("expected 1 scar in index, got %d", idx.TotalScars)
	}
	if idx.Scars[0].ID != sc.ID {
		t.Errorf("expected scar ID %s, got %s", sc.ID, idx.Scars[0].ID)
	}

	// Verify index file exists on disk
	indexPath := filepath.Join(store.LocalDir, IndexFileName)
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Fatalf("expected index file %s to exist", indexPath)
	}

	// LoadIndex should succeed
	loaded, err := store.LoadIndex()
	if err != nil {
		t.Fatalf("load index failed: %v", err)
	}

	if loaded.TotalScars != 1 {
		t.Errorf("expected 1 scar in loaded index, got %d", loaded.TotalScars)
	}
	if loaded.Scars[0].Title != "Test Index Scar" {
		t.Errorf("expected title 'Test Index Scar', got %q", loaded.Scars[0].Title)
	}
}
