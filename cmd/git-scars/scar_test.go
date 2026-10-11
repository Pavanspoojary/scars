package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAndFormatScar(t *testing.T) {
	raw := `---
id: scar-test-1
title: "Test scar title"
created: 2026-10-11T12:00:00Z
status: active
anchors:
  - path: pkg/sample.go
    symbol: RunProcess
    commit: abc1234
verifier:
  command: echo "all good"
  expected: pass
---

### Problem
Deadlock when channel is full.

### Prevention
Buffer channels appropriately.
`

	scar, err := ParseScar([]byte(raw))
	if err != nil {
		t.Fatalf("ParseScar failed: %v", err)
	}

	if scar.ID != "scar-test-1" {
		t.Errorf("expected ID scar-test-1, got %q", scar.ID)
	}
	if scar.Title != "Test scar title" {
		t.Errorf("expected Title 'Test scar title', got %q", scar.Title)
	}
	if len(scar.Anchors) != 1 {
		t.Fatalf("expected 1 anchor, got %d", len(scar.Anchors))
	}
	if scar.Anchors[0].Path != "pkg/sample.go" || scar.Anchors[0].Symbol != "RunProcess" || scar.Anchors[0].Commit != "abc1234" {
		t.Errorf("unexpected anchor fields: %+v", scar.Anchors[0])
	}
	if scar.Verifier.Command != `echo "all good"` {
		t.Errorf("unexpected verifier command: %q", scar.Verifier.Command)
	}
	if !strings.Contains(scar.Body, "Deadlock when channel is full") {
		t.Errorf("unexpected body: %q", scar.Body)
	}

	formatted := FormatScar(scar)
	reparsed, err := ParseScar([]byte(formatted))
	if err != nil {
		t.Fatalf("re-parsing formatted scar failed: %v", err)
	}

	if reparsed.ID != scar.ID || reparsed.Title != scar.Title {
		t.Errorf("roundtrip mismatch: ID=%s, Title=%s", reparsed.ID, reparsed.Title)
	}
	if len(reparsed.Anchors) != len(scar.Anchors) {
		t.Errorf("roundtrip anchor count mismatch")
	}
}

func TestParseScarInvalid(t *testing.T) {
	_, err := ParseScar([]byte("no frontmatter here"))
	if err == nil {
		t.Error("expected error for missing frontmatter, got nil")
	}

	unclosed := "---\nid: test\ntitle: test\n"
	_, err = ParseScar([]byte(unclosed))
	if err == nil {
		t.Error("expected error for unclosed frontmatter, got nil")
	}
}

func TestCLICommandsInTempDir(t *testing.T) {
	tempDir := t.TempDir()

	// 1. init
	if err := runInit(tempDir); err != nil {
		t.Fatalf("runInit failed: %v", err)
	}
	configPath := filepath.Join(tempDir, ".gitscars", "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config.yaml not created: %v", err)
	}

	// Idempotent init
	if err := runInit(tempDir); err != nil {
		t.Fatalf("second runInit failed: %v", err)
	}

	// 2. add
	dummyFile := filepath.Join(tempDir, "sample.txt")
	if err := os.WriteFile(dummyFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to create dummy file: %v", err)
	}

	addArgs := []string{
		"-id", "scar-001",
		"-title", "Avoid memory leak in parser",
		"-path", "sample.txt",
		"-symbol", "ParseData",
		"-verifier", "test -f sample.txt",
		"-body", "Always close stream before returning.",
	}
	if err := runAdd(tempDir, addArgs); err != nil {
		t.Fatalf("runAdd failed: %v", err)
	}

	scarPath := filepath.Join(tempDir, ".gitscars", "scars", "scar-001.md")
	data, err := os.ReadFile(scarPath)
	if err != nil {
		t.Fatalf("scar file not created: %v", err)
	}
	s, err := ParseScar(data)
	if err != nil {
		t.Fatalf("failed to parse created scar: %v", err)
	}
	if s.ID != "scar-001" || s.Title != "Avoid memory leak in parser" {
		t.Errorf("mismatched scar data: %+v", s)
	}

	// 3. list
	if err := runList(tempDir, nil); err != nil {
		t.Fatalf("runList failed: %v", err)
	}

	// 4. verify pass
	if err := runVerify(tempDir, []string{"scar-001"}); err != nil {
		t.Fatalf("runVerify failed unexpectedly: %v", err)
	}

	// 5. verify fail
	failingArgs := []string{
		"-id", "scar-002",
		"-title", "Failing check",
		"-verifier", "false",
	}
	if err := runAdd(tempDir, failingArgs); err != nil {
		t.Fatalf("runAdd for failing scar failed: %v", err)
	}
	if err := runVerify(tempDir, []string{"scar-002"}); err == nil {
		t.Errorf("expected runVerify to fail for scar-002, but it succeeded")
	}

	// 6. CheckDrift
	drifted, _ := CheckDrift(tempDir, s)
	if drifted {
		t.Errorf("expected no drift when sample.txt exists")
	}
	_ = os.Remove(dummyFile)
	drifted, reason := CheckDrift(tempDir, s)
	if !drifted {
		t.Errorf("expected drift when sample.txt was deleted")
	}
	if !strings.Contains(reason, "path missing") {
		t.Errorf("unexpected drift reason: %s", reason)
	}
}
