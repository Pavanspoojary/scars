package scar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, dir, content string) {
	t.Helper()
	storeDir := filepath.Join(dir, DefaultStoreDir)
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		t.Fatalf("failed creating store dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(storeDir, ConfigFileName), []byte(content), 0644); err != nil {
		t.Fatalf("failed writing config: %v", err)
	}
}

func TestLoadConfigUsesDefaultsWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	cfg := store.LoadConfig()
	if cfg.AbstainThreshold != 3 {
		t.Errorf("expected default abstain threshold 3, got %d", cfg.AbstainThreshold)
	}
	if cfg.TopK != 3 {
		t.Errorf("expected default top_k 3, got %d", cfg.TopK)
	}
}

func TestLoadConfigReadsConfiguredValues(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "# comment\nschema_version: \"1\"\nabstain_threshold: 5\ntop_k: 7\n")

	store := NewStore(dir)
	cfg := store.LoadConfig()
	if cfg.AbstainThreshold != 5 {
		t.Errorf("expected abstain threshold 5, got %d", cfg.AbstainThreshold)
	}
	if cfg.TopK != 7 {
		t.Errorf("expected top_k 7, got %d", cfg.TopK)
	}
}

func TestLoadConfigIgnoresUnusableValues(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "abstain_threshold: not-a-number\ntop_k: 0\n")

	store := NewStore(dir)
	cfg := store.LoadConfig()
	if cfg.AbstainThreshold != 3 {
		t.Errorf("expected the default abstain threshold for an unusable value, got %d", cfg.AbstainThreshold)
	}
	if cfg.TopK != 3 {
		t.Errorf("expected the default top_k for an unusable value, got %d", cfg.TopK)
	}
}

func TestRecallHonoursConfiguredThresholds(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	if err := store.Init(); err != nil {
		t.Fatalf("failed initializing store: %v", err)
	}

	// The dependency anchor scores exactly 3, which distinguishes a threshold
	// of 3 from a threshold of 4.
	gomod := "module example.com/demo\n\ngo 1.26\n\nrequire example.com/dep v1.0.0\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644); err != nil {
		t.Fatalf("failed writing go.mod: %v", err)
	}

	anchor := filepath.Join(dir, "pkg", "worker.go")
	if err := os.MkdirAll(filepath.Dir(anchor), 0755); err != nil {
		t.Fatalf("failed creating anchor dir: %v", err)
	}
	if err := os.WriteFile(anchor, []byte("package worker\n"), 0644); err != nil {
		t.Fatalf("failed writing anchor: %v", err)
	}

	s, _, err := store.CreateDraft("Configured Threshold", "Lesson", []string{"pkg/worker.go"})
	if err != nil {
		t.Fatalf("failed creating draft: %v", err)
	}
	s.Anchors.Dependencies = []DependencyAnchor{{Name: "example.com/dep", Version: "1.0.0"}}
	s.Lifecycle.State = LifecycleActive
	s.Review.Status = ReviewApproved
	s.Verifier = failToPassVerifier("go", "test", "./...")
	if err := store.RecordBaseline(s, s.Verifier.CommandHash); err != nil {
		t.Fatalf("failed recording baseline: %v", err)
	}
	if _, err := store.SaveScar(s); err != nil {
		t.Fatalf("failed saving scar: %v", err)
	}

	// A threshold of 4 must abstain where the default of 3 would retrieve.
	candidates, err := store.Recall(QueryOptions{Threshold: 4, TopK: 3})
	if err != nil {
		t.Fatalf("recall failed: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected an explicit threshold of 4 to abstain, got %d results", len(candidates))
	}

	// An unset threshold falls back to the configured value.
	writeConfig(t, dir, "abstain_threshold: 4\ntop_k: 3\n")
	candidates, err = store.Recall(QueryOptions{})
	if err != nil {
		t.Fatalf("recall failed: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected the configured threshold of 4 to abstain, got %d results", len(candidates))
	}

	writeConfig(t, dir, "abstain_threshold: 3\ntop_k: 3\n")
	candidates, err = store.Recall(QueryOptions{})
	if err != nil {
		t.Fatalf("recall failed: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected the configured threshold of 3 to retrieve the scar, got %d results", len(candidates))
	}
}

func findIssue(issues []ConfigIssue, key string) (ConfigIssue, bool) {
	for _, issue := range issues {
		if issue.Key == key {
			return issue, true
		}
	}
	return ConfigIssue{}, false
}

// TestLoadConfigReportsUnrecognizedKeys verifies that a misspelled key is
// reported instead of being dropped without comment.
func TestLoadConfigReportsUnrecognizedKeys(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "schema_version: \"1\"\nabstain_treshold: 5\n")

	store := NewStore(dir)
	cfg, issues := store.LoadConfigWithIssues()
	if cfg.AbstainThreshold != 3 {
		t.Errorf("expected the default abstain threshold for an unrecognized key, got %d", cfg.AbstainThreshold)
	}
	issue, ok := findIssue(issues, "abstain_treshold")
	if !ok {
		t.Fatalf("expected an issue for the unrecognized key, got %+v", issues)
	}
	if !strings.Contains(issue.Reason, "line 2") {
		t.Errorf("expected the diagnostic to point at line 2, got %q", issue.Reason)
	}
}

// TestLoadConfigIgnoresAFileThatDeclaresAnUnsupportedVersion verifies that a
// configuration written for a newer reader contract is set aside whole rather
// than partially applied.
func TestLoadConfigIgnoresAFileThatDeclaresAnUnsupportedVersion(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "schema_version: \"2\"\nabstain_threshold: 5\n")

	store := NewStore(dir)
	cfg, issues := store.LoadConfigWithIssues()
	if cfg.AbstainThreshold != 3 {
		t.Errorf("expected the whole file to be ignored, got threshold %d", cfg.AbstainThreshold)
	}
	issue, ok := findIssue(issues, "schema_version")
	if !ok {
		t.Fatalf("expected an issue for the declared version, got %+v", issues)
	}
	if !strings.Contains(issue.Reason, "ignored entirely") {
		t.Errorf("expected the diagnostic to state the file was ignored, got %q", issue.Reason)
	}
}

// TestLoadConfigReportsDuplicateKeys verifies that a repeated key reports
// every position while the last appearance stays in force.
func TestLoadConfigReportsDuplicateKeys(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "abstain_threshold: 5\nabstain_threshold: 4\n")

	store := NewStore(dir)
	cfg, issues := store.LoadConfigWithIssues()
	if cfg.AbstainThreshold != 4 {
		t.Errorf("expected the last appearance to be in force, got %d", cfg.AbstainThreshold)
	}
	issue, ok := findIssue(issues, "abstain_threshold")
	if !ok {
		t.Fatalf("expected an issue for the repeated key, got %+v", issues)
	}
	if !strings.Contains(issue.Reason, "line(s) 1, 2") {
		t.Errorf("expected the diagnostic to list both lines, got %q", issue.Reason)
	}
}

// TestLoadConfigAcceptsInlineComments verifies that a trailing comment does
// not render an otherwise valid value unusable.
func TestLoadConfigAcceptsInlineComments(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "abstain_threshold: 5 # keep 5 for the benchmark\n")

	store := NewStore(dir)
	cfg, issues := store.LoadConfigWithIssues()
	if cfg.AbstainThreshold != 5 {
		t.Errorf("expected a value followed by a comment to be honoured, got %d", cfg.AbstainThreshold)
	}
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %+v", issues)
	}
}

// TestLoadConfigReportsUnusableValues documents that an unusable value stays
// usable-in-error: the defaults apply and the mistake is reported.
func TestLoadConfigReportsUnusableValues(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "abstain_threshold: not-a-number\ntop_k: 0\n")

	store := NewStore(dir)
	cfg, issues := store.LoadConfigWithIssues()
	if cfg.AbstainThreshold != 3 || cfg.TopK != 3 {
		t.Errorf("expected defaults for unusable values, got %+v", cfg)
	}
	if len(issues) != 2 {
		t.Fatalf("expected an issue per unusable value, got %+v", issues)
	}
}

// TestDoctorReportsConfigurationIssues verifies that the configuration probe
// surfaces an unusable entry and that a missing file is quiet.
func TestDoctorReportsConfigurationIssues(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatalf("failed creating .git: %v", err)
	}
	store := NewStore(dir)
	if err := store.Init(); err != nil {
		t.Fatalf("failed initializing store: %v", err)
	}

	report := store.RunDoctor()
	var probe *DoctorCheck
	for i := range report.Checks {
		if report.Checks[i].Name == "Configuration" {
			probe = &report.Checks[i]
			break
		}
	}
	if probe == nil {
		t.Fatal("expected a Configuration check in the report")
	}
	if probe.Status != "OK" {
		t.Errorf("expected a missing configuration file to be quiet, got %s: %s", probe.Status, probe.Message)
	}

	writeConfig(t, dir, "abstain_treshold: 5\n")
	report = store.RunDoctor()
	for _, c := range report.Checks {
		if c.Name == "Configuration" && c.Status == "WARN" {
			return
		}
	}
	t.Errorf("expected a Configuration warning for the unrecognized key, got %+v", report.Checks)
}
