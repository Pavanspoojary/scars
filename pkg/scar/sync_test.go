package scar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncInstructionFile(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)
	_ = store.Init()

	// Create an active scar
	s, _, _ := store.CreateDraft("Deadlock Risk", "Buffer channel", []string{"pkg/worker.go"})
	s.Lifecycle.State = LifecycleActive
	s.Review.Status = ReviewApproved
	s.Verifier.Result = VerificationFailToPass
	s.Verifier.Argv = []string{"go", "test", "-race"}
	_ = store.RecordBaseline(s, "hash-test")
	_, _ = store.SaveScar(s)

	// 1. Test creation on non-existent file
	target := filepath.Join(tmpDir, "CLAUDE.md")
	res, err := store.SyncInstructionFile(target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Created || res.Action != "created" {
		t.Errorf("expected created action, got %+v", res)
	}

	data, _ := os.ReadFile(target)
	if !strings.Contains(string(data), s.ID) || !strings.Contains(string(data), "Deadlock Risk") {
		t.Errorf("expected scar details in created CLAUDE.md, got: %s", string(data))
	}

	// 2. Test idempotency
	res2, err := store.SyncInstructionFile(target)
	if err != nil {
		t.Fatalf("unexpected error on second sync: %v", err)
	}
	if res2.Action != "unchanged" {
		t.Errorf("expected unchanged action on second sync, got: %+v", res2)
	}

	// 3. Test preservation of external content
	customContent := "# Existing Custom Agent Prompt\nBe helpful and concise.\n\n" + string(data) + "\n\n# Footer\nKeep code clean."
	_ = os.WriteFile(target, []byte(customContent), 0644)

	res3, err := store.SyncInstructionFile(target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res3.Action != "unchanged" {
		t.Errorf("expected unchanged after external content added, got: %+v", res3)
	}

	updatedData, _ := os.ReadFile(target)
	if !strings.Contains(string(updatedData), "Existing Custom Agent Prompt") || !strings.Contains(string(updatedData), "Footer") {
		t.Errorf("expected external content to be preserved, got: %s", string(updatedData))
	}
}

func TestSyncExistingInstructions(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)
	_ = store.Init()

	// Pre-create AGENTS.md in repo root
	agentsPath := filepath.Join(tmpDir, "AGENTS.md")
	_ = os.WriteFile(agentsPath, []byte("# Agents\nOriginal content\n"), 0644)

	results, err := store.SyncExistingInstructions(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for AGENTS.md, got %d", len(results))
	}
	if results[0].Action != "updated" {
		t.Errorf("expected action updated, got %s", results[0].Action)
	}
}
