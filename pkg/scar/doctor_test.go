package scar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreRunDoctorHealthy(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup fake .git dir
	if err := os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755); err != nil {
		t.Fatalf("failed creating .git: %v", err)
	}

	store := NewStore(tmpDir)
	if err := store.Init(); err != nil {
		t.Fatalf("failed initializing store: %v", err)
	}

	// Create and baseline an active scar
	anchorFile := filepath.Join(tmpDir, "pkg", "core.go")
	if err := os.MkdirAll(filepath.Dir(anchorFile), 0755); err != nil {
		t.Fatalf("failed creating anchor dir: %v", err)
	}
	if err := os.WriteFile(anchorFile, []byte("package core\n"), 0644); err != nil {
		t.Fatalf("failed writing anchor file: %v", err)
	}

	sc, _, err := store.CreateDraft("Core Failure", "Handle core failures", []string{"pkg/core.go"})
	if err != nil {
		t.Fatalf("failed creating draft: %v", err)
	}

	sc.Review.Status = ReviewApproved
	sc.Review.Reviewer = "maintainer"
	sc.Review.Date = "2026-10-11"
	sc.Review.Rationale = "Verified"
	sc.Verifier = failToPassVerifier("go", "test", "-race", "./...")
	sc.Lifecycle.State = LifecycleActive

	if err := store.RecordBaseline(sc, "test-cmd-hash"); err != nil {
		t.Fatalf("failed recording baseline: %v", err)
	}
	if _, err := store.SaveScar(sc); err != nil {
		t.Fatalf("failed saving scar: %v", err)
	}

	// Rebuild index
	if _, err := store.RebuildIndex(); err != nil {
		t.Fatalf("failed rebuilding index: %v", err)
	}

	// Install hook
	if err := InstallPreCommitHook(tmpDir); err != nil {
		t.Fatalf("failed installing hook: %v", err)
	}

	report := store.RunDoctor()
	if !report.Healthy {
		t.Errorf("expected healthy report, got errors: %d", report.Errors)
	}
	if report.Errors != 0 {
		t.Errorf("expected 0 errors, got %d", report.Errors)
	}

	// Verify all checks are present
	expectedChecks := []string{
		"Git Repository",
		"Store Structure",
		"Schema & Lint",
		"Lifecycle Invariants",
		"Anchor Freshness",
		"Pre-Commit Hook",
		"Trust List",
		"Index Cache",
	}

	for _, name := range expectedChecks {
		found := false
		for _, c := range report.Checks {
			if c.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing expected check %q in doctor report", name)
		}
	}
}

func TestStoreRunDoctorMissingStore(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	report := store.RunDoctor()
	if report.Healthy {
		t.Errorf("expected unhealthy report when store is missing")
	}
	if report.Errors == 0 {
		t.Errorf("expected errors when .git and .gitscars are missing")
	}
}

func TestStoreFixDoctor(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755)

	store := NewStore(tmpDir)
	_ = store.Init()

	// Intentionally remove index.json
	_ = os.Remove(filepath.Join(store.LocalDir, "index.json"))

	actions, err := store.FixDoctor()
	if err != nil {
		t.Fatalf("FixDoctor failed: %v", err)
	}
	if len(actions) == 0 {
		t.Errorf("expected repair actions, got 0")
	}

	// Verify index was rebuilt
	if _, err := os.Stat(filepath.Join(store.LocalDir, "index.json")); os.IsNotExist(err) {
		t.Errorf("expected index.json to be recreated by FixDoctor")
	}

	// Verify .gitscars/.gitignore contains .local/
	gitignoreData, err := os.ReadFile(filepath.Join(store.RepoRoot, DefaultStoreDir, ".gitignore"))
	if err != nil || !strings.Contains(string(gitignoreData), ".local/") {
		t.Errorf("expected .local/ in .gitscars/.gitignore, got: %s", string(gitignoreData))
	}
}
