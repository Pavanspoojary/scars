package scar

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// TestHarvestConfidenceCountsBodyReferencesAndCappedScope verifies the two
// refinements of the confidence model. An issue reference cited only in the
// commit body is counted, and the scope signal is the diff-size cap itself so
// that a capped fix carrying co-located tests reaches the review bar without
// an external issue reference.
func TestHarvestConfidenceCountsBodyReferencesAndCappedScope(t *testing.T) {
	tmpDir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	write := func(rel, content string) {
		t.Helper()
		full := filepath.Join(tmpDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("failed creating dir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatalf("failed writing %s: %v", rel, err)
		}
	}

	run("init", "-q")
	run("config", "user.email", "ci@example.com")
	run("config", "user.name", "CI Builder")

	store := NewStore(tmpDir)
	if err := store.Init(); err != nil {
		t.Fatalf("store init failed: %v", err)
	}

	// A conventional base commit, then two capped fix commits.
	write("pkg/base.go", "package pkg\nfunc Base() int { return 0 }\n")
	run("add", "-A")
	run("commit", "-q", "-m", "chore: initial import")

	// Fix A: exactly five files with a co-located test and no reference.
	write("pkg/core.go", "package pkg\nfunc Core() int { return 1 }\n")
	write("pkg/core2.go", "package pkg\nfunc Core2() int { return 1 }\n")
	write("pkg/core3.go", "package pkg\nfunc Core3() int { return 1 }\n")
	write("pkg/core4.go", "package pkg\nfunc Core4() int { return 1 }\n")
	write("pkg/core_test.go", "package pkg\nimport \"testing\"\nfunc TestCore(t *testing.T){ if Core() != 1 { t.Fatal(\"regression\") } }\n")
	run("add", "-A")
	run("commit", "-q", "-m", "fix: guard the core computation")

	// Fix B: five files whose issue reference appears only in the body.
	write("pkg/key.go", "package pkg\nfunc Key() int { return 2 }\n")
	write("pkg/key2.go", "package pkg\nfunc Key2() int { return 2 }\n")
	write("pkg/key3.go", "package pkg\nfunc Key3() int { return 2 }\n")
	write("pkg/key4.go", "package pkg\nfunc Key4() int { return 2 }\n")
	write("pkg/key_test.go", "package pkg\nimport \"testing\"\nfunc TestKey(t *testing.T){ if Key() != 2 { t.Fatal(\"regression\") } }\n")
	run("add", "-A")
	run("commit", "-q", "-m", "fix: guard the key computation", "-m", "Closes #417")

	result, err := store.HarvestCommits(HarvestOptions{
		DryRun:        true,
		MaxCommits:    5,
		MaxFiles:      5,
		MinConfidence: 7,
		AllowUntested: false,
	})
	if err != nil {
		t.Fatalf("harvest failed: %v", err)
	}

	if result.FoundFixes != 2 {
		t.Fatalf("expected both capped fix commits to reach the review bar, got %d", result.FoundFixes)
	}
	for _, cand := range result.Candidates {
		if cand.Confidence < 7 {
			t.Errorf("expected confidence >= 7 for %q, got %d with reasons %q", cand.Subject, cand.Confidence, cand.Reason)
		}
	}
	bySubject := map[string]string{}
	for _, cand := range result.Candidates {
		bySubject[cand.Subject] = cand.Reason
	}
	if !containsLiteral(bySubject["fix: guard the key computation"], "issue_ref(+3)") {
		t.Errorf("expected the body cited issue reference to be counted, got reasons %q", bySubject["fix: guard the key computation"])
	}
	if !containsLiteral(bySubject["fix: guard the core computation"], "test_colocated(+3)") {
		t.Errorf("expected co-located tests to be counted, got reasons %q", bySubject["fix: guard the core computation"])
	}
}

func containsLiteral(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

// TestHarvestRootCommitIsMined pins the --root handling in both git call
// sites. The first commit of a repository has no parent, and a plain
// `git diff-tree` prints nothing for a parentless commit: every fix that
// lives in the root commit would become invisible if the flag were dropped,
// silently shrinking the harvest to the second commit onward.
func TestHarvestRootCommitIsMined(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	for _, args := range [][]string{
		{"config", "user.email", "ci@example.com"},
		{"config", "user.name", "CI Builder"},
	} {
		cmd = exec.Command("git", args...)
		cmd.Dir = tmpDir
		_ = cmd.Run()
	}

	store := NewStore(tmpDir)
	if err := store.Init(); err != nil {
		t.Fatalf("store init failed: %v", err)
	}

	// The repository's only commit is itself a qualified fix: production
	// change plus a co-located test, with an issue reference in the subject.
	pkgDir := filepath.Join(tmpDir, "pkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "auth.go"),
		[]byte("package auth\nfunc Login(token string) bool { return token != \"\" }\n"), 0644); err != nil {
		t.Fatalf("write production file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "auth_test.go"),
		[]byte("package auth\nimport \"testing\"\nfunc TestLogin(t *testing.T){}\n"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git add: %v", err)
	}
	cmd = exec.Command("git", "commit", "-m", "fix: nil pointer panic on empty token authentication (#102)")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	result, err := store.HarvestCommits(HarvestOptions{
		DryRun:        true,
		MaxCommits:    10,
		MaxFiles:      5,
		MinConfidence: 7,
		AllowUntested: false,
	})
	if err != nil {
		t.Fatalf("harvest failed: %v", err)
	}

	if result.FoundFixes != 1 {
		t.Fatalf("expected the root commit to be mined as a fix, got %d (scanned %d)", result.FoundFixes, result.ScannedCommits)
	}
	cand := result.Candidates[0]
	if len(cand.Files) == 0 || cand.Files[0] != "pkg/auth.go" {
		t.Errorf("expected the root commit's production file, got %v", cand.Files)
	}
	if len(cand.TestFiles) == 0 || cand.TestFiles[0] != "pkg/auth_test.go" {
		t.Errorf("expected the root commit's co-located test, got %v", cand.TestFiles)
	}
	if cand.Confidence < 7 {
		t.Errorf("expected confidence >= 7, got %d with reasons %q", cand.Confidence, cand.Reason)
	}
}
