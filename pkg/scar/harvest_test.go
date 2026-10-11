package scar

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestHarvestCommits(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "ci@example.com")
	cmd.Dir = tmpDir
	_ = cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "CI Builder")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	store := NewStore(tmpDir)
	if err := store.Init(); err != nil {
		t.Fatalf("store init failed: %v", err)
	}

	// 1. Initial commit (feat)
	pkgDir := filepath.Join(tmpDir, "pkg")
	_ = os.MkdirAll(pkgDir, 0755)
	_ = os.WriteFile(filepath.Join(pkgDir, "auth.go"), []byte("package auth\nfunc Login() {}\n"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	_ = cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "feat: implement initial auth logic")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	// 2. Bug fix commit (fix: nil pointer on empty token)
	_ = os.WriteFile(filepath.Join(pkgDir, "auth.go"), []byte("package auth\nfunc Login(token string) bool { return token != \"\" }\n"), 0644)
	_ = os.WriteFile(filepath.Join(pkgDir, "auth_test.go"), []byte("package auth\nimport \"testing\"\nfunc TestLogin(t *testing.T){}\n"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	_ = cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "fix: nil pointer panic on empty token authentication")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	// Test dry run
	dryResult, err := store.HarvestCommits(HarvestOptions{DryRun: true, MaxCommits: 5})
	if err != nil {
		t.Fatalf("harvest dry-run failed: %v", err)
	}
	if dryResult.FoundFixes != 1 {
		t.Fatalf("expected 1 discovered fix, got %d", dryResult.FoundFixes)
	}
	if dryResult.DraftsCreated != 0 {
		t.Errorf("expected 0 drafts created on dry-run, got %d", dryResult.DraftsCreated)
	}
	cand := dryResult.Candidates[0]
	if len(cand.Files) == 0 || cand.Files[0] != "pkg/auth.go" {
		t.Errorf("expected production file pkg/auth.go, got %v", cand.Files)
	}

	// Test real harvest execution
	realResult, err := store.HarvestCommits(HarvestOptions{DryRun: false, MaxCommits: 5})
	if err != nil {
		t.Fatalf("harvest execution failed: %v", err)
	}
	if realResult.DraftsCreated != 1 {
		t.Fatalf("expected 1 draft created, got %d", realResult.DraftsCreated)
	}

	// Verify drafted scar exists and has provenance
	scars, err := store.ListScars()
	if err != nil || len(scars) != 1 {
		t.Fatalf("expected 1 scar in store, got %d (err: %v)", len(scars), err)
	}
	draftScar := scars[0].Scar
	if draftScar.Lifecycle.State != LifecycleDraft {
		t.Errorf("expected draft lifecycle state, got %s", draftScar.Lifecycle.State)
	}
	if draftScar.Lifecycle.Provenance != "git-commit:"+cand.CommitSHA {
		t.Errorf("expected provenance git-commit:%s, got %s", cand.CommitSHA, draftScar.Lifecycle.Provenance)
	}

	// Re-running harvest should skip already harvested commit
	secondResult, err := store.HarvestCommits(HarvestOptions{DryRun: false, MaxCommits: 5})
	if err != nil {
		t.Fatalf("second harvest failed: %v", err)
	}
	if secondResult.DraftsCreated != 0 {
		t.Errorf("expected 0 drafts created on duplicate run, got %d", secondResult.DraftsCreated)
	}
}
