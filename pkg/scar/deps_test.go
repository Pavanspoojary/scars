package scar

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGoModDependencies(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitscars-deps-gomod-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	goModContent := `module example.com/my/project

go 1.22.0

require (
	github.com/google/uuid v1.6.0
	golang.org/x/crypto v0.21.0 // indirect
)

require github.com/stretchr/testify v1.9.0
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed writing go.mod: %v", err)
	}

	deps, err := ParseGoModDependencies(tmpDir)
	if err != nil {
		t.Fatalf("ParseGoModDependencies failed: %v", err)
	}

	if deps["github.com/google/uuid"] != "v1.6.0" {
		t.Errorf("expected uuid v1.6.0, got %q", deps["github.com/google/uuid"])
	}
	if deps["golang.org/x/crypto"] != "v0.21.0" {
		t.Errorf("expected crypto v0.21.0, got %q", deps["golang.org/x/crypto"])
	}
	if deps["github.com/stretchr/testify"] != "v1.9.0" {
		t.Errorf("expected testify v1.9.0, got %q", deps["github.com/stretchr/testify"])
	}
}

func TestParseNpmLockDependencies(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitscars-deps-npmlock-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	npmLockContent := `{
  "name": "sample-app",
  "version": "1.0.0",
  "lockfileVersion": 3,
  "packages": {
    "": {
      "name": "sample-app",
      "version": "1.0.0"
    },
    "node_modules/lodash": {
      "version": "4.17.21"
    },
    "node_modules/axios": {
      "version": "1.6.8"
    }
  }
}`
	if err := os.WriteFile(filepath.Join(tmpDir, "package-lock.json"), []byte(npmLockContent), 0644); err != nil {
		t.Fatalf("failed writing package-lock.json: %v", err)
	}

	deps, err := ParseNpmLockDependencies(tmpDir)
	if err != nil {
		t.Fatalf("ParseNpmLockDependencies failed: %v", err)
	}

	if deps["lodash"] != "4.17.21" {
		t.Errorf("expected lodash 4.17.21, got %q", deps["lodash"])
	}
	if deps["axios"] != "1.6.8" {
		t.Errorf("expected axios 1.6.8, got %q", deps["axios"])
	}
}

func TestMatchDependency(t *testing.T) {
	installed := map[string]string{
		"github.com/google/uuid": "v1.6.0",
		"lodash":                 "4.17.21",
	}

	// 1. Exact match with v prefix
	if !MatchDependency(installed, DependencyAnchor{Name: "github.com/google/uuid", Version: "1.6.0"}) {
		t.Error("expected match for uuid 1.6.0 against v1.6.0")
	}

	// 2. Prefix match
	if !MatchDependency(installed, DependencyAnchor{Name: "lodash", Version: "4.17"}) {
		t.Error("expected match for lodash 4.17 against 4.17.21")
	}

	// 3. Mismatch version
	if MatchDependency(installed, DependencyAnchor{Name: "lodash", Version: "3.10.0"}) {
		t.Error("expected mismatch for lodash 3.10.0 against 4.17.21")
	}

	// 4. Missing package
	if MatchDependency(installed, DependencyAnchor{Name: "react", Version: "18.0.0"}) {
		t.Error("expected mismatch for uninstalled react package")
	}
}

func TestDependencyAnchorScoringInRecall(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "gitscars-deps-recall-*")
	defer os.RemoveAll(tmpDir)

	store := NewStore(tmpDir)
	_ = store.Init()

	// Seed scar anchored to lodash dependency
	s, _, _ := store.CreateDraft("Lodash Prototype Pollution", "Upgrade lodash to >= 4.17.21", []string{"package.json"})
	s.Anchors.Dependencies = []DependencyAnchor{
		{Name: "lodash", Version: "4.17.21"},
	}
	s.Lifecycle.State = LifecycleActive
	s.Review.Status = ReviewApproved
	s.Verifier = failToPassVerifier("go", "test", "-race", "./...")

	_ = os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte("{}\n"), 0644)
	_ = store.RecordBaseline(s, "hash-pkg")
	_, _ = store.SaveScar(s)

	// Simulate installed dependencies in query
	q := QueryOptions{
		InstalledDependencies: map[string]string{
			"lodash": "4.17.21",
		},
	}

	scored := ScoreScarItem(s, q)
	if scored.Score < 3 {
		t.Errorf("expected dependency match score >= 3, got %d", scored.Score)
	}

	matchedDep := false
	for _, m := range scored.Matches {
		if m == "dependency(+3)" {
			matchedDep = true
		}
	}
	if !matchedDep {
		t.Errorf("expected matches to contain dependency(+3), got: %v", scored.Matches)
	}
}

func TestParseCargoAndPoetryLockDependencies(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitscars-deps-cargopoetry-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cargoContent := `version = 3

[[package]]
name = "serde"
version = "1.0.197"
source = "registry+https://github.com/rust-lang/crates.io-index"

[[package]]
name = "tokio"
version = "1.37.0"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "Cargo.lock"), []byte(cargoContent), 0644); err != nil {
		t.Fatalf("failed writing Cargo.lock: %v", err)
	}

	deps, err := ParseCargoLockDependencies(tmpDir)
	if err != nil {
		t.Fatalf("ParseCargoLockDependencies failed: %v", err)
	}
	if deps["serde"] != "1.0.197" {
		t.Errorf("expected serde 1.0.197, got %q", deps["serde"])
	}
	if deps["tokio"] != "1.37.0" {
		t.Errorf("expected tokio 1.37.0, got %q", deps["tokio"])
	}

	poetryContent := `[[package]]
name = "httpx"
version = "0.27.0"
description = "A next-generation HTTP client for Python."

[[package]]
name = "fastapi"
version = "0.110.0"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "poetry.lock"), []byte(poetryContent), 0644); err != nil {
		t.Fatalf("failed writing poetry.lock: %v", err)
	}

	pDeps, err := ParsePoetryLockDependencies(tmpDir)
	if err != nil {
		t.Fatalf("ParsePoetryLockDependencies failed: %v", err)
	}
	if pDeps["httpx"] != "0.27.0" {
		t.Errorf("expected httpx 0.27.0, got %q", pDeps["httpx"])
	}
	if pDeps["fastapi"] != "0.110.0" {
		t.Errorf("expected fastapi 0.110.0, got %q", pDeps["fastapi"])
	}
}

func TestParsePythonRequirements(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitscars-deps-pyreqs-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	reqsContent := `# Core dependencies
flask==3.0.2
pydantic>=2.6.0
gunicorn==21.2.0 # production server
-r base.txt
`
	if err := os.WriteFile(filepath.Join(tmpDir, "requirements.txt"), []byte(reqsContent), 0644); err != nil {
		t.Fatalf("failed writing requirements.txt: %v", err)
	}

	deps, err := ParsePythonRequirements(tmpDir)
	if err != nil {
		t.Fatalf("ParsePythonRequirements failed: %v", err)
	}
	if deps["flask"] != "3.0.2" {
		t.Errorf("expected flask 3.0.2, got %q", deps["flask"])
	}
	if deps["pydantic"] != "2.6.0" {
		t.Errorf("expected pydantic 2.6.0, got %q", deps["pydantic"])
	}
	if deps["gunicorn"] != "21.2.0" {
		t.Errorf("expected gunicorn 21.2.0, got %q", deps["gunicorn"])
	}
}
