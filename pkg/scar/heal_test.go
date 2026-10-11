package scar

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHealCommand(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)
	_ = store.Init()

	// Seed an active scar
	s, _, _ := store.CreateDraft("Worker Deadlock", "Buffer worker channels", []string{"pkg/worker.go"})
	s.Lifecycle.State = LifecycleActive
	s.Review.Status = ReviewApproved
	s.Verifier.Result = VerificationFailToPass
	s.Severity = SeverityHigh
	s.FixHint = "Buffer task channel"
	s.BadPattern = "make(chan Task)"
	s.GoodPattern = "make(chan Task, 10)"
	_ = os.MkdirAll(filepath.Join(tmpDir, "pkg"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "pkg/worker.go"), []byte("package worker\n"), 0644)
	_ = store.RecordBaseline(s, "hash-worker")
	_, _ = store.SaveScar(s)

	ctx := context.Background()

	// 1. Successful command
	successReport, err := store.RunHealCommand(ctx, []string{"echo", "all tests passed"})
	if err != nil {
		t.Fatalf("run heal success failed: %v", err)
	}
	if !successReport.Success || successReport.ExitCode != 0 {
		t.Errorf("expected success report, got: %+v", successReport)
	}
	if len(successReport.MatchedScars) != 0 {
		t.Errorf("expected 0 matched scars on success, got %d", len(successReport.MatchedScars))
	}

	// 2. Failing command with simulated deadlock
	failReport, err := store.RunHealCommand(ctx, []string{"sh", "-c", "echo 'fatal error: all goroutines asleep - deadlock!\npkg/worker.go:15 +0x34' >&2; exit 2"})
	if err != nil {
		t.Fatalf("run heal fail encountered system error: %v", err)
	}
	if failReport.Success || failReport.ExitCode != 2 {
		t.Errorf("expected exit code 2 and failure, got: %+v", failReport)
	}
	if len(failReport.MatchedScars) == 0 {
		t.Fatalf("expected triage to match deadlock scar, got 0 matches")
	}
	if failReport.MatchedScars[0].Scar.ID != s.ID {
		t.Errorf("expected matched scar %s, got %s", s.ID, failReport.MatchedScars[0].Scar.ID)
	}
	if !strings.Contains(failReport.RemediationPrompt, "<git_scars_remediation") {
		t.Errorf("expected XML remediation prompt, got: %s", failReport.RemediationPrompt)
	}
	if !strings.Contains(failReport.RemediationPrompt, "make(chan Task, 10)") {
		t.Errorf("expected good pattern in remediation prompt, got: %s", failReport.RemediationPrompt)
	}
}
