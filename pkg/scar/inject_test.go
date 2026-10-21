package scar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInjectGuardrails(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)
	_ = store.Init()

	// Create anchor file
	anchorPath := filepath.Join(tmpDir, "pkg", "auth.go")
	_ = os.MkdirAll(filepath.Dir(anchorPath), 0755)
	_ = os.WriteFile(anchorPath, []byte("package auth\n"), 0644)

	s, _, _ := store.CreateDraft("Auth Token Leak", "Never log raw bearer token", []string{"pkg/auth.go"})
	s.Lifecycle.State = LifecycleActive
	s.Review.Status = ReviewApproved
	s.Verifier = failToPassVerifier("go", "test", "-race", "./...")
	_ = store.RecordBaseline(s, "hash1")
	_, _ = store.SaveScar(s)

	// 1. Test XML formatting
	xmlOut, err := store.InjectGuardrails(InjectOptions{
		Files:        []string{"pkg/auth.go"},
		BudgetTokens: 500,
		Format:       "xml",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(xmlOut, "<git_scars") || !strings.Contains(xmlOut, s.ID) {
		t.Errorf("expected XML snippet with scar %s, got: %s", s.ID, xmlOut)
	}
	if !strings.Contains(xmlOut, "Never log raw bearer token") {
		t.Errorf("expected lesson in XML, got: %s", xmlOut)
	}

	// 2. Test Markdown formatting
	mdOut, err := store.InjectGuardrails(InjectOptions{
		Files:        []string{"pkg/auth.go"},
		BudgetTokens: 500,
		Format:       "markdown",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(mdOut, "Repository Scars & Guardrails") || !strings.Contains(mdOut, s.ID) {
		t.Errorf("expected Markdown snippet with scar %s, got: %s", s.ID, mdOut)
	}

	// 3. Test tight budget truncation
	tightOut, err := store.InjectGuardrails(InjectOptions{
		Files:        []string{"pkg/auth.go"},
		BudgetTokens: 5, // very tiny budget that cannot fit full scar
		Format:       "xml",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = tightOut // budget is handled safely without crashing

	// 4. Test clean abstention
	absOut, err := store.InjectGuardrails(InjectOptions{
		Files:        []string{"pkg/unrelated.go"},
		BudgetTokens: 500,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if absOut != "" {
		t.Errorf("expected empty string on abstention, got: %s", absOut)
	}
}

func TestInjectGuardrailsActionablePatterns(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)
	_ = store.Init()

	anchorPath := filepath.Join(tmpDir, "pkg", "worker.go")
	_ = os.MkdirAll(filepath.Dir(anchorPath), 0755)
	_ = os.WriteFile(anchorPath, []byte("package worker\n"), 0644)

	s, _, _ := store.CreateDraft("Worker deadlock", "Buffer channels properly", []string{"pkg/worker.go"})
	s.Lifecycle.State = LifecycleActive
	s.Review.Status = ReviewApproved
	s.Severity = SeverityHigh
	s.FixHint = "Buffer worker channel to worker count"
	s.BadPattern = "ch := make(chan Task)"
	s.GoodPattern = "ch := make(chan Task, workers)"
	s.Verifier = failToPassVerifier("go", "test", "-race", "./...")
	_ = store.RecordBaseline(s, s.Verifier.CommandHash)
	_, _ = store.SaveScar(s)

	xmlOut, err := store.InjectGuardrails(InjectOptions{
		Files:        []string{"pkg/worker.go"},
		BudgetTokens: 500,
		Format:       "xml",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(xmlOut, `severity="high"`) {
		t.Errorf("expected severity attribute in XML: %s", xmlOut)
	}
	if !strings.Contains(xmlOut, "<dont>ch := make(chan Task)</dont>") {
		t.Errorf("expected <dont> in XML: %s", xmlOut)
	}
	if !strings.Contains(xmlOut, "<do>ch := make(chan Task, workers)</do>") {
		t.Errorf("expected <do> in XML: %s", xmlOut)
	}
	if !strings.Contains(xmlOut, "<verify>go test -race ./...</verify>") {
		t.Errorf("expected <verify> in XML: %s", xmlOut)
	}
	if !strings.Contains(xmlOut, "<fix_hint>Buffer worker channel to worker count</fix_hint>") {
		t.Errorf("expected <fix_hint> in XML: %s", xmlOut)
	}

	// Also check markdown format
	mdOut, err := store.InjectGuardrails(InjectOptions{
		Files:        []string{"pkg/worker.go"},
		BudgetTokens: 500,
		Format:       "markdown",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(mdOut, "*Don't*: `ch := make(chan Task)`") {
		t.Errorf("expected Don't in markdown: %s", mdOut)
	}
	if !strings.Contains(mdOut, "*Do*: `ch := make(chan Task, workers)`") {
		t.Errorf("expected Do in markdown: %s", mdOut)
	}
}
