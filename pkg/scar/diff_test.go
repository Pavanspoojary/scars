package scar

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiffScarAnchors(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitscars-diff-test-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewStore(tmpDir)
	_ = store.Init()

	testFile := filepath.Join(tmpDir, "pkg/handler.go")
	_ = os.MkdirAll(filepath.Dir(testFile), 0755)
	_ = os.WriteFile(testFile, []byte("package handler\nfunc Handle() {}\n"), 0644)

	scar, _, err := store.CreateDraft("Handler Bug", "Check handler context", []string{"pkg/handler.go"})
	if err != nil {
		t.Fatalf("create draft failed: %v", err)
	}

	// 1. Unrecorded baseline
	diffs, err := store.DiffScarAnchors(scar)
	if err != nil {
		t.Fatalf("diff failed: %v", err)
	}
	if len(diffs) != 1 || diffs[0].Status != "unrecorded" {
		t.Errorf("expected status 'unrecorded', got: %+v", diffs)
	}

	// 2. Record baseline -> current
	_ = store.RecordBaseline(scar, "cmd-hash")
	diffs, err = store.DiffScarAnchors(scar)
	if err != nil {
		t.Fatalf("diff failed: %v", err)
	}
	if len(diffs) != 1 || diffs[0].Status != "current" {
		t.Errorf("expected status 'current', got: %+v", diffs)
	}

	// 3. Modify file -> modified
	_ = os.WriteFile(testFile, []byte("package handler\nfunc Handle() { // modified\n}\n"), 0644)
	diffs, err = store.DiffScarAnchors(scar)
	if err != nil {
		t.Fatalf("diff failed: %v", err)
	}
	if len(diffs) != 1 || diffs[0].Status != "modified" {
		t.Errorf("expected status 'modified', got: %+v", diffs)
	}

	// 4. Missing file -> missing
	_ = os.Remove(testFile)
	diffs, err = store.DiffScarAnchors(scar)
	if err != nil {
		t.Fatalf("diff failed: %v", err)
	}
	if len(diffs) != 1 || diffs[0].Status != "missing" {
		t.Errorf("expected status 'missing', got: %+v", diffs)
	}
}
