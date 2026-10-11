package scar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractErrorSignatures(t *testing.T) {
	sampleLog := `=== RUN   TestWorkerBurst
fatal error: all goroutines are asleep - deadlock!
goroutine 1 [chan send]:
main.main()
	/app/cmd/worker.go:42 +0x120
exit status 2
FAIL	pkg/worker	0.012s
`
	sigs := ExtractErrorSignatures(sampleLog)
	if len(sigs) == 0 {
		t.Fatalf("expected extracted signatures, got none")
	}

	foundDeadlock := false
	for _, s := range sigs {
		if strings.Contains(s, "deadlock") {
			foundDeadlock = true
			break
		}
	}
	if !foundDeadlock {
		t.Errorf("expected deadlock signature, got: %v", sigs)
	}
}

func TestTriageError(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)
	_ = store.Init()

	anchorPath := filepath.Join(tmpDir, "pkg", "worker.go")
	_ = os.MkdirAll(filepath.Dir(anchorPath), 0755)
	_ = os.WriteFile(anchorPath, []byte("package worker\n"), 0644)

	s, _, _ := store.CreateDraft("Worker Channel Deadlock", "Buffer worker task channels", []string{"pkg/worker.go"})
	s.Anchors.ErrorSignatures = []string{"all goroutines are asleep - deadlock"}
	s.Lifecycle.State = LifecycleActive
	s.Review.Status = ReviewApproved
	s.Verifier.Result = VerificationFailToPass
	_ = store.RecordBaseline(s, "hash-worker")
	_, _ = store.SaveScar(s)

	trace := `fatal error: all goroutines are asleep - deadlock!
goroutine 19 [chan send]:
pkg/worker.go:45`

	res, err := store.TriageError(trace)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Matches) == 0 {
		t.Fatalf("expected triage matches for deadlock trace, got 0")
	}
	if res.Matches[0].Scar.ID != s.ID {
		t.Errorf("expected matched scar %s, got %s", s.ID, res.Matches[0].Scar.ID)
	}
}
