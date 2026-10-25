package scar

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// seedBenchStore builds a synthetic repository with n active, baselined scars
// spread over several anchor directories. Every scar passes ValidateInvariants,
// so recall exercises the complete fast path: index load, freshness checks,
// per-candidate LoadScar, anchor hashing, and scoring.
func seedBenchStore(tb testing.TB, n int) *Store {
	tb.Helper()

	dir := tb.TempDir()
	s := NewStore(dir)
	if err := s.Init(); err != nil {
		tb.Fatalf("init store: %v", err)
	}

	const anchorDirs = 10
	for i := 0; i < anchorDirs; i++ {
		path := filepath.Join(dir, "pkg", fmt.Sprintf("mod%d", i), "code.go")
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			tb.Fatalf("mkdir anchor: %v", err)
		}
		body := fmt.Sprintf("package mod%d\n\nfunc Target%d() int { return %d }\n", i, i, i)
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			tb.Fatalf("write anchor: %v", err)
		}
	}

	for i := 0; i < n; i++ {
		file := fmt.Sprintf("pkg/mod%d/code.go", i%anchorDirs)
		scar, err := NewDraft(
			fmt.Sprintf("bench-scar-%d", i),
			fmt.Sprintf("bench scar %d", i),
			"lesson text describing the regression the scar remembers for the benchmark corpus",
			[]string{file},
		)
		if err != nil {
			tb.Fatalf("new draft: %v", err)
		}
		scar.Severity = SeverityHigh
		scar.Anchors.Symbols = []string{fmt.Sprintf("Target%d", i%anchorDirs)}
		scar.Anchors.Commands = []string{"go test ./..."}
		scar.Verifier.Argv = []string{"go", "test", "./..."}
		scar.Verifier.Result = VerificationFailToPass
		scar.Verifier.CommandHash = ComputeCommandHash(scar.Verifier.Argv, scar.Verifier.WorkingDir, scar.Verifier.EnvAllowlist)
		scar.Review.Status = ReviewApproved
		scar.Lifecycle.State = LifecycleActive
		if err := scar.ValidateInvariants(); err != nil {
			tb.Fatalf("seed scar invariants: %v", err)
		}
		// RecordBaseline mutates the struct rather than the file, so it must
		// run before SaveScar for the on-disk record to carry the baseline.
		if err := s.RecordBaseline(scar, scar.Verifier.CommandHash); err != nil {
			tb.Fatalf("record baseline: %v", err)
		}
		if _, err := s.SaveScar(scar); err != nil {
			tb.Fatalf("save scar: %v", err)
		}
	}

	if _, err := s.RebuildIndex(); err != nil {
		tb.Fatalf("rebuild index: %v", err)
	}
	return s
}

// benchRecallQuery mirrors a realistic injection query: the agent is editing
// one anchored file and naming one symbol, with dependencies supplied so the
// parser cost is excluded from the measurement.
func benchRecallQuery() QueryOptions {
	return QueryOptions{
		Files:                 []string{"pkg/mod3/code.go"},
		Symbols:               []string{"Target3"},
		Commands:              []string{"go test ./..."},
		InstalledDependencies: map[string]string{},
		Threshold:             3,
		TopK:                  3,
	}
}

// BenchmarkRecall measures the full recall path over a 200-scar corpus.
func BenchmarkRecall(b *testing.B) {
	store := seedBenchStore(b, 200)
	query := benchRecallQuery()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scored, err := store.Recall(query)
		if err != nil {
			b.Fatalf("recall: %v", err)
		}
		if len(scored) == 0 {
			b.Fatal("expected recall to return candidates")
		}
	}
}

// BenchmarkRecallUnfilteredDeps measures recall when the repository
// dependency set has to be parsed from disk on every call, as happens for
// callers that do not supply InstalledDependencies.
func BenchmarkRecallUnfilteredDeps(b *testing.B) {
	store := seedBenchStore(b, 200)
	query := benchRecallQuery()
	query.InstalledDependencies = nil

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.Recall(query); err != nil {
			b.Fatalf("recall: %v", err)
		}
	}
}

// BenchmarkRecallStaleIndex measures the fallback path where the index cache
// is unusable and every scar is read from disk.
func BenchmarkRecallStaleIndex(b *testing.B) {
	store := seedBenchStore(b, 200)
	query := benchRecallQuery()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indexPath := filepath.Join(store.LocalDir, IndexFileName)
		if err := os.Remove(indexPath); err != nil && !os.IsNotExist(err) {
			b.Fatalf("drop index: %v", err)
		}
		if _, err := store.Recall(query); err != nil {
			b.Fatalf("recall: %v", err)
		}
	}
}

// BenchmarkScoreScarItem isolates pure scoring CPU from disk and subprocess
// costs.
func BenchmarkScoreScarItem(b *testing.B) {
	store := seedBenchStore(b, 1)
	scars, err := store.ListScars()
	if err != nil {
		b.Fatalf("list scars: %v", err)
	}
	scar := scars[0].Scar
	query := benchRecallQuery()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ScoreScarItem(scar, query)
	}
}

// BenchmarkInjectGuardrails measures the complete prompt-injection path,
// including budget formatting.
func BenchmarkInjectGuardrails(b *testing.B) {
	store := seedBenchStore(b, 200)
	opts := InjectOptions{
		Files:   []string{"pkg/mod3/code.go"},
		Symbols: []string{"Target3"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, err := store.InjectGuardrails(opts)
		if err != nil {
			b.Fatalf("inject: %v", err)
		}
		if out == "" {
			b.Fatal("expected injection output")
		}
	}
}
