package scar

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreCommitHookLifecycle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitscars-hook-test-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Fails when .git does not exist
	if err := InstallPreCommitHook(tmpDir); err == nil {
		t.Error("expected error when .git directory is missing")
	}

	// Create dummy .git directory
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("failed creating .git dir: %v", err)
	}

	// Install hook
	if err := InstallPreCommitHook(tmpDir); err != nil {
		t.Fatalf("failed installing hook: %v", err)
	}

	if !IsHookInstalled(tmpDir) {
		t.Error("expected hook to be reported as installed")
	}

	hookFile := filepath.Join(gitDir, "hooks", "pre-commit")
	info, err := os.Stat(hookFile)
	if err != nil {
		t.Fatalf("hook file missing: %v", err)
	}
	// Check executable permission
	if info.Mode()&0111 == 0 {
		t.Errorf("expected hook to be executable, mode is %v", info.Mode())
	}

	// Re-install is idempotent
	if err := InstallPreCommitHook(tmpDir); err != nil {
		t.Fatalf("re-install should succeed idempotently: %v", err)
	}

	// Uninstall hook
	if err := UninstallPreCommitHook(tmpDir); err != nil {
		t.Fatalf("failed uninstalling hook: %v", err)
	}

	if IsHookInstalled(tmpDir) {
		t.Error("expected hook to be uninstalled")
	}
}
