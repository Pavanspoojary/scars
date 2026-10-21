package scar

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnforceClean(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)
	_ = store.Init()

	res, err := store.Enforce(EnforceOptions{
		Files: []string{"pkg/clean.go"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Blocked {
		t.Errorf("expected clean file to not be blocked")
	}
}

func TestEnforceBlocked(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)
	_ = store.Init()

	anchorPath := filepath.Join(tmpDir, "pkg", "worker.go")
	_ = os.MkdirAll(filepath.Dir(anchorPath), 0755)
	_ = os.WriteFile(anchorPath, []byte("package worker\n"), 0644)

	s, _, _ := store.CreateDraft("Worker deadlock", "Buffer channels properly", []string{"pkg/worker.go"})
	s.Lifecycle.State = LifecycleActive
	s.Review.Status = ReviewApproved
	s.Severity = SeverityHigh
	s.FixHint = "Buffer worker channel to worker count"
	s.BadPattern = "ch := make(chan Task)"
	s.GoodPattern = "ch := make(chan Task, workers)"
	s.Verifier = failToPassVerifier("go", "test", "-race", "./...")
	_ = store.RecordBaseline(s, s.Verifier.CommandHash)
	_, _ = store.SaveScar(s)

	// High severity matching file -> must block
	res, err := store.Enforce(EnforceOptions{
		Files:  []string{"pkg/worker.go"},
		FailOn: "high",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Blocked {
		t.Fatalf("expected enforce to block high severity match")
	}
	if len(res.BlockingScars) != 1 {
		t.Errorf("expected 1 blocking scar, got %d", len(res.BlockingScars))
	}

	// If fail-on is critical, high severity should not block unless score >= 7
	resCrit, err := store.Enforce(EnforceOptions{
		Files:  []string{"pkg/other.go"},
		FailOn: "critical",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resCrit.Blocked {
		t.Errorf("expected unrelated file to not be blocked on critical")
	}
}
