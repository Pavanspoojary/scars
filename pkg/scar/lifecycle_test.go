package scar

import (
	"os"
	"testing"
)

func TestContradictAndSupersedeScar(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitscars-lifecycle-test-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewStore(tmpDir)
	_ = store.Init()

	s1, _, _ := store.CreateDraft("Original Scar", "Old lesson", []string{"pkg/old.go"})
	s2, _, _ := store.CreateDraft("Successor Scar", "New refined lesson", []string{"pkg/new.go"})

	// Test 1: Contradict s1 by s2
	cScar, err := store.ContradictScar(s1.ID, s2.ID, "New benchmark proved approach invalid")
	if err != nil {
		t.Fatalf("contradict failed: %v", err)
	}
	if cScar.Lifecycle.State != LifecycleContradicted {
		t.Errorf("expected contradicted state, got %s", cScar.Lifecycle.State)
	}
	if len(cScar.Lifecycle.ConflictsWith) == 0 || cScar.Lifecycle.ConflictsWith[0] != s2.ID {
		t.Errorf("expected conflicts_with [%s], got %v", s2.ID, cScar.Lifecycle.ConflictsWith)
	}

	// Test 2: Supersede s1 by s2
	s3, _, _ := store.CreateDraft("Third Scar", "Third lesson", []string{"pkg/third.go"})
	supScar, err := store.SupersedeScar(s3.ID, s2.ID)
	if err != nil {
		t.Fatalf("supersede failed: %v", err)
	}
	if supScar.Lifecycle.State != LifecycleSuperseded {
		t.Errorf("expected superseded state, got %s", supScar.Lifecycle.State)
	}
	if supScar.Lifecycle.SupersededBy != s2.ID {
		t.Errorf("expected superseded_by %s, got %s", s2.ID, supScar.Lifecycle.SupersededBy)
	}

	// Verify s2 records s3 in Supersedes
	reloadedS2, _, err := store.LoadScar(s2.ID)
	if err != nil {
		t.Fatalf("load s2 failed: %v", err)
	}
	found := false
	for _, sup := range reloadedS2.Lifecycle.Supersedes {
		if sup == s3.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected s2 to track superseded scar %s in %v", s3.ID, reloadedS2.Lifecycle.Supersedes)
	}
}
