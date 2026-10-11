package scar

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractGoSymbolHash(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "worker.go")

	code := `package worker

type WorkerConfig struct {
	MaxWorkers int
}

func (c *WorkerConfig) Concurrency() int {
	return c.MaxWorkers
}

func ProcessTask(id int) bool {
	return id > 0
}
`
	if err := os.WriteFile(goFile, []byte(code), 0644); err != nil {
		t.Fatalf("failed writing test file: %v", err)
	}

	// 1. Function symbol
	fnHash, err := ExtractGoSymbolHash(goFile, "ProcessTask")
	if err != nil {
		t.Fatalf("failed extracting ProcessTask: %v", err)
	}
	if fnHash == "" {
		t.Errorf("expected non-empty hash for ProcessTask")
	}

	// 2. Method symbol with receiver
	methHash, err := ExtractGoSymbolHash(goFile, "WorkerConfig.Concurrency")
	if err != nil {
		t.Fatalf("failed extracting WorkerConfig.Concurrency: %v", err)
	}
	if methHash == "" {
		t.Errorf("expected non-empty hash for WorkerConfig.Concurrency")
	}

	// 3. Type symbol
	typeHash, err := ExtractGoSymbolHash(goFile, "WorkerConfig")
	if err != nil {
		t.Fatalf("failed extracting WorkerConfig type: %v", err)
	}
	if typeHash == "" {
		t.Errorf("expected non-empty hash for WorkerConfig type")
	}

	// 4. Missing symbol
	_, err = ExtractGoSymbolHash(goFile, "NonExistent")
	if err == nil {
		t.Errorf("expected error for non-existent symbol")
	}
}

func TestASTSymbolDriftIsolation(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)
	_ = store.Init()

	pkgDir := filepath.Join(tmpDir, "pkg")
	_ = os.MkdirAll(pkgDir, 0755)
	targetFile := filepath.Join(pkgDir, "service.go")

	initialCode := `package pkg

func CriticalOperation() int {
	return 42
}

func UnrelatedHelper() string {
	return "version-1"
}
`
	_ = os.WriteFile(targetFile, []byte(initialCode), 0644)

	scar, _, err := store.CreateDraft("Critical Failure", "Must return 42", []string{"pkg/service.go"})
	if err != nil {
		t.Fatalf("failed creating draft: %v", err)
	}
	scar.Anchors.Symbols = []string{"CriticalOperation"}
	_ = store.RecordBaseline(scar, "cmd-hash")
	_, _ = store.SaveScar(scar)

	// Baseline should have recorded both file hash and symbol hash
	if len(scar.Baseline.SymbolHashes) == 0 {
		t.Fatalf("expected symbol_hashes to be recorded in baseline")
	}
	if scar.Baseline.SymbolHashes["CriticalOperation"] == "" {
		t.Errorf("expected symbol hash for CriticalOperation")
	}

	// Baseline freshness should be current
	if f := store.ComputeFreshness(scar); f != FreshnessCurrent {
		t.Errorf("expected initial freshness current, got %s", f)
	}

	// Modification 1: Edit UnrelatedHelper only.
	// File hash changes, but CriticalOperation AST is untouched.
	modifiedUnrelated := `package pkg

func CriticalOperation() int {
	return 42
}

func UnrelatedHelper() string {
	// Added a comment and changed unrelated return
	return "version-2-unrelated"
}
`
	_ = os.WriteFile(targetFile, []byte(modifiedUnrelated), 0644)

	freshnessAfterUnrelated := store.ComputeFreshness(scar)
	if freshnessAfterUnrelated != FreshnessCurrent {
		t.Errorf("expected freshness to remain current due to AST symbol isolation, got: %s", freshnessAfterUnrelated)
	}

	// Modification 2: Edit CriticalOperation itself.
	modifiedTarget := `package pkg

func CriticalOperation() int {
	return 99 // Target symbol modified!
}

func UnrelatedHelper() string {
	return "version-2-unrelated"
}
`
	_ = os.WriteFile(targetFile, []byte(modifiedTarget), 0644)

	freshnessAfterTarget := store.ComputeFreshness(scar)
	if freshnessAfterTarget != FreshnessChanged {
		t.Errorf("expected freshness to change after modifying target symbol, got: %s", freshnessAfterTarget)
	}
}
