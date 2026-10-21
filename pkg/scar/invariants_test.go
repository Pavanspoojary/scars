package scar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewDraftInvariants(t *testing.T) {
	draft, err := NewDraft("test-slug", "Test Failure Mode", "Always check nil pointers", []string{"pkg/foo/bar.go"})
	if err != nil {
		t.Fatalf("unexpected error creating draft: %v", err)
	}

	if draft.Lifecycle.State != LifecycleDraft {
		t.Errorf("expected lifecycle state %s, got %s", LifecycleDraft, draft.Lifecycle.State)
	}
	if draft.Review.Status != ReviewUnreviewed {
		t.Errorf("expected review status %s, got %s", ReviewUnreviewed, draft.Review.Status)
	}
	if draft.Verifier.Result != VerificationNotAttempted {
		t.Errorf("expected verifier result %s, got %s", VerificationNotAttempted, draft.Verifier.Result)
	}
	if err := draft.ValidateInvariants(); err != nil {
		t.Fatalf("valid draft failed invariant check: %v", err)
	}
}

func TestActiveStateInvariants(t *testing.T) {
	s := &Scar{
		SchemaVersion: CurrentSchemaVersion,
		ID:            "scar-test-123",
		Title:         "Sample Scar",
		Lesson:        "Sample Lesson",
		Lifecycle:     Lifecycle{State: LifecycleActive},
		Review:        Review{Status: ReviewUnreviewed},
		Verifier:      Verifier{Result: VerificationNotAttempted},
	}

	// 1. Cannot be active if unreviewed
	if err := s.ValidateInvariants(); err == nil {
		t.Error("expected error: active scar cannot be unreviewed")
	}

	// 2. Cannot be active if rejected
	s.Review.Status = ReviewRejected
	if err := s.ValidateInvariants(); err == nil {
		t.Error("expected error: active scar cannot be rejected")
	}

	// 3. Cannot be active if verification failed
	s.Review.Status = ReviewApproved
	s.Verifier.Result = VerificationFailed
	if err := s.ValidateInvariants(); err == nil {
		t.Error("expected error: active scar cannot have failed verification")
	}

	// 4. Exception requires documented rationale
	s.Verifier.Result = VerificationException
	s.Review.Rationale = ""
	if err := s.ValidateInvariants(); err == nil {
		t.Error("expected error: active scar with exception must have documented rationale")
	}

	// 5. Valid exception with rationale passes
	s.Review.Rationale = "Pre-fix build environment unavailable; reviewed manually"
	if err := s.ValidateInvariants(); err != nil {
		t.Errorf("valid exception should pass invariants: %v", err)
	}

	// 6. Valid fail_to_pass passes
	s.Verifier.Result = VerificationFailToPass
	if err := s.ValidateInvariants(); err != nil {
		t.Errorf("valid fail_to_pass should pass invariants: %v", err)
	}
}

func TestUnsupportedSchemaVersion(t *testing.T) {
	doc := []byte(`---
schema_version: "99"
id: "test-001"
title: "Future scar"
lifecycle:
  state: "draft"
review:
  status: "unreviewed"
verifier:
  result: "not_attempted"
---
`)
	_, err := ParseFrontmatter(doc)
	if err == nil {
		t.Fatal("expected error parsing unsupported schema version")
	}
	if !strings.Contains(err.Error(), "unsupported schema_version") {
		t.Errorf("expected error to mention unsupported schema_version, got: %v", err)
	}
}

func TestRejectYAMLAliases(t *testing.T) {
	doc := []byte(`---
schema_version: "1"
id: "alias-test"
title: &anchor "Test Anchor"
lesson: *anchor
lifecycle:
  state: "draft"
review:
  status: "unreviewed"
verifier:
  result: "not_attempted"
---
`)
	_, err := ParseFrontmatter(doc)
	if err == nil {
		t.Fatal("expected error rejecting YAML alias reference")
	}
	if !strings.Contains(err.Error(), "rejected alias") {
		t.Errorf("expected error to mention rejected alias, got: %v", err)
	}
}

func TestParseFrontmatterSeparatesParsingFromValidation(t *testing.T) {
	doc := []byte(`---
schema_version: "1"
id: "invariant-violation"
title: "Unverified active scar"
lesson: "Claimed active without human approval or verification"
lifecycle:
  state: "active"
---
`)
	scar, err := ParseFrontmatter(doc)
	if err != nil {
		t.Fatalf("expected parse to succeed so the violation remains reportable, got: %v", err)
	}
	if err := scar.ValidateInvariants(); err == nil {
		t.Fatal("expected invariant validation to reject an active scar without approval")
	}
}

func TestCheckReportsInvariantViolations(t *testing.T) {
	dir := t.TempDir()
	scarsDir := filepath.Join(dir, DefaultStoreDir, ScarsDirName)
	if err := os.MkdirAll(scarsDir, 0755); err != nil {
		t.Fatalf("failed creating scars dir: %v", err)
	}
	doc := `---
schema_version: "1"
id: "invariant-violation"
title: "Unverified active scar"
lesson: "Claimed active without human approval or verification"
lifecycle:
  state: "active"
---
`
	if err := os.WriteFile(filepath.Join(scarsDir, "invariant-violation.md"), []byte(doc), 0644); err != nil {
		t.Fatalf("failed writing scar: %v", err)
	}

	store := NewStore(dir)
	res, err := store.RunReadOnlyCheck("")
	if err != nil {
		t.Fatalf("read-only check failed: %v", err)
	}
	if res.ExitCode != 1 {
		t.Fatalf("expected exit code 1 for an invariant violation, got %d", res.ExitCode)
	}
	if len(res.InvalidScars) != 1 {
		t.Fatalf("expected 1 invalid scar reported, got %d (%v)", len(res.InvalidScars), res.InvalidScars)
	}
}

func TestStoreOperationsWithoutGit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitscars-store-test-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := NewStore(tmpDir)
	if err := store.Init(); err != nil {
		t.Fatalf("failed init: %v", err)
	}

	// Create draft
	draft, filePath, err := store.CreateDraft("Connection Leak in Worker", "Always close sockets", []string{"worker.go"})
	if err != nil {
		t.Fatalf("failed creating draft: %v", err)
	}
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("draft file was not written to disk at %s", filePath)
	}

	// List scars
	scars, err := store.ListScars()
	if err != nil {
		t.Fatalf("failed listing scars: %v", err)
	}
	if len(scars) != 1 {
		t.Fatalf("expected 1 scar, found %d", len(scars))
	}
	if scars[0].Scar.ID != draft.ID {
		t.Errorf("expected ID %s, got %s", draft.ID, scars[0].Scar.ID)
	}
	if scars[0].Freshness != FreshnessUnavailable {
		t.Errorf("expected unbaselined scar freshness to be unavailable, got %s", scars[0].Freshness)
	}

	// Record baseline
	anchorPath := filepath.Join(tmpDir, "worker.go")
	if err := os.WriteFile(anchorPath, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("failed writing anchor file: %v", err)
	}
	if err := store.RecordBaseline(draft, "cmd-hash-123"); err != nil {
		t.Fatalf("failed recording baseline: %v", err)
	}
	if draft.Baseline.RecordedAt == "" {
		t.Error("expected baseline to record the time it was captured")
	}
	if _, err := store.SaveScar(draft); err != nil {
		t.Fatalf("failed saving scar: %v", err)
	}

	// Freshness should now be Current
	freshness := store.ComputeFreshness(draft)
	if freshness != FreshnessCurrent {
		t.Errorf("expected freshness %s, got %s", FreshnessCurrent, freshness)
	}

	// Modify anchor file -> Freshness should dynamically become Changed
	if err := os.WriteFile(anchorPath, []byte("package main // modified\n"), 0644); err != nil {
		t.Fatalf("failed modifying anchor file: %v", err)
	}
	freshnessAfterEdit := store.ComputeFreshness(draft)
	if freshnessAfterEdit != FreshnessChanged {
		t.Errorf("expected freshness %s, got %s", FreshnessChanged, freshnessAfterEdit)
	}

	// Reaffirm scar
	reaffirmed, err := store.Reaffirm(draft.ID, "Alice Reviewer", "Verified change is safe")
	if err != nil {
		t.Fatalf("failed reaffirming scar: %v", err)
	}
	freshnessAfterReaffirm := store.ComputeFreshness(reaffirmed)
	if freshnessAfterReaffirm != FreshnessCurrent {
		t.Errorf("expected freshness %s after reaffirmation, got %s", FreshnessCurrent, freshnessAfterReaffirm)
	}
}
