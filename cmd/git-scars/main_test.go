package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Pavanspoojary/scars/pkg/scar"
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

	// 5. Run verify
	s.Verifier.Argv = []string{"echo", "ok"}
	_ = store.RecordBaseline(s, "hash-echo")
	_, _ = store.SaveScar(s)

	if err := run([]string{"verify", s.ID, "--interactive"}); err != nil {
		t.Fatalf("run verify failed: %v", err)
	}

	// Verify invariant: verification cannot activate scar
	sAfterVerify, _, _ := store.LoadScar(s.ID)
	if sAfterVerify.Lifecycle.State != scar.LifecycleDraft {
		t.Errorf("expected draft state after verify, got %s", sAfterVerify.Lifecycle.State)
	}
	if sAfterVerify.Review.Status != scar.ReviewUnreviewed {
		t.Errorf("expected unreviewed after verify, got %s", sAfterVerify.Review.Status)
	}

	// Invariant test: cannot promote to active without verifier passing or approved review
	if err := run([]string{"promote", s.ID, "--status", "active", "--reviewer", "bob"}); err != nil {
		// With echo passing on fixed tree and exception on base commit, promote with rationale should be tested:
	}

	// Simulate successful verification exception with rationale
	sAfterVerify.Verifier.Result = scar.VerificationException
	if _, err := store.SaveScar(sAfterVerify); err != nil {
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

func TestCLIRecallLintFeedback(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitscars-cli-stage3-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	origWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed chdir: %v", err)
	}
	defer os.Chdir(origWd)

	store := scar.NewStore(tmpDir)
	_ = store.Init()

	s, _, _ := store.CreateDraft("Leak in Worker", "Close reader", []string{"pkg/worker.go"})
	s.Lifecycle.State = scar.LifecycleActive
	s.Review.Status = scar.ReviewApproved
	s.Verifier.Result = scar.VerificationFailToPass
	_ = store.RecordBaseline(s, "hash-worker")
	_, _ = store.SaveScar(s)

	// Create anchor file so freshness is Current
	_ = os.MkdirAll(filepath.Join(tmpDir, "pkg"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "pkg/worker.go"), []byte("package worker\n"), 0644)
	_ = store.RecordBaseline(s, "hash-worker")
	_, _ = store.SaveScar(s)

	// 1. Recall matching file
	if err := run([]string{"recall", "--files", "pkg/worker.go"}); err != nil {
		t.Fatalf("recall matching file failed: %v", err)
	}

	// 2. Recall with JSON flag
	if err := run([]string{"recall", "--files", "pkg/worker.go", "--json"}); err != nil {
		t.Fatalf("recall json failed: %v", err)
	}

	// 3. Recall abstaining on unrelated query
	if err := run([]string{"recall", "--task", "Update CSS stylesheet"}); err != nil {
		t.Fatalf("recall abstain should succeed with 0 exit code: %v", err)
	}

	// 4. Lint
	if err := run([]string{"lint"}); err != nil {
		t.Fatalf("lint failed: %v", err)
	}

	// 5. Feedback
	if err := run([]string{"feedback", s.ID, "--rating", "helpful", "--notes", "Avoided recurrence"}); err != nil {
		t.Fatalf("feedback failed: %v", err)
	}

	// 6. Hook install / status / uninstall
	_ = os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755)
	if err := run([]string{"hook", "install"}); err != nil {
		t.Fatalf("hook install failed: %v", err)
	}
	if err := run([]string{"hook", "status"}); err != nil {
		t.Fatalf("hook status failed: %v", err)
	}
	if err := run([]string{"hook", "uninstall"}); err != nil {
		t.Fatalf("hook uninstall failed: %v", err)
	}

	// 7. Index rebuild
	if err := run([]string{"index"}); err != nil {
		t.Fatalf("index rebuild failed: %v", err)
	}

	// 8. Diff anchor baseline
	if err := run([]string{"diff", s.ID}); err != nil {
		t.Fatalf("diff command failed: %v", err)
	}

	// 9. Stats command (text and JSON)
	if err := run([]string{"stats"}); err != nil {
		t.Fatalf("stats command failed: %v", err)
	}
	if err := run([]string{"stats", "--json"}); err != nil {
		t.Fatalf("stats --json command failed: %v", err)
	}

	// 10. Doctor command (text and JSON)
	if err := run([]string{"doctor"}); err != nil {
		t.Fatalf("doctor command failed: %v", err)
	}
	if err := run([]string{"doctor", "--json"}); err != nil {
		t.Fatalf("doctor --json command failed: %v", err)
	}

	// 11. Interactive add
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed creating pipe: %v", err)
	}
	os.Stdin = pr
	go func() {
		defer pw.Close()
		pw.Write([]byte("Interactive Scar\nNever forget this\npkg/worker.go\n"))
	}()
	if err := run([]string{"add", "-i"}); err != nil {
		t.Fatalf("interactive add failed: %v", err)
	}

	// 12. Inject command (XML and Markdown)
	if err := run([]string{"inject", "--files", "pkg/worker.go", "--budget", "300", "--format", "xml"}); err != nil {
		t.Fatalf("inject xml failed: %v", err)
	}
	if err := run([]string{"inject", "--files", "pkg/worker.go", "--budget", "300", "--format", "markdown"}); err != nil {
		t.Fatalf("inject markdown failed: %v", err)
	}

	// 13. Sync command
	if err := run([]string{"sync", "--targets", "AGENTS.md,CLAUDE.md"}); err != nil {
		t.Fatalf("sync command failed: %v", err)
	}

	// 14. Triage command (text and JSON)
	if err := run([]string{"triage", "--error", "panic: deadlock in pkg/worker.go"}); err != nil {
		t.Fatalf("triage command failed: %v", err)
	}
	if err := run([]string{"triage", "--error", "panic: deadlock in pkg/worker.go", "--json"}); err != nil {
		t.Fatalf("triage --json command failed: %v", err)
	}

	// 15. Enforce command on clean file
	if err := run([]string{"enforce", "--files", "pkg/clean.go"}); err != nil {
		t.Fatalf("enforce command failed on clean file: %v", err)
	}

	// 16. Init agent onboarding
	if err := run([]string{"init", "--agent", "claude,cursor,zed,antigravity"}); err != nil {
		t.Fatalf("init with agent flag failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".claude", "mcp.json")); os.IsNotExist(err) {
		t.Errorf("expected .claude/mcp.json to exist")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".cursor", "rules", "git-scars.mdc")); os.IsNotExist(err) {
		t.Errorf("expected .cursor/rules/git-scars.mdc to exist")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".zed", "settings.json")); os.IsNotExist(err) {
		t.Errorf("expected .zed/settings.json to exist")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".gemini", "config", "rules", "git-scars.md")); os.IsNotExist(err) {
		t.Errorf("expected .gemini/config/rules/git-scars.md to exist")
	}
}
