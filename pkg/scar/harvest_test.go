package scar

import (
	"fmt"
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

	// 2. High-confidence bug fix commit (fix: nil pointer on empty token #102) with co-located test
	_ = os.WriteFile(filepath.Join(pkgDir, "auth.go"), []byte("package auth\nfunc Login(token string) bool { return token != \"\" }\n"), 0644)
	_ = os.WriteFile(filepath.Join(pkgDir, "auth_test.go"), []byte("package auth\nimport \"testing\"\nfunc TestLogin(t *testing.T){}\n"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	_ = cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "fix: nil pointer panic on empty token authentication (#102)")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	// 3. Diffuse commit with 7 files (should be skipped due to MaxFiles=5)
	diffuseDir := filepath.Join(tmpDir, "diffuse")
	_ = os.MkdirAll(diffuseDir, 0755)
	for i := 1; i <= 7; i++ {
		_ = os.WriteFile(filepath.Join(diffuseDir, fmt.Sprintf("file%d.go", i)), []byte("package diffuse\n"), 0644)
	}
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	_ = cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "fix: broad refactor across multiple subsystems")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	// 4. Untested fix commit (fix: log formatting issue without tests)
	_ = os.WriteFile(filepath.Join(pkgDir, "logger.go"), []byte("package auth\nfunc Log(){}\n"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	_ = cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "fix: logger formatting bug")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	// Test dry run with default strict quality filters (RequireTests=true, MaxFiles=5, MinConfidence=7)
	dryResult, err := store.HarvestCommits(HarvestOptions{
		DryRun:        true,
		MaxCommits:    10,
		MaxFiles:      5,
		MinConfidence: 7,
		AllowUntested: false,
	})
	if err != nil {
		t.Fatalf("harvest dry-run failed: %v", err)
	}

	// Only commit #2 qualifies; #3 has > 5 files; #4 has no tests
	if dryResult.FoundFixes != 1 {
		t.Fatalf("expected 1 discovered high-confidence fix, got %d", dryResult.FoundFixes)
	}
	if dryResult.DraftsCreated != 0 {
		t.Errorf("expected 0 drafts created on dry-run, got %d", dryResult.DraftsCreated)
	}
	cand := dryResult.Candidates[0]
	if len(cand.Files) == 0 || cand.Files[0] != "pkg/auth.go" {
		t.Errorf("expected production file pkg/auth.go, got %v", cand.Files)
	}
	if cand.Confidence < 7 {
		t.Errorf("expected confidence >= 7, got %d", cand.Confidence)
	}
	if len(cand.Verifier) == 0 {
		t.Errorf("expected inferred verifier command, got empty")
	}

	// Test AllowUntested: should now find #2 and #4 (2 fixes)
	untestedResult, err := store.HarvestCommits(HarvestOptions{
		DryRun:        true,
		MaxCommits:    10,
		MaxFiles:      5,
		MinConfidence: 2,
		AllowUntested: true,
	})
	if err != nil {
		t.Fatalf("harvest with AllowUntested failed: %v", err)
	}
	if untestedResult.FoundFixes != 2 {
		t.Fatalf("expected 2 discovered fixes with AllowUntested, got %d", untestedResult.FoundFixes)
	}

	// Test real harvest execution on qualified candidate
	realResult, err := store.HarvestCommits(HarvestOptions{
		DryRun:        false,
		MaxCommits:    10,
		MaxFiles:      5,
		MinConfidence: 7,
		AllowUntested: false,
	})
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
	secondResult, err := store.HarvestCommits(HarvestOptions{
		DryRun:        false,
		MaxCommits:    10,
		MaxFiles:      5,
		MinConfidence: 7,
		AllowUntested: false,
	})
	if err != nil {
		t.Fatalf("second harvest failed: %v", err)
	}
	if secondResult.DraftsCreated != 0 {
		t.Errorf("expected 0 drafts created on duplicate run, got %d", secondResult.DraftsCreated)
	}
}
