package scar

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestLoadScarReflectsEditsAfterCacheWarm verifies that a scar rewritten on
// disk is reloaded rather than served from the process cache, which is the
// invalidation contract every cached value depends on.
func TestLoadScarReflectsEditsAfterCacheWarm(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}

	scar, path, err := store.CreateDraft("Cache Invalidation", "first lesson", []string{"pkg/a.go"})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if _, err := store.SaveScar(scar); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Warm the cache, then rewrite the file with different content.
	if _, _, err := store.LoadScar(scar.ID); err != nil {
		t.Fatalf("warm load: %v", err)
	}
	edited := *scar
	edited.Lesson = "second lesson after an edit"
	data, err := FormatFrontmatter(&edited)
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	got, _, err := store.LoadScar(scar.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Lesson != edited.Lesson {
		t.Errorf("stale lesson served from cache: got %q, want %q", got.Lesson, edited.Lesson)
	}
}

// TestLoadScarReturnsPrivateCopy proves a caller mutating a loaded scar
// cannot corrupt what the next load or recall sees. Without this guarantee a
// triage that edits a scar in memory would poison retrieval for every later
// request in the same process.
func TestLoadScarReturnsPrivateCopy(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	scar, _, err := store.CreateDraft("Private Copy", "original lesson", []string{"pkg/a.go"})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if err := store.RecordBaseline(scar, scar.Verifier.CommandHash); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	if _, err := store.SaveScar(scar); err != nil {
		t.Fatalf("save: %v", err)
	}

	first, _, err := store.LoadScar(scar.ID)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	first.Lesson = "mutated in memory"
	first.Anchors.Files = append(first.Anchors.Files, "pkg/injected.go")
	first.Anchors.Symbols = append(first.Anchors.Symbols, "Injected")
	first.Baseline.AnchorHashes["pkg/a.go"] = "tampered"
	first.Verifier.Argv = append(first.Verifier.Argv, "--tampered")

	second, _, err := store.LoadScar(scar.ID)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if second.Lesson != "original lesson" {
		t.Errorf("lesson leaked through the cache: got %q", second.Lesson)
	}
	if len(second.Anchors.Files) != 1 || second.Anchors.Files[0] != "pkg/a.go" {
		t.Errorf("anchor files leaked through the cache: %v", second.Anchors.Files)
	}
	if len(second.Anchors.Symbols) != 0 {
		t.Errorf("symbols leaked through the cache: %v", second.Anchors.Symbols)
	}
	if second.Baseline.AnchorHashes["pkg/a.go"] == "tampered" {
		t.Error("baseline hash leaked through the cache")
	}
	if len(second.Verifier.Argv) != 0 {
		t.Errorf("verifier argv leaked through the cache: %v", second.Verifier.Argv)
	}
}

// TestRecallPicksUpNewScarFile verifies that a scar created after the last
// recall is found even though the index cache predates it: the freshness
// check must notice the new file and fall back to the catalog.
func TestRecallPicksUpNewScarFile(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	anchor := filepath.Join(store.RepoRoot, "pkg", "app.go")
	if err := os.MkdirAll(filepath.Dir(anchor), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(anchor, []byte("package app\n"), 0644); err != nil {
		t.Fatalf("anchor: %v", err)
	}

	first, _, err := store.CreateDraft("First Scar", "first lesson", []string{"pkg/app.go"})
	if err != nil {
		t.Fatalf("draft one: %v", err)
	}
	activateForTest(t, store, first)
	if _, err := store.RebuildIndex(); err != nil {
		t.Fatalf("index: %v", err)
	}

	query := QueryOptions{Files: []string{"pkg/app.go"}, Threshold: 3, TopK: 5, IncludeStale: true}
	got, err := store.Recall(query)
	if err != nil || len(got) != 1 {
		t.Fatalf("expected 1 recall before the addition, got %d (err %v)", len(got), err)
	}

	second, _, err := store.CreateDraft("Second Scar", "second lesson", []string{"pkg/app.go"})
	if err != nil {
		t.Fatalf("draft two: %v", err)
	}
	activateForTest(t, store, second)

	// No index rebuild: recall must notice the new file by itself.
	got, err = store.Recall(query)
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected the newly added scar to be recalled, got %d results", len(got))
	}
}

// TestRecallDropsScarWithChangedAnchor proves the cached digest is
// invalidated when an anchored source file is edited: a scar whose anchor
// moved must leave the result set instead of being served as current.
func TestRecallDropsScarWithChangedAnchor(t *testing.T) {
	store := seedBenchStore(t, 4)
	query := benchRecallQuery()
	query.Threshold = 3
	query.TopK = 10 // all four scars match on basename + command; don't truncate

	got, err := store.Recall(query)
	if err != nil || len(got) != 4 {
		t.Fatalf("expected all 4 scars on the warm recall, got %d (err %v)", len(got), err)
	}

	anchor := filepath.Join(store.RepoRoot, "pkg", "mod3", "code.go")
	edited := []byte("package mod3\n\nfunc Target3() int { return 99 }\n\n// edited\n")
	if err := os.WriteFile(anchor, edited, 0644); err != nil {
		t.Fatalf("edit anchor: %v", err)
	}

	got, err = store.Recall(query)
	if err != nil {
		t.Fatalf("recall after edit: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("expected the stale scar to be excluded (4 -> 3), got %d results", len(got))
	}
	for _, s := range got {
		if strings.HasPrefix(s.Scar.ID, "bench-scar-3-") {
			t.Errorf("stale scar %s served as current", s.Scar.ID)
		}
	}

	got, err = store.Recall(QueryOptions{
		Files:        []string{"pkg/mod3/code.go"},
		Threshold:    3,
		TopK:         3,
		IncludeStale: true,
	})
	if err != nil || len(got) != 1 {
		t.Fatalf("expected the stale scar with --include-stale, got %d (err %v)", len(got), err)
	}
	if !strings.HasPrefix(got[0].Scar.ID, "bench-scar-3-") {
		t.Errorf("expected bench-scar-3, got %s", got[0].Scar.ID)
	}
	if got[0].Freshness != FreshnessChanged {
		t.Errorf("expected freshness %q, got %q", FreshnessChanged, got[0].Freshness)
	}
}

// TestRepoDependenciesReflectManifestEdits verifies the memoised dependency
// set is refreshed when a manifest changes between two recalls in one
// process.
func TestRepoDependenciesReflectManifestEdits(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	reqPath := filepath.Join(store.RepoRoot, "requirements.txt")
	if err := os.WriteFile(reqPath, []byte("requests==2.0.0\n"), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	if v := store.repoDependencies()["requests"]; v != "2.0.0" {
		t.Fatalf("expected requests 2.0.0, got %q", v)
	}
	if err := os.WriteFile(reqPath, []byte("requests==2.31.0\nflask==3.0.0\n"), 0644); err != nil {
		t.Fatalf("edit manifest: %v", err)
	}

	deps := store.repoDependencies()
	if deps["requests"] != "2.31.0" {
		t.Errorf("stale dependency served: requests=%q, want 2.31.0", deps["requests"])
	}
	if deps["flask"] != "3.0.0" {
		t.Errorf("missing dependency after manifest edit: flask=%q", deps["flask"])
	}
}

// TestLoadIndexReflectsRewrittenFile proves the decoded index is refreshed
// when index.json changes on disk, so a stale decode can never outrank the
// freshness check.
func TestLoadIndexReflectsRewrittenFile(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, _, err := store.CreateDraft("Index One", "lesson", []string{"pkg/a.go"}); err != nil {
		t.Fatalf("draft: %v", err)
	}
	if _, err := store.RebuildIndex(); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	first, err := store.LoadIndex()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if first.TotalScars != 1 {
		t.Fatalf("expected 1 entry, got %d", first.TotalScars)
	}

	// Rewrite the index file behind the store's back with different content.
	indexPath := filepath.Join(store.LocalDir, IndexFileName)
	rewritten := `{"schema_version":"reposcars.index.v1","updated_at":"2026-10-26T00:00:00Z","total_scars":7,"scars":[]}`
	if err := os.WriteFile(indexPath, []byte(rewritten), 0644); err != nil {
		t.Fatalf("rewrite index: %v", err)
	}

	second, err := store.LoadIndex()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if second.TotalScars != 7 {
		t.Errorf("stale index decode served: total_scars=%d, want 7", second.TotalScars)
	}
}

// TestRecallConcurrentReaders runs recalls from several goroutines against
// one store, which is how the MCP server would use the shared cache. The
// race detector is the assertion.
func TestRecallConcurrentReaders(t *testing.T) {
	store := seedBenchStore(t, 50)
	query := benchRecallQuery()

	var wg sync.WaitGroup
	errs := make([]error, 4)
	results := make([]int, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got, err := store.Recall(query)
			errs[i] = err
			results[i] = len(got)
		}(i)
	}
	wg.Wait()

	for i := range errs {
		if errs[i] != nil {
			t.Fatalf("goroutine %d: recall failed: %v", i, errs[i])
		}
		if results[i] == 0 {
			t.Fatalf("goroutine %d: expected candidates", i)
		}
		if results[i] != results[0] {
			t.Fatalf("goroutine %d saw %d candidates, goroutine 0 saw %d", i, results[i], results[0])
		}
	}
}

// activateForTest mirrors the invariants the store requires of an active
// scar so tests can build recall fixtures without repeating the ceremony.
func activateForTest(t *testing.T, store *Store, scar *Scar) {
	t.Helper()
	scar.Review.Status = ReviewApproved
	scar.Review.Reviewer = "tester"
	scar.Review.Rationale = "fixture"
	scar.Verifier.Argv = []string{"go", "test", "./..."}
	scar.Verifier.Result = VerificationFailToPass
	scar.Verifier.CommandHash = ComputeCommandHash(scar.Verifier.Argv, scar.Verifier.WorkingDir, scar.Verifier.EnvAllowlist)
	scar.Lifecycle.State = LifecycleActive
	if err := scar.ValidateInvariants(); err != nil {
		t.Fatalf("fixture invariants: %v", err)
	}
	if err := store.RecordBaseline(scar, scar.Verifier.CommandHash); err != nil {
		t.Fatalf("fixture baseline: %v", err)
	}
	if _, err := store.SaveScar(scar); err != nil {
		t.Fatalf("fixture save: %v", err)
	}
}
