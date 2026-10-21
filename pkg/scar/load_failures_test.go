package scar

import (
	"os"
	"path/filepath"
	"testing"
)

const validScarDoc = `---
schema_version: "1"
id: "valid-scar"
title: "Valid scar"
lesson: "A lesson"
lifecycle:
  state: "draft"
---
`

const malformedScarDoc = `---
schema_version: "1"
id: "malformed-scar"
title: "Unterminated frontmatter
lesson: "A lesson"
`

func writeScarFile(t *testing.T, dir, name, content string) {
	t.Helper()
	scarsDir := filepath.Join(dir, DefaultStoreDir, ScarsDirName)
	if err := os.MkdirAll(scarsDir, 0755); err != nil {
		t.Fatalf("failed creating scars dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scarsDir, name), []byte(content), 0644); err != nil {
		t.Fatalf("failed writing scar %s: %v", name, err)
	}
}

func TestListScarsDetailedReportsLoadFailures(t *testing.T) {
	dir := t.TempDir()
	writeScarFile(t, dir, "valid.md", validScarDoc)
	writeScarFile(t, dir, "malformed.md", malformedScarDoc)

	store := NewStore(dir)
	scars, failures, err := store.ListScarsDetailed()
	if err != nil {
		t.Fatalf("unexpected error listing scars: %v", err)
	}
	if len(scars) != 1 {
		t.Fatalf("expected 1 loadable scar, got %d", len(scars))
	}
	if len(failures) != 1 {
		t.Fatalf("expected 1 load failure, got %d (%+v)", len(failures), failures)
	}
	if failures[0].ID != "malformed" {
		t.Errorf("expected failure id 'malformed', got %q", failures[0].ID)
	}
	if failures[0].Error == "" {
		t.Error("expected a non-empty failure reason")
	}
}

func TestCheckReportsUnloadableScars(t *testing.T) {
	dir := t.TempDir()
	writeScarFile(t, dir, "valid.md", validScarDoc)
	writeScarFile(t, dir, "malformed.md", malformedScarDoc)

	store := NewStore(dir)
	res, err := store.RunReadOnlyCheck("")
	if err != nil {
		t.Fatalf("read-only check failed: %v", err)
	}
	if res.ExitCode != 1 {
		t.Fatalf("expected exit code 1 for an unloadable scar, got %d", res.ExitCode)
	}
	if len(res.InvalidScars) != 1 {
		t.Fatalf("expected 1 invalid scar reported, got %d (%v)", len(res.InvalidScars), res.InvalidScars)
	}
	if res.TotalScars != 2 {
		t.Errorf("expected total scars to count both files, got %d", res.TotalScars)
	}
}

func TestDoctorReportsUnloadableScars(t *testing.T) {
	dir := t.TempDir()
	writeScarFile(t, dir, "valid.md", validScarDoc)
	writeScarFile(t, dir, "malformed.md", malformedScarDoc)

	store := NewStore(dir)
	report := store.RunDoctor()
	if report.Healthy {
		t.Error("expected doctor to report an unhealthy repository")
	}
	var found bool
	for _, c := range report.Checks {
		if c.Name == "Scar Loadability" && c.Status == "FAIL" {
			found = true
		}
	}
	if !found {
		t.Error("expected a failing 'Scar Loadability' diagnostic")
	}
}
