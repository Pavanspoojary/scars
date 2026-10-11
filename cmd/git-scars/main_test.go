package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Pavanspoojary/si-scars/pkg/scar"
)

func TestCLIWorkflow(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitscars-cli-test-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	origWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed chdir: %v", err)
	}
	defer os.Chdir(origWd)

	// 1. Run init
	if err := run([]string{"init"}); err != nil {
		t.Fatalf("run init failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".gitscars", "scars")); os.IsNotExist(err) {
		t.Fatal("expected .gitscars/scars to exist after init")
	}

	// 2. Run add
	if err := run([]string{"add", "--title", "Deadlock in Mutex", "--lesson", "Avoid re-entrant locks", "--files", "pkg/lock.go"}); err != nil {
		t.Fatalf("run add failed: %v", err)
	}

	// Verify scar exists in store
	store := scar.NewStore(tmpDir)
	scars, err := store.ListScars()
	if err != nil {
		t.Fatalf("list scars failed: %v", err)
	}
	if len(scars) != 1 {
		t.Fatalf("expected 1 scar, found %d", len(scars))
	}
	s := scars[0].Scar
	if !strings.Contains(s.Title, "Deadlock in Mutex") {
		t.Errorf("unexpected title: %q", s.Title)
	}
	if s.Lifecycle.State != scar.LifecycleDraft {
		t.Errorf("expected draft state, got %s", s.Lifecycle.State)
	}

	// 3. Run list
	if err := run([]string{"list"}); err != nil {
		t.Fatalf("run list failed: %v", err)
	}

	// 4. Run show
	if err := run([]string{"show", s.ID}); err != nil {
		t.Fatalf("run show failed: %v", err)
	}

	// 5. Invariant test: cannot promote to active without verifier passing
	if err := run([]string{"promote", s.ID, "--status", "active", "--reviewer", "bob"}); err == nil {
		t.Fatal("expected promote to fail because verifier result is not_attempted")
	}

	// Simulate successful verification exception with rationale
	s.Verifier.Result = scar.VerificationException
	if _, err := store.SaveScar(s); err != nil {
		t.Fatalf("failed saving scar: %v", err)
	}

	// Now promote with rationale should succeed
	if err := run([]string{"promote", s.ID, "--status", "active", "--reviewer", "bob", "--rationale", "Reviewed manual test"}); err != nil {
		t.Fatalf("promote with exception rationale failed: %v", err)
	}

	// Check updated state
	reloaded, _, err := store.LoadScar(s.ID)
	if err != nil {
		t.Fatalf("load scar failed: %v", err)
	}
	if reloaded.Lifecycle.State != scar.LifecycleActive {
		t.Errorf("expected state active, got %s", reloaded.Lifecycle.State)
	}
	if reloaded.Review.Status != scar.ReviewApproved {
		t.Errorf("expected review status approved, got %s", reloaded.Review.Status)
	}

	// 6. Test retire
	if err := run([]string{"retire", s.ID, "--rationale", "Code path refactored out"}); err != nil {
		t.Fatalf("retire failed: %v", err)
	}
	retiredScar, _, _ := store.LoadScar(s.ID)
	if retiredScar.Lifecycle.State != scar.LifecycleRetired {
		t.Errorf("expected retired state, got %s", retiredScar.Lifecycle.State)
	}
}

func TestUsageAndHelp(t *testing.T) {
	buf := new(bytes.Buffer)
	if err := run([]string{"help"}); err != nil {
		t.Fatalf("help command failed: %v", err)
	}
	_ = buf
}
